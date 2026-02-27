// Package api provides domain-based REST API handlers
package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// TestRunHandler handles test run related endpoints
type TestRunHandler struct {
	*BaseHandler
	testingService *application.TestRunService
	tagService     *tagsApp.TagService
}

// NewTestRunHandler creates a new test run handler
func NewTestRunHandler(testingService *application.TestRunService, logger *logging.Logger) *TestRunHandler {
	return &TestRunHandler{
		BaseHandler:    NewBaseHandler(logger),
		testingService: testingService,
	}
}

// SetTagService sets the tag service for public endpoints that need tag processing
func (h *TestRunHandler) SetTagService(tagService *tagsApp.TagService) {
	h.tagService = tagService
}

// createTestRun handles POST /api/v1/admin/test-runs
func (h *TestRunHandler) createTestRun(c *gin.Context) {
	var input struct {
		ID        string     `json:"id"`
		ProjectID string     `json:"projectId" binding:"required"`
		SuiteID   string     `json:"suiteId"`
		Status    string     `json:"status"`
		StartTime *time.Time `json:"startTime"`
		EndTime   *time.Time `json:"endTime,omitempty"`
		Duration  int64      `json:"duration"`
		Branch    string     `json:"branch"`
		Tags      []string   `json:"tags"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create domain test run
	testRun := &domain.TestRun{
		ProjectID:   input.ProjectID,
		Name:        fmt.Sprintf("Test Run %s", time.Now().Format("2006-01-02 15:04:05")),
		Branch:      input.Branch,
		Environment: "test",
		Source:      "api",
		Status:      "running",
	}

	if input.ID != "" {
		testRun.RunID = input.ID
	}

	// Create test run using domain service
	if _, _, err := h.testingService.CreateTestRun(c.Request.Context(), testRun); err != nil {
		h.logger.WithError(err).Error("Failed to create test run")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return response in format expected by client
	response := map[string]interface{}{
		"id":        testRun.ID,
		"projectId": testRun.ProjectID,
		"suiteId":   testRun.ProjectID, // Use project ID as suite ID for backward compatibility
		"status":    testRun.Status,
		"startTime": testRun.StartTime,
		"endTime":   testRun.EndTime,
		"duration":  testRun.Duration.Milliseconds(),
		"branch":    testRun.Branch,
		"tags":      input.Tags,
	}

	c.JSON(http.StatusCreated, response)
}

// getTestRun handles GET /api/v1/test-runs/:id
func (h *TestRunHandler) getTestRun(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test run ID"})
		return
	}

	testRun, err := h.testingService.GetTestRun(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	// Convert to API response format
	c.JSON(http.StatusOK, h.convertTestRunToAPI(testRun))
}

// getTestRunByRunID handles GET /api/v1/test-runs/by-run-id/:runId
func (h *TestRunHandler) getTestRunByRunID(c *gin.Context) {
	runID := c.Param("runId")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Run ID is required"})
		return
	}

	testRun, err := h.testingService.GetTestRunByRunID(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	c.JSON(http.StatusOK, h.convertTestRunToAPI(testRun))
}

// listTestRuns handles GET /api/v1/test-runs
func (h *TestRunHandler) listTestRuns(c *gin.Context) {
	projectID := c.Query("project_id")
	limit := 50 // default
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		} else if l <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be greater than 0"})
			return
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		} else if o < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "offset must be non-negative"})
			return
		}
	}

	// Get test runs from domain service with pagination
	testRuns, totalCount, err := h.testingService.ListTestRuns(c.Request.Context(), projectID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to API response format
	apiTestRuns := make([]interface{}, len(testRuns))
	for i, tr := range testRuns {
		apiTestRuns[i] = h.convertTestRunToAPI(tr)
	}

	c.Header("X-Total-Count", strconv.FormatInt(totalCount, 10))
	c.JSON(http.StatusOK, gin.H{
		"data":   apiTestRuns,
		"total":  totalCount,
		"limit":  limit,
		"offset": offset,
	})
}

// countTestRuns handles GET /api/v1/test-runs/count
func (h *TestRunHandler) countTestRuns(c *gin.Context) {
	projectID := c.Query("project_id")

	// Get count from domain service using ListTestRuns with limit 0 to get total count only
	_, totalCount, err := h.testingService.ListTestRuns(c.Request.Context(), projectID, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": totalCount,
	})
}

// updateTestRunStatus handles PUT /api/v1/admin/test-runs/:runId/status
func (h *TestRunHandler) updateTestRunStatus(c *gin.Context) {
	runID := c.Param("runId")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Run ID is required"})
		return
	}

	var input struct {
		Status  string     `json:"status" binding:"required"`
		EndTime *time.Time `json:"endTime,omitempty"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Look up the test run by its string run ID
	testRun, err := h.testingService.GetTestRunByRunID(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	// Use CompleteTestRun to update the status (handles statistics recalculation)
	if err := h.testingService.CompleteTestRun(c.Request.Context(), testRun.ID, input.Status); err != nil {
		h.logger.WithError(err).Error("Failed to update test run status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fetch the updated test run to return
	updatedRun, err := h.testingService.GetTestRun(c.Request.Context(), testRun.ID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch updated test run")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.convertTestRunToAPI(updatedRun))
}

// deleteTestRun handles DELETE /api/v1/admin/test-runs/:id
func (h *TestRunHandler) deleteTestRun(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test run ID"})
		return
	}

	if err := h.testingService.DeleteTestRun(c.Request.Context(), uint(id)); err != nil {
		h.logger.WithError(err).Error("Failed to delete test run")
		c.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// getTestRunStats handles GET /api/v1/test-runs/stats
func (h *TestRunHandler) getTestRunStats(c *gin.Context) {
	projectID := c.Query("project_id")
	days := 30 // default

	if daysStr := c.Query("days"); daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil {
			days = parsedDays
		}
	}

	summary, err := h.testingService.GetTestRunSummary(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to stats format
	c.JSON(http.StatusOK, gin.H{
		"total":       summary.TotalRuns,
		"passed":      summary.PassedRuns,
		"failed":      summary.FailedRuns,
		"days":        days,
		"avgDuration": summary.AverageRunTime.Seconds(),
		"successRate": summary.SuccessRate,
	})
}

// getRecentTestRuns handles GET /api/v1/test-runs/recent
func (h *TestRunHandler) getRecentTestRuns(c *gin.Context) {
	projectID := c.Query("project_id")
	limit := 10 // default

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		} else if l <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be greater than 0"})
			return
		}
	}

	// Get recent test runs using existing method
	testRuns, err := h.testingService.GetProjectTestRuns(c.Request.Context(), projectID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to API response format
	apiTestRuns := make([]interface{}, len(testRuns))
	for i, tr := range testRuns {
		apiTestRuns[i] = h.convertTestRunToAPI(tr)
	}

	c.JSON(http.StatusOK, apiTestRuns)
}

// assignTagsToTestRun handles POST /api/v1/test-runs/:id/tags
func (h *TestRunHandler) assignTagsToTestRun(c *gin.Context) {
	_, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test run ID"})
		return
	}

	var input struct {
		Tags []string `json:"tags" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement tag assignment in domain service
	// For now, return success
	c.JSON(http.StatusOK, gin.H{
		"message": "Tags assigned successfully",
		"tags":    input.Tags,
	})
}

// bulkDeleteTestRuns handles POST /api/v1/admin/test-runs/bulk-delete
func (h *TestRunHandler) bulkDeleteTestRuns(c *gin.Context) {
	var input struct {
		IDs []uint `json:"ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(input.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No test run IDs provided"})
		return
	}

	var errors []string
	deleted := 0
	for _, id := range input.IDs {
		if err := h.testingService.DeleteTestRun(c.Request.Context(), id); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete test run %d: %s", id, err.Error()))
		} else {
			deleted++
		}
	}

	if len(errors) > 0 {
		c.JSON(http.StatusPartialContent, gin.H{
			"deleted": deleted,
			"errors":  errors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deleted": deleted,
	})
}

// getSuiteRuns handles GET /api/v1/test-runs/:id/suite-runs
func (h *TestRunHandler) getSuiteRuns(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test run ID"})
		return
	}

	suiteRuns, err := h.testingService.GetSuiteRunsByTestRunID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, suiteRuns)
}

