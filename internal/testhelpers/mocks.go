// Package testhelpers provides mock implementations for testing.
// This file contains test doubles and should be excluded from coverage reports.
package testhelpers

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	testingDomain "github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	tagsDomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
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

func (m *MockProjectRepository) List(ctx context.Context, limit, offset int) ([]*domain.Project, int, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Project), args.Int(1), args.Error(2)
}

// MockProjectPermissionRepository is a mock implementation of domain.ProjectPermissionRepository
type MockProjectPermissionRepository struct {
	mock.Mock
}

func (m *MockProjectPermissionRepository) Save(ctx context.Context, permission *domain.ProjectPermission) error {
	args := m.Called(ctx, permission)
	return args.Error(0)
}

func (m *MockProjectPermissionRepository) FindByProjectAndUser(ctx context.Context, projectID domain.ProjectID, userID string) ([]*domain.ProjectPermission, error) {
	args := m.Called(ctx, projectID, userID)
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

func (m *MockProjectPermissionRepository) FindByProject(ctx context.Context, projectID domain.ProjectID) ([]*domain.ProjectPermission, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ProjectPermission), args.Error(1)
}

func (m *MockProjectPermissionRepository) Delete(ctx context.Context, projectID domain.ProjectID, userID string, permission domain.PermissionType) error {
	args := m.Called(ctx, projectID, userID, permission)
	return args.Error(0)
}

func (m *MockProjectPermissionRepository) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockUserRepository is a mock implementation of authDomain.UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Save(ctx context.Context, user *authDomain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Update(ctx context.Context, user *authDomain.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id uint) (*authDomain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authDomain.User), args.Error(1)
}

func (m *MockUserRepository) FindByUserID(ctx context.Context, userID string) (*authDomain.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authDomain.User), args.Error(1)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*authDomain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authDomain.User), args.Error(1)
}

func (m *MockUserRepository) FindAll(ctx context.Context, limit, offset int) ([]*authDomain.User, int64, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*authDomain.User), args.Get(1).(int64), args.Error(2)
}

// MockTestRunRepository is a mock implementation of testingDomain.TestRunRepository
type MockTestRunRepository struct {
	mock.Mock
}

func (m *MockTestRunRepository) Save(ctx context.Context, testRun *testingDomain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
}

func (m *MockTestRunRepository) Update(ctx context.Context, testRun *testingDomain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
}

func (m *MockTestRunRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTestRunRepository) FindByID(ctx context.Context, id uint) (*testingDomain.TestRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*testingDomain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetByID(ctx context.Context, id uint) (*testingDomain.TestRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*testingDomain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) FindByTestRunID(ctx context.Context, testRunID string) (*testingDomain.TestRun, error) {
	args := m.Called(ctx, testRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*testingDomain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) FindByProject(ctx context.Context, projectID string, limit, offset int) ([]*testingDomain.TestRun, int64, error) {
	args := m.Called(ctx, projectID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*testingDomain.TestRun), args.Get(1).(int64), args.Error(2)
}

func (m *MockTestRunRepository) FindAll(ctx context.Context, limit, offset int) ([]*testingDomain.TestRun, int64, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*testingDomain.TestRun), args.Get(1).(int64), args.Error(2)
}

func (m *MockTestRunRepository) GetRecent(ctx context.Context, limit int) ([]*testingDomain.TestRun, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*testingDomain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetLatestByProjectID(ctx context.Context, projectID string, limit int) ([]*testingDomain.TestRun, error) {
	args := m.Called(ctx, projectID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*testingDomain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) CountByProjectID(ctx context.Context, projectID string) (int64, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTestRunRepository) GetWithDetails(ctx context.Context, id uint) (*testingDomain.TestRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*testingDomain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetByRunID(ctx context.Context, runID string) (*testingDomain.TestRun, error) {
	args := m.Called(ctx, runID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*testingDomain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetTestRunSummary(ctx context.Context, projectID string) (*testingDomain.TestRunSummary, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*testingDomain.TestRunSummary), args.Error(1)
}

func (m *MockTestRunRepository) Create(ctx context.Context, testRun *testingDomain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
}

// MockTagRepository is a mock implementation of tagsDomain.TagRepository
type MockTagRepository struct {
	mock.Mock
}

func (m *MockTagRepository) Save(ctx context.Context, tag *tagsDomain.Tag) error {
	args := m.Called(ctx, tag)
	return args.Error(0)
}

func (m *MockTagRepository) Delete(ctx context.Context, id tagsDomain.TagID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTagRepository) FindByID(ctx context.Context, id tagsDomain.TagID) (*tagsDomain.Tag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tagsDomain.Tag), args.Error(1)
}

func (m *MockTagRepository) FindByKeyValue(ctx context.Context, key, value string) (*tagsDomain.Tag, error) {
	args := m.Called(ctx, key, value)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tagsDomain.Tag), args.Error(1)
}

func (m *MockTagRepository) FindAll(ctx context.Context) ([]*tagsDomain.Tag, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*tagsDomain.Tag), args.Error(1)
}

func (m *MockTagRepository) GetAll(ctx context.Context) ([]*tagsDomain.Tag, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*tagsDomain.Tag), args.Error(1)
}

func (m *MockTagRepository) FindByName(ctx context.Context, name string) (*tagsDomain.Tag, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tagsDomain.Tag), args.Error(1)
}

func (m *MockTagRepository) AssignToTestRun(ctx context.Context, testRunID string, tagIDs []tagsDomain.TagID) error {
	args := m.Called(ctx, testRunID, tagIDs)
	return args.Error(0)
}

// MockBuilder provides a fluent interface for setting up mocks
type MockBuilder struct {
	mock *mock.Mock
}

// NewMockBuilder creates a new mock builder
func NewMockBuilder(m *mock.Mock) *MockBuilder {
	return &MockBuilder{mock: m}
}

// ExpectCall sets up an expectation with a fluent interface
func (mb *MockBuilder) ExpectCall(method string, args ...interface{}) *MockCallBuilder {
	return &MockCallBuilder{
		mock:   mb.mock,
		method: method,
		args:   args,
	}
}

// MockCallBuilder builds a mock expectation
type MockCallBuilder struct {
	mock   *mock.Mock
	method string
	args   []interface{}
}

// Return sets the return values
func (mcb *MockCallBuilder) Return(returnArgs ...interface{}) *MockCallBuilder {
	call := mcb.mock.On(mcb.method, mcb.args...)
	call.Return(returnArgs...)
	return mcb
}

// Times sets how many times the method should be called
func (mcb *MockCallBuilder) Times(n int) *MockCallBuilder {
	call := mcb.mock.On(mcb.method, mcb.args...)
	call.Times(n)
	return mcb
}

// Once sets that the method should be called once
func (mcb *MockCallBuilder) Once() *MockCallBuilder {
	return mcb.Times(1)
}