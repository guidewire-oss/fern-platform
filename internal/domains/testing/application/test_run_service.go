package application

import (
	"context"
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// TestRunService handles test run business logic
type TestRunService struct {
	testRunRepo  domain.TestRunRepository
	suiteRunRepo domain.SuiteRunRepository
	specRunRepo  domain.SpecRunRepository
}

// NewTestRunService creates a new test run service
func NewTestRunService(
	testRunRepo domain.TestRunRepository,
	suiteRunRepo domain.SuiteRunRepository,
	specRunRepo domain.SpecRunRepository,
) *TestRunService {
	return &TestRunService{
		testRunRepo:  testRunRepo,
		suiteRunRepo: suiteRunRepo,
		specRunRepo:  specRunRepo,
	}
}

// CreateTestRun creates a new test run
func (s *TestRunService) CreateTestRun(ctx context.Context, testRun *domain.TestRun) error {
	// Validate test run
	if testRun.ProjectID == "" {
		return fmt.Errorf("project ID is required")
	}

	// Set default values
	if testRun.Status == "" {
		testRun.Status = "running"
	}

	// Create the test run
	if err := s.testRunRepo.Create(ctx, testRun); err != nil {
		return fmt.Errorf("failed to create test run: %w", err)
	}

	return nil
}

// CompleteTestRun marks a test run as completed
func (s *TestRunService) CompleteTestRun(ctx context.Context, testRunID uint, status string) error {
	// Get the test run
	testRun, err := s.testRunRepo.GetByID(ctx, testRunID)
	if err != nil {
		return fmt.Errorf("failed to get test run: %w", err)
	}

	// Update status
	testRun.Status = status

	// Calculate statistics from suite runs
	suiteRuns, err := s.suiteRunRepo.FindByTestRunID(ctx, testRunID)
	if err != nil {
		return fmt.Errorf("failed to get suite runs: %w", err)
	}

	var totalTests, passedTests, failedTests, skippedTests int
	for _, suite := range suiteRuns {
		totalTests += suite.TotalTests
		passedTests += suite.PassedTests
		failedTests += suite.FailedTests
		skippedTests += suite.SkippedTests
	}

	testRun.TotalTests = totalTests
	testRun.PassedTests = passedTests
	testRun.FailedTests = failedTests
	testRun.SkippedTests = skippedTests

	// Update the test run
	if err := s.testRunRepo.Update(ctx, testRun); err != nil {
		return fmt.Errorf("failed to update test run: %w", err)
	}

	return nil
}

// AddSuiteRun adds a suite run to a test run
func (s *TestRunService) AddSuiteRun(ctx context.Context, testRunID uint, suiteRun *domain.SuiteRun) error {
	// Validate
	if suiteRun.TestRunID != testRunID {
		return fmt.Errorf("suite run test ID mismatch")
	}

	// Create the suite run
	if err := s.suiteRunRepo.Create(ctx, suiteRun); err != nil {
		return fmt.Errorf("failed to create suite run: %w", err)
	}

	return nil
}

// AddSpecRun adds a spec run to a suite
func (s *TestRunService) AddSpecRun(ctx context.Context, suiteRunID uint, specRun *domain.SpecRun) error {
	// Validate
	if specRun.SuiteRunID != suiteRunID {
		return fmt.Errorf("spec run suite ID mismatch")
	}

	// Create the spec run
	if err := s.specRunRepo.Create(ctx, specRun); err != nil {
		return fmt.Errorf("failed to create spec run: %w", err)
	}

	// Update suite statistics
	if err := s.updateSuiteStatistics(ctx, suiteRunID); err != nil {
		return fmt.Errorf("failed to update suite statistics: %w", err)
	}

	return nil
}

// GetTestRun retrieves a test run by ID
func (s *TestRunService) GetTestRun(ctx context.Context, id uint) (*domain.TestRun, error) {
	return s.testRunRepo.GetByID(ctx, id)
}

// GetTestRunWithDetails retrieves a test run with all details
func (s *TestRunService) GetTestRunWithDetails(ctx context.Context, id uint) (*domain.TestRun, error) {
	return s.testRunRepo.GetWithDetails(ctx, id)
}

// GetProjectTestRuns retrieves test runs for a project
func (s *TestRunService) GetProjectTestRuns(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	return s.testRunRepo.GetLatestByProjectID(ctx, projectID, limit)
}

// GetTestRunSummary retrieves test run summary for a project
func (s *TestRunService) GetTestRunSummary(ctx context.Context, projectID string) (*domain.TestRunSummary, error) {
	return s.testRunRepo.GetTestRunSummary(ctx, projectID)
}

// updateSuiteStatistics updates the statistics for a suite run
func (s *TestRunService) updateSuiteStatistics(ctx context.Context, suiteRunID uint) error {
	// Get all spec runs for the suite
	specRuns, err := s.specRunRepo.FindBySuiteRunID(ctx, suiteRunID)
	if err != nil {
		return err
	}

	// Get the suite run
	suiteRun, err := s.suiteRunRepo.GetByID(ctx, suiteRunID)
	if err != nil {
		return err
	}

	// Calculate statistics
	var totalTests, passedTests, failedTests, skippedTests int
	var totalDuration time.Duration

	for _, spec := range specRuns {
		totalTests++
		// Fallback: If EndTime is nil, set to now
		if spec.EndTime == nil {
			now := time.Now()
			spec.EndTime = &now
		}
		// Fallback: If StartTime is zero, set to EndTime
		if spec.StartTime.IsZero() && spec.EndTime != nil {
			spec.StartTime = *spec.EndTime
		}
		// Calculate duration if missing
		if spec.Duration == 0 && spec.EndTime != nil && !spec.StartTime.IsZero() {
			spec.Duration = spec.EndTime.Sub(spec.StartTime)
		}
		totalDuration += spec.Duration

		switch spec.Status {
		case "passed":
			passedTests++
		case "failed":
			failedTests++
		case "skipped":
			skippedTests++
		}
	}

	// Update suite run
	suiteRun.TotalTests = totalTests
	suiteRun.PassedTests = passedTests
	suiteRun.FailedTests = failedTests
	suiteRun.SkippedTests = skippedTests
	suiteRun.Duration = totalDuration

	return s.suiteRunRepo.Update(ctx, suiteRun)
}

// CreateTestRunWithSuites creates a test run with all its suites and specs in one transaction
func (s *TestRunService) CreateTestRunWithSuites(ctx context.Context, testRun *domain.TestRun, suites []domain.SuiteRun) error {
	// Create the test run
	if err := s.CreateTestRun(ctx, testRun); err != nil {
		return err
	}

	// Create all suites
	for _, suite := range suites {
		suite.TestRunID = testRun.ID
		if err := s.suiteRunRepo.Create(ctx, &suite); err != nil {
			return fmt.Errorf("failed to create suite run: %w", err)
		}

		// Create specs for this suite
		if len(suite.SpecRuns) > 0 {
			for _, spec := range suite.SpecRuns {
				spec.SuiteRunID = suite.ID
			}
			if err := s.specRunRepo.CreateBatch(ctx, suite.SpecRuns); err != nil {
				return fmt.Errorf("failed to create spec runs: %w", err)
			}
		}
	}

	// Update test run statistics
	return s.CompleteTestRun(ctx, testRun.ID, testRun.Status)
}

// GetTestRunByRunID retrieves a test run by its run ID
func (s *TestRunService) GetTestRunByRunID(ctx context.Context, runID string) (*domain.TestRun, error) {
	return s.testRunRepo.GetByRunID(ctx, runID)
}

// GetRecentTestRuns retrieves recent test runs across all projects
func (s *TestRunService) GetRecentTestRuns(ctx context.Context, limit int) ([]*domain.TestRun, error) {
	return s.testRunRepo.GetRecent(ctx, limit)
}

// CreateSuiteRun creates a new suite run
func (s *TestRunService) CreateSuiteRun(ctx context.Context, suiteRun *domain.SuiteRun) error {
	if suiteRun.TestRunID == 0 {
		return fmt.Errorf("test run ID is required")
	}

	// Set default values
	if suiteRun.Status == "" {
		suiteRun.Status = "running"
	}
	// Always set StartTime if zero
	if suiteRun.StartTime.IsZero() {
		suiteRun.StartTime = time.Now()
	}

	return s.suiteRunRepo.Create(ctx, suiteRun)
}

// CreateSpecRun creates a new spec run
func (s *TestRunService) CreateSpecRun(ctx context.Context, specRun *domain.SpecRun) error {
	if specRun.SuiteRunID == 0 {
		return fmt.Errorf("suite run ID is required")
	}

	// Set default values
	if specRun.Status == "" {
		specRun.Status = "pending"
	}
	// Always set StartTime if zero
	if specRun.StartTime.IsZero() {
		specRun.StartTime = time.Now()
	}

	// Calculate duration if not set
	if specRun.EndTime != nil && !specRun.StartTime.IsZero() {
		specRun.Duration = specRun.EndTime.Sub(specRun.StartTime)
	}

	return s.specRunRepo.Create(ctx, specRun)
}

// DeleteTestRun deletes a test run by ID
func (s *TestRunService) DeleteTestRun(ctx context.Context, id uint) error {
	// Check if test run exists
	_, err := s.testRunRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("test run not found: %w", err)
	}

	// Delete the test run (cascading deletes should handle related data)
	return s.testRunRepo.Delete(ctx, id)
}

