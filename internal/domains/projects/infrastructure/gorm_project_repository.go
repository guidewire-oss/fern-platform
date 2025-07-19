package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

// GormProjectRepository is a GORM implementation of ProjectRepository
type GormProjectRepository struct {
	db *gorm.DB
}

// NewGormProjectRepository creates a new GORM project repository
func NewGormProjectRepository(db *gorm.DB) *GormProjectRepository {
	return &GormProjectRepository{db: db}
}

// Save persists a project
func (r *GormProjectRepository) Save(ctx context.Context, project *domain.Project) error {
	// Convert domain model to database model
	snapshot := project.ToSnapshot()
	settingsJSON, err := json.Marshal(snapshot.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	dbProject := &database.ProjectDetails{
		ProjectID:     string(snapshot.ProjectID),
		Name:          snapshot.Name,
		Description:   snapshot.Description,
		Repository:    snapshot.Repository,
		DefaultBranch: snapshot.DefaultBranch,
		Team:          string(snapshot.Team),
		IsActive:      snapshot.IsActive,
		Settings:      string(settingsJSON),
	}

	if err := r.db.WithContext(ctx).Create(dbProject).Error; err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	// Update the domain model with the generated ID
	project.SetID(dbProject.ID)

	return nil
}

// FindByID retrieves a project by its internal ID
func (r *GormProjectRepository) FindByID(ctx context.Context, id uint) (*domain.Project, error) {
	var dbProject database.ProjectDetails
	if err := r.db.WithContext(ctx).First(&dbProject, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("failed to find project: %w", err)
	}

	return r.toDomainModel(&dbProject)
}

// FindByProjectID retrieves a project by its project ID
func (r *GormProjectRepository) FindByProjectID(ctx context.Context, projectID domain.ProjectID) (*domain.Project, error) {
	var dbProject database.ProjectDetails
	if err := r.db.WithContext(ctx).Where("project_id = ?", string(projectID)).First(&dbProject).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("failed to find project: %w", err)
	}

	return r.toDomainModel(&dbProject)
}

// FindByTeam retrieves all projects for a team
func (r *GormProjectRepository) FindByTeam(ctx context.Context, team domain.Team) ([]*domain.Project, error) {
	var dbProjects []database.ProjectDetails
	if err := r.db.WithContext(ctx).Where("team = ?", string(team)).Find(&dbProjects).Error; err != nil {
		return nil, fmt.Errorf("failed to find projects by team: %w", err)
	}

	projects := make([]*domain.Project, len(dbProjects))
	for i, dbProject := range dbProjects {
		project, err := r.toDomainModel(&dbProject)
		if err != nil {
			return nil, err
		}
		projects[i] = project
	}

	return projects, nil
}

// FindAll retrieves all projects with pagination
func (r *GormProjectRepository) FindAll(ctx context.Context, limit, offset int) ([]*domain.Project, int64, error) {
	var dbProjects []database.ProjectDetails
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).Model(&database.ProjectDetails{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count projects: %w", err)
	}

	// Get paginated results
	if err := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&dbProjects).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to find projects: %w", err)
	}

	projects := make([]*domain.Project, len(dbProjects))
	for i, dbProject := range dbProjects {
		project, err := r.toDomainModel(&dbProject)
		if err != nil {
			return nil, 0, err
		}
		projects[i] = project
	}

	return projects, total, nil
}

// Update updates an existing project
func (r *GormProjectRepository) Update(ctx context.Context, project *domain.Project) error {
	snapshot := project.ToSnapshot()
	settingsJSON, err := json.Marshal(snapshot.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	updates := map[string]interface{}{
		"name":           snapshot.Name,
		"description":    snapshot.Description,
		"repository":     snapshot.Repository,
		"default_branch": snapshot.DefaultBranch,
		"team":           string(snapshot.Team),
		"is_active":      snapshot.IsActive,
		"settings":       settingsJSON,
		"updated_at":     snapshot.UpdatedAt,
	}

	if err := r.db.WithContext(ctx).Model(&database.ProjectDetails{}).Where("id = ?", snapshot.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}

	return nil
}

// Delete permanently deletes a project (hard delete)
func (r *GormProjectRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Unscoped().Delete(&database.ProjectDetails{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}

// ExistsByProjectID checks if a project exists with the given project ID
func (r *GormProjectRepository) ExistsByProjectID(ctx context.Context, projectID domain.ProjectID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&database.ProjectDetails{}).Where("project_id = ?", string(projectID)).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check project existence: %w", err)
	}
	return count > 0, nil
}

// toDomainModel converts a database model to a domain model
func (r *GormProjectRepository) toDomainModel(dbProject *database.ProjectDetails) (*domain.Project, error) {
	// Unmarshal settings
	var settings map[string]interface{}
	if len(dbProject.Settings) > 0 {
		if err := json.Unmarshal([]byte(dbProject.Settings), &settings); err != nil {
			return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
		}
	} else {
		settings = make(map[string]interface{})
	}

	// Create project using constructor
	project, err := domain.NewProject(domain.ProjectID(dbProject.ProjectID), dbProject.Name, domain.Team(dbProject.Team))
	if err != nil {
		return nil, err
	}

	// Update fields that can't be set via constructor
	project.UpdateDescription(dbProject.Description)
	project.UpdateRepository(dbProject.Repository)
	if dbProject.DefaultBranch != "" {
		project.UpdateDefaultBranch(dbProject.DefaultBranch)
	}

	// Set active status
	if !dbProject.IsActive {
		project.Deactivate()
	}

	// Set settings
	for key, value := range settings {
		project.SetSetting(key, value)
	}

	// Set the database ID
	project.SetID(dbProject.ID)

	return project, nil
}
