package interfaces

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// TestServiceAdapter provides HTTP handlers for test runs
type TestServiceAdapter struct {
	service *application.TestRunService
	logger  *logging.Logger
}

// NewTestServiceAdapter creates a new test service adapter
func NewTestServiceAdapter(service *application.TestRunService, logger *logging.Logger) *TestServiceAdapter {
	return &TestServiceAdapter{
		service: service,
		logger:  logger,
	}
}

// sendJSON is a helper method to send JSON responses with error handling
func (a *TestServiceAdapter) sendJSON(c *gin.Context, status int, data interface{}) {
	if err := c.JSON(status, data); err != nil {
		a.logger.WithError(err).Error("Failed to send JSON response")
	}
}

// CreateTestRun handles POST /api/v1/test-runs
func (a *TestServiceAdapter) CreateTestRun() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateTestRunRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
			return
		}

		// Map request to domain model
		testRun := &domain.TestRun{
			ProjectID:   req.ProjectID,
			Name:        req.Name,
			Branch:      req.Branch,
			GitBranch:   req.GitBranch,
			GitCommit:   req.GitCommit,
			Environment: req.Environment,
			Source:      req.Source,
			SessionID:   req.SessionID,
		}

		// Create test run
		if err := a.service.CreateTestRun(c.Request.Context(), testRun); err != nil {
			a.logger.WithError(err).Error("Failed to create test run")
			c.JSON(500, gin.H{"error": "Failed to create test run"})
			return
		}

		c.JSON(201, gin.H{
			"id":         testRun.ID,
			"project_id": testRun.ProjectID,
			"status":     testRun.Status,
		})
	}
}

// GetTestRun handles GET /api/v1/test-runs/:id
func (a *TestServiceAdapter) GetTestRun() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c.Param("id"))
		if err != nil {
			a.sendJSON(c, 400, gin.H{"error": "Invalid test run ID"})
			return
		}

		testRun, err := a.service.GetTestRun(c.Request.Context(), id)
		if err != nil {
			if err.Error() == "test run not found" {
				c.JSON(404, gin.H{"error": "Test run not found"})
				return
			}
			a.logger.WithError(err).Error("Failed to get test run")
			c.JSON(500, gin.H{"error": "Failed to get test run"})
			return
		}

		c.JSON(200, testRun)
	}
}

// GetTestRunDetails handles GET /api/v1/test-runs/:id/details
func (a *TestServiceAdapter) GetTestRunDetails() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid test run ID"})
			return
		}

		testRun, err := a.service.GetTestRunWithDetails(c.Request.Context(), id)
		if err != nil {
			if err.Error() == "test run not found" {
				c.JSON(404, gin.H{"error": "Test run not found"})
				return
			}
			a.logger.WithError(err).Error("Failed to get test run details")
			c.JSON(500, gin.H{"error": "Failed to get test run details"})
			return
		}

		c.JSON(200, testRun)
	}
}

// CompleteTestRun handles PUT /api/v1/test-runs/:id/complete
func (a *TestServiceAdapter) CompleteTestRun() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseUintParam(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid test run ID"})
			return
		}

		var req CompleteTestRunRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
			return
		}

		if err := a.service.CompleteTestRun(c.Request.Context(), id, req.Status); err != nil {
			a.logger.WithError(err).Error("Failed to complete test run")
			c.JSON(500, gin.H{"error": "Failed to complete test run"})
			return
		}

		c.JSON(200, gin.H{"message": "Test run completed"})
	}
}

// GetProjectTestRuns handles GET /api/v1/projects/:projectId/test-runs
func (a *TestServiceAdapter) GetProjectTestRuns() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(400, gin.H{"error": "Project ID is required"})
			return
		}

		limit := 50 // default
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := parseUintParam(limitStr); err == nil {
				// Check for integer overflow before conversion
				if l > uint(^int(0)>>1) {
					c.JSON(400, gin.H{"error": "Limit value too large"})
					return
				}
				limit = int(l)
			}
		}

		testRuns, err := a.service.GetProjectTestRuns(c.Request.Context(), projectID, limit)
		if err != nil {
			a.logger.WithError(err).Error("Failed to get project test runs")
			c.JSON(500, gin.H{"error": "Failed to get test runs"})
			return
		}

		c.JSON(200, gin.H{
			"test_runs": testRuns,
			"total":     len(testRuns),
		})
	}
}

// GetProjectTestRunSummary handles GET /api/v1/projects/:projectId/test-runs/summary
func (a *TestServiceAdapter) GetProjectTestRunSummary() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(400, gin.H{"error": "Project ID is required"})
			return
		}

		summary, err := a.service.GetTestRunSummary(c.Request.Context(), projectID)
		if err != nil {
			a.logger.WithError(err).Error("Failed to get test run summary")
			c.JSON(500, gin.H{"error": "Failed to get summary"})
			return
		}

		c.JSON(200, summary)
	}
}

