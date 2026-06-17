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
var jiraKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

const jiraAPITimeout = 30 * time.Second

// fieldMappingLookup is the narrow interface CoverageService needs from JiraFieldMappingService.
// Defined here so tests can provide a simple mock without wiring up the full service.
type fieldMappingLookup interface {
	Get(ctx context.Context, projectID string) (*JiraFieldMappingSnapshot, error)
}

// CoverageService builds a RequirementCoverageTree for a project + release value.
type CoverageService struct {
	connRepo       JiraConnectionRepository
	jiraClient     CoverageJiraClient
	tagRepo        CoverageTagRepository
	mappingService fieldMappingLookup
	encryptionKey  []byte
}

// NewCoverageService wires up a CoverageService with its dependencies.
func NewCoverageService(
	connRepo JiraConnectionRepository,
	jiraClient CoverageJiraClient,
	tagRepo CoverageTagRepository,
	mappingService fieldMappingLookup,
	encryptionKey []byte,
) *CoverageService {
	return &CoverageService{
		connRepo:       connRepo,
		jiraClient:     jiraClient,
		tagRepo:        tagRepo,
		mappingService: mappingService,
		encryptionKey:  encryptionKey,
	}
}

// GetReleasesForProject returns distinct non-empty release values from Epics in the project's JIRA.
// The values come from the custom field configured via the field mapping (FernFieldReleaseVersion).
func (s *CoverageService) GetReleasesForProject(ctx context.Context, projectID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, jiraAPITimeout)
	defer cancel()

	conn, credential, err := s.resolveConnection(ctx, projectID)
	if err != nil {
		return nil, err
	}
	releaseFieldID, err := s.getReleaseFieldID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return s.jiraClient.GetEpicReleases(ctx, conn.JiraURL(), conn.ProjectKey(), releaseFieldID, conn.Username(), credential, conn.AuthenticationType())
}

// Build fetches the Epic-first three-phase hierarchy for the given release value and
// cross-references it with Fern test-run coverage.
func (s *CoverageService) Build(ctx context.Context, projectID, releaseValue string) (*CoverageTree, error) {
	ctx, cancel := context.WithTimeout(ctx, jiraAPITimeout)
	defer cancel()

	conn, credential, err := s.resolveConnection(ctx, projectID)
	if err != nil {
		return nil, err
	}
	releaseFieldID, err := s.getReleaseFieldID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// JQL uses cf[numericID]; the fields parameter uses the full customfield_NNNNN form.
	numericFieldID := extractNumericFieldID(releaseFieldID)

	baseURL := conn.JiraURL()
	username := conn.Username()
	authType := conn.AuthenticationType()
	fields := []string{"summary", "status", "issuetype", "parent"}

	// Phase 1 — Epics matching the release custom field value.
	phase1JQL := fmt.Sprintf(`project = %q AND issuetype = Epic AND cf[%s] = %q ORDER BY key`, conn.ProjectKey(), numericFieldID, releaseValue)
	epicIssues, err := s.jiraClient.SearchIssues(ctx, baseURL, username, credential, authType, phase1JQL, fields)
	if err != nil {
		return nil, fmt.Errorf("coverage: Phase 1 (Epics) search failed: %w", err)
	}

	epicsByKey := make(map[string]JiraIssue, len(epicIssues))
	epicKeys := make([]string, 0, len(epicIssues))
	for _, e := range epicIssues {
		epicsByKey[e.Key] = e
		epicKeys = append(epicKeys, e.Key)
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("coverage: context cancelled after Phase 1: %w", err)
	}

	// Phase 2 — Stories whose parent is one of the Epics, chunked ≤50 keys per request.
	var stories []JiraIssue
	for _, chunk := range chunkKeys(epicKeys, 50) {
		for _, key := range chunk {
			if !jiraKeyPattern.MatchString(key) {
				return nil, fmt.Errorf("coverage: invalid JIRA issue key %q in Phase 2 batch", key)
			}
		}
		jql := fmt.Sprintf("parent IN (%s) ORDER BY key", strings.Join(chunk, ","))
		got, err := s.jiraClient.SearchIssues(ctx, baseURL, username, credential, authType, jql, fields)
		if err != nil {
			return nil, fmt.Errorf("coverage: Phase 2 (Stories) search failed: %w", err)
		}
		stories = append(stories, got...)
	}

	// Drop any sub-tasks Phase 2 returned (e.g. projects that rename the sub-task
	// type). Coverage reports on the main task (Story) level; sub-tasks are excluded
	// as noise, so we neither keep these nor fetch their siblings. This also avoids a
	// whole extra pagination pass over story keys.
	var nonSubTasks []JiraIssue
	for _, s := range stories {
		if !s.Subtask {
			nonSubTasks = append(nonSubTasks, s)
		}
	}
	stories = nonSubTasks

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("coverage: context cancelled after Phase 2: %w", err)
	}

	coverageMap, err := s.tagRepo.GetJiraTagCoverageByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("coverage: failed to fetch tag coverage: %w", err)
	}

	return assembleTree(releaseValue, epicKeys, epicsByKey, stories, nil, coverageMap), nil
}

