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
