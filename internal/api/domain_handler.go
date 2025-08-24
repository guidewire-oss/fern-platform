// THIS FILE IS BECOMING DEPRECATED - Use the new split handler architecture in domain_handler_v2.go
package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	analyticsApp "github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/interfaces"
	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	testingApp "github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	testingDomain "github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// DomainHandler handles all domain-related HTTP requests
type DomainHandler struct {
	testingService        *testingApp.TestRunService
	projectService        *projectsApp.ProjectService
	tagService            *tagsApp.TagService
	flakyDetectionService *analyticsApp.FlakyDetectionService
	jiraConnectionService *integrations.JiraConnectionService
	authMiddleware        *interfaces.AuthMiddlewareAdapter
	logger                *logging.Logger
}

// NewDomainHandler creates a new domain handler
func NewDomainHandler(
	testingService *testingApp.TestRunService,
	projectService *projectsApp.ProjectService,
	tagService *tagsApp.TagService,
	flakyDetectionService *analyticsApp.FlakyDetectionService,
	jiraConnectionService *integrations.JiraConnectionService,
	authMiddleware *interfaces.AuthMiddlewareAdapter,
	logger *logging.Logger,
) *DomainHandler {
	return &DomainHandler{
		testingService:        testingService,
		projectService:        projectService,
		tagService:            tagService,
		flakyDetectionService: flakyDetectionService,
		jiraConnectionService: jiraConnectionService,
		authMiddleware:        authMiddleware,
		logger:                logger,
	}
}

// RegisterRoutes registers all domain handler routes
func (h *DomainHandler) RegisterRoutes(router *gin.Engine) {
	// Health check route
	router.GET("/health", h.healthCheck)

	// API v1 routes
	apiV1 := router.Group("/api/v1")
	{
		// Public routes for test result submission
		// These are compatible with the legacy Fern Reporter API
		apiV1.POST("/test-runs", h.recordTestRun)
		apiV1.POST("/test-runs/start", h.startTestRun)
		apiV1.POST("/test-runs/complete", h.completeTestRun)
		apiV1.POST("/suite-runs", h.addSuiteRun)
		apiV1.POST("/spec-runs", h.addSpecRun)
		apiV1.PUT("/test-runs/:id", h.updateTestRun)

		// Protected routes - require authentication
		protected := apiV1.Group("/")
		protected.Use(h.authMiddleware.RequireAuth())
		{
			// Test runs
			protected.GET("/test-runs", h.getTestRuns)
			protected.GET("/test-runs/:id", h.getTestRun)
			protected.GET("/test-runs/by-run-id/:id", h.getTestRunByRunId)
			protected.DELETE("/test-runs/:id", h.deleteTestRun)

			// Suites
			protected.GET("/test-runs/:id/suite-runs", h.getSuiteRuns)
			protected.GET("/test-runs/:id/suite-runs/:suiteId", h.getSuiteRun)

			// Specs
			protected.GET("/test-runs/:id/suite-runs/:suiteId/spec-runs", h.getSpecRuns)
			protected.GET("/test-runs/:id/suite-runs/:suiteId/spec-runs/:specId", h.getSpecRun)

			// Projects
			protected.GET("/projects", h.getProjects)
			protected.GET("/projects/:id", h.getProject)
			protected.GET("/projects/by-project-id/:projectId", h.getProjectByProjectId)

			// Manager-only routes
			managerRoutes := protected.Group("/")
			managerRoutes.Use(h.requireManagerRole())
			{
				// Project management
				managerRoutes.POST("/projects", h.createProject)
				managerRoutes.PUT("/projects/:id", h.updateProject)
				managerRoutes.DELETE("/projects/:id", h.deleteProject)

				// JIRA connections
				managerRoutes.GET("/projects/:id/jira-connections", h.getJiraConnections)
				managerRoutes.POST("/projects/:id/jira-connections", h.createJiraConnection)
				managerRoutes.PUT("/jira-connections/:connectionId", h.updateJiraConnection)
				managerRoutes.PUT("/jira-connections/:connectionId/credentials", h.updateJiraCredentials)
				managerRoutes.POST("/jira-connections/:connectionId/test", h.testJiraConnection)
				managerRoutes.DELETE("/jira-connections/:connectionId", h.deleteJiraConnection)
			}

			// Tags
			protected.GET("/tags", h.getTags)
			protected.GET("/tags/:id", h.getTag)

			// Flaky tests
			protected.GET("/flaky-tests", h.getFlakyTests)
			protected.POST("/flaky-tests/:id/resolve", h.resolveFlakyTest)
			protected.POST("/flaky-tests/:id/ignore", h.ignoreFlakyTest)
		}
	}

	// Auth routes
	auth := router.Group("/auth")
	{
		auth.GET("/login", h.authMiddleware.StartOAuthFlow())
		auth.GET("/callback", h.authMiddleware.HandleOAuthCallback())
		auth.POST("/logout", h.authMiddleware.Logout())
		auth.GET("/user", h.authMiddleware.RequireAuth(), h.getCurrentUser)
	}

	// Static file serving
	router.Static("/web", "./web")
	// Note: Root route "/" is handled by DomainHandlerV2 when FERN_USE_SPLIT_HANDLERS=true
	// router.GET("/", func(c *gin.Context) {
	//	c.Redirect(http.StatusMovedPermanently, "/web/")
	// })
}

