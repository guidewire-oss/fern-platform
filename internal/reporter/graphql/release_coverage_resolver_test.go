package graphql

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

// fakeCoverageJiraClient implements integrations.CoverageJiraClient.
//
// SearchIssues routes by a JQL discriminator rather than exact-string match, so
// the fake does not break when the resolver tweaks query whitespace/quoting,
// and so the epic and children queries can be staged (and made to fail)
// independently. It records every JQL it receives for assertions.
type fakeCoverageJiraClient struct {
	versions []integrations.JiraVersion
	versErr  error

	epicIssues  []integrations.JiraIssue
	epicErr     error
	childIssues []integrations.JiraIssue
	childErr    error

	gotJQL []string // ordered record of every JQL the resolver sent
}

func (f *fakeCoverageJiraClient) GetVersions(_ context.Context, _ string, _ string, _ string, _ string, _ integrations.AuthenticationType) ([]integrations.JiraVersion, error) {
	return f.versions, f.versErr
}

func (f *fakeCoverageJiraClient) SearchIssues(_ context.Context, _, _, _ string, _ integrations.AuthenticationType, jql string, _ []string) ([]integrations.JiraIssue, error) {
	f.gotJQL = append(f.gotJQL, jql)
	switch {
	case strings.Contains(jql, "issuetype = Epic"):
		return f.epicIssues, f.epicErr
	case strings.Contains(jql, "parent in"):
		return f.childIssues, f.childErr
	default:
		return nil, nil
	}
}

// fakeCoverageTagRepo implements integrations.CoverageTagRepository.
type fakeCoverageTagRepo struct {
	coverage map[string]tagsdomain.CoverageCount
	err      error
}

func (f *fakeCoverageTagRepo) GetJiraTagCoverageByProject(_ context.Context, _ string) (map[string]tagsdomain.CoverageCount, error) {
	return f.coverage, f.err
}

type epicAssert struct {
	covered, total, passing int
}

// newReleaseCoverageResolver wires a resolver with a permissive JIRA connection
// (real service, fake repo) plus the supplied coverage client and tag repo.
func newReleaseCoverageResolver(t *testing.T, projectID string, client integrations.CoverageJiraClient, tagRepo integrations.CoverageTagRepository) *queryResolver {
	t.Helper()
	connRepo := &fakeConnRepo{connections: []*integrations.JiraConnection{
		buildActiveConnection(projectID),
	}}
	connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
	r := newTestResolverWithJiraMapping(t, nil, connSvc)
	r.jiraCoverageClient = client
	r.tagCoverageRepo = tagRepo
	return &queryResolver{r}
}

// ---------------------------------------------------------------------------
// Resolver happy path
// ---------------------------------------------------------------------------

