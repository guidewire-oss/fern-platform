package graphql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

// fakeCoverageJiraClient implements integrations.CoverageJiraClient. It records
// the JQL strings it receives so tests can assert on the queries the resolver
// fires.
type fakeCoverageJiraClient struct {
	versions []integrations.JiraVersion
	versErr  error

	// searchResults maps a JQL string to the issues returned for that query.
	// The resolver fires two queries: one for epics in the fixVersion, one for
	// children of those epics. Tests stage both under their respective JQL.
	searchResults map[string][]integrations.JiraIssue
	searchErr     error

	gotJQL []string // ordered record of every JQL the resolver sent
}

func (f *fakeCoverageJiraClient) GetVersions(_ context.Context, _ string, _ string, _ string, _ string, _ integrations.AuthenticationType) ([]integrations.JiraVersion, error) {
	return f.versions, f.versErr
}

func (f *fakeCoverageJiraClient) SearchIssues(_ context.Context, _, _, _ string, _ integrations.AuthenticationType, jql string, _ []string) ([]integrations.JiraIssue, error) {
	f.gotJQL = append(f.gotJQL, jql)
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	if issues, ok := f.searchResults[jql]; ok {
		return issues, nil
	}
	return nil, nil
}

// fakeCoverageTagRepo implements integrations.CoverageTagRepository.
type fakeCoverageTagRepo struct {
	coverage map[string]tagsdomain.CoverageCount
	err      error
}

func (f *fakeCoverageTagRepo) GetJiraTagCoverageByProject(_ context.Context, _ string) (map[string]tagsdomain.CoverageCount, error) {
	return f.coverage, f.err
}

