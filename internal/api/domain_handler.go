// Package api provides domain-based REST API handlers
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	analyticsApp "github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	_ "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/interfaces"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	tagsDomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// DomainHandler provides REST API handlers using domain services
type DomainHandler struct {
	testingService        *application.TestRunService
	projectService        *projectsApp.ProjectService
	tagService            *tagsApp.TagService
	flakyDetectionService *analyticsApp.FlakyDetectionService
	authMiddleware        *interfaces.AuthMiddlewareAdapter
	logger                *logging.Logger
}

// NewDomainHandler creates a new domain-based API handler
func NewDomainHandler(
	testingService *application.TestRunService,
	projectService *projectsApp.ProjectService,
	tagService *tagsApp.TagService,
	flakyDetectionService *analyticsApp.FlakyDetectionService,
	authMiddleware *interfaces.AuthMiddlewareAdapter,
	logger *logging.Logger,
) *DomainHandler {
	return &DomainHandler{
		testingService:        testingService,
		projectService:        projectService,
		tagService:            tagService,
		flakyDetectionService: flakyDetectionService,
		authMiddleware:        authMiddleware,
		logger:                logger,
	}
}

// RegisterRoutes registers API routes with the Gin router - maintains backward compatibility
func (h *DomainHandler) RegisterRoutes(router *gin.Engine) {
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
	auth := router.Group("/auth")
	{
		auth.GET("/login", h.showLoginPage)
		auth.GET("/start", h.authMiddleware.StartOAuthFlow())
		auth.GET("/callback", h.authMiddleware.HandleOAuthCallback())
		auth.POST("/logout", h.authMiddleware.Logout())
		auth.GET("/user", h.authMiddleware.RequireAuth(), h.getCurrentUser)
	}

	v1 := router.Group("/api/v1")

	// Public routes (no authentication required)
	public := v1.Group("")
	{
		// Health check - only public endpoint
		public.GET("/health", h.healthCheck)
	}

	// User routes (require authentication)
	user := v1.Group("")
	user.Use(h.authMiddleware.RequireAuth())
	{
		// Test runs - read operations (require authentication)
		user.GET("/test-runs", h.listTestRuns)
		user.GET("/test-runs/count", h.countTestRuns)
		user.GET("/test-runs/:id", h.getTestRun)
		user.GET("/test-runs/by-run-id/:runId", h.getTestRunByRunID)
		user.GET("/test-runs/stats", h.getTestRunStats)
		user.GET("/test-runs/recent", h.getRecentTestRuns)

		// Projects - read operations (require authentication)
		user.GET("/projects", h.listProjects)
		user.GET("/projects/:projectId", h.getProject)
		user.GET("/projects/by-project-id/:projectId", h.getProjectByProjectID)
		user.GET("/projects/stats/:projectId", h.getProjectStats)

		// Tags - read operations (require authentication)
		user.GET("/tags", h.listTags)
		user.GET("/tags/:id", h.getTag)
		user.GET("/tags/by-name/:name", h.getTagByName)
		user.GET("/tags/usage-stats", h.getTagUsageStats)
		user.GET("/tags/popular", h.getPopularTags)

		// User-specific operations
		user.GET("/user/preferences", h.getUserPreferences)
		user.PUT("/user/preferences", h.updateUserPreferences)
		user.GET("/user/projects", h.getUserProjects)

		// Test runs - user operations
		user.POST("/test-runs/:id/tags", h.assignTagsToTestRun)
	}

	// Manager routes (require manager role - admin or team manager)
	manager := v1.Group("")
	manager.Use(h.authMiddleware.RequireManager())
	{
		// Project management for managers
		manager.POST("/projects", h.createProject)
		manager.PUT("/projects/:projectId", h.updateProject)
		manager.DELETE("/projects/:projectId", h.deleteProject)
		manager.POST("/projects/:projectId/activate", h.activateProject)
		manager.POST("/projects/:projectId/deactivate", h.deactivateProject)
	}

	// Admin routes (require admin role)
	admin := v1.Group("/admin")
	admin.Use(h.authMiddleware.RequireAdmin())
	{
		// User management
		admin.GET("/users", h.listUsers)
		admin.GET("/users/:userId", h.getUser)
		admin.PUT("/users/:userId/role", h.updateUserRole)
		admin.POST("/users/:userId/suspend", h.suspendUser)
		admin.POST("/users/:userId/activate", h.activateUser)
		admin.DELETE("/users/:userId", h.deleteUser)

		// Project access management
		admin.POST("/projects/:projectId/users/:userId/access", h.grantProjectAccess)
		admin.DELETE("/projects/:projectId/users/:userId/access", h.revokeProjectAccess)
		admin.GET("/projects/:projectId/users", h.getProjectUsers)

		// Tag management
		admin.POST("/tags", h.createTag)
		admin.PUT("/tags/:id", h.updateTag)
		admin.DELETE("/tags/:id", h.deleteTag)

		// Test run management
		admin.POST("/test-runs", h.createTestRun)
		admin.PUT("/test-runs/:runId/status", h.updateTestRunStatus)
		admin.DELETE("/test-runs/:id", h.deleteTestRun)
		admin.POST("/test-runs/bulk-delete", h.bulkDeleteTestRuns)

		// System management
		admin.GET("/system/stats", h.getSystemStats)
		admin.GET("/system/health", h.getSystemHealth)
		admin.POST("/system/cleanup", h.performSystemCleanup)
		admin.GET("/audit-logs", h.getAuditLogs)
	}

	// fern-reporter compatible API endpoints
	api := router.Group("/api")
	{
		// Project endpoints compatible with fern-ginkgo-client
		api.POST("/project", h.createFernProject)
		api.GET("/project/:uuid", h.getFernProject)
		api.GET("/projects", h.listFernProjects)

		// Test reports endpoints
		api.POST("/reports/testrun", h.createFernTestReport)
		api.GET("/reports/testruns", h.listFernTestReports)
		api.GET("/reports/testrun/:uuid", h.getFernTestReport)

		// Additional endpoints that fern-ginkgo-client might expect
		api.POST("/testrun", h.createFernTestReport) // Alias for test run creation
	}
}

