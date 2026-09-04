package domain

import (
	"fmt"
	"time"
)

// BuildTestID renders a flaky test's display ID as
// "<projectID>:<suiteName>:<testName>". All three parts, or two rows keyed on
// (project, test, suite) would report as one. Derived only: no test_id column,
// rebuilt on every read, never parsed back.
func BuildTestID(projectID, suiteName, testName string) string {
	return fmt.Sprintf("%s:%s:%s", projectID, suiteName, testName)
}

// FlakyTest represents a test that has been identified as flaky
type FlakyTest struct {
	// ID is the row identifier resolve/ignore use; zero until saved.
	ID           uint
	TestID       string
	ProjectID    string
	TestName     string
	SuiteName    string
	PackageName  string
	FirstSeen    time.Time
	LastSeen     time.Time
	TotalRuns    int
	FailureCount int
	FlakeScore   float64 // 0.0 to 1.0, higher means more flaky
	Status       FlakyTestStatus
	Metadata     FlakyTestMetadata
}

// FlakyTestStatus represents the current status of a flaky test
type FlakyTestStatus string

const (
	StatusActive   FlakyTestStatus = "active"   // Currently flaky
	StatusResolved FlakyTestStatus = "resolved" // No longer flaky
	StatusIgnored  FlakyTestStatus = "ignored"  // Manually ignored
)

// FlakyTestMetadata contains additional information about the flaky test
type FlakyTestMetadata struct {
	FailurePatterns []string          // Common failure messages
	Environments    []string          // Environments where it fails
	RecentFailures  []TestFailureInfo // Recent failure details
	Tags            []string          // User-defined tags
}

// TestFailureInfo contains information about a specific test failure
type TestFailureInfo struct {
	TestRunID    string
	FailedAt     time.Time
	ErrorMessage string
	Duration     time.Duration
	Environment  string
}

// TestRunAnalysis represents the analysis of a single test run
type TestRunAnalysis struct {
	TestRunID     string
	ProjectID     string
	AnalyzedAt    time.Time
	TotalTests    int
	NewFlaky      []string // Test IDs newly identified as flaky
	StillFlaky    []string // Test IDs that remain flaky
	ResolvedFlaky []string // Test IDs no longer flaky
}

// FlakyTestDetectionConfig contains configuration for flaky test detection
type FlakyTestDetectionConfig struct {
	// Minimum number of runs before a test can be considered flaky
	MinimumRuns int

	// Minimum failure rate to be considered flaky (e.g., 0.1 = 10%)
	MinFailureRate float64

	// Maximum failure rate to be considered flaky (e.g., 0.9 = 90%)
	// Tests failing more than this are likely broken, not flaky
	MaxFailureRate float64

	// Time window to consider for recent test runs
	AnalysisWindow time.Duration

	// Number of consecutive passes required to mark as resolved
	ConsecutivePassesForResolution int
}

// DefaultFlakyTestDetectionConfig returns the default configuration
func DefaultFlakyTestDetectionConfig() FlakyTestDetectionConfig {
	return FlakyTestDetectionConfig{
		MinimumRuns:                    10,
		MinFailureRate:                 0.05,               // 5%
		MaxFailureRate:                 0.95,               // 95%
		AnalysisWindow:                 7 * 24 * time.Hour, // 7 days
		ConsecutivePassesForResolution: 20,
	}
}
