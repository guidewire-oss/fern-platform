package integrations

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

// jiraKeyPattern matches standard JIRA issue keys (e.g. PROJ-123, CLOUD-4567).
// Used to validate Phase 2 keys before embedding them in JQL.
var jiraKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

const jiraAPITimeout = 30 * time.Second

// CoverageService builds a RequirementCoverageTree for a project + fix version.
type CoverageService struct {
	connRepo      JiraConnectionRepository
	jiraClient    CoverageJiraClient
	tagRepo       CoverageTagRepository
	encryptionKey []byte
}

// NewCoverageService wires up a CoverageService with its dependencies.
func NewCoverageService(
	connRepo JiraConnectionRepository,
	jiraClient CoverageJiraClient,
	tagRepo CoverageTagRepository,
	encryptionKey []byte,
) *CoverageService {
	return &CoverageService{
		connRepo:      connRepo,
		jiraClient:    jiraClient,
		tagRepo:       tagRepo,
		encryptionKey: encryptionKey,
	}
}

// GetVersionsForProject returns all JIRA fix versions for the project's active connection.
func (s *CoverageService) GetVersionsForProject(ctx context.Context, projectID string) ([]JiraVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, jiraAPITimeout)
	defer cancel()

	conns, err := s.connRepo.FindActiveByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("coverage: failed to find JIRA connection: %w", err)
	}
	if len(conns) == 0 {
		return nil, fmt.Errorf("coverage: no active JIRA connection for project %q", projectID)
	}
	conn := conns[0]

	credential, err := DecryptCredential(conn.GetEncryptedCredentialDirect(), s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("coverage: failed to decrypt credential: %w", err)
	}

	return s.jiraClient.GetVersions(ctx, conn.JiraURL(), conn.ProjectKey(), conn.Username(), credential, conn.AuthenticationType())
}

// Build fetches JIRA issues for the given fix version, cross-references them with
// Fern test-run coverage, and returns the assembled tree.
func (s *CoverageService) Build(ctx context.Context, projectID, fixVersionName string) (*CoverageTree, error) {
	ctx, cancel := context.WithTimeout(ctx, jiraAPITimeout)
	defer cancel()

	// Resolve the active JIRA connection for this project.
	conns, err := s.connRepo.FindActiveByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("coverage: failed to find JIRA connection: %w", err)
	}
	if len(conns) == 0 {
		return nil, fmt.Errorf("coverage: no active JIRA connection for project %q", projectID)
	}
	conn := conns[0]

	credential, err := DecryptCredential(conn.GetEncryptedCredentialDirect(), s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("coverage: failed to decrypt credential: %w", err)
	}

	baseURL := conn.JiraURL()
	username := conn.Username()
	authType := conn.AuthenticationType()
	projectKey := conn.ProjectKey()

	// Validate fixVersionName before embedding in JQL.
	if strings.ContainsRune(fixVersionName, ';') {
		return nil, fmt.Errorf("coverage: fix version name contains invalid character ';'")
	}

	// Resolve the JiraVersion object for the requested name.
	fixVersion, err := s.resolveVersion(ctx, baseURL, projectKey, username, credential, authType, fixVersionName)
	if err != nil {
		return nil, err
	}

	// Phase 1: fetch all issues for the fix version.
	// Use fixVersion.Name (validated against JIRA's own version list) rather than
	// the raw user-supplied fixVersionName.
	phase1JQL := fmt.Sprintf(`fixVersion = %q ORDER BY issuetype`, fixVersion.Name)
	fields := []string{"summary", "status", "issuetype", "parent"}
	phase1Issues, err := s.jiraClient.SearchIssues(ctx, baseURL, username, credential, authType, phase1JQL, fields)
	if err != nil {
		return nil, fmt.Errorf("coverage: Phase 1 search failed: %w", err)
	}

	// Separate epics, stories, and sub-tasks from the Phase 1 results.
	// Sub-task detection uses the JIRA issuetype.subtask boolean flag so it works
	// regardless of how the sub-task issue type is named in the project.
	epicsByKey := make(map[string]JiraIssue)
	var stories, subTasks []JiraIssue
	for _, issue := range phase1Issues {
		switch {
		case issue.IssueType == "Epic":
			epicsByKey[issue.Key] = issue
		case issue.Subtask:
			subTasks = append(subTasks, issue)
		default:
			stories = append(stories, issue)
		}
	}

	// Only look at stories (not sub-tasks) for missing parent epics.
	missingEpicKeys := s.collectMissingEpicKeys(stories, epicsByKey)

	// Phase 2: fetch any parent epics not returned by Phase 1.
	if len(missingEpicKeys) > 0 {
		for _, key := range missingEpicKeys {
			if !jiraKeyPattern.MatchString(key) {
				return nil, fmt.Errorf("coverage: invalid JIRA issue key %q in Phase 2 batch", key)
			}
		}
		phase2JQL := fmt.Sprintf("issueKey IN (%s)", strings.Join(missingEpicKeys, ","))
		phase2Issues, err := s.jiraClient.SearchIssues(ctx, baseURL, username, credential, authType, phase2JQL, fields)
		if err != nil {
			return nil, fmt.Errorf("coverage: Phase 2 search failed: %w", err)
		}
		for _, issue := range phase2Issues {
			epicsByKey[issue.Key] = issue
		}
	}

	// Fetch Fern test-run coverage for the project.
	coverageMap, err := s.tagRepo.GetJiraTagCoverageByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("coverage: failed to fetch tag coverage: %w", err)
	}

	return assembleTree(fixVersion, epicsByKey, stories, subTasks, coverageMap), nil
}

