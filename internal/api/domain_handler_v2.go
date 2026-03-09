// Package api provides domain-based REST API handlers
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	analyticsApp "github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/interfaces"
	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	summaryInterfaces "github.com/guidewire-oss/fern-platform/internal/domains/summary/interfaces"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// DomainHandlerV2 provides REST API handlers using domain services with split handlers
type DomainHandlerV2 struct {
	// Sub-handlers
	authHandler           *AuthHandler
	healthHandler         *HealthHandler
	testRunHandler        *TestRunHandler
	projectHandler        *ProjectHandler
	tagHandler            *TagHandler
	systemHandler         *SystemHandler
	jiraConnectionHandler *JiraConnectionHandler
	summaryHandler        *summaryInterfaces.SummaryHandler

	// Services used for inline route handlers
	flakyDetectionService *analyticsApp.FlakyDetectionService

	// Middleware
	authMiddleware *interfaces.AuthMiddlewareAdapter
	logger         *logging.Logger
}

// NewDomainHandlerV2 creates a new domain-based API handler with split handlers
func NewDomainHandlerV2(
	testingService *application.TestRunService,
	projectService *projectsApp.ProjectService,
	tagService *tagsApp.TagService,
	flakyDetectionService *analyticsApp.FlakyDetectionService,
	jiraConnectionService *integrations.JiraConnectionService,
	summaryHandler *summaryInterfaces.SummaryHandler,
	authMiddleware *interfaces.AuthMiddlewareAdapter,
	logger *logging.Logger,
) *DomainHandlerV2 {
	baseHandler := NewBaseHandler(logger)
	return &DomainHandlerV2{
		authHandler:           NewAuthHandler(authMiddleware, logger),
		healthHandler:         NewHealthHandler(logger),
		testRunHandler:        NewTestRunHandler(testingService, tagService, logger),
		projectHandler:        NewProjectHandler(projectService, logger),
		tagHandler:            NewTagHandler(tagService, logger),
		systemHandler:         NewSystemHandler(logger),
		jiraConnectionHandler: NewJiraConnectionHandler(baseHandler, jiraConnectionService, projectService),
		summaryHandler:        summaryHandler,
		flakyDetectionService: flakyDetectionService,
		authMiddleware:        authMiddleware,
		logger:                logger,
	}
}

// RegisterRoutes registers API routes with the Gin router using split handlers
func (h *DomainHandlerV2) RegisterRoutes(router *gin.Engine) {
	// Static file serving for web interface
	router.Static("/web", "./web")
	router.Static("/docs", "./docs")

	// Root route - redirect to login if not authenticated, otherwise serve app
	router.GET("/", func(c *gin.Context) {
		if !h.isUserAuthenticated(c) {
			c.Redirect(302, "/auth/login")
			return
		}
		c.File("./web/index.html")
	})

	// OAuth authentication routes
	authGroup := router.Group("/auth")

	// API v1 routes
	v1 := router.Group("/api/v1")

	// Public routes (no authentication required)
	publicGroup := v1.Group("")
	h.healthHandler.RegisterRoutes(publicGroup)

	// User routes (require authentication)
	userGroup := v1.Group("")
	userGroup.Use(h.authMiddleware.RequireAuth())

	// Manager routes (require manager role - admin or team manager)
	managerGroup := v1.Group("")
	managerGroup.Use(h.authMiddleware.RequireManager())

	// Admin routes (require admin role)
	adminGroup := v1.Group("/admin")
	adminGroup.Use(h.authMiddleware.RequireAdmin())

	// Register all handler routes
	h.authHandler.RegisterRoutes(router, authGroup, userGroup, adminGroup)
	h.testRunHandler.RegisterRoutes(publicGroup, userGroup, adminGroup)
	h.projectHandler.RegisterRoutes(userGroup, managerGroup, adminGroup)
	h.tagHandler.RegisterRoutes(userGroup, adminGroup)
	h.systemHandler.RegisterRoutes(adminGroup)

	// Summary routes
	userGroup.GET("/summary/:projectId/:seed", h.summaryHandler.GetSummary)

	// Flaky test routes
	userGroup.GET("/flaky-tests", h.getFlakyTests)
	userGroup.POST("/flaky-tests/:id/resolve", h.resolveFlakyTest)
	userGroup.POST("/flaky-tests/:id/ignore", h.ignoreFlakyTest)

	// JIRA connection routes (new path: /projects/:projectId/integrations/jira/...)
	h.registerJiraConnectionRoutes(managerGroup)

	// JIRA backward-compatible routes (old path: /projects/:id/jira-connections)
	h.registerJiraConnectionRoutesLegacy(managerGroup)

	h.logger.Info("All routes registered successfully with split handlers")
}

