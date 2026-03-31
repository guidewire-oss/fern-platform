// Package middleware provides OAuth 2.0 authentication middleware
package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"gorm.io/gorm"
)

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// debugHTTPClient creates an HTTP client that logs all requests and responses
func (m *OAuthMiddleware) debugHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &debugTransport{
			logger: m.logger,
			base:   http.DefaultTransport,
		},
	}
}

// debugTransport is an HTTP transport that logs requests and responses
type debugTransport struct {
	logger *logging.Logger
	base   http.RoundTripper
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Log request
	reqDump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		t.logger.WithError(err).Error("Failed to dump HTTP request")
	} else {
		// Sanitize authorization header for logging
		reqStr := string(reqDump)
		if strings.Contains(reqStr, "Authorization: Bearer") {
			reqStr = strings.ReplaceAll(reqStr, req.Header.Get("Authorization"), "Authorization: Bearer [REDACTED]")
		}
		t.logger.WithField("request", reqStr).Debug("HTTP Request")
	}

	// Make the request
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		t.logger.WithError(err).Error("HTTP request failed")
		return nil, err
	}

	// Log response
	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		t.logger.WithError(err).Error("Failed to dump HTTP response")
	} else {
		t.logger.WithField("response", string(respDump)).Debug("HTTP Response")
	}

	return resp, nil
}

// OAuthMiddleware provides OAuth 2.0 authentication middleware
type OAuthMiddleware struct {
	config *config.AuthConfig
	db     *gorm.DB
	logger *logging.Logger
}

// NewOAuthMiddleware creates a new OAuth authentication middleware
func NewOAuthMiddleware(cfg *config.AuthConfig, db *gorm.DB, logger *logging.Logger) *OAuthMiddleware {
	return &OAuthMiddleware{
		config: cfg,
		db:     db,
		logger: logger,
	}
}

// RequireOAuth middleware validates OAuth tokens and ensures user is authenticated
func (m *OAuthMiddleware) RequireOAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Enabled || !m.config.OAuth.Enabled {
			c.Next()
			return
		}

		user, session, err := m.validateOAuthSession(c)
		if err != nil {
			m.logger.WithRequest(c.GetString("request_id"), c.Request.Method, c.Request.URL.Path).
				WithError(err).Warn("OAuth authentication failed")

			// Clear expired session and oauth state cookies
			isSecure := m.shouldUseSecureCookie(c)
			cookieDomain := m.getCookieDomain()
			c.SetSameSite(http.SameSiteLaxMode)
			c.SetCookie("session_id", "", -1, "/", cookieDomain, isSecure, true)
			c.SetCookie("oauth_state", "", -1, "/", cookieDomain, isSecure, true)

			// Redirect to login for browser requests, return 401 for API requests
			if m.isAPIRequest(c) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			} else {
				c.Redirect(http.StatusFound, "/auth/login")
			}
			c.Abort()
			return
		}

		// Set user context
		m.setUserContext(c, user, session)
		c.Next()
	}
}