// Helper function to check if user is authenticated
func (h *DomainHandler) isUserAuthenticated(c *gin.Context) bool {
	user, exists := c.Get("user")
	return exists && user != nil
}

// requireManagerRole returns middleware that checks for manager or admin role
func (h *DomainHandler) requireManagerRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		authUser, ok := user.(*authDomain.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user data"})
			c.Abort()
			return
		}

		// Check if user has manager or admin role
		if !authUser.IsTeamManager() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Manager or admin role required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Health check handler
func (h *DomainHandler) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// Test Run Handlers

func (h *DomainHandler) recordTestRun(c *gin.Context) {
	var req struct {
		ProjectID   string                 `json:"projectId" binding:"required"`
		RunID       string                 `json:"runId"`
		Branch      string                 `json:"branch"`
		CommitSHA   string                 `json:"commitSha"`
		Environment string                 `json:"environment"`
		Metadata    map[string]interface{} `json:"metadata"`
		Status      string                 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate a unique run ID if not provided
	if req.RunID == "" {
		req.RunID = uuid.New().String()
	}

	// Set defaults
	if req.Status == "" {
		req.Status = "pending"
	}

	// Create test run object
	testRun := &testingDomain.TestRun{
		RunID:       req.RunID,
		ProjectID:   req.ProjectID,
		Branch:      req.Branch,
		GitCommit:   req.CommitSHA,
		Environment: req.Environment,
		Metadata:    req.Metadata,
		Status:      req.Status,
		StartTime:   time.Now(),
	}

	err := h.testingService.CreateTestRun(c.Request.Context(), testRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := h.convertTestRunToAPI(testRun)
	c.JSON(http.StatusCreated, response)
}

func (h *DomainHandler) startTestRun(c *gin.Context) {
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

	// Generate run ID if not provided
	if req.RunID == "" {
		req.RunID = uuid.New().String()
	}

	// Create the test run object
	testRun := &testingDomain.TestRun{
		ProjectID:   req.ProjectID,
		RunID:       req.RunID,
		Branch:      req.Branch,
		GitCommit:   req.CommitSha,
		Environment: req.Environment,
		Status:      "running",
		StartTime:   time.Now(),
		Metadata:    req.Metadata,
	}

	err := h.testingService.CreateTestRun(c.Request.Context(), testRun)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":    testRun.ID,
		"runId": testRun.RunID,
	})
}

