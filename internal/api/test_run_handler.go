// Package api provides domain-based REST API handlers
package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// TestRunHandler handles test run related endpoints
type TestRunHandler struct {
	*BaseHandler
	testingService *application.TestRunService
	tagService     *tagsApp.TagService
	projectService *projectsApp.ProjectService
}

// NewTestRunHandler creates a new test run handler
func NewTestRunHandler(testingService *application.TestRunService, projectService *projectsApp.ProjectService, logger *logging.Logger) *TestRunHandler {
	return &TestRunHandler{
		BaseHandler:    NewBaseHandler(logger),
		testingService: testingService,
		projectService: projectService,
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

	// Validate ProjectID
	_, err := h.projectService.GetProject(c.Request.Context(), projectsDomain.ProjectID(input.ProjectID))
	if err != nil {
		if errors.Is(err, projectsDomain.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Invalid project ID"})
			return
		}
		h.logger.WithError(err).Error("Failed to validate project")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate project"})
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
	c.JSON(http.StatusOK, convertTestRunToAPI(testRun))
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

	c.JSON(http.StatusOK, convertTestRunToAPI(testRun))
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
		apiTestRuns[i] = convertTestRunToAPI(tr)
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

	c.JSON(http.StatusOK, convertTestRunToAPI(updatedRun))
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
		apiTestRuns[i] = convertTestRunToAPI(tr)
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
	testRunID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test run ID"})
		return
	}

	suiteID, err := strconv.ParseUint(c.Param("suiteId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid suite run ID"})
		return
	}

	suiteRun, err := h.testingService.GetSuiteRunWithParentValidation(c.Request.Context(), uint(testRunID), uint(suiteID))
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Suite run not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, suiteRun)
}

// getSpecRuns handles GET /api/v1/test-runs/:id/suite-runs/:suiteId/spec-runs
func (h *TestRunHandler) getSpecRuns(c *gin.Context) {
	testRunID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test run ID"})
		return
	}

	suiteID, err := strconv.ParseUint(c.Param("suiteId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid suite run ID"})
		return
	}

	specRuns, err := h.testingService.GetSpecRunsWithParentValidation(c.Request.Context(), uint(testRunID), uint(suiteID))
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Suite run not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, specRuns)
}

// getSpecRun handles GET /api/v1/test-runs/:id/suite-runs/:suiteId/spec-runs/:specId
func (h *TestRunHandler) getSpecRun(c *gin.Context) {
	testRunID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test run ID"})
		return
	}

	suiteID, err := strconv.ParseUint(c.Param("suiteId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid suite run ID"})
		return
	}

	specID, err := strconv.ParseUint(c.Param("specId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid spec run ID"})
		return
	}

	specRun, err := h.testingService.GetSpecRunWithParentValidation(c.Request.Context(), uint(testRunID), uint(suiteID), uint(specID))
	if err != nil {
		if errors.Is(err, application.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Spec run not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, specRun)
}