// GetSpecRunsByJiraTag returns spec runs tagged with the given JIRA issue key.
func (s *CoverageService) GetSpecRunsByJiraTag(ctx context.Context, projectID, issueKey string) ([]tagsdomain.CoveredSpecRun, error) {
	return s.tagRepo.GetSpecRunsByJiraTag(ctx, projectID, issueKey)
}

// resolveConnection fetches the active JIRA connection and decrypts its credential.
func (s *CoverageService) resolveConnection(ctx context.Context, projectID string) (*JiraConnection, string, error) {
	conns, err := s.connRepo.FindActiveByProjectID(ctx, projectID)
	if err != nil {
		return nil, "", fmt.Errorf("coverage: failed to find JIRA connection: %w", err)
	}
	if len(conns) == 0 {
		return nil, "", fmt.Errorf("coverage: no active JIRA connection for project %q", projectID)
	}
	conn := conns[0]
	credential, err := DecryptCredential(conn.GetEncryptedCredentialDirect(), s.encryptionKey)
	if err != nil {
		return nil, "", fmt.Errorf("coverage: failed to decrypt credential: %w", err)
	}
	return conn, credential, nil
}

// getReleaseFieldID reads the JIRA field ID (e.g. "customfield_10077") mapped to
// FernFieldReleaseVersion from the project's field mapping.
func (s *CoverageService) getReleaseFieldID(ctx context.Context, projectID string) (string, error) {
	snapshot, err := s.mappingService.Get(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("coverage: failed to get field mapping: %w", err)
	}
	for _, entry := range snapshot.Entries {
		if entry.FernField == FernFieldReleaseVersion {
			return entry.JiraFieldID, nil
		}
	}
	return "", fmt.Errorf("coverage: release_version field not mapped for project %q; configure it in the field mapping settings", projectID)
}

// extractNumericFieldID converts a JIRA field ID to the numeric form used in JQL (cf[N]).
// Accepts: "customfield_10077" → "10077", "cf[10077]" → "10077", "10077" → "10077".
func extractNumericFieldID(jiraFieldID string) string {
	if strings.HasPrefix(jiraFieldID, "customfield_") {
		return strings.TrimPrefix(jiraFieldID, "customfield_")
	}
	if strings.HasPrefix(jiraFieldID, "cf[") && strings.HasSuffix(jiraFieldID, "]") {
		return jiraFieldID[3 : len(jiraFieldID)-1]
	}
	return jiraFieldID
}

// chunkKeys splits keys into slices of at most size elements.
func chunkKeys(keys []string, size int) [][]string {
	if len(keys) == 0 {
		return nil
	}
	var chunks [][]string
	for len(keys) > 0 {
		n := size
		if len(keys) < n {
			n = len(keys)
		}
		chunks = append(chunks, keys[:n])
		keys = keys[n:]
	}
	return chunks
}

// assembleTree builds the CoverageTree from the fetched issues and tag coverage map.
// epicKeys preserves the fetch order so epics appear deterministically.
func assembleTree(releaseValue string, epicKeys []string, epicsByKey map[string]JiraIssue, stories, subTasks []JiraIssue, coverageMap map[string]tagsdomain.CoverageCount) *CoverageTree {
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
	for _, epicKey := range epicKeys {
		epicIssue := epicsByKey[epicKey]
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
		Release:    releaseValue,
		Epics:      epicNodes,
		Unassigned: unassigned,
	}
}

func buildStoryNode(issue JiraIssue, coverageMap map[string]tagsdomain.CoverageCount) StoryNode {
	node := StoryNode{Issue: issue}
	// Coverage keys are canonical uppercase (tags are lowercased on ingest, JIRA
	// keys are uppercase); match case-insensitively against the JIRA issue key.
	if count, ok := coverageMap[strings.ToUpper(issue.Key)]; ok && count.Total > 0 {
		node.Covered = true
		c := count
		node.TestRunCoverage = &c
	}
	return node
}