func (h *DomainHandler) completeTestRun(c *gin.Context) {
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

	// Set end time if not provided
	if req.EndTime == nil {
		now := time.Now()
		req.EndTime = &now
	}

	// Get test run by run ID to get the internal ID
	testRun, err := h.testingService.GetTestRunByRunID(c.Request.Context(), req.RunID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	// Complete the test run using the internal ID
	if err := h.testingService.CompleteTestRun(c.Request.Context(), testRun.ID, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Test run completed successfully"})
}

func (h *DomainHandler) addSuiteRun(c *gin.Context) {
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

	// Get test run by run ID to get the internal ID
	testRun, err := h.testingService.GetTestRunByRunID(c.Request.Context(), req.TestRunID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	// Create suite run
	suiteRun := &testingDomain.SuiteRun{
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

func (h *DomainHandler) addSpecRun(c *gin.Context) {
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

	// Create spec run
	specRun := &testingDomain.SpecRun{
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

func (h *DomainHandler) updateTestRun(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Update test run not yet implemented"})
}

func (h *DomainHandler) getTestRuns(c *gin.Context) {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	projectID := c.Query("projectId")
	_ = c.Query("status") // status filtering not implemented yet

	// Get test runs - simplified version
	var testRuns []*testingDomain.TestRun
	var err error

	if projectID != "" {
		testRuns, err = h.testingService.GetProjectTestRuns(c.Request.Context(), projectID, limit)
	} else {
		testRuns, err = h.testingService.GetRecentTestRuns(c.Request.Context(), limit)
	}

	total := int64(len(testRuns))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to API response
	response := make([]gin.H, len(testRuns))
	for i, tr := range testRuns {
		response[i] = h.convertTestRunToAPI(tr)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   response,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// getCurrentUser returns the current authenticated user information
func (h *DomainHandler) getCurrentUser(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	authUser, ok := user.(*authDomain.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user data"})
		return
	}

	// Extract groups/teams from the user's groups
	var teams []string
	for _, group := range authUser.Groups {
		teams = append(teams, group.GroupName)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":    authUser.UserID,
		"email": authUser.Email,
		"name":  authUser.Name,
		"role":  string(authUser.Role),
		"teams": teams,
	})
}

// Project Handlers

func (h *DomainHandler) getProjects(c *gin.Context) {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Get projects
	projects, total, err := h.projectService.ListProjects(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to API response
	response := make([]gin.H, len(projects))
	for i, p := range projects {
		response[i] = h.convertProjectToAPI(p)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   response,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *DomainHandler) createProject(c *gin.Context) {
	var req struct {
		ProjectID     string                 `json:"projectId"`
		Name          string                 `json:"name" binding:"required"`
		Description   string                 `json:"description"`
		Repository    string                 `json:"repository"`
		DefaultBranch string                 `json:"defaultBranch"`
		Team          string                 `json:"team" binding:"required"`
		Settings      map[string]interface{} `json:"settings"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate project ID if not provided
	projectID := req.ProjectID
	if projectID == "" {
		projectID = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}

	// Get current user ID for creator
	user, _ := c.Get("user")
	var creatorID string
	if authUser, ok := user.(*authDomain.User); ok {
		creatorID = authUser.UserID
	}

	// Create project
	project, err := h.projectService.CreateProject(
		c.Request.Context(),
		projectsDomain.ProjectID(projectID),
		req.Name,
		projectsDomain.Team(req.Team),
		creatorID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update additional fields if provided
	if req.Description != "" || req.Repository != "" || req.DefaultBranch != "" || req.Settings != nil {
		updateReq := projectsApp.UpdateProjectRequest{
			Description:   &req.Description,
			Repository:    &req.Repository,
			DefaultBranch: &req.DefaultBranch,
			Settings:      req.Settings,
		}
		if err := h.projectService.UpdateProject(c.Request.Context(), project.ProjectID(), updateReq); err != nil {
			// Log error but don't fail the creation
			h.logger.WithError(err).Error("Failed to update project details after creation")
		}
	}

	response := h.convertProjectToAPI(project)
	c.JSON(http.StatusCreated, response)
}

func (h *DomainHandler) getTestRun(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test run ID"})
		return
	}

	testRun, err := h.testingService.GetTestRun(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	c.JSON(http.StatusOK, h.convertTestRunToAPI(testRun))
}

func (h *DomainHandler) getTestRunByRunId(c *gin.Context) {
	runID := c.Param("id")
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get test run by run ID not yet implemented"})
	_ = runID // TODO: Implement
}

func (h *DomainHandler) deleteTestRun(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Delete test run not yet implemented"})
}

func (h *DomainHandler) getSuiteRuns(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get suite runs not yet implemented"})
}

func (h *DomainHandler) getSuiteRun(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get suite run not yet implemented"})
}

func (h *DomainHandler) getSpecRuns(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get spec runs not yet implemented"})
}

func (h *DomainHandler) getSpecRun(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get spec run not yet implemented"})
}

func (h *DomainHandler) getProject(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get project not yet implemented"})
}

func (h *DomainHandler) getProjectByProjectId(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get project by project ID not yet implemented"})
}

func (h *DomainHandler) updateProject(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Update project not yet implemented"})
}

func (h *DomainHandler) deleteProject(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Delete project not yet implemented"})
}

func (h *DomainHandler) getTags(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get tags not yet implemented"})
}

func (h *DomainHandler) getTag(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get tag not yet implemented"})
}

func (h *DomainHandler) getFlakyTests(c *gin.Context) {
	// This method is deprecated - use domain_handler_v2.go
	c.JSON(http.StatusNotImplemented, gin.H{"error": "This endpoint is deprecated"})
}

func (h *DomainHandler) resolveFlakyTest(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Resolve flaky test not yet implemented"})
}

func (h *DomainHandler) ignoreFlakyTest(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Ignore flaky test not yet implemented"})
}

// Conversion helpers

func (h *DomainHandler) convertTestRunToAPI(tr *testingDomain.TestRun) gin.H {
	// Convert test run to API response
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
		"metadata":     tr.Metadata,
	}
}

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

// convertFlakyTestToAPI is deprecated - flaky test types have changed
// func (h *DomainHandler) convertFlakyTestToAPI(ft *testingDomain.FlakyTest) gin.H {
// 	return gin.H{}
// }

// JIRA Connection Handlers

func (h *DomainHandler) getJiraConnections(c *gin.Context) {
	projectID := c.Param("id")

	connections, err := h.jiraConnectionService.GetProjectConnections(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to API response
	response := make([]gin.H, len(connections))
	for i, conn := range connections {
		snapshot := conn.Snapshot()
		response[i] = gin.H{
			"id":                 snapshot.ID,
			"projectId":          snapshot.ProjectID,
			"name":               snapshot.Name,
			"jiraUrl":            snapshot.JiraURL,
			"authenticationType": string(snapshot.AuthenticationType),
			"projectKey":         snapshot.ProjectKey,
			"username":           snapshot.Username,
			"status":             string(snapshot.Status),
			"isActive":           snapshot.IsActive,
			"lastTestedAt":       snapshot.LastTestedAt,
			"createdAt":          snapshot.CreatedAt,
			"updatedAt":          snapshot.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *DomainHandler) createJiraConnection(c *gin.Context) {
	projectID := c.Param("id")

	var req struct {
		Name               string `json:"connectionName" binding:"required"`
		JiraURL            string `json:"jiraUrl" binding:"required,url"`
		AuthenticationType string `json:"authenticationType" binding:"required,oneof=api_token personal_access_token basic"`
		ProjectKey         string `json:"projectKey" binding:"required"`
		Username           string `json:"username"`
		Credential         string `json:"apiToken" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create the connection
	connection, err := h.jiraConnectionService.CreateConnection(
		c.Request.Context(),
		projectID,
		req.Name,
		req.JiraURL,
		integrations.AuthenticationType(req.AuthenticationType),
		req.ProjectKey,
		req.Username,
		req.Credential,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create JIRA connection: %v", err)})
		return
	}

	// Return the created connection
	snapshot := connection.Snapshot()
	c.JSON(http.StatusCreated, gin.H{
		"id":                 snapshot.ID,
		"projectId":          snapshot.ProjectID,
		"name":               snapshot.Name,
		"jiraUrl":            snapshot.JiraURL,
		"authenticationType": string(snapshot.AuthenticationType),
		"projectKey":         snapshot.ProjectKey,
		"username":           snapshot.Username,
		"status":             string(snapshot.Status),
		"isActive":           snapshot.IsActive,
		"lastTestedAt":       snapshot.LastTestedAt,
		"createdAt":          snapshot.CreatedAt,
		"updatedAt":          snapshot.UpdatedAt,
	})
}

func (h *DomainHandler) updateJiraConnection(c *gin.Context) {
	connectionID := c.Param("connectionId")

	var req struct {
		Name       string `json:"name"`
		JiraURL    string `json:"jiraUrl"`
		ProjectKey string `json:"projectKey"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update the connection
	updated, err := h.jiraConnectionService.UpdateConnection(
		c.Request.Context(),
		connectionID,
		req.Name,
		req.JiraURL,
		req.ProjectKey,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update JIRA connection: %v", err)})
		return
	}

	// Return the updated connection
	snapshot := updated.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		"id":                 snapshot.ID,
		"projectId":          snapshot.ProjectID,
		"name":               snapshot.Name,
		"jiraUrl":            snapshot.JiraURL,
		"authenticationType": string(snapshot.AuthenticationType),
		"projectKey":         snapshot.ProjectKey,
		"username":           snapshot.Username,
		"status":             string(snapshot.Status),
		"isActive":           snapshot.IsActive,
		"lastTestedAt":       snapshot.LastTestedAt,
		"createdAt":          snapshot.CreatedAt,
		"updatedAt":          snapshot.UpdatedAt,
	})
}

func (h *DomainHandler) updateJiraCredentials(c *gin.Context) {
	connectionID := c.Param("connectionId")

	var req struct {
		AuthenticationType string `json:"authenticationType" binding:"required,oneof=api_token personal_access_token basic"`
		Username           string `json:"username"`
		Credential         string `json:"credential" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update credentials
	updated, err := h.jiraConnectionService.UpdateCredentials(
		c.Request.Context(),
		connectionID,
		integrations.AuthenticationType(req.AuthenticationType),
		req.Username,
		req.Credential,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update credentials: %v", err)})
		return
	}

	// Return the updated connection
	snapshot := updated.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		"id":                 snapshot.ID,
		"projectId":          snapshot.ProjectID,
		"name":               snapshot.Name,
		"jiraUrl":            snapshot.JiraURL,
		"authenticationType": string(snapshot.AuthenticationType),
		"projectKey":         snapshot.ProjectKey,
		"username":           snapshot.Username,
		"status":             string(snapshot.Status),
		"isActive":           snapshot.IsActive,
		"lastTestedAt":       snapshot.LastTestedAt,
		"createdAt":          snapshot.CreatedAt,
		"updatedAt":          snapshot.UpdatedAt,
	})
}

func (h *DomainHandler) testJiraConnection(c *gin.Context) {
	connectionID := c.Param("connectionId")

	// Test the connection
	if err := h.jiraConnectionService.TestConnection(c.Request.Context(), connectionID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Connection test failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Connection test successful"})
}

func (h *DomainHandler) deleteJiraConnection(c *gin.Context) {
	connectionID := c.Param("connectionId")

	// Delete the connection
	if err := h.jiraConnectionService.DeleteConnection(c.Request.Context(), connectionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete connection: %v", err)})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
