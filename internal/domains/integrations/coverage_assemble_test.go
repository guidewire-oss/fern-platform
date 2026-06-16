package integrations

import (
	"testing"

	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

func TestAssembleTree(t *testing.T) {
	ver := JiraVersion{ID: "1", Name: "v1.0"}

	epic := func(key string) JiraIssue {
		return JiraIssue{Key: key, Summary: key + " epic", IssueType: "Epic"}
	}
	story := func(key, parentEpicKey string) JiraIssue {
		i := JiraIssue{Key: key, Summary: key + " story", IssueType: "Story"}
		if parentEpicKey != "" {
			i.Parent = &JiraParent{Key: parentEpicKey, IssueType: "Epic"}
		}
		return i
	}
	subtask := func(key, parentStoryKey string) JiraIssue {
		return JiraIssue{
			Key:       key,
			Summary:   key + " sub-task",
			IssueType: "Sub-task",
			Subtask:   true,
			Parent:    &JiraParent{Key: parentStoryKey, IssueType: "Story"},
		}
	}
	covered := func(key string) map[string]tagsdomain.CoverageCount {
		return map[string]tagsdomain.CoverageCount{key: {Total: 3, Passed: 2, Failed: 1}}
	}
	noCoverage := map[string]tagsdomain.CoverageCount{}

	t.Run("empty version returns empty tree", func(t *testing.T) {
		tree := assembleTree(ver, nil, nil, nil, noCoverage)
		if tree == nil {
			t.Fatal("expected non-nil tree")
		}
		if tree.FixVersion.Name != "v1.0" {
			t.Errorf("unexpected version: %q", tree.FixVersion.Name)
		}
		if len(tree.Epics) != 0 {
			t.Errorf("expected 0 epics, got %d", len(tree.Epics))
		}
		if len(tree.Unassigned) != 0 {
			t.Errorf("expected 0 unassigned, got %d", len(tree.Unassigned))
		}
	})

	t.Run("story without epic parent goes to Unassigned", func(t *testing.T) {
		orphanStory := story("PROJ-2", "")
		tree := assembleTree(ver, nil, []JiraIssue{orphanStory}, nil, noCoverage)
		if len(tree.Unassigned) != 1 {
			t.Fatalf("expected 1 unassigned story, got %d", len(tree.Unassigned))
		}
		if tree.Unassigned[0].Issue.Key != "PROJ-2" {
			t.Errorf("expected PROJ-2 in unassigned, got %q", tree.Unassigned[0].Issue.Key)
		}
	})

	t.Run("orphaned sub-task whose parent story is absent goes to Unassigned", func(t *testing.T) {
		orphan := subtask("PROJ-10", "PROJ-99") // PROJ-99 is not in stories
		tree := assembleTree(ver, nil, nil, []JiraIssue{orphan}, noCoverage)
		if len(tree.Unassigned) != 1 {
			t.Fatalf("expected 1 unassigned sub-task, got %d", len(tree.Unassigned))
		}
		if tree.Unassigned[0].Issue.Key != "PROJ-10" {
			t.Errorf("expected PROJ-10 in unassigned, got %q", tree.Unassigned[0].Issue.Key)
		}
	})

	t.Run("sub-task is attached to its parent story", func(t *testing.T) {
		s := story("PROJ-2", "PROJ-1")
		st := subtask("PROJ-3", "PROJ-2")
		epics := map[string]JiraIssue{"PROJ-1": epic("PROJ-1")}
		tree := assembleTree(ver, epics, []JiraIssue{s}, []JiraIssue{st}, noCoverage)
		if len(tree.Epics) != 1 {
			t.Fatalf("expected 1 epic, got %d", len(tree.Epics))
		}
		epicNode := tree.Epics[0]
		if len(epicNode.Stories) != 1 {
			t.Fatalf("expected 1 story in epic, got %d", len(epicNode.Stories))
		}
		storyNode := epicNode.Stories[0]
		if len(storyNode.SubTasks) != 1 {
			t.Fatalf("expected 1 sub-task on story, got %d", len(storyNode.SubTasks))
		}
		if storyNode.SubTasks[0].Issue.Key != "PROJ-3" {
			t.Errorf("expected sub-task PROJ-3, got %q", storyNode.SubTasks[0].Issue.Key)
		}
	})

	t.Run("epic covered/total counts reflect covered stories", func(t *testing.T) {
		s1 := story("PROJ-2", "PROJ-1")
		s2 := story("PROJ-3", "PROJ-1")
		epics := map[string]JiraIssue{"PROJ-1": epic("PROJ-1")}
		// only PROJ-2 has coverage
		cov := covered("PROJ-2")
		tree := assembleTree(ver, epics, []JiraIssue{s1, s2}, nil, cov)
		if len(tree.Epics) != 1 {
			t.Fatalf("expected 1 epic, got %d", len(tree.Epics))
		}
		epicNode := tree.Epics[0]
		if epicNode.CoveredCount != 1 {
			t.Errorf("expected CoveredCount=1, got %d", epicNode.CoveredCount)
		}
		if epicNode.TotalCount != 2 {
			t.Errorf("expected TotalCount=2, got %d", epicNode.TotalCount)
		}
	})

	t.Run("buildStoryNode marks covered when coverage entry has Total > 0", func(t *testing.T) {
		issue := story("PROJ-5", "")
		cov := map[string]tagsdomain.CoverageCount{"PROJ-5": {Total: 5, Passed: 2, Failed: 3}}
		node := buildStoryNode(issue, cov)
		if !node.Covered {
			t.Error("expected Covered=true")
		}
		if node.TestRunCoverage == nil {
			t.Fatal("expected TestRunCoverage to be set")
		}
		if node.TestRunCoverage.Total != 5 || node.TestRunCoverage.Passed != 2 {
			t.Errorf("expected Total=5 Passed=2, got Total=%d Passed=%d", node.TestRunCoverage.Total, node.TestRunCoverage.Passed)
		}
	})

	t.Run("buildStoryNode not covered when coverage entry has Total == 0", func(t *testing.T) {
		issue := story("PROJ-6", "")
		cov := map[string]tagsdomain.CoverageCount{"PROJ-6": {Total: 0}}
		node := buildStoryNode(issue, cov)
		if node.Covered {
			t.Error("expected Covered=false when Total=0")
		}
		if node.TestRunCoverage != nil {
			t.Error("expected TestRunCoverage to be nil when Total=0")
		}
	})
}
