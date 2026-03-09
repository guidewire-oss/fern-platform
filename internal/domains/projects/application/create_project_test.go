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

// ===== CreateProjectHandler tests =====

func TestCreateProjectHandler_Handle_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewCreateProjectHandler(mockRepo, mockPermRepo)

	mockRepo.On("ExistsByProjectID", ctx, mock.AnythingOfType("domain.ProjectID")).Return(false, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*domain.Project")).Return(nil)
	mockPermRepo.On("Save", ctx, mock.AnythingOfType("*domain.ProjectPermission")).Return(nil)

	cmd := application.CreateProjectCommand{
		Name:      "My Project",
		Team:      "engineering",
		CreatedBy: "user-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Equal(t, "My Project", snapshot.Name)
	assert.Equal(t, domain.Team("engineering"), snapshot.Team)
	assert.NotEmpty(t, snapshot.ProjectID) // auto-generated
	mockRepo.AssertExpectations(t)
	mockPermRepo.AssertExpectations(t)
}

func TestCreateProjectHandler_Handle_WithProjectID(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewCreateProjectHandler(mockRepo, mockPermRepo)

	mockRepo.On("ExistsByProjectID", ctx, domain.ProjectID("custom-id")).Return(false, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*domain.Project")).Return(nil)
	mockPermRepo.On("Save", ctx, mock.AnythingOfType("*domain.ProjectPermission")).Return(nil)

	cmd := application.CreateProjectCommand{
		ProjectID: "custom-id",
		Name:      "My Project",
		Team:      "engineering",
		CreatedBy: "user-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	require.NoError(t, err)
	assert.Equal(t, domain.ProjectID("custom-id"), snapshot.ProjectID)
	mockRepo.AssertExpectations(t)
}

func TestCreateProjectHandler_Handle_WithOptionalFields(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewCreateProjectHandler(mockRepo, mockPermRepo)

	mockRepo.On("ExistsByProjectID", ctx, mock.AnythingOfType("domain.ProjectID")).Return(false, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*domain.Project")).Return(nil)
	mockPermRepo.On("Save", ctx, mock.AnythingOfType("*domain.ProjectPermission")).Return(nil)

	cmd := application.CreateProjectCommand{
		Name:          "My Project",
		Description:   "A description",
		Repository:    "https://github.com/org/repo",
		DefaultBranch: "develop",
		Team:          "engineering",
		Settings:      map[string]interface{}{"feature_x": true},
		CreatedBy:     "user-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	require.NoError(t, err)
	assert.Equal(t, "A description", snapshot.Description)
	assert.Equal(t, "https://github.com/org/repo", snapshot.Repository)
	assert.Equal(t, "develop", snapshot.DefaultBranch)
	assert.Equal(t, true, snapshot.Settings["feature_x"])
	mockRepo.AssertExpectations(t)
}

func TestCreateProjectHandler_Handle_MissingName(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewCreateProjectHandler(mockRepo, mockPermRepo)

	cmd := application.CreateProjectCommand{
		Team:      "engineering",
		CreatedBy: "user-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "project name is required")
}

func TestCreateProjectHandler_Handle_MissingTeam(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewCreateProjectHandler(mockRepo, mockPermRepo)

	cmd := application.CreateProjectCommand{
		Name:      "My Project",
		CreatedBy: "user-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "team is required")
}

func TestCreateProjectHandler_Handle_AlreadyExists(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewCreateProjectHandler(mockRepo, mockPermRepo)

	mockRepo.On("ExistsByProjectID", ctx, domain.ProjectID("existing-id")).Return(true, nil)

	cmd := application.CreateProjectCommand{
		ProjectID: "existing-id",
		Name:      "My Project",
		Team:      "engineering",
		CreatedBy: "user-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "project already exists")
	mockRepo.AssertExpectations(t)
}

func TestCreateProjectHandler_Handle_ExistenceCheckError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewCreateProjectHandler(mockRepo, mockPermRepo)

	mockRepo.On("ExistsByProjectID", ctx, mock.AnythingOfType("domain.ProjectID")).Return(false, errors.New("db error"))

	cmd := application.CreateProjectCommand{
		Name:      "My Project",
		Team:      "engineering",
		CreatedBy: "user-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "failed to check project existence")
	mockRepo.AssertExpectations(t)
}

func TestCreateProjectHandler_Handle_SaveError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewCreateProjectHandler(mockRepo, mockPermRepo)

	mockRepo.On("ExistsByProjectID", ctx, mock.AnythingOfType("domain.ProjectID")).Return(false, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*domain.Project")).Return(errors.New("save failed"))

	cmd := application.CreateProjectCommand{
		Name:      "My Project",
		Team:      "engineering",
		CreatedBy: "user-1",
	}

	snapshot, err := handler.Handle(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "failed to save project")
	mockRepo.AssertExpectations(t)
}

func TestCreateProjectHandler_Handle_NoCreatedBy(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProjectRepository)
	mockPermRepo := new(MockProjectPermissionRepository)
	handler := application.NewCreateProjectHandler(mockRepo, mockPermRepo)

	mockRepo.On("ExistsByProjectID", ctx, mock.AnythingOfType("domain.ProjectID")).Return(false, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*domain.Project")).Return(nil)
	// permissionRepo.Save should NOT be called when CreatedBy is empty

	cmd := application.CreateProjectCommand{
		Name: "My Project",
		Team: "engineering",
		// CreatedBy is empty
	}

	snapshot, err := handler.Handle(ctx, cmd)

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	mockRepo.AssertExpectations(t)
	// Verify Save was not called on permission repo
	mockPermRepo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}
