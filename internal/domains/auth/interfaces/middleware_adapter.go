package interfaces

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// AuthMiddlewareAdapter provides Gin middleware using auth domain services
type AuthMiddlewareAdapter struct {
	authService  *application.AuthenticationService
	authzService *application.AuthorizationService
	oauthAdapter *OAuthAdapter
	config       *config.AuthConfig
	logger       *logging.Logger
}

// NewAuthMiddlewareAdapter creates a new auth middleware adapter
func NewAuthMiddlewareAdapter(
	authService *application.AuthenticationService,
	authzService *application.AuthorizationService,
	oauthAdapter *OAuthAdapter,
	config *config.AuthConfig,
	logger *logging.Logger,
) *AuthMiddlewareAdapter {
	return &AuthMiddlewareAdapter{
		authService:  authService,
		authzService: authzService,
		oauthAdapter: oauthAdapter,
		config:       config,
		logger:       logger,
	}
}

// RequireAuth middleware validates OAuth sessions and ensures user is authenticated
func (m *AuthMiddlewareAdapter) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Enabled || !m.config.OAuth.Enabled {
			c.Next()
			return
		}

		var sessionID string

		// 1. Try to get token from Authorization header
		authHeader := c.GetHeader("Authorization")

		if strings.HasPrefix(authHeader, "Bearer ") {
			accessToken := strings.TrimPrefix(authHeader, "Bearer ")

			// Extract user info and craete session

			// Get user info
			userInfo, err := m.oauthAdapter.GetUserInfo(accessToken)
			if err != nil {
				m.logger.WithError(err).Error("Failed to get user info due to: " + err.Error())
				c.JSON(400, gin.H{"error": "Failed to get user information"})
				return
			}

			tokenInfo := application.TokenInfo{AccessToken: accessToken}

			//Authenticate user
			result, err := m.authService.AuthenticateWithOAuth(
				c.Request.Context(),
				*userInfo,
				tokenInfo,
				c.ClientIP(),
				c.GetHeader("User-Agent"),
			)
			if err != nil {
				m.logger.WithError(err).Error("Failed to authenticate user")
				c.JSON(500, gin.H{"error": "Authentication failed"})
				return
			}
			sessionID = result.Session.SessionID
		}

		// 2. If not in header, try cookie
		if sessionID == "" {
			m.logger.Debug("Cookie header received")
			cookie, err := c.Cookie("session_id")
			if err != nil || cookie == "" {
				m.handleUnauthenticated(c)
				return
			}
			sessionID = cookie
		}

		session, err := m.authService.ValidateSession(c.Request.Context(), sessionID)
		if err != nil {
			m.logger.WithRequest(c.GetString("request_id"), c.Request.Method, c.Request.URL.Path).
				WithError(err).Debug("Session validation failed")
			m.handleUnauthenticated(c)
			return
		}

		// Set user context
		m.setUserContext(c, session.User, session)
		c.Next()
	}
}

