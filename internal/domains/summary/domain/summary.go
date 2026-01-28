package domain

import "time"

// SummaryRequest represents a request to get test summary
type SummaryRequest struct {
	ProjectUUID string
	Seed        string
	GroupBy     []string
}

// SummaryResponse represents the aggregated test summary response
type SummaryResponse struct {
	ProjectID string                   `json:"project_id"`
	Seed      string                   `json:"seed"`
	Branch    string                   `json:"branch"`
	SHA       string                   `json:"sha,omitempty"`
	Status    string                   `json:"status"`
	Tests     int                      `json:"tests"`
	StartTime string                   `json:"start_time,omitempty"`
	EndTime   string                   `json:"end_time,omitempty"`
	Summary   []map[string]interface{} `json:"summary"`
}

// GroupedTestSummary represents test results grouped by specified criteria
type GroupedTestSummary struct {
	GroupKeys    map[string]string // The grouping keys and their values
	Total        int
	Passed       int
	Failed       int
	Skipped      int
	Pending      int
}

// TestRunData represents minimal test run data needed for summary
type TestRunData struct {
	GitBranch string
	GitSHA    string
	StartTime time.Time
	EndTime   time.Time
	SuiteRuns []SuiteRunData
}

// SuiteRunData represents minimal suite run data
type SuiteRunData struct {
	SpecRuns []SpecRunData
	Tags     []TagData
}

// SpecRunData represents minimal spec run data
type SpecRunData struct {
	Status string
	Tags   []TagData
}

// TagData represents a tag with category and value
type TagData struct {
	Category string
	Value    string
}

// StatusCounts holds counts for each status type
type StatusCounts struct {
	Total   int
	Passed  int
	Failed  int
	Skipped int
	Pending int
}