// RequireAdmin middleware ensures user has admin role
func (m *OAuthMiddleware) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		m.RequireOAuth()(c)
		if c.IsAborted() {
			return
		}

		user, exists := m.getUserFromContext(c)
		if !exists || user.Role != string(database.RoleAdmin) {
			m.logger.WithRequest(c.GetString("request_id"), c.Request.Method, c.Request.URL.Path).
				WithField("user_id", user.UserID).
				WithField("user_role", user.Role).
				Warn("Admin access denied - insufficient privileges")

			if m.isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			} else {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied - admin privileges required"})
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireManager middleware ensures user has manager privileges (admin or team manager)
func (m *OAuthMiddleware) RequireManager() gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		m.RequireOAuth()(c)
		if c.IsAborted() {
			return
		}

		if !IsTeamManager(c) {
			m.logger.WithRequest(c.GetString("request_id"), c.Request.Method, c.Request.URL.Path).
				Warn("Manager access denied - insufficient privileges")

			if m.isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Manager privileges required"})
			} else {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied - manager privileges required"})
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireProjectAccess middleware ensures user has access to specific project
func (m *OAuthMiddleware) RequireProjectAccess(minRole database.ProjectRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		m.RequireOAuth()(c)
		if c.IsAborted() {
			return
		}

		projectID := c.Param("projectId")
		if projectID == "" {
			projectID = c.Query("project_id")
		}

		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID required"})
			c.Abort()
			return
		}

		user, exists := m.getUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		// Admin users have access to all projects
		if user.Role == string(database.RoleAdmin) {
			c.Next()
			return
		}

		// Check project-specific access
		hasAccess, err := m.checkProjectAccess(user.UserID, projectID, minRole)
		if err != nil {
			m.logger.WithError(err).Error("Failed to check project access")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Access check failed"})
			c.Abort()
			return
		}

		if !hasAccess {
			m.logger.WithRequest(c.GetString("request_id"), c.Request.Method, c.Request.URL.Path).
				WithField("user_id", user.UserID).
				WithField("project_id", projectID).
				WithField("required_role", minRole).
				Warn("Project access denied")

			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient project access"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// StartOAuthFlow initiates the OAuth authentication flow
func (m *OAuthMiddleware) StartOAuthFlow() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.OAuth.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth not enabled"})
			return
		}

		// Generate state parameter for security
		state, err := m.generateState()
		if err != nil {
			m.logger.WithError(err).Error("Failed to generate OAuth state")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start authentication"})
			return
		}

		// Prevent proxies and browsers from caching this redirect — a cached response
		// would have the Set-Cookie header stripped, so the oauth_state cookie would
		// never reach the browser and every subsequent login attempt would fail.
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")

		// Store state in session/cookie for validation
		isSecure := m.shouldUseSecureCookie(c)
		cookieDomain := m.getCookieDomain()

		// Clear any existing oauth_state cookie first to prevent conflicts
		// This is important when user logs out and tries to login again
		// We need to clear with both SameSite modes to ensure removal
		if isSecure {
			// Clear with SameSiteNone first (in case it was set that way)
			c.SetSameSite(http.SameSiteNoneMode)
			c.SetCookie("oauth_state", "", -1, "/", cookieDomain, isSecure, true)
		}
		// Always clear with SameSiteLax as well
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie("oauth_state", "", -1, "/", cookieDomain, isSecure, true)

		// Set SameSite to None for cross-site requests when using HTTPS
		// Use Lax for HTTP to maintain CSRF protection
		if isSecure {
			c.SetSameSite(http.SameSiteNoneMode)
		} else {
			c.SetSameSite(http.SameSiteLaxMode)
		}

		// Set the new state cookie with explicit path
		c.SetCookie("oauth_state", state, 600, "/", cookieDomain, isSecure, true) // 10 minutes

		// For debugging: also set a backup cookie without HttpOnly to check via JavaScript
		if os.Getenv("DEBUG_OAUTH") == "true" {
			c.SetCookie("oauth_state_debug", state, 600, "/", cookieDomain, isSecure, false)

			// Additional debug logging
			m.logger.WithFields(map[string]interface{}{
				"state":         state,
				"cookie_domain": cookieDomain,
				"cookie_secure": isSecure,
				"request_url":   c.Request.URL.String(),
				"request_host":  c.Request.Host,
				"user_agent":    c.GetHeader("User-Agent"),
			}).Info("OAuth login initiated with state")
		}

		// Log cookie settings for debugging
		m.logger.WithFields(map[string]interface{}{
			"secure": isSecure,
			"domain": cookieDomain,
			"sameSite": func() string {
				if isSecure {
					return "None"
				}
				return "Lax"
			}(),
			"x_forwarded_proto": c.GetHeader("X-Forwarded-Proto"),
			"x_forwarded_host":  c.GetHeader("X-Forwarded-Host"),
			"host":              c.Request.Host,
		}).Debug("Setting OAuth state cookie")

		// Build authorization URL
		authURL := m.buildAuthURL(state)

		c.Redirect(http.StatusFound, authURL)
	}
}

