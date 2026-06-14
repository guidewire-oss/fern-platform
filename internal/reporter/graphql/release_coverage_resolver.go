package graphql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/model"
)

// ReleaseCoverage_domain returns the JIRA-issue-level coverage roll-up for one
// fixVersion in a project: how many epics (and their non-epic children) under
// that release have any test_run tagged against them, and how many have all
// passing test_runs.
//
// The gqlgen-managed wrapper in schema.resolvers.go (matching the
// JiraFieldMapping_domain pattern) delegates here so this implementation
// survives schema regeneration.
//
// Wire flow:
//
//  1. Verify the caller is authenticated.
//  2. Look up the project's active JIRA connection (with decrypted credential).
//  3. Resolve fixVersionName -> JiraVersion via GetVersions.
//  4. Enumerate epics in the fixVersion (one JQL).
//  5. Enumerate the epics' non-epic children (one JQL).
//  6. Pull this project's jira:<KEY> coverage counts in one query.
//  7. Aggregate per epic and roll up.
func (r *queryResolver) ReleaseCoverage_domain(ctx context.Context, projectID string, fixVersionName string) (*model.ReleaseCoverage, error) {
	if _, err := getCurrentUser(ctx); err != nil {
		return nil, errors.New("unauthorized")
	}
	if projectID == "" {
		return nil, errors.New("projectID is required")
	}
	if fixVersionName == "" {
		return nil, errors.New("fixVersionName is required")
	}
	if r.jiraConnectionService == nil || r.jiraCoverageClient == nil || r.tagCoverageRepo == nil {
		return nil, errors.New("release coverage is not configured")
	}

	conn, credential, err := r.jiraConnectionService.GetActiveConnectionWithCredential(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("project %q has no active JIRA connection", projectID)
	}

	target, err := r.findFixVersion(ctx, conn, credential, fixVersionName)
	if err != nil {
		return nil, err
	}

	epics, err := r.jiraCoverageClient.SearchIssues(
		ctx, conn.JiraURL(), conn.Username(), credential, conn.AuthenticationType(),
		buildEpicJQL(conn.ProjectKey(), fixVersionName),
		[]string{"summary", "status", "issuetype"},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch epics in fixVersion: %w", err)
	}

	children, err := r.fetchChildren(ctx, conn, credential, epics)
	if err != nil {
		return nil, err
	}

	coverage, err := r.tagCoverageRepo.GetJiraTagCoverageByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load JIRA tag coverage: %w", err)
	}

	return aggregateReleaseCoverage(*target, epics, children, coverage), nil
}

// findFixVersion resolves a fixVersion by name. The JIRA API returns all
// versions for the project; we filter by Name on our side.
func (r *queryResolver) findFixVersion(ctx context.Context, conn *integrations.JiraConnection, credential, name string) (*integrations.JiraVersion, error) {
	versions, err := r.jiraCoverageClient.GetVersions(
		ctx, conn.JiraURL(), conn.ProjectKey(), conn.Username(), credential, conn.AuthenticationType(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JIRA versions: %w", err)
	}
	for i := range versions {
		if versions[i].Name == name {
			return &versions[i], nil
		}
	}
	return nil, fmt.Errorf("fixVersion %q not found in project %q", name, conn.ProjectKey())
}

// fetchChildren returns all non-epic descendants of the given epics in one JQL.
// Empty input -> empty output (no JQL fired).
func (r *queryResolver) fetchChildren(ctx context.Context, conn *integrations.JiraConnection, credential string, epics []integrations.JiraIssue) ([]integrations.JiraIssue, error) {
	if len(epics) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(epics))
	for _, e := range epics {
		keys = append(keys, fmt.Sprintf("%q", e.Key))
	}
	jql := fmt.Sprintf("parent in (%s)", strings.Join(keys, ", "))
	issues, err := r.jiraCoverageClient.SearchIssues(
		ctx, conn.JiraURL(), conn.Username(), credential, conn.AuthenticationType(),
		jql, []string{"summary", "status", "issuetype", "parent"},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch epic children: %w", err)
	}
	return issues, nil
}