// resolveVersion finds the JiraVersion matching fixVersionName for the project.
func (s *CoverageService) resolveVersion(ctx context.Context, baseURL, projectKey, username, credential string, authType AuthenticationType, fixVersionName string) (JiraVersion, error) {
	versions, err := s.jiraClient.GetVersions(ctx, baseURL, projectKey, username, credential, authType)
	if err != nil {
		return JiraVersion{}, fmt.Errorf("coverage: failed to fetch versions: %w", err)
	}
	for _, v := range versions {
		if v.Name == fixVersionName {
			return v, nil
		}
	}
	return JiraVersion{}, fmt.Errorf("coverage: fix version %q not found in project %q", fixVersionName, projectKey)
}

// collectMissingEpicKeys returns parent epic keys that are referenced by nonEpics but absent from epicsByKey.
func (s *CoverageService) collectMissingEpicKeys(nonEpics []JiraIssue, epicsByKey map[string]JiraIssue) []string {
	seen := make(map[string]bool)
	var missing []string
	for _, issue := range nonEpics {
		if issue.Parent == nil {
			continue
		}
		key := issue.Parent.Key
		if _, exists := epicsByKey[key]; !exists && !seen[key] {
			seen[key] = true
			missing = append(missing, key)
		}
	}
	return missing
}

// assembleTree builds the CoverageTree from the fetched issues and tag coverage map.
func assembleTree(fixVersion JiraVersion, epicsByKey map[string]JiraIssue, stories, subTasks []JiraIssue, coverageMap map[string]tagsdomain.CoverageCount) *CoverageTree {
	// Build story nodes indexed by key so sub-tasks can be attached.
	storyNodesByKey := make(map[string]*StoryNode, len(stories))
	for i := range stories {
		node := buildStoryNode(stories[i], coverageMap)
		storyNodesByKey[stories[i].Key] = &node
	}

	// Attach sub-tasks to their parent story; orphans go to Unassigned.
	var unassignedSubTasks []StoryNode
	for _, st := range subTasks {
		node := buildStoryNode(st, coverageMap)
		if st.Parent != nil && st.Parent.Key != "" {
			if parent, ok := storyNodesByKey[st.Parent.Key]; ok {
				parent.SubTasks = append(parent.SubTasks, node)
				continue
			}
		}
		unassignedSubTasks = append(unassignedSubTasks, node)
	}

	// Group stories by parent epic.
	storiesByEpic := make(map[string][]StoryNode)
	var unassigned []StoryNode
	for _, issue := range stories {
		node := *storyNodesByKey[issue.Key]
		if issue.Parent != nil && issue.Parent.Key != "" {
			storiesByEpic[issue.Parent.Key] = append(storiesByEpic[issue.Parent.Key], node)
		} else {
			unassigned = append(unassigned, node)
		}
	}

	var epicNodes []EpicNode
	for epicKey, epicIssue := range epicsByKey {
		epicStories := storiesByEpic[epicKey]
		covered := 0
		for _, sn := range epicStories {
			if sn.Covered {
				covered++
			}
		}
		epicNodes = append(epicNodes, EpicNode{
			Issue:        epicIssue,
			Stories:      epicStories,
			CoveredCount: covered,
			TotalCount:   len(epicStories),
		})
	}

	unassigned = append(unassigned, unassignedSubTasks...)
	return &CoverageTree{
		FixVersion: fixVersion,
		Epics:      epicNodes,
		Unassigned: unassigned,
	}
}

func buildStoryNode(issue JiraIssue, coverageMap map[string]tagsdomain.CoverageCount) StoryNode {
	node := StoryNode{Issue: issue}
	if count, ok := coverageMap[issue.Key]; ok && count.Total > 0 {
		node.Covered = true
		c := count
		node.TestRunCoverage = &c
	}
	return node
}
