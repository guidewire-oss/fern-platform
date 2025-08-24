// Package api provides domain-based REST API handlers
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// FernLegacyHandler handles legacy fern-reporter compatible endpoints
type FernLegacyHandler struct {
	*BaseHandler
	testingService TestRunService
	projectService ProjectService
}

// TestRunService is an interface for test run operations
type TestRunService interface {
	CreateTestRun(ctx context.Context, tr *domain.TestRun) error
	GetTestRunByRunID(ctx context.Context, runID string) (*domain.TestRun, error)
	ListTestRuns(ctx context.Context, projectUUID string, limit, offset int) ([]*domain.TestRun, int64, error)
	CreateSuiteRun(ctx context.Context, sr *domain.SuiteRun) error
	CreateSpecRun(ctx context.Context, sr *domain.SpecRun) error
}

// ProjectService is an interface for project operations
type ProjectService interface {
	CreateProject(ctx context.Context, id projectsDomain.ProjectID, name string, team projectsDomain.Team, source string) (*projectsDomain.Project, error)
	GetProject(ctx context.Context, id projectsDomain.ProjectID) (*projectsDomain.Project, error)
	UpdateProject(ctx context.Context, id projectsDomain.ProjectID, req projectsApp.UpdateProjectRequest) error
	ListProjects(ctx context.Context, limit, offset int) ([]*projectsDomain.Project, int64, error)
}

// NewFernLegacyHandler creates a new fern legacy handler
func NewFernLegacyHandler(
	testingService TestRunService,
	projectService ProjectService,
	logger *logging.Logger,
) *FernLegacyHandler {
	return &FernLegacyHandler{
		BaseHandler:    NewBaseHandler(logger),
		testingService: testingService,
		projectService: projectService,
	}
}

