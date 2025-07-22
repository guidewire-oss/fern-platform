package testhelpers

import (
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	testingDomain "github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// FixtureBuilder provides methods for creating test fixtures
type FixtureBuilder struct {
	idCounter uint
}

// NewFixtureBuilder creates a new fixture builder
func NewFixtureBuilder() *FixtureBuilder {
	return &FixtureBuilder{idCounter: 1}
}

// nextID returns the next available ID
func (fb *FixtureBuilder) nextID() uint {
	id := fb.idCounter
	fb.idCounter++
	return id
}

// Project creates a test project with sensible defaults
func (fb *FixtureBuilder) Project(opts ...ProjectOption) *domain.Project {
	config := &projectConfig{
		projectID: domain.ProjectID("test-project-" + time.Now().Format("20060102150405")),
		name:      "Test Project",
		team:      domain.Team("fern"),
	}

	for _, opt := range opts {
		opt(config)
	}

	project, err := domain.NewProject(config.projectID, config.name, config.team)
	if err != nil {
		// Panic is appropriate here as this is a test fixture builder
		// and a failure indicates a programming error in the test setup
		panic(fmt.Sprintf("failed to create test project fixture: %v", err))
	}
	project.SetID(fb.nextID())
	return project
}

// ProjectOption configures a project fixture
type ProjectOption func(*projectConfig)

type projectConfig struct {
	projectID domain.ProjectID
	name      string
	team      domain.Team
}

// WithProjectID sets the project ID
func WithProjectID(id string) ProjectOption {
	return func(c *projectConfig) {
		c.projectID = domain.ProjectID(id)
	}
}

// WithProjectName sets the project name
func WithProjectName(name string) ProjectOption {
	return func(c *projectConfig) {
		c.name = name
	}
}

// WithTeam sets the project team
func WithTeam(team string) ProjectOption {
	return func(c *projectConfig) {
		c.team = domain.Team(team)
	}
}

// User creates a test user with sensible defaults
func (fb *FixtureBuilder) User(opts ...UserOption) *authDomain.User {
	config := &userConfig{
		userID: "user-" + time.Now().Format("20060102150405"),
		email:  "test@example.com",
		name:   "Test User",
		role:   authDomain.RoleUser,
		status: authDomain.StatusActive,
	}

	for _, opt := range opts {
		opt(config)
	}

	user := &authDomain.User{
		UserID:        config.userID,
		Email:         config.email,
		Name:          config.name,
		Role:          config.role,
		Status:        config.status,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	return user
}

// UserOption configures a user fixture
type UserOption func(*userConfig)

type userConfig struct {
	userID string
	email  string
	name   string
	role   authDomain.UserRole
	status authDomain.UserStatus
}

// WithUserID sets the user ID
func WithUserID(id string) UserOption {
	return func(c *userConfig) {
		c.userID = id
	}
}

// WithEmail sets the user email
func WithEmail(email string) UserOption {
	return func(c *userConfig) {
		c.email = email
	}
}

// TestRun creates a test run with sensible defaults
func (fb *FixtureBuilder) TestRun(projectID domain.ProjectID, opts ...TestRunOption) *testingDomain.TestRun {
	config := &testRunConfig{
		testRunID: "test-run-" + time.Now().Format("20060102150405"),
		branch:    "main",
		sha:       "abc123def456",
		status:    "completed",
		startTime: time.Now().Add(-10 * time.Minute),
		endTime:   time.Now(),
	}

	for _, opt := range opts {
		opt(config)
	}

	endTime := config.endTime
	testRun := &testingDomain.TestRun{
		ID:        fb.nextID(),
		RunID:     config.testRunID,
		ProjectID: string(projectID),
		Branch:    config.branch,
		GitCommit: config.sha,
		Status:    config.status,
		StartTime: config.startTime,
		EndTime:   &endTime,
	}

	return testRun
}

// TestRunOption configures a test run fixture
type TestRunOption func(*testRunConfig)

type testRunConfig struct {
	testRunID string
	branch    string
	sha       string
	status    string
	startTime time.Time
	endTime   time.Time
}

// WithTestRunID sets the test run ID
func WithTestRunID(id string) TestRunOption {
	return func(c *testRunConfig) {
		c.testRunID = id
	}
}

// WithBranch sets the branch
func WithBranch(branch string) TestRunOption {
	return func(c *testRunConfig) {
		c.branch = branch
	}
}

// WithStatus sets the test run status
func WithStatus(status string) TestRunOption {
	return func(c *testRunConfig) {
		c.status = status
	}
}

// SuiteRun creates a suite run with sensible defaults
func (fb *FixtureBuilder) SuiteRun(testRunID uint, opts ...SuiteRunOption) *testingDomain.SuiteRun {
	config := &suiteRunConfig{
		suiteName:    "Test Suite",
		status:       "passed",
		duration:     5 * time.Second,
		passedTests:  10,
		failedTests:  0,
		skippedTests: 0,
	}

	for _, opt := range opts {
		opt(config)
	}

	return &testingDomain.SuiteRun{
		ID:           fb.nextID(),
		TestRunID:    testRunID,
		Name:         config.suiteName,
		Status:       config.status,
		Duration:     config.duration,
		PassedTests:  config.passedTests,
		FailedTests:  config.failedTests,
		SkippedTests: config.skippedTests,
	}
}

// SuiteRunOption configures a suite run fixture
type SuiteRunOption func(*suiteRunConfig)

type suiteRunConfig struct {
	suiteName    string
	status       string
	duration     time.Duration
	passedTests  int
	failedTests  int
	skippedTests int
}

// WithSuiteName sets the suite name
func WithSuiteName(name string) SuiteRunOption {
	return func(c *suiteRunConfig) {
		c.suiteName = name
	}
}


// WithSuiteStatus sets the suite status
func WithSuiteStatus(status string) SuiteRunOption {
	return func(c *suiteRunConfig) {
		c.status = status
	}
}

// WithFailures sets the suite to have failures
func WithFailures(failed int) SuiteRunOption {
	return func(c *suiteRunConfig) {
		c.failedTests = failed
		c.status = "failed"
	}
}