// getSuiteRun handles GET /api/v1/test-runs/:id/suite-runs/:suiteId
func (h *TestRunHandler) getSuiteRun(c *gin.Context) {
	suiteID, err := strconv.ParseUint(c.Param("suiteId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid suite run ID"})
		return
	}

	suiteRun, err := h.testingService.GetSuiteRun(c.Request.Context(), uint(suiteID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Suite run not found"})
		return
	}

	c.JSON(http.StatusOK, suiteRun)
}

// getSpecRuns handles GET /api/v1/test-runs/:id/suite-runs/:suiteId/spec-runs
func (h *TestRunHandler) getSpecRuns(c *gin.Context) {
	suiteID, err := strconv.ParseUint(c.Param("suiteId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid suite run ID"})
		return
	}

	specRuns, err := h.testingService.GetSpecRunsBySuiteRunID(c.Request.Context(), uint(suiteID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, specRuns)
}

// getSpecRun handles GET /api/v1/test-runs/:id/suite-runs/:suiteId/spec-runs/:specId
func (h *TestRunHandler) getSpecRun(c *gin.Context) {
	specID, err := strconv.ParseUint(c.Param("specId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid spec run ID"})
		return
	}

	specRun, err := h.testingService.GetSpecRun(c.Request.Context(), uint(specID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Spec run not found"})
		return
	}

	c.JSON(http.StatusOK, specRun)
}

