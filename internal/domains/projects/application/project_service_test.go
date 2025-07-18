package application_test

import (
	"context"
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockProjectRepository is a mock implementation of domain.ProjectRepository
type MockProjectRepository struct {
	mock.Mock
}

func (m *MockProjectRepository) Save(ctx context.Context, project *domain.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockProjectRepository) Update(ctx context.Context, project *domain.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockProjectRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProjectRepository) FindByID(ctx context.Context, id uint) (*domain.Project, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Project), args.Error(1)
}

func (m *MockProjectRepository) FindByProjectID(ctx context.Context, projectID domain.ProjectID) (*domain.Project, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Project), args.Error(1)
}

func (m *MockProjectRepository) FindByTeam(ctx context.Context, team domain.Team) ([]*domain.Project, error) {
	args := m.Called(ctx, team)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Project), args.Error(1)
}

func (m *MockProjectRepository) FindAll(ctx context.Context, limit, offset int) ([]*domain.Project, int64, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domain.Project), args.Get(1).(int64), args.Error(2)
}

func (m *MockProjectRepository) ExistsByProjectID(ctx context.Context, projectID domain.ProjectID) (bool, error) {
	args := m.Called(ctx, projectID)
	return args.Bool(0), args.Error(1)
}

// MockProjectPermissionRepository is a mock implementation
type MockProjectPermissionRepository struct {
	mock.Mock
}

func (m *MockProjectPermissionRepository) Save(ctx context.Context, permission *domain.ProjectPermission) error {
	args := m.Called(ctx, permission)
	return args.Error(0)
}

func (m *MockProjectPermissionRepository) Delete(ctx context.Context, projectID domain.ProjectID, userID string, permission domain.PermissionType) error {
	args := m.Called(ctx, projectID, userID, permission)
	return args.Error(0)
}

func (m *MockProjectPermissionRepository) FindByProjectAndUser(ctx context.Context, projectID domain.ProjectID, userID string) ([]*domain.ProjectPermission, error) {
	args := m.Called(ctx, projectID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ProjectPermission), args.Error(1)
}

func (m *MockProjectPermissionRepository) FindByProject(ctx context.Context, projectID domain.ProjectID) ([]*domain.ProjectPermission, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ProjectPermission), args.Error(1)
}

func (m *MockProjectPermissionRepository) FindByUser(ctx context.Context, userID string) ([]*domain.ProjectPermission, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ProjectPermission), args.Error(1)
}

func (m *MockProjectPermissionRepository) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestProjectService_DeleteProject(t *testing.T) {
	t.Run("should delete project successfully", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		mockRepo := new(MockProjectRepository)
		mockPermRepo := new(MockProjectPermissionRepository)
		service := application.NewProjectService(mockRepo, mockPermRepo)
		
		projectID := domain.ProjectID("test-project-123")
		project, _ := domain.NewProject(projectID, "Test Project", "test-team")
		project.SetID(1)
		
		// Set up expectations
		mockRepo.On("FindByProjectID", ctx, projectID).Return(project, nil)
		mockRepo.On("Delete", ctx, uint(1)).Return(nil)
		
		// Act
		err := service.DeleteProject(ctx, projectID)
		
		// Assert
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
	
	t.Run("should return error when project not found", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		mockRepo := new(MockProjectRepository)
		mockPermRepo := new(MockProjectPermissionRepository)
		service := application.NewProjectService(mockRepo, mockPermRepo)
		
		projectID := domain.ProjectID("non-existent-project")
		
		// Set up expectations
		mockRepo.On("FindByProjectID", ctx, projectID).Return(nil, assert.AnError)
		
		// Act
		err := service.DeleteProject(ctx, projectID)
		
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get project")
		mockRepo.AssertExpectations(t)
	})
	
	t.Run("should return error when delete fails", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		mockRepo := new(MockProjectRepository)
		mockPermRepo := new(MockProjectPermissionRepository)
		service := application.NewProjectService(mockRepo, mockPermRepo)
		
		projectID := domain.ProjectID("test-project-123")
		project, _ := domain.NewProject(projectID, "Test Project", "test-team")
		project.SetID(1)
		
		// Set up expectations
		mockRepo.On("FindByProjectID", ctx, projectID).Return(project, nil)
		mockRepo.On("Delete", ctx, uint(1)).Return(assert.AnError)
		
		// Act
		err := service.DeleteProject(ctx, projectID)
		
		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete project")
		mockRepo.AssertExpectations(t)
	})
}

func TestProjectService_CreateProject(t *testing.T) {
	t.Run("should create project successfully", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		mockRepo := new(MockProjectRepository)
		mockPermRepo := new(MockProjectPermissionRepository)
		service := application.NewProjectService(mockRepo, mockPermRepo)
		
		projectID := domain.ProjectID("test-project-123")
		name := "Test Project"
		team := domain.Team("fern")
		creatorUserID := "user123"
		
		// Set up expectations
		mockRepo.On("ExistsByProjectID", ctx, projectID).Return(false, nil)
		mockRepo.On("Save", ctx, mock.AnythingOfType("*domain.Project")).Return(nil)
		mockPermRepo.On("Save", ctx, mock.AnythingOfType("*domain.ProjectPermission")).Return(nil)
		
		// Act
		project, err := service.CreateProject(ctx, projectID, name, team, creatorUserID)
		
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, project)
		assert.Equal(t, name, project.Name())
		assert.Equal(t, team, project.Team())
		mockRepo.AssertExpectations(t)
		mockPermRepo.AssertExpectations(t)
	})
	
	t.Run("should return error when project already exists", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		mockRepo := new(MockProjectRepository)
		mockPermRepo := new(MockProjectPermissionRepository)
		service := application.NewProjectService(mockRepo, mockPermRepo)
		
		projectID := domain.ProjectID("test-project-123")
		name := "Test Project"
		team := domain.Team("fern")
		creatorUserID := "user123"
		
		// Set up expectations
		mockRepo.On("ExistsByProjectID", ctx, projectID).Return(true, nil)
		
		// Act
		project, err := service.CreateProject(ctx, projectID, name, team, creatorUserID)
		
		// Assert
		assert.Error(t, err)
		assert.Nil(t, project)
		assert.Contains(t, err.Error(), "already exists")
		mockRepo.AssertExpectations(t)
	})
}