// Test_ReleaseCoverage_HappyPath: a fixVersion with two epics and three
// children, mixed coverage, returns aggregate counts plus a per-epic breakdown.
func Test_ReleaseCoverage_HappyPath(t *testing.T) {
	const projectID = "proj-1"
	const fixVersionName = "2026.Bolinas"

	coverClient := &fakeCoverageJiraClient{
		versions: []integrations.JiraVersion{
			{ID: "v1", Name: "2026.Bolinas", Released: false},
		},
		epicIssues: []integrations.JiraIssue{
			{Key: "GWCP-100", Summary: "Epic A", StatusName: "In Progress", IssueType: "Epic"},
			{Key: "GWCP-200", Summary: "Epic B", StatusName: "To Do", IssueType: "Epic"},
		},
		childIssues: []integrations.JiraIssue{
			{Key: "GWCP-110", Summary: "Story A1", StatusName: "Done", IssueType: "Story",
				Parent: &integrations.JiraParent{Key: "GWCP-100", IssueType: "Epic"}},
			{Key: "GWCP-120", Summary: "Story A2", StatusName: "To Do", IssueType: "Story",
				Parent: &integrations.JiraParent{Key: "GWCP-100", IssueType: "Epic"}},
			{Key: "GWCP-210", Summary: "Story B1", StatusName: "Done", IssueType: "Story",
				Parent: &integrations.JiraParent{Key: "GWCP-200", IssueType: "Epic"}},
		},
	}

	// Tag coverage keys are lowercased per domain.NewTag.
	// GWCP-100 (epic) 3/3 pass; GWCP-110 (child) 1/1 pass; GWCP-210 (child) 2 runs, 1 fail.
	// GWCP-120 and GWCP-200 have no coverage.
	tagRepo := &fakeCoverageTagRepo{coverage: map[string]tagsdomain.CoverageCount{
		"gwcp-100": {Total: 3, Passed: 3, Failed: 0},
		"gwcp-110": {Total: 1, Passed: 1, Failed: 0},
		"gwcp-210": {Total: 2, Passed: 1, Failed: 1},
	}}

	q := newReleaseCoverageResolver(t, projectID, coverClient, tagRepo)
	got, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, fixVersionName)
	require.NoError(t, err)
	require.NotNil(t, got)

	require.NotNil(t, got.FixVersion)
	assert.Equal(t, "v1", got.FixVersion.ID)
	assert.Equal(t, "2026.Bolinas", got.FixVersion.Name)
	assert.False(t, got.FixVersion.Released)

	assert.Equal(t, 2, got.TotalEpics, "totalEpics")
	assert.Equal(t, 2, got.CoveredEpics, "coveredEpics (epic group has any covered work item)")
	assert.Equal(t, 0, got.FullyCoveredEpics, "fullyCoveredEpics")
	assert.Equal(t, 3, got.TotalChildren, "totalChildren")
	assert.Equal(t, 2, got.CoveredChildren, "coveredChildren")

	require.Len(t, got.Epics, 2)
	epicsByKey := map[string]*epicAssert{}
	for _, e := range got.Epics {
		require.NotNil(t, e)
		require.NotNil(t, e.Issue)
		epicsByKey[e.Issue.Key] = &epicAssert{e.CoveredCount, e.TotalCount, e.PassingCount}
	}

	// GWCP-100: epic + 2 children = 3 items; covered epic+GWCP-110 = 2; passing both = 2.
	require.Contains(t, epicsByKey, "GWCP-100")
	assert.Equal(t, epicAssert{covered: 2, total: 3, passing: 2}, *epicsByKey["GWCP-100"])
	// GWCP-200: epic + 1 child = 2 items; covered GWCP-210 = 1; GWCP-210 has a fail -> passing 0.
	require.Contains(t, epicsByKey, "GWCP-200")
	assert.Equal(t, epicAssert{covered: 1, total: 2, passing: 0}, *epicsByKey["GWCP-200"])

	// Exactly two JQLs fired: epics, then children.
	require.Len(t, coverClient.gotJQL, 2)
}

// ---------------------------------------------------------------------------
// Resolver guard / validation branches
// ---------------------------------------------------------------------------

func Test_ReleaseCoverage_GuardBranches(t *testing.T) {
	const projectID = "proj-1"
	okClient := &fakeCoverageJiraClient{versions: []integrations.JiraVersion{{ID: "v1", Name: "R"}}}
	okRepo := &fakeCoverageTagRepo{}

	t.Run("unauthenticated caller", func(t *testing.T) {
		q := newReleaseCoverageResolver(t, projectID, okClient, okRepo)
		_, err := q.ReleaseCoverage(context.Background(), projectID, "R")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
	})

	t.Run("empty projectID", func(t *testing.T) {
		q := newReleaseCoverageResolver(t, projectID, okClient, okRepo)
		_, err := q.ReleaseCoverage(managerCtxForMapping(), "", "R")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "projectID is required")
	})

	t.Run("empty fixVersionName", func(t *testing.T) {
		q := newReleaseCoverageResolver(t, projectID, okClient, okRepo)
		_, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fixVersionName is required")
	})

	t.Run("nil coverage dependencies", func(t *testing.T) {
		// Wire a resolver but leave the coverage deps nil.
		q := newReleaseCoverageResolver(t, projectID, okClient, okRepo)
		q.jiraCoverageClient = nil
		q.tagCoverageRepo = nil
		_, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, "R")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})
}

// ---------------------------------------------------------------------------
// Resolver error propagation
// ---------------------------------------------------------------------------

