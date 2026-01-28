package api

import (
	"time"

	"github.com/gin-gonic/gin"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	testingDomain "github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// Request/Response type definitions

type TestRunRequest struct {
	ID                uint64     `json:"id"`
	TestProjectName   string     `json:"test_project_name"`
	TestProjectID     string     `json:"test_project_id"`
	TestSeed          uint64     `json:"test_seed"`
	StartTime         time.Time  `json:"start_time"`
	EndTime           time.Time  `json:"end_time"`
	GitBranch         string     `json:"git_branch"`
	GitSha            string     `json:"git_sha"`
	BuildTriggerActor string     `json:"build_trigger_actor"`
	BuildUrl          string     `json:"build_url"`
	Environment       string     `json:"environment"`
	Tags              []Tag      `json:"tags"`
	SuiteRuns         []SuiteRun `json:"suite_runs"`
}

type SuiteRun struct {
	ID        uint      `json:"id"`
	TestRunID uint64    `json:"test_run_id"`
	SuiteName string    `json:"suite_name"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Tags      []Tag     `json:"tags"`
	SpecRuns  []SpecRun `json:"spec_runs"`
}

type SpecRun struct {
	ID              uint      `json:"id"`
	SuiteID         uint64    `json:"suite_id"`
	SpecDescription string    `json:"spec_description"`
	Status          string    `json:"status"`
	Message         string    `json:"message"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	Tags            []Tag     `json:"tags"`
}

type Tag struct {
	ID       uint64 `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	Value    string `json:"value,omitempty"`
}

type ProjectDetails struct {
	ID        uint64    `json:"-" gorm:"primaryKey"`
	UUID      string    `json:"uuid" gorm:"column:uuid;uniqueIndex"`
	Name      string    `json:"name"`
	TeamName  string    `json:"team_name"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// Domain to API conversion methods

// convertDomainTestRunToAPI converts a domain TestRun to API response format
func (h *DomainHandler) convertDomainTestRunToAPI(tr *testingDomain.TestRun) gin.H {
	return gin.H{
		"id":           tr.ID,
		"runId":        tr.RunID,
		"projectId":    tr.ProjectID,
		"branch":       tr.Branch,
		"commitSha":    tr.GitCommit,
		"status":       tr.Status,
		"startTime":    tr.StartTime,
		"endTime":      tr.EndTime,
		"duration":     tr.Duration.Seconds(),
		"totalTests":   tr.TotalTests,
		"passedTests":  tr.PassedTests,
		"failedTests":  tr.FailedTests,
		"skippedTests": tr.SkippedTests,
		"environment":  tr.Environment,
		"tags":         tr.Tags,
		"metadata":     tr.Metadata,
	}
}

// convertProjectToAPI converts a domain Project to API response format
func (h *DomainHandler) convertProjectToAPI(p *projectsDomain.Project) gin.H {
	snapshot := p.ToSnapshot()
	return gin.H{
		"id":            snapshot.ID,
		"projectId":     string(snapshot.ProjectID),
		"name":          snapshot.Name,
		"description":   snapshot.Description,
		"repository":    snapshot.Repository,
		"defaultBranch": snapshot.DefaultBranch,
		"team":          string(snapshot.Team),
		"isActive":      snapshot.IsActive,
		"settings":      snapshot.Settings,
		"createdAt":     snapshot.CreatedAt,
		"updatedAt":     snapshot.UpdatedAt,
	}
}

// Request to Domain conversion methods

// convertApiSuiteRunstoDomain converts request SuiteRuns to domain SuiteRuns
// Returns []testingDomain.SuiteRun (slice of values, not pointers)
func (h *DomainHandler) convertApiSuiteRunstoDomain(reqSuiteRuns []SuiteRun) []testingDomain.SuiteRun {
	domainSuiteRuns := make([]testingDomain.SuiteRun, len(reqSuiteRuns))

	for i, reqSuite := range reqSuiteRuns {
		// Convert SpecRuns (returns []*testingDomain.SpecRun)
		domainSpecRuns := h.convertSpecRuns(reqSuite.SpecRuns)

		// Calculate test counts and status
		totalTests, passedTests, failedTests, skippedTests := h.calculateTestCounts(domainSpecRuns)
		status := h.calculateSuiteStatus(domainSpecRuns)

		// Calculate duration
		var duration time.Duration
		if !reqSuite.EndTime.IsZero() && !reqSuite.StartTime.IsZero() {
			duration = reqSuite.EndTime.Sub(reqSuite.StartTime)
		}

		// Set EndTime pointer
		var endTime *time.Time
		if !reqSuite.EndTime.IsZero() {
			endTime = &reqSuite.EndTime
		}

		// Convert tags
		domainTags := h.convertApiTagsToDomain(reqSuite.Tags)

		// Create value (not pointer) for slice of values
		domainSuiteRuns[i] = testingDomain.SuiteRun{
			ID:           reqSuite.ID,
			TestRunID:    0, // will be set later in recordTestRun
			Name:         reqSuite.SuiteName,
			PackageName:  "", // Set based on your requirements
			ClassName:    "", // Set based on your requirements
			Status:       status,
			StartTime:    reqSuite.StartTime,
			EndTime:      endTime,
			TotalTests:   totalTests,
			PassedTests:  passedTests,
			FailedTests:  failedTests,
			SkippedTests: skippedTests,
			Duration:     duration,
			Tags:         domainTags,
			SpecRuns:     domainSpecRuns, // []*testingDomain.SpecRun
		}
	}

	return domainSuiteRuns // []testingDomain.SuiteRun
}

// convertSpecRuns converts request SpecRuns to domain SpecRuns
// Returns []*testingDomain.SpecRun (slice of pointers)
func (h *DomainHandler) convertSpecRuns(reqSpecRuns []SpecRun) []*testingDomain.SpecRun {
	domainSpecRuns := make([]*testingDomain.SpecRun, len(reqSpecRuns))

	for i, reqSpec := range reqSpecRuns {
		// Calculate duration
		var duration time.Duration
		if !reqSpec.EndTime.IsZero() && !reqSpec.StartTime.IsZero() {
			duration = reqSpec.EndTime.Sub(reqSpec.StartTime)
		}

		// Set EndTime pointer
		var endTime *time.Time
		if !reqSpec.EndTime.IsZero() {
			endTime = &reqSpec.EndTime
		}

		// Determine error/failure message based on status
		var errorMessage, failureMessage string
		if reqSpec.Status == "failed" || reqSpec.Status == "error" {
			if reqSpec.Status == "error" {
				errorMessage = reqSpec.Message
			} else {
				failureMessage = reqSpec.Message
			}
		}

		// Convert tags
		domainTags := h.convertApiTagsToDomain(reqSpec.Tags)

		// Create pointer for slice of pointers
		domainSpecRuns[i] = &testingDomain.SpecRun{
			ID:             reqSpec.ID,
			SuiteRunID:     uint(reqSpec.SuiteID),
			Name:           reqSpec.SpecDescription,
			ClassName:      "", // Set based on your requirements
			Status:         reqSpec.Status,
			StartTime:      reqSpec.StartTime,
			EndTime:        endTime,
			Duration:       duration,
			ErrorMessage:   errorMessage,
			FailureMessage: failureMessage,
			StackTrace:     "",    // Set if available in your data
			RetryCount:     0,     // Set based on your requirements
			IsFlaky:        false, // Set based on your requirements
			Tags:           domainTags,
		}
	}

	return domainSpecRuns // []*testingDomain.SpecRun
}

// Calculation and status helper methods

// calculateOverallStatus calculates the overall test run status from suite runs
func (h *DomainHandler) calculateOverallStatus(suiteRuns []SuiteRun) string {
	for _, suite := range suiteRuns {
		for _, spec := range suite.SpecRuns {
			if spec.Status == "failed" {
				return "failed"
			}
		}
	}
	return "passed"
}

// calculateTestCounts calculates test statistics from SpecRuns
func (h *DomainHandler) calculateTestCounts(specRuns []*testingDomain.SpecRun) (total, passed, failed, skipped int) {
	total = len(specRuns)

	for _, spec := range specRuns {
		switch spec.Status {
		case "passed", "pass":
			passed++
		case "failed", "fail", "error":
			failed++
		case "skipped", "skip", "pending":
			skipped++
		}
	}

	return total, passed, failed, skipped
}

// calculateOverallTestCounts calculates total test statistics from all suite runs
func (h *DomainHandler) calculateOverallTestCounts(suiteRuns []testingDomain.SuiteRun) (total, passed, failed, skipped int) {
	for _, suite := range suiteRuns {
		total += suite.TotalTests
		passed += suite.PassedTests
		failed += suite.FailedTests
		skipped += suite.SkippedTests
	}
	return total, passed, failed, skipped
}

// calculateSuiteStatus determines suite status based on spec runs
func (h *DomainHandler) calculateSuiteStatus(specRuns []*testingDomain.SpecRun) string {
	if len(specRuns) == 0 {
		return "unknown"
	}

	hasFailures := false
	hasSkipped := false

	for _, spec := range specRuns {
		switch spec.Status {
		case "failed", "fail", "error":
			hasFailures = true
		case "skipped", "skip", "pending":
			hasSkipped = true
		}
	}

	if hasFailures {
		return "failed"
	}
	if hasSkipped {
		return "skipped"
	}
	return "passed"
}

// convertApiTagsToDomain converts API tags to domain tags
func (h *DomainHandler) convertApiTagsToDomain(apiTags []Tag) []testingDomain.Tag {
	if len(apiTags) == 0 {
		return nil
	}

	domainTags := make([]testingDomain.Tag, len(apiTags))
	for i, tag := range apiTags {
		domainTags[i] = testingDomain.Tag{
			ID:       uint(tag.ID),
			Name:     tag.Name,
			Category: tag.Category,
			Value:    tag.Value,
		}
	}
	return domainTags
}

// mergeUniqueTags merges two tag slices, removing duplicates by ID
func (h *DomainHandler) mergeUniqueTags(existingTags, newTags []testingDomain.Tag) []testingDomain.Tag {
	tagMap := make(map[uint]testingDomain.Tag)

	// Add existing tags
	for _, tag := range existingTags {
		if tag.ID != 0 {
			tagMap[tag.ID] = tag
		}
	}

	// Add new tags (will overwrite if ID exists, but that's fine - same tag)
	for _, tag := range newTags {
		if tag.ID != 0 {
			tagMap[tag.ID] = tag
		}
	}

	// Convert map to slice
	tags := make([]testingDomain.Tag, 0, len(tagMap))
	for _, tag := range tagMap {
		tags = append(tags, tag)
	}

	return tags
}
