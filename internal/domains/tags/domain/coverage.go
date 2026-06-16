package domain

import "time"

// CoverageCount holds aggregated test-run counts for a single JIRA issue key.
type CoverageCount struct {
	Total     int
	Passed    int
	Failed    int
	Skipped   int
	LastRunAt *time.Time
}

// CoveredSpecRun holds the denormalised fields needed by the coverage drill-down.
type CoveredSpecRun struct {
	SpecName  string
	Status    string
	SuiteName string
	TestRunID string
	Branch    string
	StartTime time.Time
	Duration  int64
}