func buildEpicJQL(projectKey, fixVersionName string) string {
	return fmt.Sprintf(`project = %q AND issuetype = Epic AND fixVersion = %q`, projectKey, fixVersionName)
}

// aggregateReleaseCoverage rolls the JIRA issue tree + tag coverage into the
// GraphQL response. Pulled out as a pure function so it's testable in
// isolation from the JIRA / tag dependencies. Takes target by value so a nil
// version can't panic here.
//
// "Covered" = at least one test_run is tagged against the work item.
// "Fully covered" epic = epic itself AND every child has coverage.
// "Passing" = every test_run tagged against the work item passed.
func aggregateReleaseCoverage(
	target integrations.JiraVersion,
	epics []integrations.JiraIssue,
	children []integrations.JiraIssue,
	coverage map[string]tagsdomain.CoverageCount,
) *model.ReleaseCoverage {
	childrenByParent := groupChildrenByParent(children)

	result := &model.ReleaseCoverage{
		FixVersion: jiraVersionToRelease(target),
		TotalEpics: len(epics),
		Epics:      make([]*model.EpicCoverageSummary, 0, len(epics)),
	}

	for _, epic := range epics {
		childList := childrenByParent[epic.Key]
		group := summarizeEpic(epic, childList, coverage)

		result.Epics = append(result.Epics, group.summary)
		result.TotalChildren += len(childList)
		result.CoveredChildren += group.coveredChildren
		if group.summary.CoveredCount > 0 {
			result.CoveredEpics++
		}
		if group.summary.CoveredCount == group.summary.TotalCount {
			result.FullyCoveredEpics++
		}
	}
	return result
}

type epicGroup struct {
	summary         *model.EpicCoverageSummary
	coveredChildren int
}

// summarizeEpic counts work items (the epic itself + its children) that have
// coverage and that are all-passing. A single addItem closure handles both the
// epic and each child; the isChild flag drives the children-only tally.
func summarizeEpic(epic integrations.JiraIssue, children []integrations.JiraIssue, coverage map[string]tagsdomain.CoverageCount) epicGroup {
	total := 1 + len(children)
	covered, passing, coveredChildren := 0, 0, 0

	addItem := func(key string, isChild bool) {
		c, ok := coverage[strings.ToLower(key)]
		if !ok || c.Total == 0 {
			return
		}
		covered++
		if isChild {
			coveredChildren++
		}
		if c.Passed == c.Total {
			passing++
		}
	}

	addItem(epic.Key, false)
	for _, child := range children {
		addItem(child.Key, true)
	}

	return epicGroup{
		summary: &model.EpicCoverageSummary{
			Issue:        jiraIssueToSummary(epic),
			CoveredCount: covered,
			TotalCount:   total,
			PassingCount: passing,
		},
		coveredChildren: coveredChildren,
	}
}

func groupChildrenByParent(children []integrations.JiraIssue) map[string][]integrations.JiraIssue {
	out := make(map[string][]integrations.JiraIssue)
	for _, c := range children {
		if c.Parent == nil {
			continue
		}
		out[c.Parent.Key] = append(out[c.Parent.Key], c)
	}
	return out
}

func jiraVersionToRelease(v integrations.JiraVersion) *model.JiraRelease {
	r := &model.JiraRelease{ID: v.ID, Name: v.Name, Released: v.Released}
	if v.ReleaseDate != "" {
		d := v.ReleaseDate
		r.ReleaseDate = &d
	}
	return r
}

func jiraIssueToSummary(i integrations.JiraIssue) *model.JiraIssueSummary {
	return &model.JiraIssueSummary{
		Key:        i.Key,
		Summary:    i.Summary,
		StatusName: i.StatusName,
		IssueType:  i.IssueType,
	}
}
