package integrations

import (
	"context"
	"fmt"
	"strings"

	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

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

	// Resolve the JiraVersion object for the requested name.
	fixVersion, err := s.resolveVersion(ctx, baseURL, projectKey, username, credential, authType, fixVersionName)
	if err != nil {
		return nil, err
	}

	// Phase 1: fetch all issues for the fix version.
	phase1JQL := fmt.Sprintf(`fixVersion = %q ORDER BY issuetype`, fixVersionName)
	fields := []string{"summary", "status", "issuetype", "parent"}
	phase1Issues, err := s.jiraClient.SearchIssues(ctx, baseURL, username, credential, authType, phase1JQL, fields)
	if err != nil {
		return nil, fmt.Errorf("coverage: Phase 1 search failed: %w", err)
	}

	// Separate epics from non-epics; collect parent keys missing from Phase 1.
	epicsByKey := make(map[string]JiraIssue)
	var nonEpics []JiraIssue
	for _, issue := range phase1Issues {
		if issue.IssueType == "Epic" {
			epicsByKey[issue.Key] = issue
		} else {
			nonEpics = append(nonEpics, issue)
		}
	}

	missingEpicKeys := s.collectMissingEpicKeys(nonEpics, epicsByKey)

	// Phase 2: fetch any parent epics not returned by Phase 1.
	if len(missingEpicKeys) > 0 {
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

	return s.assembleTree(fixVersion, epicsByKey, nonEpics, coverageMap), nil
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
func (s *CoverageService) assembleTree(fixVersion JiraVersion, epicsByKey map[string]JiraIssue, nonEpics []JiraIssue, coverageMap map[string]tagsdomain.CoverageCount) *CoverageTree {
	storiesByEpic := make(map[string][]StoryNode)
	var unassigned []StoryNode

	for _, issue := range nonEpics {
		node := s.buildStoryNode(issue, coverageMap)
		if issue.Parent != nil && issue.Parent.Key != "" {
			storiesByEpic[issue.Parent.Key] = append(storiesByEpic[issue.Parent.Key], node)
		} else {
			unassigned = append(unassigned, node)
		}
	}

	var epicNodes []EpicNode
	for epicKey, epicIssue := range epicsByKey {
		stories := storiesByEpic[epicKey]
		covered := 0
		for _, s := range stories {
			if s.Covered {
				covered++
			}
		}
		epicNodes = append(epicNodes, EpicNode{
			Issue:        epicIssue,
			Stories:      stories,
			CoveredCount: covered,
			TotalCount:   len(stories),
		})
	}

	return &CoverageTree{
		FixVersion: fixVersion,
		Epics:      epicNodes,
		Unassigned: unassigned,
	}
}

func (s *CoverageService) buildStoryNode(issue JiraIssue, coverageMap map[string]tagsdomain.CoverageCount) StoryNode {
	node := StoryNode{Issue: issue}
	if count, ok := coverageMap[issue.Key]; ok && count.Total > 0 {
		node.Covered = true
		c := count
		node.TestRunCoverage = &c
	}
	return node
}
