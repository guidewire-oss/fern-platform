package integrations

import (
	"context"
	"errors"

	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

// AuthenticationType represents the type of authentication used for JIRA
type AuthenticationType string

const (
	// AuthTypeAPIToken represents API token authentication
	AuthTypeAPIToken AuthenticationType = "api_token"
	// AuthTypeOAuth represents OAuth authentication
	AuthTypeOAuth AuthenticationType = "oauth"
	// AuthTypePersonalAccessToken represents personal access token authentication
	AuthTypePersonalAccessToken AuthenticationType = "personal_access_token"
)

// ConnectionStatus represents the current status of a JIRA connection
type ConnectionStatus string

const (
	// ConnectionStatusPending indicates the connection hasn't been tested yet
	ConnectionStatusPending ConnectionStatus = "pending"
	// ConnectionStatusConnected indicates the connection is active and working
	ConnectionStatusConnected ConnectionStatus = "connected"
	// ConnectionStatusFailed indicates the connection test failed
	ConnectionStatusFailed ConnectionStatus = "failed"
)

// JiraProject represents a JIRA project
type JiraProject struct {
	ID   string
	Key  string
	Name string
}

// JiraField represents a JIRA field
type JiraField struct {
	ID         string
	Name       string
	Custom     bool
	MultiValue bool
	SchemaType string
}

// JiraIssueType represents a JIRA issue type
type JiraIssueType struct {
	ID          string
	Name        string
	Description string
	IconURL     string
	Subtask     bool
}

// FernField represents a Fern requirement field that can be mapped from a JIRA field
type FernField string

const (
	FernFieldRequirementID     FernField = "requirement_id"
	FernFieldRequirementTitle  FernField = "requirement_title"
	FernFieldDescription       FernField = "description"
	FernFieldParentRequirement FernField = "parent_requirement"
	FernFieldRequirementType   FernField = "requirement_type"
	FernFieldReleaseVersion    FernField = "release_version"
	FernFieldRequirementStatus FernField = "requirement_status"
	FernFieldTags              FernField = "tags"
)

type fernFieldProps struct {
	required   bool
	multiValue bool
}

// fernFieldRegistry is the single source of truth for FernField properties.
// Add new fields here — IsValid, IsRequired, and IsMultiValue all derive from this map.
var fernFieldRegistry = map[FernField]fernFieldProps{
	FernFieldRequirementID:     {required: true},
	FernFieldRequirementTitle:  {required: true},
	FernFieldDescription:       {},
	FernFieldParentRequirement: {},
	FernFieldRequirementType:   {},
	FernFieldReleaseVersion:    {},
	FernFieldRequirementStatus: {},
	FernFieldTags:              {multiValue: true},
}

func (f FernField) IsRequired() bool   { return fernFieldRegistry[f].required }
func (f FernField) IsMultiValue() bool { return fernFieldRegistry[f].multiValue }
func (f FernField) IsValid() bool      { _, ok := fernFieldRegistry[f]; return ok }

// AllFernFields returns all known Fern fields in canonical order.
func AllFernFields() []FernField {
	return []FernField{
		FernFieldRequirementID,
		FernFieldRequirementTitle,
		FernFieldDescription,
		FernFieldParentRequirement,
		FernFieldRequirementType,
		FernFieldReleaseVersion,
		FernFieldRequirementStatus,
		FernFieldTags,
	}
}

// ReductionStrategy determines how a multi-value JIRA field is collapsed when
// mapped to a single-value Fern field
type ReductionStrategy string

const (
	ReductionStrategyFirstValue  ReductionStrategy = "first_value"
	ReductionStrategyConcatenate ReductionStrategy = "concatenate"
	ReductionStrategySeparate    ReductionStrategy = "separate_entries"
)

// IsValid returns true if r is one of the three known reduction strategy constants.
func (r ReductionStrategy) IsValid() bool {
	switch r {
	case ReductionStrategyFirstValue, ReductionStrategyConcatenate, ReductionStrategySeparate:
		return true
	}
	return false
}

// JiraParent is the parent reference on a JIRA issue (populated when requesting the "parent" field).
type JiraParent struct {
	Key       string
	IssueType string
}

// JiraIssue is the subset of JIRA issue fields needed for coverage hierarchy.
type JiraIssue struct {
	Key        string
	Summary    string
	StatusName string
	IssueType  string
	Subtask    bool        // true when JIRA's issuetype.subtask flag is set, regardless of type name
	Parent     *JiraParent // nil when the issue has no parent
}

// CoverageJiraClient is the narrow JIRA client interface required by the coverage service.
type CoverageJiraClient interface {
	// GetEpicReleases returns distinct non-empty values of the custom release field
	// (e.g. cf[10077]) across all Epics in the project, sorted alphabetically.
	GetEpicReleases(ctx context.Context, baseURL, projectKey, releaseFieldID, username, credential string, authType AuthenticationType) ([]string, error)
	SearchIssues(ctx context.Context, baseURL, username, credential string, authType AuthenticationType, jql string, fields []string) ([]JiraIssue, error)
}

// CoverageTagRepository is the narrow tag-repository interface required by the coverage service.
type CoverageTagRepository interface {
	GetJiraTagCoverageByProject(ctx context.Context, projectID string) (map[string]tagsdomain.CoverageCount, error)
	GetSpecRunsByJiraTag(ctx context.Context, projectID, issueKey string) ([]tagsdomain.CoveredSpecRun, error)
}

// CoverageTree is the assembled result of a requirementCoverage query.
type CoverageTree struct {
	Release    string
	Epics      []EpicNode
	Unassigned []StoryNode
}

// EpicNode groups the stories that belong to a single epic in a fix version.
type EpicNode struct {
	Issue        JiraIssue
	Stories      []StoryNode
	CoveredCount int // stories with at least one test run
	TotalCount   int // total stories under this epic
}

// StoryNode is a single non-epic issue with its Fern test-run coverage.
// Sub-tasks are represented as child StoryNodes (at most one level deep).
type StoryNode struct {
	Issue           JiraIssue
	Covered         bool
	TestRunCoverage *tagsdomain.CoverageCount // nil when not covered
	SubTasks        []StoryNode
}

// Sentinel errors for field mapping validation
var (
	ErrRequiredFieldUnmapped     = errors.New("required Fern field is unmapped")
	ErrDuplicateJiraField        = errors.New("JIRA field is already mapped to another Fern field")
	ErrDuplicateFernField        = errors.New("Fern field appears more than once in the mapping")
	ErrMissingReductionStrategy  = errors.New("reduction strategy required for single-value Fern field mapped to multi-value JIRA field")
	ErrNoJiraConnection          = errors.New("project has no JIRA connection configured")
	ErrNoFieldMapping            = errors.New("project has no saved field mapping")
	ErrUnknownFernField          = errors.New("unknown Fern field")
	ErrUnknownReductionStrategy  = errors.New("unknown reduction strategy")
)

// ErrJiraDisabled is returned by JiraConnectionService mutating methods when
// JIRA_ENCRYPTION_KEY is not configured. Callers (e.g. GraphQL resolvers) can
// check for it with errors.Is to distinguish "integration not configured"
// from operation-specific failures (bad credentials, unreachable JIRA, etc).
var ErrJiraDisabled = errors.New("JIRA integration is not configured; set JIRA_ENCRYPTION_KEY environment variable to enable it")