func Test_ReleaseCoverage_ErrorPaths(t *testing.T) {
	const projectID = "proj-1"
	const fixVersionName = "2026.Bolinas"
	sentinel := func(msg string) error { return &stringError{msg} }

	t.Run("no active JIRA connection", func(t *testing.T) {
		// Empty connection repo -> GetActiveConnectionWithCredential returns nil.
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		r := newTestResolverWithJiraMapping(t, nil, connSvc)
		r.jiraCoverageClient = &fakeCoverageJiraClient{}
		r.tagCoverageRepo = &fakeCoverageTagRepo{}
		q := &queryResolver{r}

		_, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, fixVersionName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no active JIRA connection")
	})

	t.Run("GetVersions error", func(t *testing.T) {
		client := &fakeCoverageJiraClient{versErr: sentinel("jira down")}
		q := newReleaseCoverageResolver(t, projectID, client, &fakeCoverageTagRepo{})
		_, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, fixVersionName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "jira down")
	})

	t.Run("fixVersion not found", func(t *testing.T) {
		client := &fakeCoverageJiraClient{versions: []integrations.JiraVersion{{ID: "v9", Name: "Other"}}}
		q := newReleaseCoverageResolver(t, projectID, client, &fakeCoverageTagRepo{})
		_, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, fixVersionName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("epic search error", func(t *testing.T) {
		client := &fakeCoverageJiraClient{
			versions: []integrations.JiraVersion{{ID: "v1", Name: fixVersionName}},
			epicErr:  sentinel("epic boom"),
		}
		q := newReleaseCoverageResolver(t, projectID, client, &fakeCoverageTagRepo{})
		_, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, fixVersionName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch epics")
	})

	t.Run("children search error (epics succeed)", func(t *testing.T) {
		// Per-call injection: epics return fine, children query fails. Only
		// reachable because the fake routes the two queries separately.
		client := &fakeCoverageJiraClient{
			versions:   []integrations.JiraVersion{{ID: "v1", Name: fixVersionName}},
			epicIssues: []integrations.JiraIssue{{Key: "GWCP-100", IssueType: "Epic"}},
			childErr:   sentinel("child boom"),
		}
		q := newReleaseCoverageResolver(t, projectID, client, &fakeCoverageTagRepo{})
		_, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, fixVersionName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch epic children")
	})

	t.Run("tag coverage error", func(t *testing.T) {
		client := &fakeCoverageJiraClient{
			versions:   []integrations.JiraVersion{{ID: "v1", Name: fixVersionName}},
			epicIssues: []integrations.JiraIssue{{Key: "GWCP-100", IssueType: "Epic"}},
		}
		tagRepo := &fakeCoverageTagRepo{err: sentinel("db boom")}
		q := newReleaseCoverageResolver(t, projectID, client, tagRepo)
		_, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, fixVersionName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load JIRA tag coverage")
	})
}

// Test_ReleaseCoverage_EmptyRelease: a fixVersion with zero epics fires only the
// epic query (no children query) and returns a well-formed zero result.
func Test_ReleaseCoverage_EmptyRelease(t *testing.T) {
	const projectID = "proj-1"
	const fixVersionName = "2026.Empty"

	client := &fakeCoverageJiraClient{
		versions:   []integrations.JiraVersion{{ID: "v1", Name: fixVersionName}},
		epicIssues: nil, // no epics in this release
	}
	q := newReleaseCoverageResolver(t, projectID, client, &fakeCoverageTagRepo{})

	got, err := q.ReleaseCoverage(managerCtxForMapping(), projectID, fixVersionName)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 0, got.TotalEpics)
	assert.Equal(t, 0, got.CoveredEpics)
	assert.Equal(t, 0, got.TotalChildren)
	assert.Empty(t, got.Epics)
	// Only the epic query should have fired -- no `parent in ()` query.
	require.Len(t, client.gotJQL, 1)
	assert.Contains(t, client.gotJQL[0], "issuetype = Epic")
}

// ---------------------------------------------------------------------------
// Pure aggregation logic (no JIRA / DB dependencies)
// ---------------------------------------------------------------------------

