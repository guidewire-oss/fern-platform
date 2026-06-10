package domain

// CoverageCount holds aggregated test-run pass/fail counts for a single JIRA issue key.
type CoverageCount struct {
	Total  int
	Passed int
	Failed int
}
