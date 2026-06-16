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
// release in a project: how many epics (and their non-epic children) under that
// release have any test_run tagged against them, and how many have all passing
// test_runs. The release is selected by a configurable dimension (built-in
// fixVersion, or a configured custom field) — see ReleaseDimension.
//
// The gqlgen-managed wrapper in schema.resolvers.go (matching the
// JiraFieldMapping_domain pattern) delegates here so this implementation
// survives schema regeneration.
//
// Wire flow:
//
//  1. Verify the caller is authenticated.
//  2. Look up the project's active JIRA connection (with decrypted credential).
//  3. Resolve the dimension by id and build its selector predicate for `release`.
//  4. Enumerate epics in the release (one JQL using the selector).
//  5. Enumerate the epics' non-epic children (one JQL).
//  6. Pull this project's jira:<KEY> coverage counts in one query.
//  7. Build the release echo (full JiraVersion for fixVersion; synthesized otherwise).
//  8. Aggregate per epic and roll up.
func (r *queryResolver) ReleaseCoverage_domain(ctx context.Context, projectID string, dimensionID string, release string) (*model.ReleaseCoverage, error) {
	if _, err := getCurrentUser(ctx); err != nil {
		return nil, errors.New("unauthorized")
	}
	if projectID == "" {
		return nil, errors.New("projectID is required")
	}
	if dimensionID == "" {
		return nil, errors.New("dimensionId is required")
	}
	if release == "" {
		return nil, errors.New("release is required")
	}
	if r.jiraConnectionService == nil || r.jiraCoverageClient == nil || r.tagCoverageRepo == nil {
		return nil, errors.New("release coverage is not configured")
	}

	dim, err := r.resolveReleaseDimension(ctx, projectID, dimensionID)
	if err != nil {
		return nil, err
	}
	selector, err := dim.SelectorJQL(release)
	if err != nil {
		return nil, err
	}

	conn, credential, err := r.jiraConnectionService.GetActiveConnectionWithCredential(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("project %q has no active JIRA connection", projectID)
	}

	epics, err := r.jiraCoverageClient.SearchIssues(
		ctx, conn.JiraURL(), conn.Username(), credential, conn.AuthenticationType(),
		buildEpicJQL(conn.ProjectKey(), selector),
		[]string{"summary", "status", "issuetype"},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch epics in release: %w", err)
	}

	children, err := r.fetchChildren(ctx, conn, credential, epics)
	if err != nil {
		return nil, err
	}

	coverage, err := r.tagCoverageRepo.GetJiraTagCoverageByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load JIRA tag coverage: %w", err)
	}

	releaseRef := r.resolveReleaseRef(ctx, conn, credential, dim, release)
	return aggregateReleaseCoverage(toModelDimension(dim), releaseRef, epics, children, coverage), nil
}

// ProjectReleaseDimensions_domain lists the release dimensions a project can be
// grouped by: always the built-in fixVersion, plus any configured custom
// dimensions (config persistence is a follow-up — see resolveReleaseDimension).
func (r *queryResolver) ProjectReleaseDimensions_domain(ctx context.Context, projectID string) ([]*model.ReleaseDimension, error) {
	if _, err := getCurrentUser(ctx); err != nil {
		return nil, errors.New("unauthorized")
	}
	if projectID == "" {
		return nil, errors.New("projectID is required")
	}
	dims := []*model.ReleaseDimension{toModelDimension(integrations.BuiltinFixVersionDimension())}
	// TODO(#30 release-dimension config): append configured custom dimensions.
	return dims, nil
}