// HandleOAuthCallback handles the OAuth callback
func (m *OAuthMiddleware) HandleOAuthCallback() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.OAuth.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth not enabled"})
			return
		}

		// Validate state parameter
		state := c.Query("state")
		if state == "" {
			m.logger.Warn("OAuth callback missing state parameter")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing state parameter"})
			return
		}

		expectedState, err := c.Cookie("oauth_state")
		if err != nil {
			// Log all cookies for debugging
			debugInfo := map[string]interface{}{
				"all_cookies":       c.Request.Header.Get("Cookie"),
				"host":              c.Request.Host,
				"x_forwarded_host":  c.GetHeader("X-Forwarded-Host"),
				"x_forwarded_proto": c.GetHeader("X-Forwarded-Proto"),
				"referer":           c.GetHeader("Referer"),
				"state_param":       state,
				"request_url":       c.Request.URL.String(),
			}

			// Check if debug cookie exists
			if os.Getenv("DEBUG_OAUTH") == "true" {
				if debugState, debugErr := c.Cookie("oauth_state_debug"); debugErr == nil {
					debugInfo["oauth_state_debug"] = debugState
				}
			}

			m.logger.WithError(err).WithFields(debugInfo).Warn("OAuth state cookie not found")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Session expired. Please login again."})
			return
		}

		if state != expectedState {
			debugInfo := map[string]interface{}{
				"state":           state,
				"expected":        expectedState,
				"state_length":    len(state),
				"expected_length": len(expectedState),
				"all_cookies":     c.Request.Header.Get("Cookie"),
				"host":            c.Request.Host,
				"referer":         c.GetHeader("Referer"),
			}

			// Check debug cookie in case of mismatch
			if os.Getenv("DEBUG_OAUTH") == "true" {
				if debugState, debugErr := c.Cookie("oauth_state_debug"); debugErr == nil {
					debugInfo["oauth_state_debug"] = debugState
					debugInfo["debug_matches_expected"] = (debugState == expectedState)
					debugInfo["debug_matches_state"] = (debugState == state)
				}
			}

			m.logger.WithFields(debugInfo).Warn("OAuth state mismatch")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state parameter. Please login again."})
			return
		}

		// Get authorization code
		code := c.Query("code")
		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code required"})
			return
		}

		// Exchange code for token
		token, err := m.exchangeCodeForToken(code)
		if err != nil {
			m.logger.WithError(err).Error("Failed to exchange code for token")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Token exchange failed"})
			return
		}

		// Get user info
		userInfo, err := m.getUserInfo(token.AccessToken)
		if err != nil {
			m.logger.WithError(err).Error("Failed to get user info")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get user information"})
			return
		}

		// Create or update user
		user, err := m.createOrUpdateUser(userInfo)
		if err != nil {
			m.logger.WithError(err).WithFields(map[string]interface{}{
				"email":  userInfo.Email,
				"sub":    userInfo.Sub,
				"groups": userInfo.Groups,
			}).Error("Failed to create/update user")
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("User creation failed: %v", err)})
			return
		}

		// Create session
		session, err := m.createSession(user, token, c.ClientIP(), c.GetHeader("User-Agent"))
		if err != nil {
			m.logger.WithError(err).Error("Failed to create session")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Session creation failed"})
			return
		}

		// Set session cookie
		m.setSessionCookie(c, session.SessionID)

		// Clear state cookie
		isSecure := m.shouldUseSecureCookie(c)
		cookieDomain := m.getCookieDomain()
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie("oauth_state", "", -1, "/", cookieDomain, isSecure, true)

		// Update last login
		m.updateUserLastLogin(user.UserID)

		// Log successful authentication
		m.logger.WithFields(map[string]interface{}{
			"user_id":    user.UserID,
			"email":      user.Email,
			"session_id": session.SessionID,
			"groups":     userInfo.Groups,
		}).Info("OAuth authentication successful")

		// Redirect to dashboard or intended page
		redirectURL := c.DefaultQuery("redirect", "/")
		c.Redirect(http.StatusFound, redirectURL)
	}
}

// Logout handles user logout
func (m *OAuthMiddleware) Logout() gin.HandlerFunc {
	return func(c *gin.Context) {
		var idToken string

		sessionID, err := c.Cookie("session_id")
		if err == nil && sessionID != "" {
			// Get ID token from session before invalidating
			idToken = m.getIDTokenFromSession(sessionID)

			// Invalidate session in database
			m.invalidateSession(sessionID)
		}

		// Clear session cookie
		isSecure := m.shouldUseSecureCookie(c)
		cookieDomain := m.getCookieDomain()

		// Match the SameSite setting used during login
		if isSecure {
			c.SetSameSite(http.SameSiteNoneMode)
		} else {
			c.SetSameSite(http.SameSiteLaxMode)
		}

		c.SetCookie("session_id", "", -1, "/", cookieDomain, isSecure, true)
		c.SetCookie("oauth_state", "", -1, "/", cookieDomain, isSecure, true) // Clear any residual state

		// Build provider logout URL with ID token hint
		providerLogoutURL := m.buildProviderLogoutURL(idToken)

		// For AJAX requests, return JSON response with provider logout URL
		if c.GetHeader("Content-Type") == "application/json" || c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
			c.JSON(http.StatusOK, gin.H{
				"message":    "Logged out successfully",
				"logout_url": providerLogoutURL,
			})
			return
		}

		// For direct requests, redirect to provider logout
		c.Redirect(http.StatusFound, providerLogoutURL)
	}
}