// convertTestRunToAPI converts a domain test run to API response format for admin endpoints
// This format includes additional fields like name, gitBranch, and uses milliseconds for duration
func convertTestRunToAPI(tr *domain.TestRun) gin.H {
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



// --- Public (unauthenticated) test submission endpoints ---
// These are compatible with the legacy Fern Reporter API

// recordTestRun handles POST /api/v1/test-runs (public)
func (h *TestRunHandler) recordTestRun(c *gin.Context) {
	var req TestRunRequest

	if c.Request.Body == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request body is empty"})
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Process tags before converting to domain objects
	if h.tagService != nil {
		if err := ProcessTestRunTags(c.Request.Context(), h.tagService, &req); err != nil {
			h.logger.WithError(err).Error("Failed to process tags")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error processing tags"})
			return
		}
	}

	// Convert request SuiteRuns to domain SuiteRuns
	domainSuiteRuns := ConvertApiSuiteRunsToDomain(req.SuiteRuns)

	runLevelTags := ConvertApiTagsToDomain(req.Tags)

	// Calculate counts and status for this batch
	status := CalculateOverallStatus(req.SuiteRuns)
	environment := req.Environment
	if environment == "" {
		environment = "default"
	}
	totalTests, passedTests, failedTests, skippedTests :=
		CalculateOverallTestCounts(domainSuiteRuns)

	// Determine runID
	var runID string
	if req.TestSeed != 0 {
		runID = strconv.FormatUint(req.TestSeed, 10)
	} else {
		runID = uuid.New().String()
	}

	// Look up existing run if seed provided
	var testRun *domain.TestRun
	if req.TestSeed != 0 {
		existing, err := h.testingService.GetTestRunByRunID(c.Request.Context(), runID)
		if err == nil && existing != nil {
			testRun = existing
			h.logger.Debug("Test run exists", "runID", runID)
		}
	}

	if testRun == nil {
		// brand new run
		newTestRun := &domain.TestRun{
			RunID:        runID,
			ProjectID:    req.TestProjectID,
			Branch:       req.GitBranch,
			GitCommit:    req.GitSha,
			Environment:  environment,
			Metadata:     map[string]interface{}{},
			Status:       status,
			StartTime:    time.Now(),
			Tags:         runLevelTags,
			SuiteRuns:    domainSuiteRuns,
			TotalTests:   totalTests,
			PassedTests:  passedTests,
			FailedTests:  failedTests,
			SkippedTests: skippedTests,
		}

		createdTestRun, alreadyExisted, err := h.testingService.CreateTestRun(c.Request.Context(), newTestRun)
		h.logger.Debug("Test run creation result", "alreadyExisted", alreadyExisted, "runID", runID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		testRun = createdTestRun

		// If it was newly created (not a duplicate), return immediately
		if !alreadyExisted {
			response := ConvertDomainTestRunToAPI(testRun)
			c.JSON(http.StatusCreated, response)
			return
		}
		// If it already existed (concurrent creation), continue to add suite runs below
	}

	// At this point, testRun exists (either was already there or was concurrently created)
	// Add the new suite runs to the existing test run
	if testRun != nil {
		for _, suite := range domainSuiteRuns {
			suite.TestRunID = testRun.ID
			if err := h.testingService.CreateSuiteRun(c.Request.Context(), &suite); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			for _, spec := range suite.SpecRuns {
				spec.SuiteRunID = suite.ID
				if err := h.testingService.CreateSpecRun(c.Request.Context(), spec); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
			}
		}
		// Update existing run: accumulate suite runs + counts + tags
		testRun.SuiteRuns = append(testRun.SuiteRuns, domainSuiteRuns...)
		testRun.TotalTests += totalTests
		testRun.PassedTests += passedTests
		testRun.FailedTests += failedTests
		testRun.SkippedTests += skippedTests

		// Merge run-level tags
		testRun.Tags = MergeUniqueTags(testRun.Tags, runLevelTags)

		// mark overall status as failed if any failed
		if status == "failed" || testRun.Status == "failed" {
			testRun.Status = "failed"
		} else if status == "partial" || testRun.Status == "partial" {
			testRun.Status = "partial"
		} else {
			testRun.Status = "passed"
		}

		if err := h.testingService.UpdateTestRun(c.Request.Context(), testRun); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	response := ConvertDomainTestRunToAPI(testRun)
	c.JSON(http.StatusCreated, response)
}

// startTestRun handles POST /api/v1/test-runs/start (public)
func (h *TestRunHandler) startTestRun(c *gin.Context) {
	var req struct {
		ProjectID   string                 `json:"projectId" binding:"required"`
		RunID       string                 `json:"runId"`
		Branch      string                 `json:"branch"`
		CommitSha   string                 `json:"commitSha"`
		Environment string                 `json:"environment"`
		Tags        []string               `json:"tags"`
		Metadata    map[string]interface{} `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.RunID == "" {
		req.RunID = uuid.New().String()
	}

	environment := req.Environment
	if environment == "" {
		environment = "default"
	}

	testRun := &domain.TestRun{
		ProjectID:   req.ProjectID,
		RunID:       req.RunID,
		Branch:      req.Branch,
		GitCommit:   req.CommitSha,
		Environment: environment,
		Status:      "running",
		StartTime:   time.Now(),
		Metadata:    req.Metadata,
	}

	_, _, err := h.testingService.CreateTestRun(c.Request.Context(), testRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":    testRun.ID,
		"runId": testRun.RunID,
	})
}

// completeTestRun handles POST /api/v1/test-runs/complete (public)
func (h *TestRunHandler) completeTestRun(c *gin.Context) {
	var req struct {
		RunID        string     `json:"runId" binding:"required"`
		Status       string     `json:"status"`
		EndTime      *time.Time `json:"endTime"`
		TotalTests   int        `json:"totalTests"`
		PassedTests  int        `json:"passedTests"`
		FailedTests  int        `json:"failedTests"`
		SkippedTests int        `json:"skippedTests"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.EndTime == nil {
		now := time.Now()
		req.EndTime = &now
	}

	testRun, err := h.testingService.GetTestRunByRunID(c.Request.Context(), req.RunID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	if err := h.testingService.CompleteTestRun(c.Request.Context(), testRun.ID, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test run completed successfully"})
}

// addSuiteRun handles POST /api/v1/suite-runs (public)
func (h *TestRunHandler) addSuiteRun(c *gin.Context) {
	var req struct {
		TestRunID   string     `json:"testRunId" binding:"required"`
		SuiteName   string     `json:"suiteName" binding:"required"`
		Status      string     `json:"status"`
		StartTime   *time.Time `json:"startTime"`
		EndTime     *time.Time `json:"endTime"`
		Duration    int64      `json:"duration"`
		TotalSpecs  int        `json:"totalSpecs"`
		PassedSpecs int        `json:"passedSpecs"`
		FailedSpecs int        `json:"failedSpecs"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	testRun, err := h.testingService.GetTestRunByRunID(c.Request.Context(), req.TestRunID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	suiteRun := &domain.SuiteRun{
		TestRunID:    testRun.ID,
		Name:         req.SuiteName,
		Status:       req.Status,
		StartTime:    time.Now(),
		TotalTests:   req.TotalSpecs,
		PassedTests:  req.PassedSpecs,
		FailedTests:  req.FailedSpecs,
		SkippedTests: req.TotalSpecs - req.PassedSpecs - req.FailedSpecs,
	}

	if req.StartTime != nil {
		suiteRun.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		suiteRun.EndTime = req.EndTime
	}
	if req.Duration > 0 {
		suiteRun.Duration = time.Duration(req.Duration)
	}

	err = h.testingService.CreateSuiteRun(c.Request.Context(), suiteRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":        suiteRun.ID,
		"suiteName": suiteRun.Name,
	})
}

// addSpecRun handles POST /api/v1/spec-runs (public)
func (h *TestRunHandler) addSpecRun(c *gin.Context) {
	var req struct {
		SuiteRunID   uint       `json:"suiteRunId" binding:"required"`
		SpecName     string     `json:"specName" binding:"required"`
		Status       string     `json:"status"`
		StartTime    *time.Time `json:"startTime"`
		EndTime      *time.Time `json:"endTime"`
		Duration     int64      `json:"duration"`
		ErrorMessage string     `json:"errorMessage"`
		StackTrace   string     `json:"stackTrace"`
		Stdout       string     `json:"stdout"`
		Stderr       string     `json:"stderr"`
		Retries      int        `json:"retries"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	specRun := &domain.SpecRun{
		SuiteRunID:   req.SuiteRunID,
		Name:         req.SpecName,
		Status:       req.Status,
		StartTime:    time.Now(),
		ErrorMessage: req.ErrorMessage,
		StackTrace:   req.StackTrace,
		RetryCount:   req.Retries,
	}

	if req.StartTime != nil {
		specRun.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		specRun.EndTime = req.EndTime
	}
	if req.Duration > 0 {
		specRun.Duration = time.Duration(req.Duration)
	}

	err := h.testingService.AddSpecRun(c.Request.Context(), req.SuiteRunID, specRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       specRun.ID,
		"specName": specRun.Name,
	})
}

// updateTestRunPublic handles PUT /api/v1/test-runs/:id (public)
func (h *TestRunHandler) updateTestRunPublic(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Update test run not yet implemented"})
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

// RegisterSubmissionRoutes registers authenticated test submission routes
// These are compatible with the legacy Fern Reporter API
func (h *TestRunHandler) RegisterSubmissionRoutes(userGroup *gin.RouterGroup) {
	userGroup.POST("/test-runs", h.recordTestRun)
	userGroup.POST("/test-runs/start", h.startTestRun)
	userGroup.POST("/test-runs/complete", h.completeTestRun)
	userGroup.POST("/suite-runs", h.addSuiteRun)
	userGroup.POST("/spec-runs", h.addSpecRun)
	userGroup.PUT("/test-runs/:id", h.updateTestRunPublic)
}