// createFernProject handles POST /api/project
func (h *FernLegacyHandler) createFernProject(c *gin.Context) {
	var input struct {
		ProjectID     string `json:"projectId"`
		Name          string `json:"name" binding:"required"`
		Repository    string `json:"repository"`
		DefaultBranch string `json:"default_branch"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use provided project ID or generate a new one
	projectID := input.ProjectID
	if projectID == "" {
		projectID = uuid.New().String()
	}

	project, err := h.projectService.CreateProject(
		c.Request.Context(),
		projectsDomain.ProjectID(projectID),
		input.Name,
		projectsDomain.Team("default"),
		"api",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update additional fields
	if input.Repository != "" || input.DefaultBranch != "" {
		updates := projectsApp.UpdateProjectRequest{}
		if input.Repository != "" {
			updates.Repository = &input.Repository
		}
		if input.DefaultBranch != "" {
			updates.DefaultBranch = &input.DefaultBranch
		}
		if err := h.projectService.UpdateProject(c.Request.Context(), project.ProjectID(), updates); err != nil {
			h.logger.WithError(err).Error("Failed to update project")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update project"})
			return
		}
	}

	// Return Fern-compatible response
	c.JSON(http.StatusCreated, gin.H{
		"uuid": string(project.ProjectID()),
		"name": project.Name(),
	})
}

// getFernProject handles GET /api/project/:uuid
func (h *FernLegacyHandler) getFernProject(c *gin.Context) {
	projectID := c.Param("uuid")

	project, err := h.projectService.GetProject(c.Request.Context(), projectsDomain.ProjectID(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Return Fern-compatible response
	snapshot := project.ToSnapshot()
	c.JSON(http.StatusOK, gin.H{
		"uuid":           string(snapshot.ProjectID),
		"name":           snapshot.Name,
		"repository":     snapshot.Repository,
		"default_branch": snapshot.DefaultBranch,
	})
}

// listFernProjects handles GET /api/projects
func (h *FernLegacyHandler) listFernProjects(c *gin.Context) {
	projects, _, err := h.projectService.ListProjects(c.Request.Context(), 100, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to Fern-compatible format
	fernProjects := make([]gin.H, len(projects))
	for i, p := range projects {
		snapshot := p.ToSnapshot()
		fernProjects[i] = gin.H{
			"uuid": string(snapshot.ProjectID),
			"name": snapshot.Name,
		}
	}

	c.JSON(http.StatusOK, fernProjects)
}

// createFernTestReport handles POST /api/reports/testrun
func (h *FernLegacyHandler) createFernTestReport(c *gin.Context) {
	// Read raw body for logging
	bodyBytes, err := c.GetRawData()
	if err != nil {
		h.logger.WithError(err).Error("Failed to read request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	h.logger.Debug("fern-ginkgo-client request received",
		"endpoint", c.Request.URL.Path,
		"method", c.Request.Method,
		"content-length", len(bodyBytes))

	// Parse the structured input
	var input struct {
		ID                uint64 `json:"id"`
		TestProjectName   string `json:"test_project_name"`
		TestProjectID     string `json:"test_project_id"`
		TestSeed          uint64 `json:"test_seed"`
		StartTime         string `json:"start_time"`
		EndTime           string `json:"end_time"`
		GitBranch         string `json:"git_branch"`
		GitSha            string `json:"git_sha"`
		BuildTriggerActor string `json:"build_trigger_actor"`
		BuildUrl          string `json:"build_url"`
		ClientType        string `json:"client_type"`
		SuiteRuns         []struct {
			ID        uint64 `json:"id"`
			TestRunID uint64 `json:"test_run_id"`
			SuiteName string `json:"suite_name"`
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
			SpecRuns  []struct {
				ID              uint64 `json:"id"`
				SuiteID         uint64 `json:"suite_id"`
				SpecDescription string `json:"spec_description"`
				Status          string `json:"status"`
				Message         string `json:"message"`
				Tags            []struct {
					ID   uint64 `json:"id"`
					Name string `json:"name"`
				} `json:"tags"`
				StartTime string `json:"start_time"`
				EndTime   string `json:"end_time"`
			} `json:"spec_runs"`
		} `json:"suite_runs"`
	}

	if err := json.Unmarshal(bodyBytes, &input); err != nil {
		h.logger.WithError(err).Error("Failed to parse JSON input")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Parsed fern-ginkgo-client input",
		"test_project_id", input.TestProjectID,
		"test_project_name", input.TestProjectName,
		"test_seed", input.TestSeed,
		"client_type", input.ClientType,
		"suite_runs_count", len(input.SuiteRuns))

	// Extract test_project_id which should match the project UUID in project_details table
	projectID := input.TestProjectID
	if projectID == "" {
		h.logger.Error("test_project_id is missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "test_project_id is required"})
		return
	}

	// Look up the project by ID from the project_details table
	h.logger.Info("Looking up project", "project_id", projectID)
	project, err := h.projectService.GetProject(c.Request.Context(), projectsDomain.ProjectID(projectID))
	if err != nil {
		h.logger.WithError(err).Error("Project not found", "project_id", projectID)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("project not found with ID: %s", projectID)})
		return
	}
	h.logger.Info("Found project", "project_name", project.Name(), "project_id", projectID)

	// Parse start time
	var startTime time.Time
	if input.StartTime != "" {
		if parsedTime, err := time.Parse(time.RFC3339, input.StartTime); err == nil {
			startTime = parsedTime
		} else {
			startTime = time.Now()
		}
	} else {
		startTime = time.Now()
	}

	// Parse end time
	var endTime *time.Time
	if input.EndTime != "" {
		if parsedTime, err := time.Parse(time.RFC3339, input.EndTime); err == nil {
			endTime = &parsedTime
		}
	}

	// Calculate test statistics from suite runs
	var totalTests, passedTests, failedTests, skippedTests int
	for _, suiteRun := range input.SuiteRuns {
		for _, specRun := range suiteRun.SpecRuns {
			totalTests++
			switch specRun.Status {
			case "passed":
				passedTests++
			case "failed":
				failedTests++
			case "skipped", "pending":
				skippedTests++
			}
		}
	}

	// Use git information from fern-ginkgo-client
	branch := input.GitBranch
	if branch == "" {
		branch = "main" // Default
	}

	commitSHA := input.GitSha

	// Create run ID with test seed
	runID := fmt.Sprintf("%s-run-%d", project.Name(), input.TestSeed)

	// Check if test run already exists with this run_id
	h.logger.Info("Checking if test run already exists", "run_id", runID)
	existingTestRun, err := h.testingService.GetTestRunByRunID(c.Request.Context(), runID)

	var testRun *domain.TestRun
	if err != nil || existingTestRun == nil {
		// Test run doesn't exist, create a new one
		h.logger.Info("Test run does not exist, creating new one", "run_id", runID)

		testRun = &domain.TestRun{
			ProjectID:    string(project.ProjectID()),
			RunID:        runID,
			Name:         project.Name(), // Use project name as test run name
			GitBranch:    branch,
			GitCommit:    commitSHA,
			Environment:  "test",
			Source:       input.ClientType,
			Status:       "completed",
			StartTime:    startTime,
			EndTime:      endTime,
			TotalTests:   totalTests,
			PassedTests:  passedTests,
			FailedTests:  failedTests,
			SkippedTests: skippedTests,
		}

		// Calculate duration if both start and end times are available
		if endTime != nil && !startTime.IsZero() {
			testRun.Duration = endTime.Sub(startTime)
		}

		// Store additional metadata
		if input.BuildUrl != "" || input.BuildTriggerActor != "" {
			testRun.Metadata = map[string]interface{}{
				"test_seed":           input.TestSeed,
				"build_url":           input.BuildUrl,
				"build_trigger_actor": input.BuildTriggerActor,
				"suite_runs":          input.SuiteRuns,
			}
		}

		h.logger.Info("Creating test run",
			"run_id", testRun.RunID,
			"project_id", testRun.ProjectID,
			"name", testRun.Name,
			"total_tests", testRun.TotalTests,
			"status", testRun.Status)

		if err := h.testingService.CreateTestRun(c.Request.Context(), testRun); err != nil {
			h.logger.WithError(err).Error("Failed to create test run")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Create suite runs and spec runs from input data
		for _, inputSuiteRun := range input.SuiteRuns {
			// Parse suite start and end times
			var suiteStartTime time.Time
			var suiteEndTime *time.Time

			if inputSuiteRun.StartTime != "" {
				if parsedTime, err := time.Parse(time.RFC3339, inputSuiteRun.StartTime); err == nil {
					suiteStartTime = parsedTime
				} else {
					suiteStartTime = startTime // Fallback to test run start time
				}
			} else {
				suiteStartTime = startTime
			}

			if inputSuiteRun.EndTime != "" {
				if parsedTime, err := time.Parse(time.RFC3339, inputSuiteRun.EndTime); err == nil {
					suiteEndTime = &parsedTime
				}
			}

			// Calculate suite statistics
			var suiteTotalTests, suitePassedTests, suiteFailedTests, suiteSkippedTests int
			for _, specRun := range inputSuiteRun.SpecRuns {
				suiteTotalTests++
				switch specRun.Status {
				case "passed":
					suitePassedTests++
				case "failed":
					suiteFailedTests++
				case "skipped", "pending":
					suiteSkippedTests++
				}
			}

			// Create suite run
			suiteRun := &domain.SuiteRun{
				TestRunID:    testRun.ID,
				Name:         inputSuiteRun.SuiteName,
				Status:       "completed", // Assume completed since we're getting end time
				StartTime:    suiteStartTime,
				EndTime:      suiteEndTime,
				TotalTests:   suiteTotalTests,
				PassedTests:  suitePassedTests,
				FailedTests:  suiteFailedTests,
				SkippedTests: suiteSkippedTests,
			}

			// Calculate suite duration
			if suiteEndTime != nil && !suiteStartTime.IsZero() {
				suiteRun.Duration = suiteEndTime.Sub(suiteStartTime)
			}

			if err := h.testingService.CreateSuiteRun(c.Request.Context(), suiteRun); err != nil {
				h.logger.WithError(err).Error("Failed to create suite run", "suite_name", suiteRun.Name)
				continue // Continue with other suites
			}

			// Create spec runs for this suite
			for _, inputSpecRun := range inputSuiteRun.SpecRuns {
				// Parse spec start and end times
				var specStartTime time.Time
				var specEndTime *time.Time

				if inputSpecRun.StartTime != "" {
					if parsedTime, err := time.Parse(time.RFC3339, inputSpecRun.StartTime); err == nil {
						specStartTime = parsedTime
					} else {
						specStartTime = suiteStartTime // Fallback to suite start time
					}
				} else {
					specStartTime = suiteStartTime
				}

				if inputSpecRun.EndTime != "" {
					if parsedTime, err := time.Parse(time.RFC3339, inputSpecRun.EndTime); err == nil {
						specEndTime = &parsedTime
					}
				}

				// Create spec run
				specRun := &domain.SpecRun{
					SuiteRunID:     suiteRun.ID,
					Name:           inputSpecRun.SpecDescription,
					Status:         inputSpecRun.Status,
					StartTime:      specStartTime,
					EndTime:        specEndTime,
					ErrorMessage:   inputSpecRun.Message,
					FailureMessage: inputSpecRun.Message,
				}

				// Calculate spec duration
				if specEndTime != nil && !specStartTime.IsZero() {
					specRun.Duration = specEndTime.Sub(specStartTime)
				}

				if err := h.testingService.CreateSpecRun(c.Request.Context(), specRun); err != nil {
					h.logger.WithError(err).Error("Failed to create spec run",
						"spec_name", specRun.Name,
						"suite_name", suiteRun.Name)
					continue // Continue with other specs
				}
			}
		}
	} else {
		// Test run already exists
		testRun = existingTestRun
		h.logger.Info("Test run already exists", "run_id", runID, "test_run_id", testRun.ID)
	}

	h.logger.Info("Test run created/found successfully",
		"test_run_id", testRun.ID,
		"run_id", testRun.RunID,
		"project_id", testRun.ProjectID)

	// Return Fern-compatible response structure
	response := gin.H{
		"test_run_id": testRun.ID,
		"run_id":      testRun.RunID,
		"project_id":  testRun.ProjectID,
		"name":        testRun.Name,
		"status":      testRun.Status,
		"created_at":  testRun.StartTime.Format(time.RFC3339),
	}

	h.logger.Info("Sending response", "response", response)
	c.JSON(http.StatusCreated, response)
}

// listFernTestReports handles GET /api/reports/testruns
func (h *FernLegacyHandler) listFernTestReports(c *gin.Context) {
	projectUUID := c.Query("project_uuid")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	testRuns, _, err := h.testingService.ListTestRuns(c.Request.Context(), projectUUID, limit, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list test reports"})
		return
	}

	items := make([]gin.H, 0, len(testRuns))
	for _, tr := range testRuns {
		items = append(items, gin.H{
			"uuid":         tr.RunID,
			"project_uuid": tr.ProjectID,
			"branch":       tr.GitBranch,
			"git_commit":   tr.GitCommit,
			"status":       tr.Status,
			"created_at":   tr.StartTime.Format(time.RFC3339),
			"test_stats": gin.H{
				"total_tests":   tr.TotalTests,
				"passed_tests":  tr.PassedTests,
				"failed_tests":  tr.FailedTests,
				"skipped_tests": tr.SkippedTests,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"test_runs": items})
}

// getFernTestReport handles GET /api/reports/testrun/:uuid
func (h *FernLegacyHandler) getFernTestReport(c *gin.Context) {
	runID := c.Param("uuid")
	testRun, err := h.testingService.GetTestRunByRunID(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test report not found"})
		return
	}

	// TODO: Include suite runs in response
	c.JSON(http.StatusOK, h.convertTestRunToLegacyAPI(testRun))
}

// Helper methods

func (h *FernLegacyHandler) convertTestRunToLegacyAPI(tr *domain.TestRun) gin.H {
	endTime := ""
	if tr.EndTime != nil {
		endTime = tr.EndTime.Format(time.RFC3339)
	}

	return gin.H{
		"uuid":         tr.RunID,
		"project_uuid": tr.ProjectID,
		"branch":       tr.GitBranch,
		"git_commit":   tr.GitCommit,
		"status":       tr.Status,
		"start_time":   tr.StartTime.Format(time.RFC3339),
		"end_time":     endTime,
		"duration":     int(tr.Duration.Seconds()),
		"environment":  tr.Environment,
		"created_at":   tr.StartTime.Format(time.RFC3339),
		"test_stats": gin.H{
			"total_tests":   tr.TotalTests,
			"passed_tests":  tr.PassedTests,
			"failed_tests":  tr.FailedTests,
			"skipped_tests": tr.SkippedTests,
		},
	}
}

// RegisterRoutes registers legacy fern routes
func (h *FernLegacyHandler) RegisterRoutes(apiGroup *gin.RouterGroup) {
	// Project endpoints compatible with fern-ginkgo-client
	apiGroup.POST("/project", h.createFernProject)
	apiGroup.GET("/project/:uuid", h.getFernProject)
	apiGroup.GET("/projects", h.listFernProjects)

	// Test reports endpoints
	apiGroup.POST("/reports/testrun", h.createFernTestReport)
	apiGroup.GET("/reports/testruns", h.listFernTestReports)
	apiGroup.GET("/reports/testrun/:uuid", h.getFernTestReport)

	// Additional endpoints that fern-ginkgo-client might expect
	apiGroup.POST("/testrun", h.createFernTestReport) // Alias for test run creation
}