// buildProviderLogoutURL constructs the OAuth provider logout URL with ID token hint
func (m *OAuthMiddleware) buildProviderLogoutURL(idToken string) string {
	if !m.config.OAuth.Enabled {
		return "/auth/login"
	}

	// If no ID token, just redirect to local login to avoid the error
	if idToken == "" {
		return "/auth/login"
	}

	// If a specific logout URL is configured, use it
	if m.config.OAuth.LogoutURL != "" {
		logoutURL := m.config.OAuth.LogoutURL

		// Add ID token hint parameter
		separator := "?"
		if strings.Contains(logoutURL, "?") {
			separator = "&"
		}
		logoutURL += fmt.Sprintf("%sid_token_hint=%s", separator, url.QueryEscape(idToken))

		// Add post-logout redirect URI
		redirectURL := m.config.OAuth.RedirectURL
		if redirectURL != "" {
			// Replace callback with login
			postLogoutURL := strings.Replace(redirectURL, "/auth/callback", "/auth/login", 1)
			logoutURL += fmt.Sprintf("&post_logout_redirect_uri=%s", url.QueryEscape(postLogoutURL))
		}

		return logoutURL
	}

	// Fallback: try to construct from issuer URL (common OIDC pattern)
	if m.config.OAuth.IssuerURL != "" {
		logoutURL := strings.TrimSuffix(m.config.OAuth.IssuerURL, "/") + "/protocol/openid-connect/logout"

		// Add ID token hint
		logoutURL += fmt.Sprintf("?id_token_hint=%s", url.QueryEscape(idToken))

		// Add post-logout redirect URI
		redirectURL := m.config.OAuth.RedirectURL
		if redirectURL != "" {
			// Replace callback with login
			postLogoutURL := strings.Replace(redirectURL, "/auth/callback", "/auth/login", 1)
			logoutURL += fmt.Sprintf("&post_logout_redirect_uri=%s", url.QueryEscape(postLogoutURL))
		}

		return logoutURL
	}

	// Final fallback to local login page if no provider logout available
	return "/auth/login"
}

// getIDTokenFromSession retrieves the ID token from the session
func (m *OAuthMiddleware) getIDTokenFromSession(sessionID string) string {
	var session database.UserSession
	err := m.db.Where("session_id = ? AND is_active = ?", sessionID, true).First(&session).Error
	if err != nil {
		return ""
	}
	return session.IDToken
}

// ValidateSession validates the current session and returns user info
func (m *OAuthMiddleware) ValidateSession(c *gin.Context) (*database.User, *database.UserSession, error) {
	return m.validateOAuthSession(c)
}

// Helper methods

func (m *OAuthMiddleware) validateOAuthSession(c *gin.Context) (*database.User, *database.UserSession, error) {
	sessionID, err := c.Cookie("session_id")
	if err != nil {
		m.logger.WithError(err).Debug("No session cookie found")
		return nil, nil, fmt.Errorf("no session cookie")
	}

	m.logger.WithField("session_id", sessionID).Debug("Validating session")

	var session database.UserSession
	err = m.db.Where("session_id = ? AND is_active = ? AND expires_at > ?",
		sessionID, true, time.Now()).First(&session).Error
	if err != nil {
		m.logger.WithError(err).WithField("session_id", sessionID).Debug("Session validation failed")
		return nil, nil, fmt.Errorf("invalid or expired session")
	}

	var user database.User
	err = m.db.Preload("UserGroups").Preload("UserScopes").Where("user_id = ?", session.UserID).First(&user).Error
	if err != nil {
		return nil, nil, fmt.Errorf("user not found")
	}

	if user.Status != "active" {
		return nil, nil, fmt.Errorf("user account inactive")
	}

	// Update last activity
	m.db.Model(&session).Update("last_activity", time.Now())

	return &user, &session, nil
}

func (m *OAuthMiddleware) isAPIRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, "/api/") ||
		strings.Contains(c.GetHeader("Accept"), "application/json") ||
		strings.Contains(c.GetHeader("Content-Type"), "application/json")
}

