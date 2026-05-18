package integrations

import "errors"

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