// ProjectReleases_domain enumerates the release values for a dimension. Only the
// fixVersion dimension is statically enumerable (via GetVersions); other kinds
// return an empty list and the UI falls back to manual entry.
func (r *queryResolver) ProjectReleases_domain(ctx context.Context, projectID string, dimensionID string) ([]*model.JiraRelease, error) {
	if _, err := getCurrentUser(ctx); err != nil {
		return nil, errors.New("unauthorized")
	}
	if projectID == "" {
		return nil, errors.New("projectID is required")
	}
	if r.jiraConnectionService == nil || r.jiraCoverageClient == nil {
		return nil, errors.New("release coverage is not configured")
	}

	dim, err := r.resolveReleaseDimension(ctx, projectID, dimensionID)
	if err != nil {
		return nil, err
	}
	if !dim.Enumerable() {
		return []*model.JiraRelease{}, nil
	}

	conn, credential, err := r.jiraConnectionService.GetActiveConnectionWithCredential(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("project %q has no active JIRA connection", projectID)
	}

	versions, err := r.jiraCoverageClient.GetVersions(
		ctx, conn.JiraURL(), conn.ProjectKey(), conn.Username(), credential, conn.AuthenticationType(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JIRA versions: %w", err)
	}
	out := make([]*model.JiraRelease, 0, len(versions))
	for i := range versions {
		out = append(out, jiraVersionToRelease(versions[i]))
	}
	return out, nil
}

// resolveReleaseDimension maps a dimensionId to its dimension config. The
// built-in "fixVersion" needs no config; custom dimensions are configured per
// project (Req 8). Custom-dimension config persistence is a follow-up — until
// it lands, only the built-in fixVersion dimension resolves.
func (r *queryResolver) resolveReleaseDimension(_ context.Context, _ string, dimensionID string) (integrations.ReleaseDimension, error) {
	if dimensionID == integrations.BuiltinFixVersionDimension().ID {
		return integrations.BuiltinFixVersionDimension(), nil
	}
	// TODO(#30 release-dimension config): look up configured custom dimensions
	// for the project (reuses #26 field-mapping infra). The selector logic in
	// ReleaseDimension already supports CUSTOM_FIELD/LABEL; only the persisted
	// config is missing.
	return integrations.ReleaseDimension{}, fmt.Errorf("release dimension %q is not configured for this project", dimensionID)
}

// resolveReleaseRef builds the release echo returned to the client. For the
// fixVersion dimension it looks up the full JiraVersion (id/released/date); for
// other dimensions the value itself is the only identity we have.
func (r *queryResolver) resolveReleaseRef(ctx context.Context, conn *integrations.JiraConnection, credential string, dim integrations.ReleaseDimension, release string) *model.JiraRelease {
	if dim.Kind == integrations.ReleaseDimensionFixVersion {
		if v, err := r.findFixVersion(ctx, conn, credential, release); err == nil {
			return jiraVersionToRelease(*v)
		}
		// Fall through to a synthesized ref if the version can't be resolved
		// (e.g. transient JIRA error) — coverage was still computed by name.
	}
	return &model.JiraRelease{ID: release, Name: release}
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
		// JQL escaper, not %q — epic keys are server-generated but we keep one
		// quoting rule for every JQL fragment (see JQLQuoteString).
		keys = append(keys, integrations.JQLQuoteString(e.Key))
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

// buildEpicJQL composes the epic-selection query from the project key and the
// dimension's already-escaped selector predicate. The project key is
// operator-supplied, so it goes through the JQL escaper too (not %q).
func buildEpicJQL(projectKey, selector string) string {
	return fmt.Sprintf(`project = %s AND issuetype = Epic AND %s`, integrations.JQLQuoteString(projectKey), selector)
}

func toModelDimension(d integrations.ReleaseDimension) *model.ReleaseDimension {
	return &model.ReleaseDimension{
		ID:         d.ID,
		Label:      d.Label,
		Kind:       string(d.Kind),
		Enumerable: d.Enumerable(),
		IsDefault:  d.IsDefault,
	}
}

// aggregateReleaseCoverage rolls the JIRA issue tree + tag coverage into the
// GraphQL response. Pulled out as a pure function so it's testable in isolation
// from the JIRA / tag dependencies. Takes the dimension + release echo as
// prebuilt models.
//
// "Covered" = at least one test_run is tagged against the work item.
// "Fully covered" epic = epic itself AND every child has coverage.
// "Passing" = every test_run tagged against the work item passed.
func aggregateReleaseCoverage(
	dimension *model.ReleaseDimension,
	release *model.JiraRelease,
	epics []integrations.JiraIssue,
	children []integrations.JiraIssue,
	coverage map[string]tagsdomain.CoverageCount,
) *model.ReleaseCoverage {
	childrenByParent := groupChildrenByParent(children)

	result := &model.ReleaseCoverage{
		Dimension:  dimension,
		Release:    release,
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