func (m *OAuthMiddleware) setUserContext(c *gin.Context, user *database.User, session *database.UserSession) {
	c.Set("user", user)
	c.Set("user_id", user.UserID)
	c.Set("user_role", user.Role)
	c.Set("session", session)
}

func (m *OAuthMiddleware) getUserFromContext(c *gin.Context) (*database.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	u, ok := user.(*database.User)
	return u, ok
}

func (m *OAuthMiddleware) checkProjectAccess(userID, projectID string, minRole database.ProjectRole) (bool, error) {
	var access database.ProjectAccess
	err := m.db.Where("user_id = ? AND project_id = ? AND (expires_at IS NULL OR expires_at > ?)",
		userID, projectID, time.Now()).First(&access).Error

	if err != nil {
		return false, nil // No access found
	}

	// Check role hierarchy: admin > editor > viewer
	userRole := database.ProjectRole(access.Role)

	switch minRole {
	case database.ProjectRoleViewer:
		return userRole == database.ProjectRoleViewer ||
			userRole == database.ProjectRoleEditor ||
			userRole == database.ProjectRoleAdmin, nil
	case database.ProjectRoleEditor:
		return userRole == database.ProjectRoleEditor ||
			userRole == database.ProjectRoleAdmin, nil
	case database.ProjectRoleAdmin:
		return userRole == database.ProjectRoleAdmin, nil
	default:
		return false, nil
	}
}

func (m *OAuthMiddleware) generateState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (m *OAuthMiddleware) buildAuthURL(state string) string {
	params := url.Values{}
	params.Add("response_type", "code")
	params.Add("client_id", m.config.OAuth.ClientID)
	params.Add("redirect_uri", m.config.OAuth.RedirectURL)
	params.Add("scope", strings.Join(m.config.OAuth.Scopes, " "))
	params.Add("state", state)

	return m.config.OAuth.AuthURL + "?" + params.Encode()
}

// OAuth token response structure
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"` // ID token for logout
	Scope        string `json:"scope,omitempty"`
}

func (m *OAuthMiddleware) exchangeCodeForToken(code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", m.config.OAuth.RedirectURL)
	data.Set("client_id", m.config.OAuth.ClientID)
	data.Set("client_secret", m.config.OAuth.ClientSecret)

	req, err := http.NewRequest("POST", m.config.OAuth.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := m.debugHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	m.logger.WithField("token_type", token.TokenType).
		WithField("expires_in", token.ExpiresIn).
		WithField("access_token_length", len(token.AccessToken)).
		WithField("access_token_prefix", token.AccessToken[:min(50, len(token.AccessToken))]).
		Debug("Successfully exchanged code for token")

	return &token, nil
}

// UserInfo represents user information from OAuth provider
type UserInfo struct {
	Sub        string                 `json:"sub"`
	Email      string                 `json:"email"`
	Name       string                 `json:"name"`
	Picture    string                 `json:"picture"`
	Groups     []string               `json:"groups"`
	Roles      []string               `json:"roles"`
	Attributes map[string]interface{} `json:"-"` // Store all other attributes
}

func (m *OAuthMiddleware) getUserInfo(accessToken string) (*UserInfo, error) {
	m.logger.WithField("userinfo_url", m.config.OAuth.UserInfoURL).
		WithField("token_length", len(accessToken)).
		WithField("token_prefix", accessToken[:min(50, len(accessToken))]).
		Debug("Making userinfo request")

	req, err := http.NewRequest("GET", m.config.OAuth.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := m.debugHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		m.logger.WithError(err).Error("Userinfo request failed")
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		m.logger.WithField("status", resp.StatusCode).
			WithField("response_body", string(body)).
			Error("Userinfo request returned non-200 status")
		return nil, fmt.Errorf("userinfo request failed with status: %d", resp.StatusCode)
	}

	var rawInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawInfo); err != nil {
		return nil, err
	}

	// Extract user info using configurable field mappings
	userInfo := &UserInfo{
		Attributes: rawInfo,
	}

	if sub, ok := rawInfo[m.config.OAuth.UserIDField].(string); ok {
		userInfo.Sub = sub
	}
	if email, ok := rawInfo[m.config.OAuth.EmailField].(string); ok {
		userInfo.Email = email
	}
	if name, ok := rawInfo[m.config.OAuth.NameField].(string); ok {
		userInfo.Name = name
	}
	if picture, ok := rawInfo["picture"].(string); ok {
		userInfo.Picture = picture
	}

	// Extract groups and roles
	if groups, ok := rawInfo[m.config.OAuth.GroupsField].([]interface{}); ok {
		for _, group := range groups {
			if groupStr, ok := group.(string); ok {
				userInfo.Groups = append(userInfo.Groups, groupStr)
			}
		}
	}

	if roles, ok := rawInfo[m.config.OAuth.RolesField].([]interface{}); ok {
		for _, role := range roles {
			if roleStr, ok := role.(string); ok {
				userInfo.Roles = append(userInfo.Roles, roleStr)
			}
		}
	}

	return userInfo, nil
}