// RequireAdmin middleware ensures user has admin role
func (m *AuthMiddlewareAdapter) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		m.RequireAuth()(c)
		if c.IsAborted() {
			return
		}

		user, exists := m.getUserFromContext(c)
		if !exists || !user.IsAdmin() {
			m.logger.WithRequest(c.GetString("request_id"), c.Request.Method, c.Request.URL.Path).
				WithField("user_id", user.UserID).
				WithField("user_role", user.Role).
				Warn("Admin access denied - insufficient privileges")

			if m.isAPIRequest(c) {
				c.JSON(403, gin.H{"error": "Admin privileges required"})
			} else {
				c.JSON(403, gin.H{"error": "Access denied - admin privileges required"})
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireManager middleware ensures user has manager privileges
func (m *AuthMiddlewareAdapter) RequireManager() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		m.RequireAuth()(c)
		if c.IsAborted() {
			return
		}

		user, exists := m.getUserFromContext(c)
		if !exists || !user.IsTeamManager() {
			m.logger.WithRequest(c.GetString("request_id"), c.Request.Method, c.Request.URL.Path).
				Warn("Manager access denied - insufficient privileges")

			if m.isAPIRequest(c) {
				c.JSON(403, gin.H{"error": "Manager privileges required"})
			} else {
				c.JSON(403, gin.H{"error": "Access denied - manager privileges required"})
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

// generateCodeVerifier creates a random 43-128 character string
func generateCodeVerifier() (string, error) {
	verifierBytes := make([]byte, 32)
	_, err := rand.Read(verifierBytes)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(verifierBytes), nil
}

// generateCodeChallenge creates a base64url-encoded SHA256 hash of the verifier
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// StartOAuthFlow initiates the OAuth authentication flow
func (m *AuthMiddlewareAdapter) StartOAuthFlow() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.OAuth.Enabled {
			c.JSON(400, gin.H{"error": "OAuth not enabled"})
			return
		}

		// Generate state parameter for security
		state, err := m.oauthAdapter.GenerateState()
		if err != nil {
			m.logger.WithError(err).Error("Failed to generate OAuth state")
			c.JSON(500, gin.H{"error": "Failed to start authentication"})
			return
		}

		// Store state in session/cookie for validation
		isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
		c.SetCookie("oauth_state", state, 600, "/", "", isSecure, true) // 10 minutes

		authURL := ""
		if m.config.OAuth.ClientSecret == "" {
			// PKCE Auth Flow

			// Generate code_verifier and code_challenge
			codeVerifier, err := generateCodeVerifier()
			if err != nil {
				m.logger.WithError(err).Error("Failed to generate PKCE code_verifier")
				c.JSON(500, gin.H{"error": "Failed to generate PKCE verifier"})
				return
			}
			codeChallenge := generateCodeChallenge(codeVerifier)

			// Store pkce_verifier in session/cookie for validation
			c.SetCookie("pkce_verifier", codeVerifier, 600, "/", "", isSecure, true)

			// Build auth URL with PKCE parameters
			authURL = m.oauthAdapter.BuildAuthURL(state, codeChallenge)
		} else {
			// Build authorization URL
			authURL = m.oauthAdapter.BuildAuthURL(state)
		}

		c.Redirect(302, authURL)
	}
}

// HandleOAuthCallback handles the OAuth callback
func (m *AuthMiddlewareAdapter) HandleOAuthCallback() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.OAuth.Enabled {
			c.JSON(400, gin.H{"error": "OAuth not enabled"})
			return
		}

		// Debug: see what cookies actually arrived
		m.logger.Debug("OAuth callback invoked")

		// Validate state parameter
		state := c.Query("state")
		expectedState, err := c.Cookie("oauth_state")
		if err != nil || state != expectedState {
			m.logger.WithFields(map[string]interface{}{
				"query_state":  state,
				"cookie_state": expectedState,
				"cookie_err":   err,
			}).Warn("OAuth state validation failed")
			c.JSON(400, gin.H{"error": "Invalid state parameter"})
			return
		}

		// Get authorization code
		code := c.Query("code")
		if code == "" {
			c.JSON(400, gin.H{"error": "Authorization code required"})
			return
		}

		// Get code_verifier from cookie - this will dictate if we are using PKCE
		codeVerifier, _ := c.Cookie("pkce_verifier")

		// Exchange code for token
		tokenInfo, err := m.oauthAdapter.ExchangeCodeForToken(code, codeVerifier)
		if err != nil {
			m.logger.WithError(err).Error("Failed to exchange code for token")
			c.JSON(400, gin.H{"error": "Token exchange failed"})
			return
		}

		// Get user info
		userInfo, err := m.oauthAdapter.GetUserInfo(tokenInfo.AccessToken)
		if err != nil {
			m.logger.WithError(err).Error("Failed to get user info")
			c.JSON(400, gin.H{"error": "Failed to get user information"})
			return
		}

		// Authenticate user
		result, err := m.authService.AuthenticateWithOAuth(
			c.Request.Context(),
			*userInfo,
			*tokenInfo,
			c.ClientIP(),
			c.GetHeader("User-Agent"),
		)
		if err != nil {
			m.logger.WithError(err).Error("Failed to authenticate user")
			c.JSON(500, gin.H{"error": "Authentication failed"})
			return
		}

		// Set session cookie
		m.setSessionCookie(c, result.Session.SessionID)

		// Clear state cookie
		isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
		c.SetCookie("oauth_state", "", -1, "/", "", isSecure, true)

		// Log successful authentication
		m.logger.WithFields(map[string]interface{}{
			"user_id":     result.User.UserID,
			"email":       result.User.Email,
			"session_id":  result.Session.SessionID,
			"is_new_user": result.IsNewUser,
		}).Info("OAuth authentication successful")

		// Redirect to dashboard or intended page
		redirectURL := c.DefaultQuery("redirect", "/")
		c.Redirect(http.StatusFound, redirectURL)
	}
}

