package integrations

import (
	"context"
)

// JiraConnectionRepository defines the interface for JIRA connection persistence
type JiraConnectionRepository interface {
	// Create saves a new JIRA connection
	Create(ctx context.Context, connection *JiraConnection) error
	
	// Update updates an existing JIRA connection
	Update(ctx context.Context, connection *JiraConnection) error
	
	// Delete removes a JIRA connection
	Delete(ctx context.Context, connectionID string) error
	
	// FindByID retrieves a connection by ID
	FindByID(ctx context.Context, connectionID string) (*JiraConnection, error)
	
	// FindByProjectID retrieves all connections for a project
	FindByProjectID(ctx context.Context, projectID string) ([]*JiraConnection, error)
	
	// FindActiveByProjectID retrieves all active connections for a project
	FindActiveByProjectID(ctx context.Context, projectID string) ([]*JiraConnection, error)
}