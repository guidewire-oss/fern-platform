package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// CompleteTestRunCommand represents the command to complete a test run
type CompleteTestRunCommand struct {
	RunID string `json:"run_id"`
}

// CompleteTestRunHandler handles the complete test run use case
type CompleteTestRunHandler struct {
	testRunRepo domain.TestRunRepository
	flakyRepo   domain.FlakyTestRepository
}

// NewCompleteTestRunHandler creates a new handler
func NewCompleteTestRunHandler(
	testRunRepo domain.TestRunRepository,
	flakyRepo domain.FlakyTestRepository,
) *CompleteTestRunHandler {
	return &CompleteTestRunHandler{
		testRunRepo: testRunRepo,
		flakyRepo:   flakyRepo,
	}
}

// Handle executes the use case
func (h *CompleteTestRunHandler) Handle(ctx context.Context, cmd CompleteTestRunCommand) error {
	if cmd.RunID == "" {
		return errors.New("run ID is required")
	}

	// Find the test run
	testRun, err := h.testRunRepo.GetByRunID(ctx, cmd.RunID)
	if err != nil {
		return fmt.Errorf("failed to find test run: %w", err)
	}
	if testRun == nil {
		return errors.New("test run not found")
	}

	// Complete the test run
	now := time.Now()
	// Always set EndTime and Status
	testRun.EndTime = &now
	testRun.Status = "completed"

	// If StartTime is zero, set it to now (fallback)
	if testRun.StartTime.IsZero() {
		testRun.StartTime = now
	}

	// Calculate duration if both times are set
	if testRun.EndTime != nil && !testRun.StartTime.IsZero() {
		testRun.Duration = testRun.EndTime.Sub(testRun.StartTime)
	}

	// Update flaky test statistics
	if err := h.updateFlakyTests(ctx, testRun); err != nil {
		// Log error but don't fail the completion
		fmt.Printf("failed to update flaky tests: %v\n", err)
	}

	// Save the updated test run
	if err := h.testRunRepo.Update(ctx, testRun); err != nil {
		return fmt.Errorf("failed to update test run: %w", err)
	}

	return nil
}

// updateFlakyTests analyzes test results and updates flaky test statistics
func (h *CompleteTestRunHandler) updateFlakyTests(ctx context.Context, testRun *domain.TestRun) error {
	// This would analyze the test results and update flaky test tracking
	// For now, this is a placeholder for the flaky test detection logic
	// In a real implementation, this would:
	// 1. Iterate through all suite runs and spec runs
	// 2. Identify tests that have been marked as flaky
	// 3. Update or create FlakyTest records
	// 4. Calculate new flakiness metrics

	return nil
}