// Logout handles user logout
func (m *AuthMiddlewareAdapter) Logout() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID, err := c.Cookie("session_id")
		if err == nil && sessionID != "" {
			// Get session for ID token
			session, _ := m.authService.ValidateSession(c.Request.Context(), sessionID)

			// Invalidate session
			m.authService.Logout(c.Request.Context(), sessionID)

			// Clear session cookie
			isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
			c.SetCookie("session_id", "", -1, "/", "", isSecure, true)

			// Build provider logout URL
			var providerLogoutURL string
			if session != nil {
				providerLogoutURL = m.oauthAdapter.BuildProviderLogoutURL(session.IDToken)
			} else {
				providerLogoutURL = m.oauthAdapter.BuildProviderLogoutURL("")
			}

			// For AJAX requests, return JSON response
			if c.GetHeader("Content-Type") == "application/json" || c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
				c.JSON(200, gin.H{
					"message":    "Logged out successfully",
					"logout_url": providerLogoutURL,
				})
				return
			}

			// For direct requests, redirect to provider logout
			c.Redirect(302, providerLogoutURL)
			return
		}

		// No session to logout
		c.Redirect(302, "/auth/login")
	}
}

// Helper methods

func (m *AuthMiddlewareAdapter) handleUnauthenticated(c *gin.Context) {
	m.logger.WithRequest(c.GetString("request_id"), c.Request.Method, c.Request.URL.Path).
		Debug("Authentication required")

	// Redirect to login for browser requests, return 401 for API requests
	if m.isAPIRequest(c) {
		c.JSON(401, gin.H{"error": "Authentication required"})
	} else {
		c.Redirect(302, "/auth/login")
	}
	c.Abort()
}

func (m *AuthMiddlewareAdapter) isAPIRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, "/api/") ||
		strings.Contains(c.GetHeader("Accept"), "application/json") ||
		strings.Contains(c.GetHeader("Content-Type"), "application/json")
}

func (m *AuthMiddlewareAdapter) setUserContext(c *gin.Context, user *domain.User, session *domain.Session) {
	c.Set("user", user)
	c.Set("user_id", user.UserID)
	c.Set("user_role", string(user.Role))
	c.Set("session", session)
}

func (m *AuthMiddlewareAdapter) getUserFromContext(c *gin.Context) (*domain.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	u, ok := user.(*domain.User)
	return u, ok
}

func (m *AuthMiddlewareAdapter) setSessionCookie(c *gin.Context, sessionID string) {
	// Set secure cookie for 24 hours
	isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetCookie("session_id", sessionID, 86400, "/", "", isSecure, true)
}

// GetAuthUser extracts the authenticated user from Gin context
func GetAuthUser(c *gin.Context) (*domain.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	u, ok := user.(*domain.User)
	return u, ok
}

// GetAuthSession extracts the session from Gin context
func GetAuthSession(c *gin.Context) (*domain.Session, bool) {
	session, exists := c.Get("session")
	if !exists {
		return nil, false
	}

	s, ok := session.(*domain.Session)
	return s, ok
}

// IsAdmin checks if the current user is an admin
func IsAdmin(c *gin.Context) bool {
	user, exists := GetAuthUser(c)
	return exists && user.IsAdmin()
}

// IsTeamManager checks if user is a manager for any team
func IsTeamManager(c *gin.Context) bool {
	user, exists := GetAuthUser(c)
	return exists && user.IsTeamManager()
}

// IsManagerForTeam checks if user is a manager for a specific team
func IsManagerForTeam(c *gin.Context, team string) bool {
	user, exists := GetAuthUser(c)
	return exists && user.IsManagerForTeam(team)
}

// CanAccessTeamProjects checks if user can access projects for a specific team
func CanAccessTeamProjects(c *gin.Context, team string) bool {
	user, exists := GetAuthUser(c)
	if !exists {
		return false
	}

	// Admins can access all teams
	if user.IsAdmin() {
		return true
	}

	// Check if user is in any group for this team
	teamGroups := []string{team + "-managers", team + "-users"}
	for _, group := range user.Groups {
		for _, teamGroup := range teamGroups {
			if group.GroupName == teamGroup || group.GroupName == "/"+teamGroup {
				return true
			}
		}
	}

	return false
}
