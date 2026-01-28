package application

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/summary/domain"
)

// SummaryService handles test summary business logic
type SummaryService struct {
	repo domain.Repository
}

// NewSummaryService creates a new summary service
func NewSummaryService(repo domain.Repository) *SummaryService {
	return &SummaryService{repo: repo}
}

// GetSummary retrieves and aggregates test summary data
func (s *SummaryService) GetSummary(req domain.SummaryRequest) (*domain.SummaryResponse, error) {
	// Fetch test runs from repository
	testRuns, err := s.repo.GetTestRunsBySeed(req.ProjectUUID, req.Seed)
	if err != nil {
		return nil, err
	}

	// If no test runs found, return empty response
	if len(testRuns) == 0 {
		return &domain.SummaryResponse{
			ProjectID: req.ProjectUUID,
			Seed:      req.Seed,
			Branch:    "NA",
			Status:    "NA",
			Tests:     0,
			Summary:   []map[string]interface{}{},
		}, nil
	}

	// Aggregate the test data
	return s.aggregateSummary(testRuns, req.ProjectUUID, req.Seed, req.GroupBy), nil
}

// aggregateSummary processes test runs and creates aggregated summary
func (s *SummaryService) aggregateSummary(testRuns []domain.TestRunData, projectUUID, seed string, groupBy []string) *domain.SummaryResponse {
	groupCounts := make(map[string]map[string]int)
	groupKeyMap := make(map[string]map[string]string)

	totalTests := 0
	statusCounts := map[string]int{
		"passed":  0,
		"failed":  0,
		"skipped": 0,
		"pending": 0,
	}

	// Iterate through all test runs and aggregate data
	for _, testRun := range testRuns {
		for _, suite := range testRun.SuiteRuns {
			for _, spec := range suite.SpecRuns {
				totalTests++

				// Build tag map for this spec
				tagMap := make(map[string]string)
				for _, tag := range spec.Tags {
					tagMap[tag.Category] = tag.Value
				}

				// Compose dynamic key based on groupBy criteria
				keyParts := []string{}
				keyKV := make(map[string]string)
				for _, key := range groupBy {
					value := tagMap[key]
					if value == "" {
						value = "unspecified"
					}
					keyParts = append(keyParts, value)
					keyKV[key] = value
				}
				compositeKey := strings.Join(keyParts, "|")

				// Initialize group counts if not exists
				if _, ok := groupCounts[compositeKey]; !ok {
					groupCounts[compositeKey] = map[string]int{
						"total":   0,
						"passed":  0,
						"failed":  0,
						"skipped": 0,
						"pending": 0,
					}
					groupKeyMap[compositeKey] = keyKV
				}

				// Update counts
				status := spec.Status
				statusCounts[status]++
				groupCounts[compositeKey]["total"]++
				groupCounts[compositeKey][status]++
			}
		}
	}

	// Determine overall status
	overallStatus := "passed"
	if statusCounts["failed"] > 0 {
		overallStatus = "failed"
	}

	// Build summary response
	summary := []map[string]interface{}{}
	for key, counts := range groupCounts {
		entry := make(map[string]interface{})

		// Add grouping keys
		for _, tag := range groupBy {
			entry[tag] = groupKeyMap[key][tag]
		}

		// Add counts (only non-zero values)
		for k, v := range counts {
			if v > 0 {
				entry[k] = v
			}
		}
		summary = append(summary, entry)
	}

	// Sort summary by groupBy keys to ensure consistent output order
	sort.Slice(summary, func(i, j int) bool {
		a, b := summary[i], summary[j]
		for _, key := range groupBy {
			va, oka := a[key].(string)
			vb, okb := b[key].(string)
			if !oka || !okb {
				continue
			}
			if va != vb {
				return va < vb
			}
		}
		// As a final tie-breaker, sort by JSON encoding
		sa, _ := json.Marshal(a)
		sb, _ := json.Marshal(b)
		return string(sa) < string(sb)
	})

	// Build the response
	response := &domain.SummaryResponse{
		ProjectID: projectUUID,
		Seed:      seed,
		Branch:    testRuns[0].GitBranch,
		SHA:       testRuns[0].GitSHA,
		Status:    overallStatus,
		Tests:     totalTests,
		Summary:   summary,
	}

	// Add time information if available
	if !testRuns[0].StartTime.IsZero() {
		response.StartTime = testRuns[0].StartTime.Format(time.RFC3339)
	}
	if !testRuns[len(testRuns)-1].EndTime.IsZero() {
		response.EndTime = testRuns[len(testRuns)-1].EndTime.Format(time.RFC3339)
	}

	return response
}