// isUserAuthenticated checks if the user is authenticated
func (h *DomainHandlerV2) isUserAuthenticated(c *gin.Context) bool {
	sessionID, err := c.Cookie("session_id")
	return err == nil && sessionID != ""
}

// registerJiraConnectionRoutes registers JIRA connection routes under the new path structure
func (h *DomainHandlerV2) registerJiraConnectionRoutes(managerGroup *gin.RouterGroup) {
	jira := managerGroup.Group("/projects/:projectId/integrations/jira")
	{
		jira.GET("/connections", h.jiraConnectionHandler.GetConnections)
		jira.POST("/connections", h.jiraConnectionHandler.CreateConnection)
		jira.PUT("/connections/:connectionId", h.jiraConnectionHandler.UpdateConnection)
		jira.PUT("/connections/:connectionId/credentials", h.jiraConnectionHandler.UpdateCredentials)
		jira.POST("/connections/:connectionId/test", h.jiraConnectionHandler.TestConnection)
		jira.DELETE("/connections/:connectionId", h.jiraConnectionHandler.DeleteConnection)
	}
}

// registerJiraConnectionRoutesLegacy registers JIRA routes at the legacy v1 paths for backward compatibility.
// Uses :projectId (not :id) to avoid wildcard conflicts with project routes.
func (h *DomainHandlerV2) registerJiraConnectionRoutesLegacy(managerGroup *gin.RouterGroup) {
	// Legacy: /projects/:projectId/jira-connections (old flat path)
	managerGroup.GET("/projects/:projectId/jira-connections", h.jiraConnectionHandler.GetConnections)
	managerGroup.POST("/projects/:projectId/jira-connections", h.jiraConnectionHandler.CreateConnection)
	// Legacy: /jira-connections/:connectionId (flat path from old handler)
	managerGroup.PUT("/jira-connections/:connectionId", h.jiraConnectionHandler.UpdateConnection)
	managerGroup.PUT("/jira-connections/:connectionId/credentials", h.jiraConnectionHandler.UpdateCredentials)
	managerGroup.POST("/jira-connections/:connectionId/test", h.jiraConnectionHandler.TestConnection)
	managerGroup.DELETE("/jira-connections/:connectionId", h.jiraConnectionHandler.DeleteConnection)
}

// getFlakyTests handles GET /api/v1/flaky-tests
func (h *DomainHandlerV2) getFlakyTests(c *gin.Context) {
	projectID := c.Query("projectId")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "projectId query parameter is required"})
		return
	}

	flakyTests, err := h.flakyDetectionService.GetFlakyTests(c.Request.Context(), projectID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get flaky tests")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, flakyTests)
}

// resolveFlakyTest handles POST /api/v1/flaky-tests/:id/resolve
func (h *DomainHandlerV2) resolveFlakyTest(c *gin.Context) {
	testID := c.Param("id")

	if err := h.flakyDetectionService.MarkTestResolved(c.Request.Context(), testID); err != nil {
		h.logger.WithError(err).Error("Failed to resolve flaky test")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Flaky test resolved successfully"})
}

// ignoreFlakyTest handles POST /api/v1/flaky-tests/:id/ignore
func (h *DomainHandlerV2) ignoreFlakyTest(c *gin.Context) {
	testID := c.Param("id")

	if err := h.flakyDetectionService.IgnoreTest(c.Request.Context(), testID); err != nil {
		h.logger.WithError(err).Error("Failed to ignore flaky test")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Flaky test ignored successfully"})
}

// Backward compatibility - delegate to sub-handlers

func (h *DomainHandlerV2) healthCheck(c *gin.Context) {
	h.healthHandler.healthCheck(c)
}

func (h *DomainHandlerV2) getCurrentUser(c *gin.Context) {
	h.authHandler.getCurrentUser(c)
}

func (h *DomainHandlerV2) createTestRun(c *gin.Context) {
	h.testRunHandler.createTestRun(c)
}

func (h *DomainHandlerV2) getTestRun(c *gin.Context) {
	h.testRunHandler.getTestRun(c)
}

func (h *DomainHandlerV2) createProject(c *gin.Context) {
	h.projectHandler.createProject(c)
}

func (h *DomainHandlerV2) getProject(c *gin.Context) {
	h.projectHandler.getProject(c)
}

func (h *DomainHandlerV2) createTag(c *gin.Context) {
	h.tagHandler.createTag(c)
}

func (h *DomainHandlerV2) getTag(c *gin.Context) {
	h.tagHandler.getTag(c)
}