func (m *OAuthMiddleware) createOrUpdateUser(userInfo *UserInfo) (*database.User, error) {
	var user database.User

	// Try to find existing user
	err := m.db.Where("user_id = ? OR email = ?", userInfo.Sub, userInfo.Email).First(&user).Error

	if err == gorm.ErrRecordNotFound {
		// Create new user
		user = database.User{
			UserID:     userInfo.Sub,
			Email:      userInfo.Email,
			Name:       userInfo.Name,
			Role:       m.determineUserRole(userInfo),
			Status:     "active",
			ProfileURL: userInfo.Picture,
		}

		// Safely extract additional fields from attributes
		if firstName, ok := userInfo.Attributes["given_name"].(string); ok {
			user.FirstName = firstName
		}
		if lastName, ok := userInfo.Attributes["family_name"].(string); ok {
			user.LastName = lastName
		}
		if emailVerified, ok := userInfo.Attributes["email_verified"].(bool); ok {
			user.EmailVerified = emailVerified
		}

		if err := m.db.Create(&user).Error; err != nil {
			return nil, err
		}

		// Create user group memberships
		for _, group := range userInfo.Groups {
			userGroup := database.UserGroup{
				UserID:    user.UserID,
				GroupName: group,
			}
			if err := m.db.Create(&userGroup).Error; err != nil {
				m.logger.WithError(err).WithField("group", group).Warn("Failed to create user group membership")
			}
		}
	} else if err != nil {
		return nil, err
	} else {
		// Update existing user
		user.Email = userInfo.Email
		user.Name = userInfo.Name
		user.ProfileURL = userInfo.Picture

		// Safely extract additional fields from attributes
		if firstName, ok := userInfo.Attributes["given_name"].(string); ok {
			user.FirstName = firstName
		}
		if lastName, ok := userInfo.Attributes["family_name"].(string); ok {
			user.LastName = lastName
		}
		if emailVerified, ok := userInfo.Attributes["email_verified"].(bool); ok {
			user.EmailVerified = emailVerified
		}

		// Update role if needed
		newRole := m.determineUserRole(userInfo)
		if newRole != user.Role {
			user.Role = newRole
		}

		if err := m.db.Save(&user).Error; err != nil {
			return nil, err
		}

		// Update user group memberships - remove old ones and add new ones
		if err := m.db.Where("user_id = ?", user.UserID).Delete(&database.UserGroup{}).Error; err != nil {
			m.logger.WithError(err).Warn("Failed to delete old user groups")
		}

		for _, group := range userInfo.Groups {
			userGroup := database.UserGroup{
				UserID:    user.UserID,
				GroupName: group,
			}
			if err := m.db.Create(&userGroup).Error; err != nil {
				m.logger.WithError(err).WithField("group", group).Warn("Failed to create user group membership")
			}
		}
	}

	return &user, nil
}

func (m *OAuthMiddleware) determineUserRole(userInfo *UserInfo) string {
	// Check if user is in admin users list
	for _, adminUser := range m.config.OAuth.AdminUsers {
		if adminUser == userInfo.Email || adminUser == userInfo.Sub {
			return string(database.RoleAdmin)
		}
	}

	// Check if user is in admin groups
	for _, group := range userInfo.Groups {
		// Check for admin group
		if group == "admin" || group == "/admin" {
			return string(database.RoleAdmin)
		}

		// Check configured admin groups
		for _, adminGroup := range m.config.OAuth.AdminGroups {
			if group == adminGroup {
				return string(database.RoleAdmin)
			}
		}

		// Check for manager groups (team-managers pattern)
		if strings.HasSuffix(group, "-managers") || strings.Contains(group, "managers") {
			return string(database.RoleUser) // Managers are still "users" at the system level
		}
	}

	// Check user role mapping
	if role, exists := m.config.OAuth.UserRoleMapping[userInfo.Email]; exists {
		return role
	}
	if role, exists := m.config.OAuth.UserRoleMapping[userInfo.Sub]; exists {
		return role
	}

	// Check group role mapping
	for _, group := range userInfo.Groups {
		if role, exists := m.config.OAuth.GroupRoleMapping[group]; exists {
			return role
		}
	}

	// Default to regular user
	return string(database.RoleUser)
}

