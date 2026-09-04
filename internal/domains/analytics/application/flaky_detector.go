package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
)

// FlakyDetectionService handles flaky test detection and analysis
type FlakyDetectionService struct {
	repo   domain.FlakyDetectionRepository
	config domain.FlakyTestDetectionConfig
}

// NewFlakyDetectionService creates a new flaky detection service
func NewFlakyDetectionService(repo domain.FlakyDetectionRepository, config domain.FlakyTestDetectionConfig) *FlakyDetectionService {
	return &FlakyDetectionService{
		repo:   repo,
		config: config,
	}
}

// AnalyzeTestRun analyzes a test run for flaky tests
func (s *FlakyDetectionService) AnalyzeTestRun(ctx context.Context, projectID string, testRunID string) (*domain.TestRunAnalysis, error) {
	analysis := &domain.TestRunAnalysis{
		TestRunID:  testRunID,
		ProjectID:  projectID,
		AnalyzedAt: time.Now(),
	}

	// Get every (spec, suite) pair from recent history
	since := time.Now().Add(-s.config.AnalysisWindow)
	tests, err := s.repo.GetUniqueTests(ctx, projectID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get test names: %w", err)
	}

	analysis.TotalTests = len(tests)

	// Analyze each test
	for _, test := range tests {
		result, err := s.analyzeTest(ctx, projectID, test.TestName, test.SuiteName, since)
		if err != nil {
			// Log error but continue with other tests
			continue
		}

		switch result.action {
		case actionNewFlaky:
			analysis.NewFlaky = append(analysis.NewFlaky, result.testID)
		case actionStillFlaky:
			analysis.StillFlaky = append(analysis.StillFlaky, result.testID)
		case actionResolved:
			analysis.ResolvedFlaky = append(analysis.ResolvedFlaky, result.testID)
		}
	}

	// Save the analysis
	if err := s.repo.SaveTestRunAnalysis(ctx, analysis); err != nil {
		return nil, fmt.Errorf("failed to save analysis: %w", err)
	}

	return analysis, nil
}

// GetFlakyTests returns all active flaky tests for a project
func (s *FlakyDetectionService) GetFlakyTests(ctx context.Context, projectID string) ([]*domain.FlakyTest, error) {
	return s.repo.FindFlakyTestsByProject(ctx, projectID, domain.StatusActive)
}

// MarkTestResolved marks a flaky test as resolved, by row ID.
func (s *FlakyDetectionService) MarkTestResolved(ctx context.Context, id uint) error {
	return s.repo.UpdateFlakyTestStatus(ctx, id, domain.StatusResolved)
}

// IgnoreTest marks a flaky test as ignored, by row ID.
func (s *FlakyDetectionService) IgnoreTest(ctx context.Context, id uint) error {
	return s.repo.UpdateFlakyTestStatus(ctx, id, domain.StatusIgnored)
}

// Internal types and methods

type analysisAction string

const (
	actionNone       analysisAction = "none"
	actionNewFlaky   analysisAction = "new_flaky"
	actionStillFlaky analysisAction = "still_flaky"
	actionResolved   analysisAction = "resolved"
)

type testAnalysisResult struct {
	testID string
	action analysisAction
}

