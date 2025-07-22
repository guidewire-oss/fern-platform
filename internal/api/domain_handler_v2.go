// Package api provides domain-based REST API handlers
package api

import (
	"github.com/gin-gonic/gin"
	analyticsApp "github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/interfaces"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// DomainHandlerV2 provides REST API handlers using domain services with split handlers
type DomainHandlerV2 struct {
	// Sub-handlers
	authHandler       *AuthHandler
	healthHandler     *HealthHandler
	testRunHandler    *TestRunHandler
	projectHandler    *ProjectHandler
	tagHandler        *TagHandler
	systemHandler     *SystemHandler
	fernLegacyHandler *FernLegacyHandler

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
	authMiddleware *interfaces.AuthMiddlewareAdapter,
	logger *logging.Logger,
) *DomainHandlerV2 {
	return &DomainHandlerV2{
		authHandler:       NewAuthHandler(authMiddleware, logger),
		healthHandler:     NewHealthHandler(logger),
		testRunHandler:    NewTestRunHandler(testingService, logger),
		projectHandler:    NewProjectHandler(projectService, logger),
		tagHandler:        NewTagHandler(tagService, logger),
		systemHandler:     NewSystemHandler(logger),
		fernLegacyHandler: NewFernLegacyHandler(testingService, projectService, logger),
		authMiddleware:    authMiddleware,
		logger:            logger,
	}
}

// RegisterRoutes registers API routes with the Gin router using split handlers
func (h *DomainHandlerV2) RegisterRoutes(router *gin.Engine) {
	// Static file serving for web interface
	router.Static("/web", "./web")
	router.Static("/docs", "./docs")

	// Root route - redirect to login if not authenticated, otherwise serve app
	router.GET("/", func(c *gin.Context) {
		// Check if user is authenticated
		if !h.isUserAuthenticated(c) {
			// Redirect to login
			c.Redirect(302, "/auth/login")
			return
		}
		// Serve the main application
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
	h.testRunHandler.RegisterRoutes(userGroup, adminGroup)
	h.projectHandler.RegisterRoutes(userGroup, managerGroup, adminGroup)
	h.tagHandler.RegisterRoutes(userGroup, adminGroup)
	h.systemHandler.RegisterRoutes(adminGroup)

	// Legacy fern-reporter compatible API endpoints
	apiGroup := router.Group("/api")
	h.fernLegacyHandler.RegisterRoutes(apiGroup)

	// Log route registration
	h.logger.Info("All routes registered successfully with split handlers")
}

// Backward compatibility - delegate to sub-handlers
// These methods allow existing code to continue working

// healthCheck delegates to health handler
func (h *DomainHandlerV2) healthCheck(c *gin.Context) {
	h.healthHandler.healthCheck(c)
}

// getCurrentUser delegates to auth handler
func (h *DomainHandlerV2) getCurrentUser(c *gin.Context) {
	h.authHandler.getCurrentUser(c)
}

// createTestRun delegates to test run handler
func (h *DomainHandlerV2) createTestRun(c *gin.Context) {
	h.testRunHandler.createTestRun(c)
}

// getTestRun delegates to test run handler
func (h *DomainHandlerV2) getTestRun(c *gin.Context) {
	h.testRunHandler.getTestRun(c)
}

// createProject delegates to project handler
func (h *DomainHandlerV2) createProject(c *gin.Context) {
	h.projectHandler.createProject(c)
}

// getProject delegates to project handler
func (h *DomainHandlerV2) getProject(c *gin.Context) {
	h.projectHandler.getProject(c)
}

// createTag delegates to tag handler
func (h *DomainHandlerV2) createTag(c *gin.Context) {
	h.tagHandler.createTag(c)
}

// getTag delegates to tag handler
func (h *DomainHandlerV2) getTag(c *gin.Context) {
	h.tagHandler.getTag(c)
}

// Helper methods

func (h *DomainHandlerV2) isUserAuthenticated(c *gin.Context) bool {
	sessionID, err := c.Cookie("session_id")
	return err == nil && sessionID != ""
}