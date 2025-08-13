package repositories

import (
	"context"
	"fmt"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

// GormJiraConnectionRepository implements JiraConnectionRepository using GORM
type GormJiraConnectionRepository struct {
	db *gorm.DB
}

// NewGormJiraConnectionRepository creates a new GORM-based JIRA connection repository
func NewGormJiraConnectionRepository(db *gorm.DB) integrations.JiraConnectionRepository {
	return &GormJiraConnectionRepository{db: db}
}

// Create saves a new JIRA connection
func (r *GormJiraConnectionRepository) Create(ctx context.Context, connection *integrations.JiraConnection) error {
	model := r.toModel(connection)
	
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("failed to create JIRA connection: %w", err)
	}
	
	return nil
}

// Update updates an existing JIRA connection
func (r *GormJiraConnectionRepository) Update(ctx context.Context, connection *integrations.JiraConnection) error {
	model := r.toModel(connection)
	
	if err := r.db.WithContext(ctx).Save(&model).Error; err != nil {
		return fmt.Errorf("failed to update JIRA connection: %w", err)
	}
	
	return nil
}

// Delete removes a JIRA connection
func (r *GormJiraConnectionRepository) Delete(ctx context.Context, connectionID string) error {
	if err := r.db.WithContext(ctx).Delete(&database.JiraConnection{}, "id = ?", connectionID).Error; err != nil {
		return fmt.Errorf("failed to delete JIRA connection: %w", err)
	}
	
	return nil
}

// FindByID retrieves a connection by ID
func (r *GormJiraConnectionRepository) FindByID(ctx context.Context, connectionID string) (*integrations.JiraConnection, error) {
	var model database.JiraConnection
	
	if err := r.db.WithContext(ctx).First(&model, "id = ?", connectionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("JIRA connection not found")
		}
		return nil, fmt.Errorf("failed to find JIRA connection: %w", err)
	}
	
	return r.toDomain(&model), nil
}

// FindByProjectID retrieves all connections for a project
func (r *GormJiraConnectionRepository) FindByProjectID(ctx context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	var models []database.JiraConnection
	
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to find JIRA connections: %w", err)
	}
	
	connections := make([]*integrations.JiraConnection, len(models))
	for i, model := range models {
		connections[i] = r.toDomain(&model)
	}
	
	return connections, nil
}

// FindActiveByProjectID retrieves all active connections for a project
func (r *GormJiraConnectionRepository) FindActiveByProjectID(ctx context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	var models []database.JiraConnection
	
	if err := r.db.WithContext(ctx).Where("project_id = ? AND is_active = ?", projectID, true).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to find active JIRA connections: %w", err)
	}
	
	connections := make([]*integrations.JiraConnection, len(models))
	for i, model := range models {
		connections[i] = r.toDomain(&model)
	}
	
	return connections, nil
}

// toModel converts a domain entity to a database model
func (r *GormJiraConnectionRepository) toModel(conn *integrations.JiraConnection) *database.JiraConnection {
	snapshot := conn.Snapshot()
	model := &database.JiraConnection{
		ProjectID:           snapshot.ProjectID,
		Name:                snapshot.Name,
		JiraURL:             snapshot.JiraURL,
		AuthenticationType:  string(snapshot.AuthenticationType),
		ProjectKey:          snapshot.ProjectKey,
		Username:            snapshot.Username,
		EncryptedCredential: conn.GetEncryptedCredentialDirect(), // This needs to be added to domain
		Status:              string(snapshot.Status),
		IsActive:            snapshot.IsActive,
		LastTestedAt:        snapshot.LastTestedAt,
	}
	
	// CRITICAL: Set the ID to ensure updates work correctly
	// Convert string ID to uint (assuming numeric IDs)
	if id := snapshot.ID; id != "" {
		var numericID uint
		if _, err := fmt.Sscanf(id, "%d", &numericID); err == nil {
			model.ID = numericID
		}
	}
	
	// Set timestamps if they exist
	model.CreatedAt = snapshot.CreatedAt
	model.UpdatedAt = snapshot.UpdatedAt
	
	return model
}

// toDomain converts a database model to a domain entity
func (r *GormJiraConnectionRepository) toDomain(model *database.JiraConnection) *integrations.JiraConnection {
	return integrations.ReconstructJiraConnection(
		fmt.Sprintf("%d", model.ID),
		model.ProjectID,
		model.Name,
		model.JiraURL,
		integrations.AuthenticationType(model.AuthenticationType),
		model.ProjectKey,
		model.Username,
		model.EncryptedCredential,
		integrations.ConnectionStatus(model.Status),
		model.IsActive,
		model.LastTestedAt,
		model.CreatedAt,
		model.UpdatedAt,
	)
}