func (m *OAuthMiddleware) createSession(user *database.User, token *TokenResponse, ipAddress, userAgent string) (*database.UserSession, error) {
	sessionID, err := m.generateState() // Reuse state generation for session ID
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	if token.ExpiresIn == 0 {
		expiresAt = time.Now().Add(24 * time.Hour) // Default 24 hours
	}

	session := database.UserSession{
		UserID:       user.UserID,
		SessionID:    sessionID,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		IDToken:      token.IDToken,
		ExpiresAt:    expiresAt,
		IsActive:     true,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		LastActivity: time.Now(),
	}

	if err := m.db.Create(&session).Error; err != nil {
		return nil, err
	}

	return &session, nil
}

func (m *OAuthMiddleware) setSessionCookie(c *gin.Context, sessionID string) {
	// Set secure cookie for 24 hours
	// Use Secure flag in production (HTTPS)
	isSecure := m.shouldUseSecureCookie(c)
	cookieDomain := m.getCookieDomain()

	// Set SameSite to Lax for CSRF protection
	c.SetSameSite(http.SameSiteLaxMode)

	// Default to 24 hours
	var maxAge int = 86400

	c.SetCookie("session_id", sessionID, maxAge, "/", cookieDomain, isSecure, true)
}

func (m *OAuthMiddleware) updateUserLastLogin(userID string) {
	m.db.Model(&database.User{}).Where("user_id = ?", userID).Update("last_login_at", time.Now())
}

func (m *OAuthMiddleware) invalidateSession(sessionID string) {
	m.db.Model(&database.UserSession{}).Where("session_id = ?", sessionID).Update("is_active", false)
}

// shouldUseSecureCookie determines if secure cookies should be used
func (m *OAuthMiddleware) shouldUseSecureCookie(c *gin.Context) bool {
	// Check if explicitly configured
	if cookieSecure := os.Getenv("COOKIE_SECURE"); cookieSecure != "" {
		return cookieSecure == "true"
	}

	// Check if we trust proxy headers
	if trustProxy := os.Getenv("TRUST_PROXY"); trustProxy == "true" {
		// Trust X-Forwarded-Proto header
		if proto := c.GetHeader("X-Forwarded-Proto"); proto == "https" {
			return true
		}
	}

	// Fall back to TLS check
	return c.Request.TLS != nil
}

// getCookieDomain returns the configured cookie domain
func (m *OAuthMiddleware) getCookieDomain() string {
	if domain := os.Getenv("COOKIE_DOMAIN"); domain != "" {
		return domain
	}
	return "" // Empty string means use current domain
}

// Helper functions for Gin context

// GetOAuthUser extracts the OAuth user from Gin context
func GetOAuthUser(c *gin.Context) (*database.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	u, ok := user.(*database.User)
	return u, ok
}

// GetOAuthSession extracts the OAuth session from Gin context
func GetOAuthSession(c *gin.Context) (*database.UserSession, bool) {
	session, exists := c.Get("session")
	if !exists {
		return nil, false
	}

	s, ok := session.(*database.UserSession)
	return s, ok
}

// IsAdmin checks if the current user is an admin
func IsAdmin(c *gin.Context) bool {
	user, exists := GetOAuthUser(c)
	return exists && user.Role == string(database.RoleAdmin)
}

// GetUserTeams extracts team names from user groups
func GetUserTeams(c *gin.Context) []string {
	user, exists := GetOAuthUser(c)
	if !exists {
		return nil
	}

	var teams []string
	for _, group := range user.UserGroups {
		groupName := group.GroupName
		// Remove leading slash if present
		groupName = strings.TrimPrefix(groupName, "/")

		// Extract team name from group pattern (e.g., "fern-managers" -> "fern")
		if strings.HasSuffix(groupName, "-managers") {
			team := strings.TrimSuffix(groupName, "-managers")
			teams = append(teams, team)
		} else if strings.HasSuffix(groupName, "-users") {
			team := strings.TrimSuffix(groupName, "-users")
			teams = append(teams, team)
		}
	}

	return teams
}

