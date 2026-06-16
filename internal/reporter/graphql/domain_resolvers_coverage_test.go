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
	releases []string
	tree     *integrations.CoverageTree
	err      error
}

func (f *fakeCoverageService) GetReleasesForProject(_ context.Context, _ string) ([]string, error) {
	return f.releases, f.err
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
			releases: []string{"OLOS (2025.06M)", "PALISADES (2025.10M)"},
		}
		qr := newCoverageResolver(t, svc)

		result, err := qr.JiraFixVersions(adminCtxForMapping(), "proj-1")

		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "OLOS (2025.06M)", result[0].Name)
		assert.Equal(t, "PALISADES (2025.10M)", result[1].Name)
	})

	t.Run("returns empty list when project has no releases", func(t *testing.T) {
		svc := &fakeCoverageService{releases: []string{}}
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
			Release: "OLOS (2025.06M)",
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

		result, err := qr.RequirementCoverage(adminCtxForMapping(), "proj-1", "OLOS (2025.06M)")

		require.NoError(t, err)
		require.NotNil(t, result)

		// release
		assert.Equal(t, "OLOS (2025.06M)", result.FixVersion.Name)

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
		releases: []string{"v1.0"},
		tree: &integrations.CoverageTree{
			Release: "v1.0",
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