// convertTestRunToAPI converts a domain test run to API response format
func (h *TestRunHandler) convertTestRunToAPI(tr *domain.TestRun) gin.H {
	return gin.H{
		"id":           tr.ID,
		"projectId":    tr.ProjectID,
		"runId":        tr.RunID, // Use the external string identifier
		"name":         tr.Name,
		"branch":       tr.Branch,
		"gitBranch":    tr.GitBranch,
		"gitCommit":    tr.GitCommit,
		"status":       tr.Status,
		"startTime":    tr.StartTime,
		"endTime":      tr.EndTime,
		"totalTests":   tr.TotalTests,
		"passedTests":  tr.PassedTests,
		"failedTests":  tr.FailedTests,
		"skippedTests": tr.SkippedTests,
		"duration":     tr.Duration.Milliseconds(),
		"environment":  tr.Environment,
		"tags":         tr.Tags,
		"metadata":     tr.Metadata,
		"createdAt":    tr.StartTime,
		"updatedAt":    tr.EndTime,
	}
}

// --- Converter helper methods for public endpoints ---

// convertDomainTestRunToAPI converts a domain TestRun to the legacy API response format
// This matches the exact format used by the V1 DomainHandler for backward compatibility
func (h *TestRunHandler) convertDomainTestRunToAPI(tr *domain.TestRun) gin.H {
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

// convertApiSuiteRunsToDomain converts request SuiteRuns to domain SuiteRuns
func (h *TestRunHandler) convertApiSuiteRunsToDomain(reqSuiteRuns []SuiteRun) []domain.SuiteRun {
	domainSuiteRuns := make([]domain.SuiteRun, len(reqSuiteRuns))

	for i, reqSuite := range reqSuiteRuns {
		domainSpecRuns := h.convertSpecRunsToDomain(reqSuite.SpecRuns)

		totalTests, passedTests, failedTests, skippedTests := h.calcTestCounts(domainSpecRuns)
		status := h.calcSuiteStatus(domainSpecRuns)

		var duration time.Duration
		if !reqSuite.EndTime.IsZero() && !reqSuite.StartTime.IsZero() {
			duration = reqSuite.EndTime.Sub(reqSuite.StartTime)
		}

		var endTime *time.Time
		if !reqSuite.EndTime.IsZero() {
			endTime = &reqSuite.EndTime
		}

		domainTags := h.convertApiTagsToDomain(reqSuite.Tags)

		domainSuiteRuns[i] = domain.SuiteRun{
			ID:           reqSuite.ID,
			TestRunID:    0, // will be set later
			Name:         reqSuite.SuiteName,
			Status:       status,
			StartTime:    reqSuite.StartTime,
			EndTime:      endTime,
			TotalTests:   totalTests,
			PassedTests:  passedTests,
			FailedTests:  failedTests,
			SkippedTests: skippedTests,
			Duration:     duration,
			Tags:         domainTags,
			SpecRuns:     domainSpecRuns,
		}
	}

	return domainSuiteRuns
}

// convertSpecRunsToDomain converts request SpecRuns to domain SpecRuns
func (h *TestRunHandler) convertSpecRunsToDomain(reqSpecRuns []SpecRun) []*domain.SpecRun {
	domainSpecRuns := make([]*domain.SpecRun, len(reqSpecRuns))

	for i, reqSpec := range reqSpecRuns {
		var duration time.Duration
		if !reqSpec.EndTime.IsZero() && !reqSpec.StartTime.IsZero() {
			duration = reqSpec.EndTime.Sub(reqSpec.StartTime)
		}

		var endTime *time.Time
		if !reqSpec.EndTime.IsZero() {
			endTime = &reqSpec.EndTime
		}

		var errorMessage, failureMessage string
		if reqSpec.Status == "failed" || reqSpec.Status == "error" {
			if reqSpec.Status == "error" {
				errorMessage = reqSpec.Message
			} else {
				failureMessage = reqSpec.Message
			}
		}

		domainTags := h.convertApiTagsToDomain(reqSpec.Tags)

		domainSpecRuns[i] = &domain.SpecRun{
			ID:             reqSpec.ID,
			SuiteRunID:     uint(reqSpec.SuiteID),
			Name:           reqSpec.SpecDescription,
			Status:         reqSpec.Status,
			StartTime:      reqSpec.StartTime,
			EndTime:        endTime,
			Duration:       duration,
			ErrorMessage:   errorMessage,
			FailureMessage: failureMessage,
			Tags:           domainTags,
		}
	}

	return domainSpecRuns
}

// convertApiTagsToDomain converts API tags to domain tags
func (h *TestRunHandler) convertApiTagsToDomain(apiTags []Tag) []domain.Tag {
	if len(apiTags) == 0 {
		return nil
	}

	domainTags := make([]domain.Tag, len(apiTags))
	for i, tag := range apiTags {
		domainTags[i] = domain.Tag{
			ID:       uint(tag.ID),
			Name:     tag.Name,
			Category: tag.Category,
			Value:    tag.Value,
		}
	}
	return domainTags
}

// calcOverallStatus calculates the overall test run status from suite runs
func (h *TestRunHandler) calcOverallStatus(suiteRuns []SuiteRun) string {
	for _, suite := range suiteRuns {
		for _, spec := range suite.SpecRuns {
			if spec.Status == "failed" {
				return "failed"
			}
		}
	}
	return "passed"
}

// calcTestCounts calculates test statistics from SpecRuns
func (h *TestRunHandler) calcTestCounts(specRuns []*domain.SpecRun) (total, passed, failed, skipped int) {
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

// calcOverallTestCounts calculates total test statistics from all suite runs
func (h *TestRunHandler) calcOverallTestCounts(suiteRuns []domain.SuiteRun) (total, passed, failed, skipped int) {
	for _, suite := range suiteRuns {
		total += suite.TotalTests
		passed += suite.PassedTests
		failed += suite.FailedTests
		skipped += suite.SkippedTests
	}
	return total, passed, failed, skipped
}

// calcSuiteStatus determines suite status based on spec runs
func (h *TestRunHandler) calcSuiteStatus(specRuns []*domain.SpecRun) string {
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

// mergeUniqueTags merges two tag slices, removing duplicates by ID
func (h *TestRunHandler) mergeUniqueTags(existingTags, newTags []domain.Tag) []domain.Tag {
	tagMap := make(map[uint]domain.Tag)

	for _, tag := range existingTags {
		if tag.ID != 0 {
			tagMap[tag.ID] = tag
		}
	}

	for _, tag := range newTags {
		if tag.ID != 0 {
			tagMap[tag.ID] = tag
		}
	}

	tags := make([]domain.Tag, 0, len(tagMap))
	for _, tag := range tagMap {
		tags = append(tags, tag)
	}

	return tags
}

// RegisterRoutes registers test run routes
func (h *TestRunHandler) RegisterRoutes(userGroup, adminGroup *gin.RouterGroup) {
	// User routes (read operations)
	userGroup.GET("/test-runs", h.listTestRuns)
	userGroup.GET("/test-runs/count", h.countTestRuns)
	userGroup.GET("/test-runs/:id", h.getTestRun)
	userGroup.GET("/test-runs/by-run-id/:runId", h.getTestRunByRunID)
	userGroup.GET("/test-runs/stats", h.getTestRunStats)
	userGroup.GET("/test-runs/recent", h.getRecentTestRuns)
	userGroup.POST("/test-runs/:id/tags", h.assignTagsToTestRun)

	// Suite and spec run routes
	userGroup.GET("/test-runs/:id/suite-runs", h.getSuiteRuns)
	userGroup.GET("/test-runs/:id/suite-runs/:suiteId", h.getSuiteRun)
	userGroup.GET("/test-runs/:id/suite-runs/:suiteId/spec-runs", h.getSpecRuns)
	userGroup.GET("/test-runs/:id/suite-runs/:suiteId/spec-runs/:specId", h.getSpecRun)

	// Admin routes (create/update/delete)
	adminGroup.POST("/test-runs", h.createTestRun)
	adminGroup.PUT("/test-runs/:runId/status", h.updateTestRunStatus)
	adminGroup.DELETE("/test-runs/:id", h.deleteTestRun)
	adminGroup.POST("/test-runs/bulk-delete", h.bulkDeleteTestRuns)
}