// IsTeamManager checks if user is a manager for any team
func IsTeamManager(c *gin.Context) bool {
	user, exists := GetOAuthUser(c)
	if !exists {
		return false
	}

	// Admins are always managers
	if user.Role == string(database.RoleAdmin) {
		return true
	}

	// Check if user is in any manager group
	for _, group := range user.UserGroups {
		groupName := strings.TrimPrefix(group.GroupName, "/")
		if strings.HasSuffix(groupName, "-managers") {
			return true
		}
	}

	return false
}

// IsManagerForTeam checks if user is a manager for a specific team
func IsManagerForTeam(c *gin.Context, team string) bool {
	user, exists := GetOAuthUser(c)
	if !exists {
		return false
	}

	// Admins can manage all teams
	if user.Role == string(database.RoleAdmin) {
		return true
	}

	// Check if user is in the specific team's manager group
	managerGroup := team + "-managers"
	for _, group := range user.UserGroups {
		groupName := strings.TrimPrefix(group.GroupName, "/")
		if groupName == managerGroup {
			return true
		}
	}

	return false
}

// CanAccessTeamProjects checks if user can access projects for a specific team
func CanAccessTeamProjects(c *gin.Context, team string) bool {
	user, exists := GetOAuthUser(c)
	if !exists {
		return false
	}

	// Admins can access all teams
	if user.Role == string(database.RoleAdmin) {
		return true
	}

	// Check if user is in any group for this team
	teamGroups := []string{team + "-managers", team + "-users"}
	for _, group := range user.UserGroups {
		groupName := strings.TrimPrefix(group.GroupName, "/")
		for _, teamGroup := range teamGroups {
			if groupName == teamGroup {
				return true
			}
		}
	}

	return false
}

// GetUserScopes extracts scopes from user
func GetUserScopes(c *gin.Context) []string {
	user, exists := GetOAuthUser(c)
	if !exists {
		return nil
	}

	scopes := make([]string, 0, len(user.UserScopes))
	now := time.Now()

	for _, scope := range user.UserScopes {
		// Skip expired scopes
		if scope.ExpiresAt != nil && scope.ExpiresAt.Before(now) {
			continue
		}
		scopes = append(scopes, scope.Scope)
	}

	return scopes
}

// HasScope checks if user has a specific scope
func HasScope(c *gin.Context, requiredScope string) bool {
	scopes := GetUserScopes(c)
	for _, scope := range scopes {
		if matchScope(scope, requiredScope) {
			return true
		}
	}
	return false
}

// matchScope matches a scope pattern with wildcards
func matchScope(userScope, requiredScope string) bool {
	// Exact match
	if userScope == requiredScope {
		return true
	}

	// Split scopes into parts
	userParts := strings.Split(userScope, ":")
	requiredParts := strings.Split(requiredScope, ":")

	// Must have same number of parts
	if len(userParts) != len(requiredParts) {
		return false
	}

	// Check each part
	for i := range userParts {
		if userParts[i] == "*" || requiredParts[i] == "*" {
			continue
		}
		if userParts[i] != requiredParts[i] {
			return false
		}
	}

	return true
}

// CanManageProject checks if user can perform a specific action on a project
func (m *OAuthMiddleware) CanManageProject(c *gin.Context, projectID string, action string) bool {
	user, exists := GetOAuthUser(c)
	if !exists {
		return false
	}

	// Admin can do anything
	if user.Role == string(database.RoleAdmin) {
		return true
	}

	// Get project to check team
	var project database.ProjectDetails
	if err := m.db.Where("project_id = ?", projectID).First(&project).Error; err != nil {
		return false
	}

	// Check scopes
	requiredScopes := []string{
		fmt.Sprintf("project:%s:%s", action, projectID),      // Specific project
		fmt.Sprintf("project:%s:%s:*", action, project.Team), // Team wildcard
		fmt.Sprintf("project:*:%s", projectID),               // All actions on project
		fmt.Sprintf("project:*:%s:*", project.Team),          // All actions on team
		"project:*:*", // Global project admin
	}

	userScopes := GetUserScopes(c)
	for _, scope := range userScopes {
		for _, required := range requiredScopes {
			if matchScope(scope, required) {
				return true
			}
		}
	}

	// Check explicit project permissions in database
	var perm database.ProjectPermission
	now := time.Now()
	err := m.db.Where("project_id = ? AND user_id = ? AND permission IN ? AND (expires_at IS NULL OR expires_at > ?)",
		projectID, user.UserID, []string{action, "admin"}, now).First(&perm).Error

	return err == nil
}
