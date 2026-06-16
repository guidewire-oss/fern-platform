package graphql

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

// --- fake coverage service ---

type fakeCoverageService struct {
	versions []integrations.JiraVersion
	tree     *integrations.CoverageTree
	err      error
}

func (f *fakeCoverageService) GetVersionsForProject(_ context.Context, _ string) ([]integrations.JiraVersion, error) {
	return f.versions, f.err
}

func (f *fakeCoverageService) Build(_ context.Context, _, _ string) (*integrations.CoverageTree, error) {
	return f.tree, f.err
}

// --- helper ---

func newCoverageResolver(t *testing.T, svc coverageServicer) *queryResolver {
	t.Helper()
	r := newTestResolverWithJiraMapping(t, nil, nil)
	r.coverageService = svc
	return &queryResolver{r}
}

// --- JiraFixVersions ---

func TestJiraFixVersionsResolver(t *testing.T) {
	t.Run("returns mapped JiraRelease list on success", func(t *testing.T) {
		svc := &fakeCoverageService{
			versions: []integrations.JiraVersion{
				{ID: "10001", Name: "v1.0", Released: true, ReleaseDate: "2025-01-15"},
				{ID: "10002", Name: "v2.0", Released: false},
			},
		}
		qr := newCoverageResolver(t, svc)

		result, err := qr.JiraFixVersions(adminCtxForMapping(), "proj-1")

		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "10001", result[0].ID)
		assert.Equal(t, "v1.0", result[0].Name)
		assert.True(t, result[0].Released)
		assert.NotNil(t, result[0].ReleaseDate)
		assert.Equal(t, "2025-01-15", *result[0].ReleaseDate)
		assert.Equal(t, "10002", result[1].ID)
		assert.False(t, result[1].Released)
		assert.Nil(t, result[1].ReleaseDate)
	})

	t.Run("returns empty list when project has no versions", func(t *testing.T) {
		svc := &fakeCoverageService{versions: []integrations.JiraVersion{}}
		qr := newCoverageResolver(t, svc)

		result, err := qr.JiraFixVersions(adminCtxForMapping(), "proj-1")

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns GraphQL error when service fails", func(t *testing.T) {
		svc := &fakeCoverageService{err: errors.New("no active JIRA connection")}
		qr := newCoverageResolver(t, svc)

		result, err := qr.JiraFixVersions(adminCtxForMapping(), "proj-1")

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns GraphQL error when coverage service not configured", func(t *testing.T) {
		qr := newCoverageResolver(t, nil)

		result, err := qr.JiraFixVersions(adminCtxForMapping(), "proj-1")

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// --- RequirementCoverage ---

func TestRequirementCoverageResolver(t *testing.T) {
	covered := tagsdomain.CoverageCount{Total: 3, Passed: 2, Failed: 1}

	t.Run("maps CoverageTree to RequirementCoverageTree model", func(t *testing.T) {
		tree := &integrations.CoverageTree{
			FixVersion: integrations.JiraVersion{ID: "10001", Name: "v1.0", Released: false},
			Epics: []integrations.EpicNode{
				{
					Issue:        integrations.JiraIssue{Key: "PROJ-1", Summary: "Epic one", IssueType: "Epic"},
					CoveredCount: 1,
					TotalCount:   2,
					Stories: []integrations.StoryNode{
						{
							Issue:           integrations.JiraIssue{Key: "PROJ-10", Summary: "Story A", StatusName: "Done", IssueType: "Story"},
							Covered:         true,
							TestRunCoverage: &covered,
						},
						{
							Issue:   integrations.JiraIssue{Key: "PROJ-11", Summary: "Story B", StatusName: "To Do", IssueType: "Story"},
							Covered: false,
						},
					},
				},
			},
			Unassigned: []integrations.StoryNode{
				{Issue: integrations.JiraIssue{Key: "PROJ-99", Summary: "Orphan", IssueType: "Story"}},
			},
		}
		svc := &fakeCoverageService{tree: tree}
		qr := newCoverageResolver(t, svc)

		result, err := qr.RequirementCoverage(adminCtxForMapping(), "proj-1", "v1.0")

		require.NoError(t, err)
		require.NotNil(t, result)

		// fix version
		assert.Equal(t, "10001", result.FixVersion.ID)
		assert.Equal(t, "v1.0", result.FixVersion.Name)
		assert.False(t, result.FixVersion.Released)

		// epics
		require.Len(t, result.Epics, 1)
		epic := result.Epics[0]
		assert.Equal(t, "PROJ-1", epic.Issue.Key)
		assert.Equal(t, 1, epic.CoveredCount)
		assert.Equal(t, 2, epic.TotalCount)

		// stories under epic
		require.Len(t, epic.Stories, 2)
		s10 := epic.Stories[0]
		assert.Equal(t, "PROJ-10", s10.Issue.Key)
		assert.True(t, s10.Covered)
		require.NotNil(t, s10.TestRunCoverage)
		assert.Equal(t, 3, s10.TestRunCoverage.Total)
		assert.Equal(t, 2, s10.TestRunCoverage.Passed)
		assert.Equal(t, 1, s10.TestRunCoverage.Failed)

		s11 := epic.Stories[1]
		assert.Equal(t, "PROJ-11", s11.Issue.Key)
		assert.False(t, s11.Covered)
		assert.Nil(t, s11.TestRunCoverage)

		// unassigned
		require.Len(t, result.Unassigned, 1)
		assert.Equal(t, "PROJ-99", result.Unassigned[0].Issue.Key)
	})

	t.Run("returns GraphQL error when service fails", func(t *testing.T) {
		svc := &fakeCoverageService{err: errors.New("JIRA unavailable")}
		qr := newCoverageResolver(t, svc)

		result, err := qr.RequirementCoverage(adminCtxForMapping(), "proj-1", "v1.0")

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns GraphQL error when coverage service not configured", func(t *testing.T) {
		qr := newCoverageResolver(t, nil)

		result, err := qr.RequirementCoverage(adminCtxForMapping(), "proj-1", "v1.0")

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// --- Authorization tests ---

func TestCoverageResolversAuthorization(t *testing.T) {
	svc := &fakeCoverageService{
		versions: []integrations.JiraVersion{{ID: "1", Name: "v1.0"}},
		tree: &integrations.CoverageTree{
			FixVersion: integrations.JiraVersion{ID: "1", Name: "v1.0"},
		},
	}

	t.Run("JiraFixVersions rejects unauthenticated request", func(t *testing.T) {
		qr := newCoverageResolver(t, svc)
		_, err := qr.JiraFixVersions(context.Background(), "proj-1")
		assert.Error(t, err)
	})

	t.Run("JiraFixVersions rejects user with no project permissions", func(t *testing.T) {
		qr := newCoverageResolver(t, svc)
		_, err := qr.JiraFixVersions(regularUserCtxForMapping(), "proj-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "forbidden")
	})

	t.Run("JiraFixVersions allows admin user", func(t *testing.T) {
		qr := newCoverageResolver(t, svc)
		result, err := qr.JiraFixVersions(adminCtxForMapping(), "proj-1")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("RequirementCoverage rejects user with no project permissions", func(t *testing.T) {
		qr := newCoverageResolver(t, svc)
		_, err := qr.RequirementCoverage(regularUserCtxForMapping(), "proj-1", "v1.0")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "forbidden")
	})

	t.Run("RequirementCoverage allows manager user", func(t *testing.T) {
		qr := newCoverageResolver(t, svc)
		result, err := qr.RequirementCoverage(managerCtxForMapping(), "proj-1", "v1.0")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}