func (s *FlakyDetectionService) analyzeTest(ctx context.Context, projectID string, testName string, suiteName string, since time.Time) (*testAnalysisResult, error) {
	// Get test execution history
	history, err := s.repo.GetTestRunHistory(ctx, projectID, testName, suiteName, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get test history: %w", err)
	}

	// Not enough runs to determine flakiness
	if len(history) < s.config.MinimumRuns {
		return &testAnalysisResult{action: actionNone}, nil
	}

	// Calculate failure rate
	failureCount := 0
	consecutivePasses := 0
	var lastFailure *domain.TestFailureInfo

	for _, exec := range history {
		if exec.Status == "failed" {
			failureCount++
			consecutivePasses = 0

			if lastFailure == nil || exec.ExecutedAt.After(lastFailure.FailedAt) {
				lastFailure = &domain.TestFailureInfo{
					TestRunID:    exec.TestRunID,
					FailedAt:     exec.ExecutedAt,
					ErrorMessage: exec.Error,
					Duration:     exec.Duration,
					Environment:  fmt.Sprintf("%v", exec.Environment),
				}
			}
		} else if exec.Status == "passed" {
			consecutivePasses++
		}
	}

	failureRate := float64(failureCount) / float64(len(history))
	testID := domain.BuildTestID(projectID, suiteName, testName)

	// Only a clean not-found means absent; anything else is a real failure.
	existingFlaky, err := s.repo.GetFlakyTestByName(ctx, projectID, testName, suiteName)
	if err != nil && !errors.Is(err, domain.ErrFlakyTestNotFound) {
		return nil, fmt.Errorf("failed to get existing flaky test: %w", err)
	}

	// Determine action based on failure rate and existing status
	if failureRate >= s.config.MinFailureRate && failureRate <= s.config.MaxFailureRate {
		// Test is flaky
		flakeScore := s.calculateFlakeScore(failureRate, len(history), consecutivePasses)

		if existingFlaky == nil {
			// New flaky test
			flaky := &domain.FlakyTest{
				TestID:       testID,
				ProjectID:    projectID,
				TestName:     testName,
				SuiteName:    suiteName,
				PackageName:  "", // no package column to read back from
				FirstSeen:    time.Now(),
				LastSeen:     time.Now(),
				TotalRuns:    len(history),
				FailureCount: failureCount,
				FlakeScore:   flakeScore,
				Status:       domain.StatusActive,
				Metadata: domain.FlakyTestMetadata{
					RecentFailures: []domain.TestFailureInfo{},
				},
			}

			if lastFailure != nil {
				flaky.Metadata.RecentFailures = append(flaky.Metadata.RecentFailures, *lastFailure)
			}

			if err := s.repo.SaveFlakyTest(ctx, flaky); err != nil {
				return nil, fmt.Errorf("failed to save new flaky test: %w", err)
			}

			return &testAnalysisResult{testID: testID, action: actionNewFlaky}, nil
		} else {
			// Update existing flaky test
			existingFlaky.LastSeen = time.Now()
			existingFlaky.TotalRuns = len(history)
			existingFlaky.FailureCount = failureCount
			existingFlaky.FlakeScore = flakeScore
			existingFlaky.Status = domain.StatusActive

			if lastFailure != nil {
				// Add to recent failures, keep only last 10
				existingFlaky.Metadata.RecentFailures = append([]domain.TestFailureInfo{*lastFailure}, existingFlaky.Metadata.RecentFailures...)
				if len(existingFlaky.Metadata.RecentFailures) > 10 {
					existingFlaky.Metadata.RecentFailures = existingFlaky.Metadata.RecentFailures[:10]
				}
			}

			if err := s.repo.SaveFlakyTest(ctx, existingFlaky); err != nil {
				return nil, fmt.Errorf("failed to update flaky test: %w", err)
			}

			return &testAnalysisResult{testID: testID, action: actionStillFlaky}, nil
		}
	} else if existingFlaky != nil && existingFlaky.Status == domain.StatusActive {
		// Test is no longer flaky
		if consecutivePasses >= s.config.ConsecutivePassesForResolution {
			existingFlaky.Status = domain.StatusResolved
			if err := s.repo.SaveFlakyTest(ctx, existingFlaky); err != nil {
				return nil, fmt.Errorf("failed to update resolved test: %w", err)
			}
			return &testAnalysisResult{testID: testID, action: actionResolved}, nil
		}
	}

	return &testAnalysisResult{action: actionNone}, nil
}

func (s *FlakyDetectionService) calculateFlakeScore(failureRate float64, totalRuns int, consecutivePasses int) float64 {
	// Base score is the failure rate
	score := failureRate

	// Adjust based on total runs (more runs = more confidence)
	runConfidence := math.Min(float64(totalRuns)/100.0, 1.0)
	score = score * (0.7 + 0.3*runConfidence)

	// Adjust based on recent stability
	if consecutivePasses > 5 {
		stabilityFactor := math.Min(float64(consecutivePasses)/20.0, 0.5)
		score = score * (1.0 - stabilityFactor)
	}

	return math.Min(math.Max(score, 0.0), 1.0)
}

// GetFlakyTestTrends returns trend data for flaky tests over time
func (s *FlakyDetectionService) GetFlakyTestTrends(ctx context.Context, projectID string, period time.Duration) ([]FlakyTestTrend, error) {
	// Get all analyses for the project within the period
	// endTime := time.Now()
	// startTime := endTime.Add(-period)

	// This would be implemented by the repository
	// For now, return empty trends
	return []FlakyTestTrend{}, nil
}

// FlakyTestTrend represents flaky test counts over time
type FlakyTestTrend struct {
	Date          time.Time
	ActiveCount   int
	NewCount      int
	ResolvedCount int
}
