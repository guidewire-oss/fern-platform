package integrations

import (
	"context"
	"fmt"
	"log"
)

// JiraConnectionService handles JIRA connection operations
type JiraConnectionService struct {
	repo           JiraConnectionRepository
	jiraClient     JiraClient
	encryptionKey  []byte
}

// NewJiraConnectionService creates a new JIRA connection service
func NewJiraConnectionService(repo JiraConnectionRepository, jiraClient JiraClient, encryptionKey []byte) *JiraConnectionService {
	return &JiraConnectionService{
		repo:          repo,
		jiraClient:    jiraClient,
		encryptionKey: encryptionKey,
	}
}

// CreateConnection creates a new JIRA connection
func (s *JiraConnectionService) CreateConnection(ctx context.Context, projectID, name, jiraURL string, authType AuthenticationType, projectKey, username, credential string) (*JiraConnection, error) {
	// Check if a connection already exists for this project
	existingConnections, err := s.repo.FindByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing connections: %w", err)
	}
	
	// Check for any non-deleted connections
	// Since we use soft deletion with deleted_at timestamp, we only need to check
	// if any active connections exist (the repository should filter out deleted ones)
	if len(existingConnections) > 0 {
		return nil, fmt.Errorf("project already has a JIRA connection")
	}
	
	// Create the connection
	conn, err := NewJiraConnection(projectID, name, jiraURL, authType, projectKey, username, credential)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	// Encrypt the credential before saving
	encrypted, err := conn.GetEncryptedCredential(s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt credential: %w", err)
	}
	conn.encryptedCredential = encrypted

	// Save to repository
	if err := s.repo.Create(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to save connection: %w", err)
	}

	return conn, nil
}

// UpdateConnection updates an existing JIRA connection
func (s *JiraConnectionService) UpdateConnection(ctx context.Context, connectionID, name, jiraURL, projectKey string) (*JiraConnection, error) {
	// Retrieve the connection
	conn, err := s.repo.FindByID(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find connection: %w", err)
	}

	// Update connection info
	if err := conn.UpdateConnectionInfo(name, jiraURL, projectKey); err != nil {
		return nil, fmt.Errorf("failed to update connection info: %w", err)
	}

	// Save changes
	if err := s.repo.Update(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to save connection: %w", err)
	}

	return conn, nil
}

// UpdateCredentials updates the credentials for a JIRA connection
func (s *JiraConnectionService) UpdateCredentials(ctx context.Context, connectionID string, authType AuthenticationType, username, credential string) (*JiraConnection, error) {
	// Retrieve the connection
	conn, err := s.repo.FindByID(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("failed to find connection: %w", err)
	}

	// Update credentials
	if err := conn.UpdateCredentials(authType, username, credential); err != nil {
		return nil, fmt.Errorf("failed to update credentials: %w", err)
	}

	// Encrypt the new credential
	encrypted, err := conn.GetEncryptedCredential(s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt credential: %w", err)
	}
	conn.encryptedCredential = encrypted

	// Save changes
	if err := s.repo.Update(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to save connection: %w", err)
	}

	return conn, nil
}

// TestConnection tests a JIRA connection
func (s *JiraConnectionService) TestConnection(ctx context.Context, connectionID string) error {
	// Retrieve the connection
	conn, err := s.repo.FindByID(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("failed to find connection: %w", err)
	}

	// Log connection details (without sensitive info)
	log.Printf("[JiraConnectionService] Testing connection ID: %s, URL: %s, Project: %s, Username: %s", 
		connectionID, conn.jiraURL, conn.projectKey, conn.username)

	// Decrypt the credential
	decrypted, err := DecryptCredential(conn.encryptedCredential, s.encryptionKey)
	if err != nil {
		log.Printf("[JiraConnectionService] Failed to decrypt credential: %v", err)
		return fmt.Errorf("failed to decrypt credential: %w", err)
	}

	// Save the original encrypted credential
	originalEncrypted := conn.GetEncryptedCredentialDirect()
	
	// Temporarily set the decrypted credential for testing
	conn.encryptedCredential = decrypted
	
	// Test the connection
	log.Printf("[JiraConnectionService] Calling TestConnection on JIRA client for URL: %s", conn.jiraURL)
	err = conn.TestConnection(ctx, s.jiraClient)
	
	// CRITICAL: Restore the original encrypted credential before saving
	conn.encryptedCredential = originalEncrypted
	
	// Now save with the original encrypted credential
	if err != nil {
		log.Printf("[JiraConnectionService] Test failed for %s: %v", conn.jiraURL, err)
		updateErr := s.repo.Update(ctx, conn)
		if updateErr != nil {
			log.Printf("[JiraConnectionService] Failed to update connection after test failure: %v", updateErr)
		}
		return err
	}
	
	log.Printf("[JiraConnectionService] Test successful for %s, updating connection", conn.jiraURL)
	return s.repo.Update(ctx, conn)
}

// ActivateConnection activates a JIRA connection
func (s *JiraConnectionService) ActivateConnection(ctx context.Context, connectionID string) error {
	conn, err := s.repo.FindByID(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("failed to find connection: %w", err)
	}

	conn.Activate()
	return s.repo.Update(ctx, conn)
}

// DeactivateConnection deactivates a JIRA connection
func (s *JiraConnectionService) DeactivateConnection(ctx context.Context, connectionID string) error {
	conn, err := s.repo.FindByID(ctx, connectionID)
	if err != nil {
		return fmt.Errorf("failed to find connection: %w", err)
	}

	conn.Deactivate()
	return s.repo.Update(ctx, conn)
}

// DeleteConnection deletes a JIRA connection
func (s *JiraConnectionService) DeleteConnection(ctx context.Context, connectionID string) error {
	return s.repo.Delete(ctx, connectionID)
}

// GetConnection retrieves a JIRA connection by ID
func (s *JiraConnectionService) GetConnection(ctx context.Context, connectionID string) (*JiraConnection, error) {
	return s.repo.FindByID(ctx, connectionID)
}

// GetProjectConnections retrieves all connections for a project
func (s *JiraConnectionService) GetProjectConnections(ctx context.Context, projectID string) ([]*JiraConnection, error) {
	return s.repo.FindByProjectID(ctx, projectID)
}

// GetActiveProjectConnections retrieves all active connections for a project
func (s *JiraConnectionService) GetActiveProjectConnections(ctx context.Context, projectID string) ([]*JiraConnection, error) {
	return s.repo.FindActiveByProjectID(ctx, projectID)
}