// CreateTestRunWithSuites handles POST /api/v1/test-runs/with-suites
func (a *TestServiceAdapter) CreateTestRunWithSuites() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateTestRunWithSuitesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
			return
		}

		// Map request to domain models
		testRun := &domain.TestRun{
			ProjectID:   req.ProjectID,
			Name:        req.Name,
			Branch:      req.Branch,
			GitBranch:   req.GitBranch,
			GitCommit:   req.GitCommit,
			Environment: req.Environment,
			Source:      req.Source,
			SessionID:   req.SessionID,
			Status:      "completed",
		}

		// Map suites
		suites := make([]domain.SuiteRun, len(req.Suites))
		for i, suiteReq := range req.Suites {
			suite := domain.SuiteRun{
				Name:        suiteReq.Name,
				PackageName: suiteReq.PackageName,
				ClassName:   suiteReq.ClassName,
			}

			// Map specs
			specs := make([]*domain.SpecRun, len(suiteReq.Specs))
			for j, specReq := range suiteReq.Specs {
				specs[j] = &domain.SpecRun{
					Name:           specReq.Name,
					ClassName:      specReq.ClassName,
					Status:         specReq.Status,
					Duration:       time.Duration(specReq.Duration) * time.Millisecond,
					ErrorMessage:   specReq.ErrorMessage,
					FailureMessage: specReq.FailureMessage,
				}
			}
			suite.SpecRuns = specs
			suites[i] = suite
		}

		// Create test run with suites
		if err := a.service.CreateTestRunWithSuites(c.Request.Context(), testRun, suites); err != nil {
			a.logger.WithError(err).Error("Failed to create test run with suites")
			c.JSON(500, gin.H{"error": "Failed to create test run"})
			return
		}

		c.JSON(201, gin.H{
			"id":         testRun.ID,
			"project_id": testRun.ProjectID,
			"status":     testRun.Status,
		})
	}
}

// RegisterRoutes registers all test service routes
func (a *TestServiceAdapter) RegisterRoutes(router *gin.RouterGroup) {
	// Test run endpoints
	router.POST("/test-runs", a.CreateTestRun())
	router.GET("/test-runs/:id", a.GetTestRun())
	router.GET("/test-runs/:id/details", a.GetTestRunDetails())
	router.PUT("/test-runs/:id/complete", a.CompleteTestRun())
	router.POST("/test-runs/with-suites", a.CreateTestRunWithSuites())

	// Project test run endpoints
	router.GET("/projects/:projectId/test-runs", a.GetProjectTestRuns())
	router.GET("/projects/:projectId/test-runs/summary", a.GetProjectTestRunSummary())
}

// Request/Response types

type CreateTestRunRequest struct {
	ProjectID   string `json:"project_id" binding:"required"`
	Name        string `json:"name"`
	Branch      string `json:"branch"`
	GitBranch   string `json:"git_branch"`
	GitCommit   string `json:"git_commit"`
	Environment string `json:"environment"`
	Source      string `json:"source"`
	SessionID   string `json:"session_id"`
}

type CompleteTestRunRequest struct {
	Status string `json:"status" binding:"required"`
}

type CreateTestRunWithSuitesRequest struct {
	ProjectID   string     `json:"project_id" binding:"required"`
	Name        string     `json:"name"`
	Branch      string     `json:"branch"`
	GitBranch   string     `json:"git_branch"`
	GitCommit   string     `json:"git_commit"`
	Environment string     `json:"environment"`
	Source      string     `json:"source"`
	SessionID   string     `json:"session_id"`
	Suites      []SuiteReq `json:"suites"`
}

type SuiteReq struct {
	Name        string    `json:"name"`
	PackageName string    `json:"package_name"`
	ClassName   string    `json:"class_name"`
	Specs       []SpecReq `json:"specs"`
}

type SpecReq struct {
	Name           string `json:"name"`
	ClassName      string `json:"class_name"`
	Status         string `json:"status"`
	Duration       int64  `json:"duration"` // milliseconds
	ErrorMessage   string `json:"error_message"`
	FailureMessage string `json:"failure_message"`
}

// Helper functions

func parseUintParam(param string) (uint, error) {
	var id uint
	if _, err := fmt.Sscanf(param, "%d", &id); err != nil {
		return 0, err
	}
	return id, nil
}

// GraphQL resolvers

// ResolveTestRuns resolves test runs for GraphQL
func (a *TestServiceAdapter) ResolveTestRuns(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	return a.service.GetProjectTestRuns(ctx, projectID, limit)
}

// ResolveTestRunDetails resolves test run details for GraphQL
func (a *TestServiceAdapter) ResolveTestRunDetails(ctx context.Context, id uint) (*domain.TestRun, error) {
	return a.service.GetTestRunWithDetails(ctx, id)
}
