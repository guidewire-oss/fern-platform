package application

import (
	"context"
	"fmt"

	"github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
)

// ProjectService handles project business logic
type ProjectService struct {
	projectRepo    domain.ProjectRepository
	permissionRepo domain.ProjectPermissionRepository
}

// NewProjectService creates a new project service
func NewProjectService(
	projectRepo domain.ProjectRepository,
	permissionRepo domain.ProjectPermissionRepository,
) *ProjectService {
	return &ProjectService{
		projectRepo:    projectRepo,
		permissionRepo: permissionRepo,
	}
}

// CreateProject creates a new project
func (s *ProjectService) CreateProject(ctx context.Context, projectID domain.ProjectID, name string, team domain.Team, creatorUserID string) (*domain.Project, error) {
	// Check if project already exists
	exists, err := s.projectRepo.ExistsByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to check project existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("project with ID %s already exists", projectID)
	}

	// Create the project
	project, err := domain.NewProject(projectID, name, team)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// Save the project
	if err := s.projectRepo.Save(ctx, project); err != nil {
		return nil, fmt.Errorf("failed to save project: %w", err)
	}

	// Grant admin permission to the creator
	permission, err := domain.NewProjectPermission(projectID, creatorUserID, domain.PermissionAdmin, creatorUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to create permission: %w", err)
	}

	if err := s.permissionRepo.Save(ctx, permission); err != nil {
		// Log error but don't fail the project creation
		// In production, this might be handled differently
		fmt.Printf("Warning: failed to grant admin permission to creator: %v\n", err)
	}

	return project, nil
}

// GetProject retrieves a project by ID
func (s *ProjectService) GetProject(ctx context.Context, projectID domain.ProjectID) (*domain.Project, error) {
	project, err := s.projectRepo.FindByProjectID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return project, nil
}

// UpdateProject updates project details
func (s *ProjectService) UpdateProject(ctx context.Context, projectID domain.ProjectID, updates UpdateProjectRequest) error {
	// Get the project
	project, err := s.projectRepo.FindByProjectID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Apply updates
	if updates.Name != nil {
		if err := project.UpdateName(*updates.Name); err != nil {
			return fmt.Errorf("failed to update name: %w", err)
		}
	}

	if updates.Description != nil {
		project.UpdateDescription(*updates.Description)
	}

	if updates.Repository != nil {
		project.UpdateRepository(*updates.Repository)
	}

	if updates.DefaultBranch != nil {
		if err := project.UpdateDefaultBranch(*updates.DefaultBranch); err != nil {
			return fmt.Errorf("failed to update default branch: %w", err)
		}
	}

	if updates.Team != nil {
		if err := project.UpdateTeam(*updates.Team); err != nil {
			return fmt.Errorf("failed to update team: %w", err)
		}
	}

	// Update settings
	if updates.Settings != nil {
		for key, value := range updates.Settings {
			project.SetSetting(key, value)
		}
	}

	// Save the updates
	if err := s.projectRepo.Update(ctx, project); err != nil {
		return fmt.Errorf("failed to save project updates: %w", err)
	}

	return nil
}

// DeactivateProject deactivates a project
func (s *ProjectService) DeactivateProject(ctx context.Context, projectID domain.ProjectID) error {
	project, err := s.projectRepo.FindByProjectID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	project.Deactivate()

	if err := s.projectRepo.Update(ctx, project); err != nil {
		return fmt.Errorf("failed to deactivate project: %w", err)
	}

	return nil
}

// ActivateProject activates a project
func (s *ProjectService) ActivateProject(ctx context.Context, projectID domain.ProjectID) error {
	project, err := s.projectRepo.FindByProjectID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	project.Activate()

	if err := s.projectRepo.Update(ctx, project); err != nil {
		return fmt.Errorf("failed to activate project: %w", err)
	}

	return nil
}

// DeleteProject deletes a project
func (s *ProjectService) DeleteProject(ctx context.Context, projectID domain.ProjectID) error {
	project, err := s.projectRepo.FindByProjectID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	if err := s.projectRepo.Delete(ctx, project.ID()); err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	return nil
}

// ListProjects lists projects with pagination
func (s *ProjectService) ListProjects(ctx context.Context, limit, offset int) ([]*domain.Project, int64, error) {
	return s.projectRepo.FindAll(ctx, limit, offset)
}

// ListTeamProjects lists all projects for a team
func (s *ProjectService) ListTeamProjects(ctx context.Context, team domain.Team) ([]*domain.Project, error) {
	return s.projectRepo.FindByTeam(ctx, team)
}

// GrantPermission grants a permission to a user for a project
func (s *ProjectService) GrantPermission(ctx context.Context, projectID domain.ProjectID, userID string, permissionType domain.PermissionType, grantedBy string) error {
	// Check if project exists
	_, err := s.projectRepo.FindByProjectID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w", err)
	}

	// Create permission
	permission, err := domain.NewProjectPermission(projectID, userID, permissionType, grantedBy)
	if err != nil {
		return fmt.Errorf("failed to create permission: %w", err)
	}

	// Save permission
	if err := s.permissionRepo.Save(ctx, permission); err != nil {
		return fmt.Errorf("failed to grant permission: %w", err)
	}

	return nil
}

// RevokePermission revokes a permission from a user for a project
func (s *ProjectService) RevokePermission(ctx context.Context, projectID domain.ProjectID, userID string, permissionType domain.PermissionType) error {
	if err := s.permissionRepo.Delete(ctx, projectID, userID, permissionType); err != nil {
		return fmt.Errorf("failed to revoke permission: %w", err)
	}
	return nil
}

// GetUserPermissions gets all permissions for a user on a project
func (s *ProjectService) GetUserPermissions(ctx context.Context, projectID domain.ProjectID, userID string) ([]*domain.ProjectPermission, error) {
	return s.permissionRepo.FindByProjectAndUser(ctx, projectID, userID)
}

// GetOrCreateProject gets an existing project or creates a new one
func (s *ProjectService) GetOrCreateProject(ctx context.Context, projectID domain.ProjectID, name string, team domain.Team, creatorUserID string) (*domain.Project, error) {
	// Try to get existing project
	project, err := s.projectRepo.FindByProjectID(ctx, projectID)
	if err == nil {
		return project, nil
	}

	// Create new project if not found
	return s.CreateProject(ctx, projectID, name, team, creatorUserID)
}

// UpdateProjectRequest contains fields that can be updated
type UpdateProjectRequest struct {
	Name          *string
	Description   *string
	Repository    *string
	DefaultBranch *string
	Team          *domain.Team
	Settings      map[string]interface{}
}
