package integrations_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

// --- mock types ---

type mockCoverageJiraClient struct {
	getVersionsFn    func() ([]integrations.JiraVersion, error)
	searchIssuesFn   func(jql string) ([]integrations.JiraIssue, error)
	searchIssuesCalls []string // JQL strings captured per call
}

func (m *mockCoverageJiraClient) GetVersions(_ context.Context, _, _, _, _ string, _ integrations.AuthenticationType) ([]integrations.JiraVersion, error) {
	return m.getVersionsFn()
}

func (m *mockCoverageJiraClient) SearchIssues(_ context.Context, _, _, _ string, _ integrations.AuthenticationType, jql string, _ []string) ([]integrations.JiraIssue, error) {
	m.searchIssuesCalls = append(m.searchIssuesCalls, jql)
	return m.searchIssuesFn(jql)
}

type mockCoverageTagRepo struct {
	data map[string]tagsdomain.CoverageCount
}

func (m *mockCoverageTagRepo) GetJiraTagCoverageByProject(_ context.Context, _ string) (map[string]tagsdomain.CoverageCount, error) {
	return m.data, nil
}

// fixedConnRepo returns a single pre-built connection for any project ID.
type fixedConnRepo struct {
	conn *integrations.JiraConnection
	err  error
}

func (r *fixedConnRepo) Create(_ context.Context, _ *integrations.JiraConnection) error { return nil }
func (r *fixedConnRepo) Update(_ context.Context, _ *integrations.JiraConnection) error { return nil }
func (r *fixedConnRepo) Delete(_ context.Context, _ string) error                       { return nil }
func (r *fixedConnRepo) FindByID(_ context.Context, _ string) (*integrations.JiraConnection, error) {
	return r.conn, r.err
}
func (r *fixedConnRepo) FindByProjectID(_ context.Context, _ string) ([]*integrations.JiraConnection, error) {
	if r.conn == nil {
		return nil, r.err
	}
	return []*integrations.JiraConnection{r.conn}, r.err
}
func (r *fixedConnRepo) FindActiveByProjectID(_ context.Context, _ string) ([]*integrations.JiraConnection, error) {
	if r.conn == nil {
		return nil, r.err
	}
	return []*integrations.JiraConnection{r.conn}, r.err
}

// testEncryptionKey is a fixed 32-byte AES-256 key for use in unit tests.
var testEncryptionKey = []byte("test-key-32-bytes-padding-xxxxxx")

// makeTestConnection builds a JiraConnection with an encrypted credential ready for
// the CoverageService to decrypt using testEncryptionKey.
func makeTestConnection(t *testing.T) *integrations.JiraConnection {
	t.Helper()
	conn, err := integrations.NewJiraConnection(
		"proj-id", "test-conn", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "secret-token",
	)
	if err != nil {
		t.Fatalf("makeTestConnection: %v", err)
	}
	encrypted, err := conn.GetEncryptedCredential(testEncryptionKey)
	if err != nil {
		t.Fatalf("makeTestConnection encrypt: %v", err)
	}
	conn.SetEncryptedCredentialForTest(encrypted)
	return conn
}

// --- helpers ---

func version(id, name string, released bool) integrations.JiraVersion {
	return integrations.JiraVersion{ID: id, Name: name, Released: released}
}

func epic(key string) integrations.JiraIssue {
	return integrations.JiraIssue{Key: key, IssueType: "Epic"}
}

func story(key, parentKey string) integrations.JiraIssue {
	issue := integrations.JiraIssue{Key: key, IssueType: "Story"}
	if parentKey != "" {
		issue.Parent = &integrations.JiraParent{Key: parentKey, IssueType: "Epic"}
	}
	return issue
}

// --- tests ---