// Test_ReleaseCoverage_HappyPath defines the contract for the releaseCoverage
// resolver: given a fixVersion with two epics and three children, with mixed
// test coverage across them, the resolver returns aggregated counts plus a
// per-epic breakdown.
//
// This is the RED test for Task 2.3a. Currently the resolver stub panics with
// "not implemented", so this test will fail. Task 2.2 implements the resolver
// to make it pass.
func Test_ReleaseCoverage_HappyPath(t *testing.T) {
	const projectID = "proj-1"
	const fixVersionName = "2026.Bolinas"

	// --- Set up the JIRA connection (real service, fake repo) ---
	connRepo := &fakeConnRepo{connections: []*integrations.JiraConnection{
		buildActiveConnection(projectID),
	}}
	connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)

	// --- Stage JIRA responses ---
	// Epic JQL: fetched for "fix the release window" -- two epics in 2026.Bolinas.
	epicJQL := `project = "PROJ" AND issuetype = Epic AND fixVersion = "2026.Bolinas"`
	// Children JQL: fetched once for the epics' children.
	childrenJQL := `parent in ("GWCP-100", "GWCP-200")`

	coverClient := &fakeCoverageJiraClient{
		versions: []integrations.JiraVersion{
			{ID: "v1", Name: "2026.Bolinas", Released: false, ReleaseDate: ""},
		},
		searchResults: map[string][]integrations.JiraIssue{
			epicJQL: {
				{Key: "GWCP-100", Summary: "Epic A", StatusName: "In Progress", IssueType: "Epic"},
				{Key: "GWCP-200", Summary: "Epic B", StatusName: "To Do", IssueType: "Epic"},
			},
			childrenJQL: {
				{Key: "GWCP-110", Summary: "Story A1", StatusName: "Done", IssueType: "Story",
					Parent: &integrations.JiraParent{Key: "GWCP-100", IssueType: "Epic"}},
				{Key: "GWCP-120", Summary: "Story A2", StatusName: "To Do", IssueType: "Story",
					Parent: &integrations.JiraParent{Key: "GWCP-100", IssueType: "Epic"}},
				{Key: "GWCP-210", Summary: "Story B1", StatusName: "Done", IssueType: "Story",
					Parent: &integrations.JiraParent{Key: "GWCP-200", IssueType: "Epic"}},
			},
		},
	}

	// --- Stage tag coverage. Tag values are lowercased per domain.NewTag. ---
	// GWCP-100 (epic itself) -- 3 runs, all passing.
	// GWCP-110 (child of GWCP-100) -- 1 run, passing.
	// GWCP-210 (child of GWCP-200) -- 2 runs, 1 passed, 1 failed.
	// GWCP-120 has NO coverage.
	// GWCP-200 (epic itself) has NO coverage.
	tagRepo := &fakeCoverageTagRepo{coverage: map[string]tagsdomain.CoverageCount{
		"gwcp-100": {Total: 3, Passed: 3, Failed: 0},
		"gwcp-110": {Total: 1, Passed: 1, Failed: 0},
		"gwcp-210": {Total: 2, Passed: 1, Failed: 1},
	}}

	// --- Build the resolver ---
	r := newTestResolverWithJiraMapping(t, nil, connSvc)
	r.jiraCoverageClient = coverClient
	r.tagCoverageRepo = tagRepo

	q := &queryResolver{r}
	got, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, fixVersionName)
	require.NoError(t, err)
	require.NotNil(t, got)

	// FixVersion echo
	require.NotNil(t, got.FixVersion)
	assert.Equal(t, "v1", got.FixVersion.ID)
	assert.Equal(t, "2026.Bolinas", got.FixVersion.Name)
	assert.False(t, got.FixVersion.Released)

	// Roll-up: 2 epics; both have at least some work item covered.
	// GWCP-100 group: epic itself + GWCP-110 covered.
	// GWCP-200 group: child GWCP-210 covered (epic itself isn't, but the group is).
	assert.Equal(t, 2, got.TotalEpics, "totalEpics")
	assert.Equal(t, 2, got.CoveredEpics, "coveredEpics (epic group has any covered work item)")
	// Fully covered = every work item (epic + all children) under it has coverage.
	// GWCP-100 group: GWCP-120 not covered -> not fully covered.
	// GWCP-200 group: epic itself not covered -> not fully covered.
	assert.Equal(t, 0, got.FullyCoveredEpics, "fullyCoveredEpics")

	// Children: 3 total (GWCP-110, GWCP-120, GWCP-210); 2 covered (GWCP-110, GWCP-210).
	assert.Equal(t, 3, got.TotalChildren, "totalChildren")
	assert.Equal(t, 2, got.CoveredChildren, "coveredChildren")

	// Per-epic breakdown
	require.Len(t, got.Epics, 2)

	epicsByKey := map[string]*epicAssert{}
	for _, e := range got.Epics {
		require.NotNil(t, e)
		require.NotNil(t, e.Issue)
		epicsByKey[e.Issue.Key] = &epicAssert{
			covered: e.CoveredCount,
			total:   e.TotalCount,
			passing: e.PassingCount,
		}
	}

	// GWCP-100 group: epic + 2 children = 3 work items. Covered: epic, GWCP-110 = 2.
	// Passing: epic (3/3 passed -> 1), GWCP-110 (1/1 passed -> 1) = 2.
	require.Contains(t, epicsByKey, "GWCP-100")
	assert.Equal(t, 3, epicsByKey["GWCP-100"].total)
	assert.Equal(t, 2, epicsByKey["GWCP-100"].covered)
	assert.Equal(t, 2, epicsByKey["GWCP-100"].passing)

	// GWCP-200 group: epic + 1 child = 2 work items. Covered: GWCP-210 = 1.
	// Passing: GWCP-210 has 1 failed -> not all-passing -> 0.
	require.Contains(t, epicsByKey, "GWCP-200")
	assert.Equal(t, 2, epicsByKey["GWCP-200"].total)
	assert.Equal(t, 1, epicsByKey["GWCP-200"].covered)
	assert.Equal(t, 0, epicsByKey["GWCP-200"].passing)

	// Sanity: the resolver fired exactly the two JQLs (epic + children).
	require.Len(t, coverClient.gotJQL, 2)
}

type epicAssert struct {
	covered, total, passing int
}
