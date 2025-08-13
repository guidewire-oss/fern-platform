package integrations

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