func TestCoverageService_Build(t *testing.T) {
	ctx := context.Background()

	// helper: build service with the given JIRA client and tag repo
	newSvc := func(jira *mockCoverageJiraClient, tags *mockCoverageTagRepo) *integrations.CoverageService {
		conn := makeTestConnection(t)
		return integrations.NewCoverageService(
			&fixedConnRepo{conn: conn},
			jira,
			tags,
			testEncryptionKey,
		)
	}

	t.Run("Phase 2 fires when parent epics absent from Phase 1", func(t *testing.T) {
		jira := &mockCoverageJiraClient{
			getVersionsFn: func() ([]integrations.JiraVersion, error) {
				return []integrations.JiraVersion{version("10001", "v1.0", false)}, nil
			},
			searchIssuesFn: func(jql string) ([]integrations.JiraIssue, error) {
				if strings.Contains(jql, "fixVersion") {
					// Phase 1: a story whose parent epic is NOT in the results
					return []integrations.JiraIssue{story("PROJ-10", "PROJ-5")}, nil
				}
				// Phase 2: return the missing epic
				return []integrations.JiraIssue{epic("PROJ-5")}, nil
			},
		}
		tags := &mockCoverageTagRepo{data: map[string]tagsdomain.CoverageCount{}}

		svc := newSvc(jira, tags)
		_, err := svc.Build(ctx, "proj-id", "v1.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(jira.searchIssuesCalls) != 2 {
			t.Errorf("expected 2 SearchIssues calls (Phase 1 + Phase 2), got %d", len(jira.searchIssuesCalls))
		}
		if !strings.Contains(jira.searchIssuesCalls[1], "issueKey IN") {
			t.Errorf("Phase 2 JQL should contain issueKey IN, got: %q", jira.searchIssuesCalls[1])
		}
	})

	t.Run("Phase 2 skipped when all parent epics already in Phase 1", func(t *testing.T) {
		jira := &mockCoverageJiraClient{
			getVersionsFn: func() ([]integrations.JiraVersion, error) {
				return []integrations.JiraVersion{version("10001", "v1.0", false)}, nil
			},
			searchIssuesFn: func(_ string) ([]integrations.JiraIssue, error) {
				// Phase 1 includes the epic already
				return []integrations.JiraIssue{epic("PROJ-1"), story("PROJ-10", "PROJ-1")}, nil
			},
		}
		tags := &mockCoverageTagRepo{data: map[string]tagsdomain.CoverageCount{}}

		svc := newSvc(jira, tags)
		_, err := svc.Build(ctx, "proj-id", "v1.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(jira.searchIssuesCalls) != 1 {
			t.Errorf("expected 1 SearchIssues call (Phase 1 only), got %d", len(jira.searchIssuesCalls))
		}
	})

	t.Run("stories grouped under correct epic", func(t *testing.T) {
		jira := &mockCoverageJiraClient{
			getVersionsFn: func() ([]integrations.JiraVersion, error) {
				return []integrations.JiraVersion{version("10001", "v1.0", false)}, nil
			},
			searchIssuesFn: func(_ string) ([]integrations.JiraIssue, error) {
				return []integrations.JiraIssue{
					epic("PROJ-1"),
					story("PROJ-10", "PROJ-1"),
					story("PROJ-11", "PROJ-1"),
				}, nil
			},
		}
		tags := &mockCoverageTagRepo{data: map[string]tagsdomain.CoverageCount{}}

		svc := newSvc(jira, tags)
		tree, err := svc.Build(ctx, "proj-id", "v1.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tree.Epics) != 1 {
			t.Fatalf("expected 1 epic node, got %d", len(tree.Epics))
		}
		if tree.Epics[0].Issue.Key != "PROJ-1" {
			t.Errorf("expected epic key PROJ-1, got %q", tree.Epics[0].Issue.Key)
		}
		if len(tree.Epics[0].Stories) != 2 {
			t.Errorf("expected 2 stories under PROJ-1, got %d", len(tree.Epics[0].Stories))
		}
		if len(tree.Unassigned) != 0 {
			t.Errorf("expected 0 unassigned stories, got %d", len(tree.Unassigned))
		}
	})

	t.Run("orphan stories appear in Unassigned", func(t *testing.T) {
		jira := &mockCoverageJiraClient{
			getVersionsFn: func() ([]integrations.JiraVersion, error) {
				return []integrations.JiraVersion{version("10001", "v1.0", false)}, nil
			},
			searchIssuesFn: func(_ string) ([]integrations.JiraIssue, error) {
				return []integrations.JiraIssue{
					story("PROJ-10", ""), // no parent
					story("PROJ-11", ""), // no parent
				}, nil
			},
		}
		tags := &mockCoverageTagRepo{data: map[string]tagsdomain.CoverageCount{}}

		svc := newSvc(jira, tags)
		tree, err := svc.Build(ctx, "proj-id", "v1.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tree.Epics) != 0 {
			t.Errorf("expected 0 epics, got %d", len(tree.Epics))
		}
		if len(tree.Unassigned) != 2 {
			t.Errorf("expected 2 unassigned stories, got %d", len(tree.Unassigned))
		}
	})

	t.Run("covered status set when Total > 0 in tag map", func(t *testing.T) {
		jira := &mockCoverageJiraClient{
			getVersionsFn: func() ([]integrations.JiraVersion, error) {
				return []integrations.JiraVersion{version("10001", "v1.0", false)}, nil
			},
			searchIssuesFn: func(_ string) ([]integrations.JiraIssue, error) {
				return []integrations.JiraIssue{
					epic("PROJ-1"),
					story("PROJ-10", "PROJ-1"), // covered
					story("PROJ-11", "PROJ-1"), // not covered
				}, nil
			},
		}
		tags := &mockCoverageTagRepo{data: map[string]tagsdomain.CoverageCount{
			"PROJ-10": {Total: 3, Passed: 2, Failed: 1},
		}}

		svc := newSvc(jira, tags)
		tree, err := svc.Build(ctx, "proj-id", "v1.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		stories := tree.Epics[0].Stories
		var s10, s11 integrations.StoryNode
		for _, s := range stories {
			switch s.Issue.Key {
			case "PROJ-10":
				s10 = s
			case "PROJ-11":
				s11 = s
			}
		}
		if !s10.Covered {
			t.Error("PROJ-10 should be covered (Total=3)")
		}
		if s10.TestRunCoverage == nil || s10.TestRunCoverage.Total != 3 {
			t.Errorf("expected TestRunCoverage.Total=3 for PROJ-10, got %v", s10.TestRunCoverage)
		}
		if s11.Covered {
			t.Error("PROJ-11 should not be covered (not in tag map)")
		}
		if s11.TestRunCoverage != nil {
			t.Error("PROJ-11 should have nil TestRunCoverage")
		}
		if tree.Epics[0].CoveredCount != 1 {
			t.Errorf("expected CoveredCount=1, got %d", tree.Epics[0].CoveredCount)
		}
		if tree.Epics[0].TotalCount != 2 {
			t.Errorf("expected TotalCount=2, got %d", tree.Epics[0].TotalCount)
		}
	})

	t.Run("empty fix version returns empty tree without error", func(t *testing.T) {
		jira := &mockCoverageJiraClient{
			getVersionsFn: func() ([]integrations.JiraVersion, error) {
				return []integrations.JiraVersion{version("10001", "v1.0", false)}, nil
			},
			searchIssuesFn: func(_ string) ([]integrations.JiraIssue, error) {
				return []integrations.JiraIssue{}, nil
			},
		}
		tags := &mockCoverageTagRepo{data: map[string]tagsdomain.CoverageCount{}}

		svc := newSvc(jira, tags)
		tree, err := svc.Build(ctx, "proj-id", "v1.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tree.Epics) != 0 {
			t.Errorf("expected 0 epics, got %d", len(tree.Epics))
		}
		if len(tree.Unassigned) != 0 {
			t.Errorf("expected 0 unassigned, got %d", len(tree.Unassigned))
		}
	})

	t.Run("returns error when project has no active JIRA connection", func(t *testing.T) {
		jira := &mockCoverageJiraClient{}
		tags := &mockCoverageTagRepo{data: map[string]tagsdomain.CoverageCount{}}

		svc := integrations.NewCoverageService(
			&fixedConnRepo{err: errors.New("not found")},
			jira,
			tags,
			testEncryptionKey,
		)
		_, err := svc.Build(ctx, "proj-id", "v1.0")
		if err == nil {
			t.Error("expected error when no connection found, got nil")
		}
	})

	t.Run("FixVersion populated in returned tree", func(t *testing.T) {
		jira := &mockCoverageJiraClient{
			getVersionsFn: func() ([]integrations.JiraVersion, error) {
				return []integrations.JiraVersion{
					version("10001", "v1.0", true),
					version("10002", "v2.0", false),
				}, nil
			},
			searchIssuesFn: func(_ string) ([]integrations.JiraIssue, error) {
				return []integrations.JiraIssue{}, nil
			},
		}
		tags := &mockCoverageTagRepo{data: map[string]tagsdomain.CoverageCount{}}

		svc := newSvc(jira, tags)
		tree, err := svc.Build(ctx, "proj-id", "v1.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tree.FixVersion.ID != "10001" {
			t.Errorf("expected FixVersion.ID=10001, got %q", tree.FixVersion.ID)
		}
		if tree.FixVersion.Name != "v1.0" {
			t.Errorf("expected FixVersion.Name=v1.0, got %q", tree.FixVersion.Name)
		}
		if !tree.FixVersion.Released {
			t.Error("expected FixVersion.Released=true")
		}
	})
}
