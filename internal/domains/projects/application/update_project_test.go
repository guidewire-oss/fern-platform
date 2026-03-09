package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func stringPtr(s string) *string { return &s }

// helper to create a project and set up write permission expectations
func setupUpdateTest(t *testing.T, ctx context.Context, projectID uint) (*MockProjectRepository, *MockProjectPermissionRepository, *application.UpdateProjectHandler, *domain.Project) {
	t.Helper()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewUpdateProjectHandler(mockRepo, mockPermRepo)

	project, _ := domain.NewProject("proj-1", "Original Name", "team-a")
	project.SetID(projectID)

	// Set up permission check: user has write permission
	writePerm, _ := domain.NewProjectPermission("proj-1", "editor-1", domain.PermissionWrite, "admin-1")
	mockPermRepo.On("FindByProjectAndUser", ctx, domain.ProjectID("proj-1"), "editor-1").
		Return([]*domain.ProjectPermission{writePerm}, nil)

	return mockRepo, mockPermRepo, handler, project
}

// ===== UpdateProjectHandler tests =====

func TestUpdateProjectHandler_Handle_Success_UpdateName(t *testing.T) {
	ctx := context.Background()
	mockRepo, _, handler, project := setupUpdateTest(t, ctx, 1)

	mockRepo.On("FindByID", ctx, uint(1)).Return(project, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.Project")).Return(nil)

	cmd := application.UpdateProjectCommand{
		ID:        1,
		Name:      stringPtr("Updated Name"),
		UpdatedBy: "editor-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Equal(t, "Updated Name", snapshot.Name)
	mockRepo.AssertExpectations(t)
}

func TestUpdateProjectHandler_Handle_Success_MultipleFields(t *testing.T) {
	ctx := context.Background()
	mockRepo, _, handler, project := setupUpdateTest(t, ctx, 1)

	mockRepo.On("FindByID", ctx, uint(1)).Return(project, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.Project")).Return(nil)

	cmd := application.UpdateProjectCommand{
		ID:            1,
		Name:          stringPtr("New Name"),
		Description:   stringPtr("New description"),
		Repository:    stringPtr("https://github.com/new/repo"),
		DefaultBranch: stringPtr("develop"),
		Team:          stringPtr("team-b"),
		Settings:      map[string]interface{}{"key1": "val1"},
		UpdatedBy:     "editor-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	require.NoError(t, err)
	assert.Equal(t, "New Name", snapshot.Name)
	assert.Equal(t, "New description", snapshot.Description)
	assert.Equal(t, "https://github.com/new/repo", snapshot.Repository)
	assert.Equal(t, "develop", snapshot.DefaultBranch)
	assert.Equal(t, domain.Team("team-b"), snapshot.Team)
	assert.Equal(t, "val1", snapshot.Settings["key1"])
	mockRepo.AssertExpectations(t)
}

func TestUpdateProjectHandler_Handle_MissingID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewUpdateProjectHandler(mockRepo, mockPermRepo)

	cmd := application.UpdateProjectCommand{
		ID:        0,
		UpdatedBy: "editor-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "project ID is required")
}

func TestUpdateProjectHandler_Handle_MissingUpdatedBy(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewUpdateProjectHandler(mockRepo, mockPermRepo)

	cmd := application.UpdateProjectCommand{
		ID: 1,
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "updated by is required")
}

func TestUpdateProjectHandler_Handle_ProjectNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewUpdateProjectHandler(mockRepo, mockPermRepo)

	mockRepo.On("FindByID", ctx, uint(999)).Return(nil, errors.New("not found"))

	cmd := application.UpdateProjectCommand{
		ID:        999,
		UpdatedBy: "editor-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "failed to find project")
	mockRepo.AssertExpectations(t)
}

func TestUpdateProjectHandler_Handle_ProjectNil(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewUpdateProjectHandler(mockRepo, mockPermRepo)

	mockRepo.On("FindByID", ctx, uint(1)).Return(nil, nil)

	cmd := application.UpdateProjectCommand{
		ID:        1,
		UpdatedBy: "editor-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "project not found")
	mockRepo.AssertExpectations(t)
}

func TestUpdateProjectHandler_Handle_InsufficientPermissions(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewUpdateProjectHandler(mockRepo, mockPermRepo)

	project, _ := domain.NewProject("proj-1", "Original Name", "team-a")
	project.SetID(1)

	mockRepo.On("FindByID", ctx, uint(1)).Return(project, nil)
	// User only has read permission
	readPerm, _ := domain.NewProjectPermission("proj-1", "reader-1", domain.PermissionRead, "admin-1")
	mockPermRepo.On("FindByProjectAndUser", ctx, domain.ProjectID("proj-1"), "reader-1").
		Return([]*domain.ProjectPermission{readPerm}, nil)

	cmd := application.UpdateProjectCommand{
		ID:        1,
		Name:      stringPtr("New Name"),
		UpdatedBy: "reader-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "insufficient permissions")
	mockRepo.AssertExpectations(t)
	mockPermRepo.AssertExpectations(t)
}

func TestUpdateProjectHandler_Handle_NoPermissions(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewUpdateProjectHandler(mockRepo, mockPermRepo)

	project, _ := domain.NewProject("proj-1", "Original Name", "team-a")
	project.SetID(1)

	mockRepo.On("FindByID", ctx, uint(1)).Return(project, nil)
	mockPermRepo.On("FindByProjectAndUser", ctx, domain.ProjectID("proj-1"), "nobody").
		Return([]*domain.ProjectPermission{}, nil)

	cmd := application.UpdateProjectCommand{
		ID:        1,
		Name:      stringPtr("New Name"),
		UpdatedBy: "nobody",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestUpdateProjectHandler_Handle_PermissionCheckError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewUpdateProjectHandler(mockRepo, mockPermRepo)

	project, _ := domain.NewProject("proj-1", "Original Name", "team-a")
	project.SetID(1)

	mockRepo.On("FindByID", ctx, uint(1)).Return(project, nil)
	mockPermRepo.On("FindByProjectAndUser", ctx, domain.ProjectID("proj-1"), "editor-1").
		Return(nil, errors.New("db error"))

	cmd := application.UpdateProjectCommand{
		ID:        1,
		Name:      stringPtr("New Name"),
		UpdatedBy: "editor-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "failed to check permissions")
}

func TestUpdateProjectHandler_Handle_InvalidName(t *testing.T) {
	ctx := context.Background()
	mockRepo, _, handler, project := setupUpdateTest(t, ctx, 1)

	mockRepo.On("FindByID", ctx, uint(1)).Return(project, nil)

	cmd := application.UpdateProjectCommand{
		ID:        1,
		Name:      stringPtr(""), // empty name
		UpdatedBy: "editor-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "failed to update name")
}

func TestUpdateProjectHandler_Handle_InvalidDefaultBranch(t *testing.T) {
	ctx := context.Background()
	mockRepo, _, handler, project := setupUpdateTest(t, ctx, 1)

	mockRepo.On("FindByID", ctx, uint(1)).Return(project, nil)

	cmd := application.UpdateProjectCommand{
		ID:            1,
		DefaultBranch: stringPtr(""), // empty branch
		UpdatedBy:     "editor-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "failed to update default branch")
}

func TestUpdateProjectHandler_Handle_InvalidTeam(t *testing.T) {
	ctx := context.Background()
	mockRepo, _, handler, project := setupUpdateTest(t, ctx, 1)

	mockRepo.On("FindByID", ctx, uint(1)).Return(project, nil)

	cmd := application.UpdateProjectCommand{
		ID:        1,
		Team:      stringPtr(""), // empty team
		UpdatedBy: "editor-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "failed to update team")
}

func TestUpdateProjectHandler_Handle_SaveError(t *testing.T) {
	ctx := context.Background()
	mockRepo, _, handler, project := setupUpdateTest(t, ctx, 1)

	mockRepo.On("FindByID", ctx, uint(1)).Return(project, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.Project")).Return(errors.New("save failed"))

	cmd := application.UpdateProjectCommand{
		ID:        1,
		Name:      stringPtr("New Name"),
		UpdatedBy: "editor-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "failed to update project")
	mockRepo.AssertExpectations(t)
}
