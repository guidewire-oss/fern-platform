package repositories

import (
	"context"
	"fmt"
	"strconv"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

type GormJiraConnectionRepository struct {
	db *gorm.DB
}

func NewGormJiraConnectionRepository(db *gorm.DB) integrations.JiraConnectionRepository {
	return &GormJiraConnectionRepository{db: db}
}

// Create saves a new JIRA connection
func (r *GormJiraConnectionRepository) Create(ctx context.Context, connection *integrations.JiraConnection) error {
	// A brand-new connection's domain ID is empty (see NewJiraConnection),
	// so toModel already leaves model.ID at 0 here. The explicit reset is a
	// defensive guard, not a no-op: it ensures Create never lets any
	// pre-existing ID reach the insert, even from a mis-constructed domain
	// object (e.g. the throwaway UUID a prior version of NewJiraConnection
	// used to mint) -- the database always assigns the real primary key.
	model, _ := r.toModel(connection)
	model.ID = 0

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("failed to create JIRA connection: %w", err)
	}

	if model.ID == 0 {
		return fmt.Errorf("JIRA connection created but no row ID was returned by the database")
	}

	connection.SetID(fmt.Sprintf("%d", model.ID))

	return nil
}

// Update updates an existing JIRA connection
func (r *GormJiraConnectionRepository) Update(ctx context.Context, connection *integrations.JiraConnection) error {
	model, err := r.toModel(connection)
	if err != nil {
		return fmt.Errorf("failed to update JIRA connection: %w", err)
	}
	if model.ID == 0 {
		return fmt.Errorf("cannot update JIRA connection with unset row ID")
	}

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

// toModel converts a domain entity to a database model. A non-empty domain
// ID must be a valid numeric row ID -- callers that don't yet have a real
// row ID (e.g. Create, before the DB assigns one) should discard the error
// and force model.ID to 0 themselves.
func (r *GormJiraConnectionRepository) toModel(conn *integrations.JiraConnection) (*database.JiraConnection, error) {
	snapshot := conn.Snapshot()
	model := &database.JiraConnection{
		ProjectID:           snapshot.ProjectID,
		Name:                snapshot.Name,
		JiraURL:             snapshot.JiraURL,
		AuthenticationType:  string(snapshot.AuthenticationType),
		ProjectKey:          snapshot.ProjectKey,
		Username:            snapshot.Username,
		EncryptedCredential: conn.GetEncryptedCredentialDirect(),
		Status:              string(snapshot.Status),
		IsActive:            snapshot.IsActive,
		VersionFilter:       snapshot.VersionFilter,
		LastTestedAt:        snapshot.LastTestedAt,
	}

	model.CreatedAt = snapshot.CreatedAt
	model.UpdatedAt = snapshot.UpdatedAt

	if id := snapshot.ID; id != "" {
		// Parse at uint's actual bit width (32 on a 32-bit platform), not a
		// hardcoded 64: ParseUint(..., 64) always returns a uint64, and
		// converting that down to uint would silently wrap an out-of-range
		// value instead of rejecting it.
		numericID, err := strconv.ParseUint(id, 10, strconv.IntSize)
		if err != nil {
			return model, fmt.Errorf("invalid JIRA connection ID %q: %w", id, err)
		}
		model.ID = uint(numericID)
	}

	return model, nil
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
		model.VersionFilter,
		model.LastTestedAt,
		model.CreatedAt,
		model.UpdatedAt,
	)
}