// Package api provides domain-based REST API handlers
package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/interfaces"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// AuthHandler handles authentication and user related endpoints
type AuthHandler struct {
	*BaseHandler
	authMiddleware *interfaces.AuthMiddlewareAdapter
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authMiddleware *interfaces.AuthMiddlewareAdapter, logger *logging.Logger) *AuthHandler {
	return &AuthHandler{
		BaseHandler:    NewBaseHandler(logger),
		authMiddleware: authMiddleware,
	}
}

// showLoginPage handles GET /auth/login
func (h *AuthHandler) showLoginPage(c *gin.Context) {
	// Check if already authenticated
	if h.isUserAuthenticated(c) {
		c.Redirect(302, "/")
		return
	}

	returnURL := c.Query("return")
	if returnURL == "" {
		returnURL = "/"
	}

	// Store return URL in session/cookie for post-auth redirect
	c.SetCookie("auth_return_url", returnURL, 3600, "/", "", false, true)

	// Generate OAuth URL
	oauthURL := h.generateOAuthURL(c)

	// Serve the login page HTML
	html := h.getLoginPageHTML(oauthURL)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// getCurrentUser handles GET /auth/user
func (h *AuthHandler) getCurrentUser(c *gin.Context) {
	userID := h.getUserID(c)
	userName, _ := c.Get("user_name")
	userEmail, _ := c.Get("user_email")
	userRole, _ := c.Get("role")
	teamID, _ := c.Get("team_id")
	teamName, _ := c.Get("team_name")

	response := gin.H{
		"id":    userID,
		"name":  userName,
		"email": userEmail,
		"role":  userRole,
		"team": gin.H{
			"id":   teamID,
			"name": teamName,
		},
	}

	h.respondWithJSON(c, http.StatusOK, response)
}

// getUserPreferences handles GET /api/v1/user/preferences
func (h *AuthHandler) getUserPreferences(c *gin.Context) {
	userID := h.getUserID(c)

	// TODO: Implement user preferences storage and retrieval
	// For now, return default preferences
	preferences := gin.H{
		"user_id": userID,
		"theme":   "light",
		"notifications": gin.H{
			"email":   true,
			"in_app":  true,
			"desktop": false,
		},
		"dashboard": gin.H{
			"default_view":     "grid",
			"items_per_page":   20,
			"show_failed_only": false,
		},
	}

	h.respondWithJSON(c, http.StatusOK, preferences)
}

// updateUserPreferences handles PUT /api/v1/user/preferences
func (h *AuthHandler) updateUserPreferences(c *gin.Context) {
	var preferences map[string]interface{}
	if err := c.ShouldBindJSON(&preferences); err != nil {
		h.respondWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	// TODO: Implement user preferences storage
	// For now, just echo back the preferences
	h.respondWithJSON(c, http.StatusOK, preferences)
}

// getUserProjects handles GET /api/v1/user/projects
func (h *AuthHandler) getUserProjects(c *gin.Context) {
	userID := h.getUserID(c)

	// TODO: This should be moved to project handler and use project service
	// For now, return empty list
	response := gin.H{
		"items": []gin.H{},
		"total": 0,
	}

	h.logger.WithField("user_id", userID).Debug("Getting user projects")
	h.respondWithJSON(c, http.StatusOK, response)
}

// Admin user management endpoints

// listUsers handles GET /api/v1/admin/users
func (h *AuthHandler) listUsers(c *gin.Context) {
	// TODO: Implement user listing from auth service
	h.respondWithJSON(c, http.StatusOK, gin.H{"items": []gin.H{}, "total": 0})
}

// getUser handles GET /api/v1/admin/users/:userId
func (h *AuthHandler) getUser(c *gin.Context) {
	userID := c.Param("userId")
	// TODO: Implement get user from auth service
	h.respondWithJSON(c, http.StatusOK, gin.H{"id": userID})
}

// updateUserRole handles PUT /api/v1/admin/users/:userId/role
func (h *AuthHandler) updateUserRole(c *gin.Context) {
	// TODO: Implement update user role
	h.respondWithJSON(c, http.StatusOK, gin.H{"message": "Role updated successfully"})
}

// suspendUser handles POST /api/v1/admin/users/:userId/suspend
func (h *AuthHandler) suspendUser(c *gin.Context) {
	// TODO: Implement suspend user
	h.respondWithJSON(c, http.StatusOK, gin.H{"message": "User suspended successfully"})
}

// activateUser handles POST /api/v1/admin/users/:userId/activate
func (h *AuthHandler) activateUser(c *gin.Context) {
	// TODO: Implement activate user
	h.respondWithJSON(c, http.StatusOK, gin.H{"message": "User activated successfully"})
}

// deleteUser handles DELETE /api/v1/admin/users/:userId
func (h *AuthHandler) deleteUser(c *gin.Context) {
	// TODO: Implement delete user
	h.respondWithJSON(c, http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// Helper methods

// isUserAuthenticated checks if the user is authenticated
func (h *AuthHandler) isUserAuthenticated(c *gin.Context) bool {
	userID, exists := c.Get("user_id")
	return exists && userID != nil && userID != ""
}

// generateOAuthURL generates the OAuth authorization URL
func (h *AuthHandler) generateOAuthURL(c *gin.Context) string {
	// Get the base URL from the request
	scheme := "http"
	if c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	
	host := c.Request.Host
	if forwardedHost := c.Request.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}

	baseURL := fmt.Sprintf("%s://%s", scheme, host)
	return fmt.Sprintf("%s/auth/start", baseURL)
}

// getLoginPageHTML returns the login page HTML
func (h *AuthHandler) getLoginPageHTML(oauthURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - Fern Platform</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }
        
        .login-container {
            background: white;
            border-radius: 10px;
            box-shadow: 0 14px 28px rgba(0,0,0,0.25), 0 10px 10px rgba(0,0,0,0.22);
            padding: 40px;
            width: 100%%;
            max-width: 400px;
            animation: slideIn 0.3s ease-out;
        }
        
        @keyframes slideIn {
            from {
                opacity: 0;
                transform: translateY(-20px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
        
        .logo-container {
            text-align: center;
            margin-bottom: 30px;
        }
        
        .logo {
            width: 120px;
            height: 120px;
            margin: 0 auto;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            border-radius: 20px;
            display: flex;
            align-items: center;
            justify-content: center;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        
        .logo svg {
            width: 80px;
            height: 80px;
            fill: white;
        }
        
        h1 {
            color: #333;
            font-size: 28px;
            margin-bottom: 10px;
            text-align: center;
        }
        
        .subtitle {
            color: #666;
            font-size: 16px;
            text-align: center;
            margin-bottom: 30px;
        }
        
        .login-button {
            width: 100%%;
            padding: 14px 20px;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            border: none;
            border-radius: 5px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s ease;
            text-decoration: none;
            display: block;
            text-align: center;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        
        .login-button:hover {
            transform: translateY(-2px);
            box-shadow: 0 6px 12px rgba(0,0,0,0.15);
        }
        
        .login-button:active {
            transform: translateY(0);
        }
        
        .security-note {
            margin-top: 30px;
            padding: 15px;
            background: #f8f9fa;
            border-radius: 5px;
            font-size: 14px;
            color: #666;
            text-align: center;
        }
        
        .security-note strong {
            color: #333;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <div class="logo-container">
            <div class="logo">
                <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                    <path d="M12 2L2 7v10c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-10-5z"/>
                </svg>
            </div>
        </div>
        
        <h1>Welcome to Fern</h1>
        <p class="subtitle">Test Intelligence Platform</p>
        
        <a href="%s" class="login-button">
            Sign in with OAuth
        </a>
        
        <div class="security-note">
            <strong>Secure Sign-In</strong><br>
            You will be redirected to our secure authentication provider.
        </div>
    </div>
</body>
</html>`, oauthURL)
}

// RegisterRoutes registers auth routes
func (h *AuthHandler) RegisterRoutes(router *gin.Engine, authGroup, userGroup, adminGroup *gin.RouterGroup) {
	// Root route handler
	router.GET("/", func(c *gin.Context) {
		if !h.isUserAuthenticated(c) {
			c.Redirect(302, "/auth/login")
			return
		}
		c.File("./web/index.html")
	})

	// Auth routes (no authentication required for login)
	authGroup.GET("/login", h.showLoginPage)
	authGroup.GET("/start", h.authMiddleware.StartOAuthFlow())
	authGroup.GET("/callback", h.authMiddleware.HandleOAuthCallback())
	authGroup.POST("/logout", h.authMiddleware.Logout())
	authGroup.GET("/user", h.authMiddleware.RequireAuth(), h.getCurrentUser)

	// User routes
	userGroup.GET("/user/preferences", h.getUserPreferences)
	userGroup.PUT("/user/preferences", h.updateUserPreferences)
	userGroup.GET("/user/projects", h.getUserProjects)

	// Admin routes for user management
	adminGroup.GET("/users", h.listUsers)
	adminGroup.GET("/users/:userId", h.getUser)
	adminGroup.PUT("/users/:userId/role", h.updateUserRole)
	adminGroup.POST("/users/:userId/suspend", h.suspendUser)
	adminGroup.POST("/users/:userId/activate", h.activateUser)
	adminGroup.DELETE("/users/:userId", h.deleteUser)
}