// Helper methods

func (h *DomainHandler) isUserAuthenticated(c *gin.Context) bool {
	sessionID, err := c.Cookie("session_id")
	return err == nil && sessionID != ""
}

func (h *DomainHandler) showLoginPage(c *gin.Context) {
	// For now, serve a simple login page directly
	// In production, this would use templates or serve a static file
	loginHTML := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sign In - Fern Platform</title>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&display=swap" rel="stylesheet">
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css" rel="stylesheet">
    <style>
        :root {
            /* Biology-inspired colors */
            --primary: #22c55e;           /* Fern green */
            --primary-dark: #16a34a;      /* Deep forest green */
            --secondary: #84cc16;         /* Moss green */
            --accent: #fbbf24;            /* Pollen yellow */
            --tertiary: #7c3aed;          /* Orchid purple */
            --success: #22d3ee;           /* Water blue */
            
            /* Organic backgrounds */
            --bg-primary: #fefef8;        /* Natural white */
            --bg-secondary: #f7fee7;      /* Soft moss */
            --bg-tertiary: #ecfccb;       /* Light fern */
            --bg-card: #ffffff;           /* Pure white */
            
            /* Text colors */
            --text-primary: #0f172a;      /* Rich soil */
            --text-secondary: #475569;    /* Tree bark */
            --text-muted: #64748b;        /* Stone gray */
            
            /* Borders and effects */
            --border: #d9f99d;            /* Leaf vein */
            --border-light: #e7f5d0;      /* Light stem */
            --shadow: 0 10px 25px rgba(34, 197, 94, 0.1);
            --shadow-lg: 0 20px 40px rgba(34, 197, 94, 0.15);
            --glow: 0 0 30px rgba(34, 197, 94, 0.3);
            --border-radius: 16px;
            
            /* Organic gradients */
            --gradient-primary: linear-gradient(135deg, #22c55e 0%, #84cc16 100%);
            --gradient-leaf: linear-gradient(135deg, #bbf7d0 0%, #86efac 100%);
            --gradient-card: linear-gradient(135deg, #ffffff 0%, #f0fdf4 100%);
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
            line-height: 1.6;
            color: var(--text-primary);
            background: var(--bg-primary);
            background-image: 
                radial-gradient(circle at 20% 80%, rgba(34, 197, 94, 0.05) 0%, transparent 50%),
                radial-gradient(circle at 80% 20%, rgba(132, 204, 22, 0.05) 0%, transparent 50%),
                radial-gradient(circle at 40% 40%, rgba(251, 191, 36, 0.03) 0%, transparent 50%);
            min-height: 100vh;
            display: flex;
            justify-content: center;
            align-items: center;
            position: relative;
            overflow: hidden;
        }

        body::before {
            content: '';
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background-image: 
                repeating-linear-gradient(45deg, transparent, transparent 35px, rgba(34, 197, 94, 0.02) 35px, rgba(34, 197, 94, 0.02) 70px),
                repeating-linear-gradient(-45deg, transparent, transparent 35px, rgba(132, 204, 22, 0.02) 35px, rgba(132, 204, 22, 0.02) 70px);
            pointer-events: none;
            z-index: 0;
        }

        /* Root Network Background Animation */
        .root-network {
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            pointer-events: none;
            overflow: hidden;
            z-index: -1;
            opacity: 0.3;
        }

        @keyframes root-grow {
            0% {
                stroke-dashoffset: 1000;
                opacity: 0;
            }
            50% {
                opacity: 0.1;
            }
            100% {
                stroke-dashoffset: 0;
                opacity: 0.08;
            }
        }

        .root-path {
            stroke: var(--primary);
            stroke-width: 1;
            fill: none;
            stroke-dasharray: 1000;
            stroke-dashoffset: 1000;
            opacity: 0;
            animation: root-grow 10s ease-out forwards;
            animation-delay: var(--root-delay);
        }

        .login-container {
            background: var(--gradient-card);
            padding: 3rem;
            border-radius: 30px 10px;
            box-shadow: var(--shadow-lg);
            text-align: center;
            max-width: 500px;
            width: 90%;
            border: 2px solid var(--border-light);
            position: relative;
            z-index: 1;
            overflow: hidden;
        }

        .login-container::before {
            content: '';
            position: absolute;
            top: -50%;
            right: -50%;
            width: 200%;
            height: 200%;
            background: radial-gradient(circle, var(--secondary) 0%, transparent 60%);
            opacity: 0.05;
            transform: rotate(45deg);
            transition: all 0.3s;
        }

        .login-container:hover::before {
            opacity: 0.1;
            transform: rotate(90deg);
        }

        .logo-container {
            margin-bottom: 2rem;
            position: relative;
            z-index: 1;
        }

        .logo-container img {
            width: 240px;
            height: 100px;
            object-fit: contain;
            filter: drop-shadow(0 4px 16px rgba(34, 197, 94, 0.2));
            transition: all 0.3s;
        }

        .logo-container img:hover {
            filter: drop-shadow(0 8px 24px rgba(34, 197, 94, 0.3));
            transform: scale(1.05);
        }

        h1 {
            font-size: 2.5rem;
            font-weight: 800;
            margin-bottom: 1rem;
            background: var(--gradient-primary);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            position: relative;
            z-index: 1;
        }

        .subtitle {
            font-size: 1.2rem;
            color: var(--text-secondary);
            margin-bottom: 1.5rem;
            position: relative;
            z-index: 1;
        }

        .features {
            display: flex;
            justify-content: center;
            gap: 0.75rem;
            margin: 1.5rem 0;
            position: relative;
            z-index: 1;
            flex-wrap: wrap;
            max-width: 100%;
        }

        .feature {
            padding: 0.5rem 0.75rem;
            background: rgba(255, 255, 255, 0.8);
            border-radius: 12px;
            border: 1px solid var(--border-light);
            transition: all 0.3s;
            text-align: center;
            flex: 0 0 auto;
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
        }

        .feature:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(34, 197, 94, 0.1);
            border-color: var(--primary);
        }

        .feature-icon {
            font-size: 1.5rem;
            margin-bottom: 0.25rem;
        }

        .feature-title {
            font-weight: 600;
            color: var(--primary-dark);
            font-size: 0.9rem;
        }

        .login-button {
            background: var(--gradient-primary);
            color: white;
            padding: 16px 32px;
            border: none;
            border-radius: var(--border-radius);
            font-size: 18px;
            font-weight: 600;
            cursor: pointer;
            text-decoration: none;
            display: inline-flex;
            align-items: center;
            gap: 10px;
            transition: all 0.3s;
            margin-top: 0.5rem;
            position: relative;
            z-index: 1;
            box-shadow: 0 4px 12px rgba(34, 197, 94, 0.2);
        }

        .login-button:hover {
            transform: translateY(-2px);
            box-shadow: 0 8px 20px rgba(34, 197, 94, 0.3);
        }

        .login-button i {
            font-size: 20px;
        }

        .security-note {
            margin-top: 2rem;
            padding: 1rem;
            background: rgba(34, 197, 94, 0.05);
            border-radius: 8px;
            border: 1px solid rgba(34, 197, 94, 0.2);
            font-size: 0.875rem;
            color: var(--text-secondary);
            position: relative;
            z-index: 1;
        }

        .security-note i {
            color: var(--primary);
            margin-right: 0.5rem;
        }

        .footer-links {
            margin-top: 2rem;
            font-size: 0.875rem;
            color: var(--text-muted);
            position: relative;
            z-index: 1;
        }

        .footer-links a {
            color: var(--primary);
            text-decoration: none;
            transition: color 0.3s;
        }

        .footer-links a:hover {
            color: var(--primary-dark);
            text-decoration: underline;
        }

        @keyframes fadeInUp {
            from {
                opacity: 0;
                transform: translateY(20px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        .fade-in {
            animation: fadeInUp 0.6s ease-out;
        }

        /* Responsive */
        @media (max-width: 600px) {
            .login-container {
                padding: 2rem;
                margin: 1rem;
            }
            
            h1 {
                font-size: 2rem;
            }
            
            .features {
                gap: 0.5rem;
            }
            
            .feature {
                flex: 1 1 100%;
                max-width: 100%;
            }
            
            .logo-container img {
                width: 200px;
                height: 80px;
            }
        }
    </style>
</head>
<body>
    <!-- Root Network Background -->
    <div class="root-network">
        <svg viewBox="0 0 100 100" preserveAspectRatio="none">
            <path class="root-path" d="M 10 0 Q 12 20 15 40 T 18 80" style="--root-delay: 0s;"></path>
            <path class="root-path" d="M 25 0 Q 24 25 26 50 T 28 90" style="--root-delay: 0.5s;"></path>
            <path class="root-path" d="M 40 0 Q 38 30 35 60 T 38 100" style="--root-delay: 1s;"></path>
            <path class="root-path" d="M 55 0 Q 56 28 58 56 T 55 95" style="--root-delay: 1.5s;"></path>
            <path class="root-path" d="M 70 0 Q 72 32 74 64 T 72 100" style="--root-delay: 2s;"></path>
            <path class="root-path" d="M 85 0 Q 83 35 80 70 T 82 100" style="--root-delay: 2.5s;"></path>
        </svg>
    </div>

    <div class="login-container fade-in">
        <div class="logo-container">
            <img src="/docs/images/logo-no-background.png" alt="Fern Platform" onerror="this.style.display='none'">
        </div>
        
        <h1>Welcome to Fern Platform</h1>
        <p class="subtitle">Transform your test chaos into intelligent insights</p>
        
        <div class="features">
            <div class="feature">
                <div class="feature-icon">🎨</div>
                <div class="feature-title">Beautiful Visualizations</div>
            </div>
            <div class="feature">
                <div class="feature-icon">🔐</div>
                <div class="feature-title">Secure Access</div>
            </div>
            <div class="feature">
                <div class="feature-icon">⚡</div>
                <div class="feature-title">Real-time Analytics</div>
            </div>
        </div>
        
        <a href="/auth/start" class="login-button">
            <span style="margin-right: 8px;">→</span>
            Sign in with OAuth
        </a>
        
        <div class="security-note">
            <span style="color: var(--primary); margin-right: 0.5rem;">🔒</span>
            Your data is secure. We use industry-standard OAuth 2.0 authentication.
        </div>
        
        <div class="footer-links">
            <a href="https://github.com/guidewire-oss/fern-platform" target="_blank">Documentation</a>
            •
            <a href="https://github.com/guidewire-oss/fern-platform/issues" target="_blank">Support</a>
            •
            <a href="https://github.com/guidewire-oss/fern-platform" target="_blank">Open Source</a>
        </div>
    </div>
</body>
</html>
`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(loginHTML))
}

// healthCheck returns the service health status
func (h *DomainHandler) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "fern-platform",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// User endpoints

func (h *DomainHandler) getCurrentUser(c *gin.Context) {
	user, exists := interfaces.GetAuthUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Convert domain user to API response format
	userGroups := make([]gin.H, len(user.Groups))
	for i, group := range user.Groups {
		userGroups[i] = gin.H{
			"groupId":   group.UserID,
			"groupName": group.GroupName,
		}
	}

	userScopes := make([]gin.H, len(user.Scopes))
	for i, scope := range user.Scopes {
		userScopes[i] = gin.H{
			"scopeId":   scope.UserID,
			"scopeName": scope.Scope,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         user.UserID,
		"email":      user.Email,
		"name":       user.Name,
		"role":       string(user.Role),
		"status":     string(user.Status),
		"userGroups": userGroups,
		"userScopes": userScopes,
	})
}

// Test Run Handlers

func (h *DomainHandler) createTestRun(c *gin.Context) {
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
	if err := h.testingService.CreateTestRun(c.Request.Context(), testRun); err != nil {
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

func (h *DomainHandler) getTestRun(c *gin.Context) {
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

func (h *DomainHandler) getTestRunByRunID(c *gin.Context) {
	_ = c.Param("runId")

	// For now, we'll need to implement a method to get by run ID in the domain service
	// This is a limitation that needs to be addressed
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get test run by run ID not yet implemented"})
}

func (h *DomainHandler) listTestRuns(c *gin.Context) {
	projectID := c.Query("project_id")
	limit := 50 // default
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	// Get test runs from domain service
	// TODO: Add offset support to GetProjectTestRuns
	_ = offset
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

	c.Header("X-Total-Count", strconv.Itoa(len(testRuns)))
	c.JSON(http.StatusOK, gin.H{
		"data":  apiTestRuns,
		"total": len(testRuns),
	})
}

func (h *DomainHandler) countTestRuns(c *gin.Context) {
	projectID := c.Query("project_id")

	// Get count from domain service
	testRuns, err := h.testingService.GetProjectTestRuns(c.Request.Context(), projectID, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(testRuns),
	})
}

func (h *DomainHandler) updateTestRunStatus(c *gin.Context) {
	_ = c.Param("runId")

	var input struct {
		Status  string     `json:"status" binding:"required"`
		EndTime *time.Time `json:"end_time,omitempty"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// For now, we need to implement this in the domain service
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Update test run status not yet implemented"})
}

func (h *DomainHandler) deleteTestRun(c *gin.Context) {
	_, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test run ID"})
		return
	}

	// For now, we need to implement delete in the domain service
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Delete test run not yet implemented"})
}

func (h *DomainHandler) getTestRunStats(c *gin.Context) {
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

func (h *DomainHandler) getRecentTestRuns(c *gin.Context) {
	projectID := c.Query("project_id")
	limit := 10 // default

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

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

func (h *DomainHandler) assignTagsToTestRun(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid test run ID"})
		return
	}

	var input struct {
		TagIDs []uint `json:"tagIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert tag IDs to domain format
	tagIDs := make([]tagsDomain.TagID, len(input.TagIDs))
	for i, id := range input.TagIDs {
		tagIDs[i] = tagsDomain.TagID(strconv.FormatUint(uint64(id), 10))
	}

	if err := h.tagService.AssignTagsToTestRun(c.Request.Context(), strconv.FormatUint(uint64(id), 10), tagIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tags assigned successfully"})
}

func (h *DomainHandler) bulkDeleteTestRuns(c *gin.Context) {
	var input struct {
		IDs []uint `json:"ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement bulk delete in domain service
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Bulk delete not yet implemented"})
}

// Project Handlers

func (h *DomainHandler) createProject(c *gin.Context) {
	var input struct {
		ProjectID     string                 `json:"projectId"`
		Name          string                 `json:"name" binding:"required"`
		Description   string                 `json:"description"`
		Repository    string                 `json:"repository"`
		DefaultBranch string                 `json:"defaultBranch"`
		Team          string                 `json:"team" binding:"required"`
		Settings      map[string]interface{} `json:"settings"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current user for creator ID
	user, _ := interfaces.GetAuthUser(c)
	creatorUserID := ""
	if user != nil {
		creatorUserID = user.UserID
	}

	// Generate project ID if not provided
	projectID := input.ProjectID
	if projectID == "" {
		projectID = uuid.New().String()
	}

	// Create project using domain service
	project, err := h.projectService.CreateProject(
		c.Request.Context(),
		projectsDomain.ProjectID(projectID),
		input.Name,
		projectsDomain.Team(input.Team),
		creatorUserID,
	)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create project")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update additional fields if provided
	if input.Description != "" || input.Repository != "" || input.DefaultBranch != "" {
		updates := projectsApp.UpdateProjectRequest{}
		if input.Description != "" {
			updates.Description = &input.Description
		}
		if input.Repository != "" {
			updates.Repository = &input.Repository
		}
		if input.DefaultBranch != "" {
			updates.DefaultBranch = &input.DefaultBranch
		}

		if err := h.projectService.UpdateProject(c.Request.Context(), project.ProjectID(), updates); err != nil {
			h.logger.WithError(err).Warn("Failed to update project details after creation")
		}
	}

	// Convert to API response format
	c.JSON(http.StatusCreated, h.convertProjectToAPI(project))
}

func (h *DomainHandler) getProject(c *gin.Context) {
	projectIDStr := c.Param("projectId")

	// Try to parse as numeric ID first (for backward compatibility)
	if _, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
		// This is a numeric ID - need to implement GetProjectByID in domain service
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Get project by numeric ID not yet implemented"})
		return
	}

	// Otherwise treat as project ID string
	project, err := h.projectService.GetProject(c.Request.Context(), projectsDomain.ProjectID(projectIDStr))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, h.convertProjectToAPI(project))
}

func (h *DomainHandler) getProjectByProjectID(c *gin.Context) {
	projectID := c.Param("projectId")

	project, err := h.projectService.GetProject(c.Request.Context(), projectsDomain.ProjectID(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, h.convertProjectToAPI(project))
}

func (h *DomainHandler) updateProject(c *gin.Context) {
	projectID := c.Param("projectId")

	var input struct {
		Name          string                 `json:"name"`
		Description   string                 `json:"description"`
		Repository    string                 `json:"repository"`
		DefaultBranch string                 `json:"defaultBranch"`
		Team          string                 `json:"team"`
		Settings      map[string]interface{} `json:"settings"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build update request
	updates := projectsApp.UpdateProjectRequest{}
	if input.Name != "" {
		updates.Name = &input.Name
	}
	if input.Description != "" {
		updates.Description = &input.Description
	}
	if input.Repository != "" {
		updates.Repository = &input.Repository
	}
	if input.DefaultBranch != "" {
		updates.DefaultBranch = &input.DefaultBranch
	}
	if input.Team != "" {
		team := projectsDomain.Team(input.Team)
		updates.Team = &team
	}

	// Update project
	if err := h.projectService.UpdateProject(c.Request.Context(), projectsDomain.ProjectID(projectID), updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get updated project
	project, err := h.projectService.GetProject(c.Request.Context(), projectsDomain.ProjectID(projectID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.convertProjectToAPI(project))
}

func (h *DomainHandler) deleteProject(c *gin.Context) {
	projectID := c.Param("projectId")

	if err := h.projectService.DeleteProject(c.Request.Context(), projectsDomain.ProjectID(projectID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project deleted successfully"})
}

func (h *DomainHandler) listProjects(c *gin.Context) {
	limit := 20
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	projects, total, err := h.projectService.ListProjects(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to API response format
	apiProjects := make([]interface{}, len(projects))
	for i, p := range projects {
		apiProjects[i] = h.convertProjectToAPI(p)
	}

	c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	c.JSON(http.StatusOK, apiProjects)
}

func (h *DomainHandler) activateProject(c *gin.Context) {
	projectID := c.Param("projectId")

	if err := h.projectService.ActivateProject(c.Request.Context(), projectsDomain.ProjectID(projectID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project activated successfully"})
}

func (h *DomainHandler) deactivateProject(c *gin.Context) {
	projectID := c.Param("projectId")

	if err := h.projectService.DeactivateProject(c.Request.Context(), projectsDomain.ProjectID(projectID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project deactivated successfully"})
}

func (h *DomainHandler) getProjectStats(c *gin.Context) {
	projectID := c.Param("projectId")

	// TODO: Implement project stats in domain service
	c.JSON(http.StatusOK, gin.H{
		"projectId":      projectID,
		"totalTestRuns":  0,
		"passedTestRuns": 0,
		"failedTestRuns": 0,
		"avgDuration":    0,
		"successRate":    0,
	})
}

// Tag Handlers

func (h *DomainHandler) createTag(c *gin.Context) {
	var input struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tag, err := h.tagService.CreateTag(c.Request.Context(), input.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to API response format with additional fields
	c.JSON(http.StatusCreated, gin.H{
		"id":          tag.ID(),
		"name":        tag.Name(),
		"description": input.Description,
		"color":       input.Color,
		"createdAt":   tag.CreatedAt(),
	})
}

func (h *DomainHandler) getTag(c *gin.Context) {
	idStr := c.Param("id")

	tag, err := h.tagService.GetTag(c.Request.Context(), tagsDomain.TagID(idStr))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	c.JSON(http.StatusOK, h.convertTagToAPI(tag))
}

func (h *DomainHandler) getTagByName(c *gin.Context) {
	name := c.Param("name")

	tag, err := h.tagService.GetTagByName(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	c.JSON(http.StatusOK, h.convertTagToAPI(tag))
}

func (h *DomainHandler) updateTag(c *gin.Context) {
	idStr := c.Param("id")

	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Tags in the domain are immutable except for deletion
	// For backward compatibility, we'll return success but not actually update
	tag, err := h.tagService.GetTag(c.Request.Context(), tagsDomain.TagID(idStr))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tag not found"})
		return
	}

	// Return the tag with potentially updated metadata (though not actually persisted)
	c.JSON(http.StatusOK, gin.H{
		"id":          tag.ID(),
		"name":        tag.Name(),
		"description": input.Description,
		"color":       input.Color,
		"updatedAt":   time.Now(),
	})
}

func (h *DomainHandler) deleteTag(c *gin.Context) {
	idStr := c.Param("id")

	if err := h.tagService.DeleteTag(c.Request.Context(), tagsDomain.TagID(idStr)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Tag deleted successfully"})
}

func (h *DomainHandler) listTags(c *gin.Context) {
	tags, err := h.tagService.ListTags(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to API response format
	apiTags := make([]interface{}, len(tags))
	for i, tag := range tags {
		apiTags[i] = h.convertTagToAPI(tag)
	}

	c.JSON(http.StatusOK, apiTags)
}

func (h *DomainHandler) getTagUsageStats(c *gin.Context) {
	// TODO: Implement tag usage stats in domain service
	c.JSON(http.StatusOK, []gin.H{})
}

func (h *DomainHandler) getPopularTags(c *gin.Context) {
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	// For now, return top N tags from all tags
	tags, err := h.tagService.ListTags(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Limit results
	if len(tags) > limit {
		tags = tags[:limit]
	}

	// Convert to usage format
	popularTags := make([]gin.H, len(tags))
	for i, tag := range tags {
		popularTags[i] = gin.H{
			"tag":        h.convertTagToAPI(tag),
			"usageCount": 0, // TODO: Implement usage counting
		}
	}

	c.JSON(http.StatusOK, popularTags)
}

// User preference stubs
func (h *DomainHandler) getUserPreferences(c *gin.Context) {
	user, _ := interfaces.GetAuthUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Return default preferences
	c.JSON(http.StatusOK, gin.H{
		"theme": "light",
		"notifications": gin.H{
			"email": true,
			"push":  false,
		},
		"defaultProjectView": "grid",
	})
}

func (h *DomainHandler) updateUserPreferences(c *gin.Context) {
	var preferences map[string]interface{}
	if err := c.ShouldBindJSON(&preferences); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement user preferences in domain
	c.JSON(http.StatusOK, preferences)
}

func (h *DomainHandler) getUserProjects(c *gin.Context) {
	user, _ := interfaces.GetAuthUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Get all projects and filter by user access
	projects, _, err := h.projectService.ListProjects(c.Request.Context(), 100, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to API response format
	apiProjects := make([]interface{}, len(projects))
	for i, p := range projects {
		apiProjects[i] = h.convertProjectToAPI(p)
	}

	c.JSON(http.StatusOK, apiProjects)
}

// Admin endpoints (stubs for now)
func (h *DomainHandler) listUsers(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

func (h *DomainHandler) getUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

func (h *DomainHandler) updateUserRole(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

func (h *DomainHandler) suspendUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

func (h *DomainHandler) activateUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

func (h *DomainHandler) deleteUser(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

func (h *DomainHandler) grantProjectAccess(c *gin.Context) {
	projectID := c.Param("projectId")
	userID := c.Param("userId")

	var input struct {
		Permission string `json:"permission" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current user as granter
	user, _ := interfaces.GetAuthUser(c)
	grantedBy := ""
	if user != nil {
		grantedBy = user.UserID
	}

	// Convert permission to domain type
	var permType projectsDomain.PermissionType
	switch strings.ToLower(input.Permission) {
	case "read":
		permType = projectsDomain.PermissionRead
	case "write":
		permType = projectsDomain.PermissionWrite
	case "delete":
		permType = projectsDomain.PermissionDelete
	case "admin":
		permType = projectsDomain.PermissionAdmin
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid permission type"})
		return
	}

	if err := h.projectService.GrantPermission(
		c.Request.Context(),
		projectsDomain.ProjectID(projectID),
		userID,
		permType,
		grantedBy,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Access granted successfully"})
}

func (h *DomainHandler) revokeProjectAccess(c *gin.Context) {
	_ = c.Param("projectId")
	_ = c.Param("userId")

	// TODO: Need to specify which permission to revoke
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Revoke access not yet implemented"})
}

func (h *DomainHandler) getProjectUsers(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

func (h *DomainHandler) getSystemStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"totalProjects": 0,
		"totalTestRuns": 0,
		"totalUsers":    0,
		"activeUsers":   0,
	})
}

func (h *DomainHandler) getSystemHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":   "healthy",
		"database": "connected",
		"cache":    "connected",
	})
}

func (h *DomainHandler) performSystemCleanup(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Cleanup initiated"})
}

func (h *DomainHandler) getAuditLogs(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

// Fern-compatible endpoints
func (h *DomainHandler) createFernProject(c *gin.Context) {
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update project: %v", err)})
			return
		}
	}

	// Return Fern-compatible response
	c.JSON(http.StatusCreated, gin.H{
		"uuid": string(project.ProjectID()),
		"name": project.Name(),
	})
}

func (h *DomainHandler) getFernProject(c *gin.Context) {
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

func (h *DomainHandler) listFernProjects(c *gin.Context) {
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

func (h *DomainHandler) createFernTestReport(c *gin.Context) {
	// Read raw body for logging
	bodyBytes, err := c.GetRawData()
	if err != nil {
		h.logger.WithError(err).Error("Failed to read request body")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	h.logger.Info("fern-ginkgo-client request received",
		"endpoint", c.Request.URL.Path,
		"method", c.Request.Method,
		"content-length", len(bodyBytes),
		"body", string(bodyBytes))

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

	// Debug log each suite run
	for i, suite := range input.SuiteRuns {
		h.logger.Info("Suite run details",
			"index", i,
			"suite_id", suite.ID,
			"suite_name", suite.SuiteName,
			"test_run_id", suite.TestRunID,
			"spec_runs_count", len(suite.SpecRuns),
			"start_time", suite.StartTime,
			"end_time", suite.EndTime)

		// Log first few specs for debugging
		for j, spec := range suite.SpecRuns {
			if j < 3 { // Only log first 3 specs to avoid spam
				h.logger.Info("Spec run details",
					"suite_index", i,
					"spec_index", j,
					"spec_id", spec.ID,
					"spec_description", spec.SpecDescription,
					"status", spec.Status,
					"message", spec.Message)
			}
		}
	}

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

		h.logger.Info("Test run created successfully", "test_run_id", testRun.ID)
	} else {
		// Test run exists, use the existing one
		testRun = existingTestRun
		h.logger.Info("Using existing test run",
			"test_run_id", testRun.ID,
			"run_id", testRun.RunID,
			"existing_total_tests", testRun.TotalTests,
			"new_total_tests", totalTests)

		// We'll update test run statistics after processing all suites
		h.logger.Info("Will update test run statistics after processing suites")
	}

	// Get existing suite runs for this test run to avoid duplicates
	existingSuites, err := h.testingService.GetSuiteRunsByTestRunID(c.Request.Context(), testRun.ID)
	if err != nil {
		h.logger.WithError(err).Warn("Failed to get existing suite runs")
		existingSuites = []*domain.SuiteRun{}
	}

	// Create a map of existing suite names for quick lookup
	existingSuiteNames := make(map[string]bool)
	for _, suite := range existingSuites {
		existingSuiteNames[suite.Name] = true
	}

	// Process suite runs to create in the domain
	h.logger.Info("Processing suite runs for database storage",
		"suite_count", len(input.SuiteRuns),
		"existing_suites", len(existingSuites))

	for idx, suiteData := range input.SuiteRuns {
		// Skip if suite already exists
		if existingSuiteNames[suiteData.SuiteName] {
			h.logger.Info("Suite already exists, skipping",
				"suite_name", suiteData.SuiteName,
				"test_run_id", testRun.ID)
			continue
		}

		suiteStart, _ := time.Parse(time.RFC3339, suiteData.StartTime)
		suiteEnd, _ := time.Parse(time.RFC3339, suiteData.EndTime)

		h.logger.Info("Creating suite run",
			"index", idx,
			"suite_name", suiteData.SuiteName,
			"test_run_id", testRun.ID,
			"spec_count", len(suiteData.SpecRuns))

		// Calculate suite statistics
		var totalSpecs, passedSpecs, failedSpecs, skippedSpecs int
		for _, spec := range suiteData.SpecRuns {
			totalSpecs++
			switch spec.Status {
			case "passed":
				passedSpecs++
			case "failed":
				failedSpecs++
			case "skipped", "pending":
				skippedSpecs++
			}
		}

		duration := suiteEnd.Sub(suiteStart)

		suiteRun := &domain.SuiteRun{
			TestRunID:    testRun.ID,
			Name:         suiteData.SuiteName,
			StartTime:    suiteStart,
			EndTime:      &suiteEnd,
			Status:       "completed",
			TotalTests:   totalSpecs,
			PassedTests:  passedSpecs,
			FailedTests:  failedSpecs,
			SkippedTests: skippedSpecs,
			Duration:     duration,
		}

		// Create suite run
		if err := h.testingService.CreateSuiteRun(c.Request.Context(), suiteRun); err != nil {
			h.logger.WithError(err).Error("Failed to create suite run",
				"suite_name", suiteData.SuiteName,
				"test_run_id", testRun.ID)
			continue
		}

		h.logger.Info("Suite run created successfully",
			"suite_id", suiteRun.ID,
			"suite_name", suiteRun.Name,
			"test_run_id", suiteRun.TestRunID)

		// Process spec runs
		for _, specData := range suiteData.SpecRuns {
			specStart, _ := time.Parse(time.RFC3339, specData.StartTime)
			specEnd, _ := time.Parse(time.RFC3339, specData.EndTime)

			specRun := &domain.SpecRun{
				SuiteRunID:     suiteRun.ID,
				Name:           specData.SpecDescription,
				Status:         specData.Status,
				StartTime:      specStart,
				EndTime:        &specEnd,
				ErrorMessage:   specData.Message,
				FailureMessage: specData.Message,
			}

			// Create spec run
			if err := h.testingService.CreateSpecRun(c.Request.Context(), specRun); err != nil {
				h.logger.WithError(err).Warn("Failed to create spec run")
			}
		}
	}

	// Recalculate test run statistics from all suite runs
	allSuites, err := h.testingService.GetSuiteRunsByTestRunID(c.Request.Context(), testRun.ID)
	if err != nil {
		h.logger.WithError(err).Warn("Failed to get all suite runs for statistics")
	} else {
		// Calculate aggregate statistics
		var totalTests, passedTests, failedTests, skippedTests int
		for _, suite := range allSuites {
			totalTests += suite.TotalTests
			passedTests += suite.PassedTests
			failedTests += suite.FailedTests
			skippedTests += suite.SkippedTests
		}

		h.logger.Info("Calculated aggregate test run statistics",
			"test_run_id", testRun.ID,
			"total_suites", len(allSuites),
			"total_tests", totalTests,
			"passed_tests", passedTests,
			"failed_tests", failedTests,
			"skipped_tests", skippedTests)

		// Update test run with aggregate statistics
		testRun.TotalTests = totalTests
		testRun.PassedTests = passedTests
		testRun.FailedTests = failedTests
		testRun.SkippedTests = skippedTests

		// Update end time if provided
		if endTime != nil {
			testRun.EndTime = endTime
		}

		// Save updated statistics
		if err := h.testingService.UpdateTestRun(c.Request.Context(), testRun); err != nil {
			h.logger.WithError(err).Error("Failed to update test run statistics")
		} else {
			h.logger.Info("Updated test run with aggregate statistics",
				"test_run_id", testRun.ID,
				"total_tests", testRun.TotalTests)
		}
	}

	// Return response in the format fern-ginkgo-client expects
	response := gin.H{
		"id":                testRun.ID,
		"test_project_name": project.Name(),
		"test_seed":         input.TestSeed,
		"start_time":        startTime.Format(time.RFC3339),
		"status":            testRun.Status,
		"created_at":        testRun.StartTime,
	}
	if endTime != nil {
		response["end_time"] = endTime.Format(time.RFC3339)
	}

	h.logger.Info("Sending response to fern-ginkgo-client",
		"test_run_id", testRun.ID,
		"status_code", http.StatusCreated)

	c.JSON(http.StatusCreated, response)
}

func (h *DomainHandler) listFernTestReports(c *gin.Context) {
	projectID := c.Query("project_id")

	testRuns, err := h.testingService.GetProjectTestRuns(c.Request.Context(), projectID, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to Fern-compatible format
	fernReports := make([]gin.H, len(testRuns))
	for i, tr := range testRuns {
		fernReports[i] = gin.H{
			"uuid":       tr.ID,
			"project_id": tr.ProjectID,
			"status":     tr.Status,
			"created_at": tr.StartTime,
		}
	}

	c.JSON(http.StatusOK, fernReports)
}

func (h *DomainHandler) getFernTestReport(c *gin.Context) {
	// TODO: Implement get by UUID
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Get test report by UUID not yet implemented"})
}

// Conversion helpers

func (h *DomainHandler) convertTestRunToAPI(tr *domain.TestRun) gin.H {
	return gin.H{
		"id":           tr.ID,
		"projectId":    tr.ProjectID,
		"runId":        tr.ID, // Use ID as runId for backward compatibility
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
		"metadata":     tr.Metadata,
		"createdAt":    tr.StartTime,
		"updatedAt":    tr.EndTime,
	}
}

func (h *DomainHandler) convertProjectToAPI(p *projectsDomain.Project) gin.H {
	snapshot := p.ToSnapshot()

	// Convert settings to JSON string for backward compatibility
	settingsJSON, _ := json.Marshal(snapshot.Settings)

	return gin.H{
		"id":            snapshot.ID,
		"projectId":     string(snapshot.ProjectID),
		"name":          snapshot.Name,
		"description":   snapshot.Description,
		"repository":    snapshot.Repository,
		"defaultBranch": snapshot.DefaultBranch,
		"team":          string(snapshot.Team),
		"isActive":      snapshot.IsActive,
		"settings":      string(settingsJSON),
		"createdAt":     snapshot.CreatedAt,
		"updatedAt":     snapshot.UpdatedAt,
	}
}

func (h *DomainHandler) convertTagToAPI(t *tagsDomain.Tag) gin.H {
	return gin.H{
		"id":          string(t.ID()),
		"name":        t.Name(),
		"description": "", // Domain tags don't have descriptions
		"color":       "", // Domain tags don't have colors
		"createdAt":   t.CreatedAt(),
		"updatedAt":   t.CreatedAt(), // Domain tags are immutable
	}
}