func Test_aggregateReleaseCoverage(t *testing.T) {
	version := integrations.JiraVersion{ID: "v1", Name: "R", Released: true, ReleaseDate: "2026-06-30"}
	epic := func(key string) integrations.JiraIssue {
		return integrations.JiraIssue{Key: key, IssueType: "Epic"}
	}
	child := func(key, parent string) integrations.JiraIssue {
		return integrations.JiraIssue{Key: key, IssueType: "Story", Parent: &integrations.JiraParent{Key: parent}}
	}
	cov := func(total, passed int) tagsdomain.CoverageCount {
		return tagsdomain.CoverageCount{Total: total, Passed: passed, Failed: total - passed}
	}

	t.Run("no epics yields zero rollup but echoes fixVersion", func(t *testing.T) {
		got := aggregateReleaseCoverage(version, nil, nil, nil)
		require.NotNil(t, got)
		require.NotNil(t, got.FixVersion)
		assert.Equal(t, "v1", got.FixVersion.ID)
		assert.True(t, got.FixVersion.Released)
		require.NotNil(t, got.FixVersion.ReleaseDate)
		assert.Equal(t, "2026-06-30", *got.FixVersion.ReleaseDate)
		assert.Equal(t, 0, got.TotalEpics)
		assert.Empty(t, got.Epics)
	})

	t.Run("epic with no children, epic itself covered -> fully covered", func(t *testing.T) {
		got := aggregateReleaseCoverage(version,
			[]integrations.JiraIssue{epic("E-1")}, nil,
			map[string]tagsdomain.CoverageCount{"e-1": cov(2, 2)})
		assert.Equal(t, 1, got.TotalEpics)
		assert.Equal(t, 1, got.CoveredEpics)
		assert.Equal(t, 1, got.FullyCoveredEpics, "epic-only group, epic covered -> fully covered")
		assert.Equal(t, 0, got.TotalChildren)
		require.Len(t, got.Epics, 1)
		assert.Equal(t, 1, got.Epics[0].TotalCount)
		assert.Equal(t, 1, got.Epics[0].PassingCount)
	})

	t.Run("fully covered requires epic AND every child covered", func(t *testing.T) {
		got := aggregateReleaseCoverage(version,
			[]integrations.JiraIssue{epic("E-1")},
			[]integrations.JiraIssue{child("C-1", "E-1"), child("C-2", "E-1")},
			map[string]tagsdomain.CoverageCount{
				"e-1": cov(1, 1), "c-1": cov(1, 1), "c-2": cov(1, 0),
			})
		require.Len(t, got.Epics, 1)
		assert.Equal(t, 3, got.Epics[0].TotalCount, "epic + 2 children")
		assert.Equal(t, 3, got.Epics[0].CoveredCount, "all three covered")
		assert.Equal(t, 2, got.Epics[0].PassingCount, "c-2 has a failure")
		assert.Equal(t, 1, got.FullyCoveredEpics, "every work item covered")
		assert.Equal(t, 2, got.TotalChildren)
		assert.Equal(t, 2, got.CoveredChildren)
	})

	t.Run("coverage entry with Total==0 counts as not covered", func(t *testing.T) {
		got := aggregateReleaseCoverage(version,
			[]integrations.JiraIssue{epic("E-1")}, nil,
			map[string]tagsdomain.CoverageCount{"e-1": cov(0, 0)})
		assert.Equal(t, 0, got.CoveredEpics, "Total==0 must not count as covered")
		assert.Equal(t, 0, got.FullyCoveredEpics)
	})

	t.Run("mixed-case JIRA key matches lowercased coverage key", func(t *testing.T) {
		// JIRA returns canonical uppercase; coverage map is keyed lowercase.
		got := aggregateReleaseCoverage(version,
			[]integrations.JiraIssue{epic("GWCP-500")}, nil,
			map[string]tagsdomain.CoverageCount{"gwcp-500": cov(1, 1)})
		require.Len(t, got.Epics, 1)
		assert.Equal(t, 1, got.Epics[0].CoveredCount, "case-fold must match")
	})

	t.Run("orphan and nil-parent children are excluded from totals", func(t *testing.T) {
		got := aggregateReleaseCoverage(version,
			[]integrations.JiraIssue{epic("E-1")},
			[]integrations.JiraIssue{
				child("C-1", "E-1"),                           // real child
				child("C-9", "E-MISSING"),                     // parent not in epic set
				{Key: "C-0", IssueType: "Story", Parent: nil}, // nil parent
			},
			map[string]tagsdomain.CoverageCount{"c-1": cov(1, 1), "c-9": cov(1, 1)})
		// Only C-1 attaches to E-1; orphan/nil-parent are dropped, not counted.
		assert.Equal(t, 1, got.TotalChildren, "only the real child counts")
		assert.Equal(t, 1, got.CoveredChildren)
		require.Len(t, got.Epics, 1)
		assert.Equal(t, 2, got.Epics[0].TotalCount, "epic + 1 attached child")
	})
}

// Test_buildEpicJQL pins the exact JQL the resolver emits for epics so a
// formatting change is a deliberate, visible edit rather than a silent shift.
func Test_buildEpicJQL(t *testing.T) {
	got := buildEpicJQL("PROJ", "2026.Bolinas")
	assert.Equal(t, `project = "PROJ" AND issuetype = Epic AND fixVersion = "2026.Bolinas"`, got)
}

// stringError is a minimal error type for sentinel test errors without pulling
// in errors.New at every call site.
type stringError struct{ msg string }

func (e *stringError) Error() string { return e.msg }