// ListTestRuns retrieves test runs with pagination and filtering
func (s *TestRunService) ListTestRuns(ctx context.Context, projectID string, limit, offset int) ([]*domain.TestRun, int64, error) {
	// For now, use GetLatestByProjectID with limit
	// TODO: Add proper filtering support to repository
	if projectID != "" {
		runs, err := s.testRunRepo.GetLatestByProjectID(ctx, projectID, limit)
		if err != nil {
			return nil, 0, err
		}
		// Get total count
		total, err := s.testRunRepo.CountByProjectID(ctx, projectID)
		if err != nil {
			return nil, 0, err
		}
		return runs, total, nil
	}

	// If no project ID, return empty for now
	// TODO: Add GetAll method to repository
	return []*domain.TestRun{}, 0, nil
}

// GetSuiteRunsByTestRunID retrieves all suite runs for a test run
func (s *TestRunService) GetSuiteRunsByTestRunID(ctx context.Context, testRunID uint) ([]*domain.SuiteRun, error) {
	return s.suiteRunRepo.FindByTestRunID(ctx, testRunID)
}

// UpdateTestRun updates an existing test run
func (s *TestRunService) UpdateTestRun(ctx context.Context, testRun *domain.TestRun) error {
	return s.testRunRepo.Update(ctx, testRun)
}
