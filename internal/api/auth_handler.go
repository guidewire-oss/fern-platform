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
    <title>Fern Platform - Modern Test Intelligence</title>
    <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>🌿</text></svg>">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #0a0f1b;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
            overflow: hidden;
            position: relative;
        }
        
        /* Animated background */
        body::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: 
                radial-gradient(circle at 20%% 50%%, rgba(16, 185, 129, 0.3) 0%%, transparent 50%%),
                radial-gradient(circle at 80%% 80%%, rgba(59, 130, 246, 0.3) 0%%, transparent 50%%),
                radial-gradient(circle at 40%% 80%%, rgba(139, 92, 246, 0.3) 0%%, transparent 50%%);
            filter: blur(100px);
            animation: float 20s ease-in-out infinite;
        }
        
        @keyframes float {
            0%%, 100%% { transform: translate(0, 0) scale(1); }
            33%% { transform: translate(-20px, -20px) scale(1.1); }
            66%% { transform: translate(20px, -10px) scale(0.9); }
        }
        
        .login-container {
            background: rgba(255, 255, 255, 0.05);
            backdrop-filter: blur(20px);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 24px;
            box-shadow: 
                0 20px 40px rgba(0, 0, 0, 0.4),
                inset 0 1px 0 rgba(255, 255, 255, 0.1);
            padding: 60px 50px;
            width: 100%%;
            max-width: 480px;
            position: relative;
            z-index: 1;
            animation: slideIn 0.5s ease-out;
        }
        
        @keyframes slideIn {
            from {
                opacity: 0;
                transform: translateY(30px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
        
        .logo-container {
            text-align: center;
            margin-bottom: 40px;
        }
        
        .logo {
            width: 100px;
            height: 100px;
            margin: 0 auto 20px;
            background: linear-gradient(135deg, #10b981 0%%, #3b82f6 100%%);
            border-radius: 24px;
            display: flex;
            align-items: center;
            justify-content: center;
            box-shadow: 
                0 10px 30px rgba(16, 185, 129, 0.3),
                inset 0 1px 0 rgba(255, 255, 255, 0.2);
            position: relative;
            overflow: hidden;
        }
        
        .logo::before {
            content: '';
            position: absolute;
            top: -50%%;
            left: -50%%;
            width: 200%%;
            height: 200%%;
            background: linear-gradient(
                45deg,
                transparent,
                rgba(255, 255, 255, 0.1),
                transparent
            );
            transform: rotate(45deg);
            animation: shimmer 3s ease-in-out infinite;
        }
        
        @keyframes shimmer {
            0%% { transform: translateX(-100%%) translateY(-100%%) rotate(45deg); }
            100%% { transform: translateX(100%%) translateY(100%%) rotate(45deg); }
        }
        
        .logo-icon {
            font-size: 60px;
            position: relative;
            z-index: 1;
            filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.2));
        }
        
        h1 {
            color: #ffffff;
            font-size: 32px;
            font-weight: 700;
            margin-bottom: 12px;
            text-align: center;
            letter-spacing: -0.5px;
        }
        
        .subtitle {
            color: rgba(255, 255, 255, 0.7);
            font-size: 18px;
            text-align: center;
            margin-bottom: 40px;
            font-weight: 400;
        }
        
        .features {
            display: grid;
            gap: 16px;
            margin-bottom: 40px;
        }
        
        .feature {
            display: flex;
            align-items: center;
            gap: 16px;
            padding: 16px;
            background: rgba(255, 255, 255, 0.05);
            border-radius: 12px;
            border: 1px solid rgba(255, 255, 255, 0.1);
        }
        
        .feature-icon {
            width: 40px;
            height: 40px;
            background: linear-gradient(135deg, #10b981 0%%, #3b82f6 100%%);
            border-radius: 10px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 20px;
            flex-shrink: 0;
        }
        
        .feature-text {
            flex: 1;
        }
        
        .feature-title {
            color: #ffffff;
            font-size: 16px;
            font-weight: 600;
            margin-bottom: 4px;
        }
        
        .feature-desc {
            color: rgba(255, 255, 255, 0.6);
            font-size: 14px;
            line-height: 1.4;
        }
        
        .login-button {
            width: 100%%;
            padding: 16px 24px;
            background: linear-gradient(135deg, #10b981 0%%, #3b82f6 100%%);
            color: white;
            border: none;
            border-radius: 12px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s ease;
            text-decoration: none;
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 10px;
            box-shadow: 
                0 4px 14px rgba(16, 185, 129, 0.4),
                inset 0 1px 0 rgba(255, 255, 255, 0.2);
            position: relative;
            overflow: hidden;
        }
        
        .login-button::before {
            content: '';
            position: absolute;
            top: 0;
            left: -100%%;
            width: 100%%;
            height: 100%%;
            background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.2), transparent);
            transition: left 0.5s ease;
        }
        
        .login-button:hover::before {
            left: 100%%;
        }
        
        .login-button:hover {
            transform: translateY(-2px);
            box-shadow: 
                0 6px 20px rgba(16, 185, 129, 0.5),
                inset 0 1px 0 rgba(255, 255, 255, 0.2);
        }
        
        .login-button:active {
            transform: translateY(0);
        }
        
        .login-icon {
            font-size: 18px;
        }
        
        .security-note {
            margin-top: 30px;
            padding: 20px;
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 12px;
            font-size: 14px;
            color: rgba(255, 255, 255, 0.7);
            text-align: center;
            display: flex;
            align-items: center;
            gap: 12px;
        }
        
        .security-icon {
            font-size: 20px;
            color: #10b981;
        }
        
        .security-note strong {
            color: #ffffff;
            display: block;
            margin-bottom: 4px;
        }
        
        /* Floating particles */
        .particle {
            position: absolute;
            width: 4px;
            height: 4px;
            background: rgba(16, 185, 129, 0.6);
            border-radius: 50%%;
            animation: particle-float 10s infinite linear;
        }
        
        @keyframes particle-float {
            from {
                transform: translateY(100vh) rotate(0deg);
                opacity: 0;
            }
            10%% {
                opacity: 1;
            }
            90%% {
                opacity: 1;
            }
            to {
                transform: translateY(-100vh) rotate(360deg);
                opacity: 0;
            }
        }
        
        .particle:nth-child(1) { left: 10%%; animation-delay: 0s; animation-duration: 8s; }
        .particle:nth-child(2) { left: 20%%; animation-delay: 1s; animation-duration: 10s; background: rgba(59, 130, 246, 0.6); }
        .particle:nth-child(3) { left: 30%%; animation-delay: 2s; animation-duration: 9s; }
        .particle:nth-child(4) { left: 40%%; animation-delay: 3s; animation-duration: 11s; background: rgba(139, 92, 246, 0.6); }
        .particle:nth-child(5) { left: 50%%; animation-delay: 4s; animation-duration: 8s; }
        .particle:nth-child(6) { left: 60%%; animation-delay: 5s; animation-duration: 10s; background: rgba(59, 130, 246, 0.6); }
        .particle:nth-child(7) { left: 70%%; animation-delay: 6s; animation-duration: 9s; }
        .particle:nth-child(8) { left: 80%%; animation-delay: 7s; animation-duration: 11s; background: rgba(139, 92, 246, 0.6); }
        .particle:nth-child(9) { left: 90%%; animation-delay: 8s; animation-duration: 8s; }
        
        @media (max-width: 640px) {
            .login-container {
                padding: 40px 30px;
            }
            
            h1 {
                font-size: 28px;
            }
            
            .subtitle {
                font-size: 16px;
            }
        }
    </style>
</head>
<body>
    <!-- Floating particles -->
    <div class="particle"></div>
    <div class="particle"></div>
    <div class="particle"></div>
    <div class="particle"></div>
    <div class="particle"></div>
    <div class="particle"></div>
    <div class="particle"></div>
    <div class="particle"></div>
    <div class="particle"></div>
    
    <div class="login-container">
        <div class="logo-container">
            <div class="logo">
                <span class="logo-icon">🌿</span>
            </div>
        </div>
        
        <h1>Welcome to Fern Platform</h1>
        <p class="subtitle">Transform test chaos into actionable intelligence</p>
        
        <div class="features">
            <div class="feature">
                <div class="feature-icon">📊</div>
                <div class="feature-text">
                    <div class="feature-title">Unified Test Analytics</div>
                    <div class="feature-desc">Consolidate results from all your testing frameworks</div>
                </div>
            </div>
            
            <div class="feature">
                <div class="feature-icon">🤖</div>
                <div class="feature-text">
                    <div class="feature-title">AI-Powered Insights</div>
                    <div class="feature-desc">Smart detection of flaky tests and failure patterns</div>
                </div>
            </div>
            
            <div class="feature">
                <div class="feature-icon">⚡</div>
                <div class="feature-text">
                    <div class="feature-title">Real-time Visibility</div>
                    <div class="feature-desc">Interactive dashboards for instant test intelligence</div>
                </div>
            </div>
        </div>
        
        <a href="%s" class="login-button">
            <span class="login-icon">🔐</span>
            <span>Sign in with OAuth</span>
        </a>
        
        <div class="security-note">
            <span class="security-icon">🛡️</span>
            <div>
                <strong>Enterprise-grade Security</strong>
                Your authentication is handled by our trusted OAuth provider
            </div>
        </div>
    </div>
</body>
</html>`, oauthURL)
}

// RegisterRoutes registers auth routes
func (h *AuthHandler) RegisterRoutes(router *gin.Engine, authGroup, userGroup, adminGroup *gin.RouterGroup) {
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