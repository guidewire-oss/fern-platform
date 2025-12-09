package interfaces

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/application"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// OAuthAdapter handles OAuth flow interactions
type OAuthAdapter struct {
	config *config.AuthConfig
	logger *logging.Logger
	client *http.Client
}

// NewOAuthAdapter creates a new OAuth adapter
func NewOAuthAdapter(config *config.AuthConfig, logger *logging.Logger) *OAuthAdapter {
	return &OAuthAdapter{
		config: config,
		logger: logger,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GenerateState generates a random state parameter for OAuth
func (a *OAuthAdapter) GenerateState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// BuildAuthURL builds the OAuth authorization URL with optional PKCE support
func (a *OAuthAdapter) BuildAuthURL(state string, codeChallenge ...string) string {
	params := url.Values{}
	params.Add("response_type", "code")
	params.Add("client_id", a.config.OAuth.ClientID)
	params.Add("redirect_uri", a.config.OAuth.RedirectURL)
	params.Add("scope", strings.Join(a.config.OAuth.Scopes, " "))
	params.Add("state", state)

	// Add PKCE parameters if code challenge is provided
	if len(codeChallenge) > 0 && codeChallenge[0] != "" {
		params.Add("code_challenge", codeChallenge[0])
		params.Add("code_challenge_method", "S256")
	}

	return a.config.OAuth.AuthURL + "?" + params.Encode()
}

// ExchangeCodeForToken exchanges authorization code for tokens
func (a *OAuthAdapter) ExchangeCodeForToken(code string, codeVerifier string) (*application.TokenInfo, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", a.config.OAuth.RedirectURL)
	data.Set("client_id", a.config.OAuth.ClientID)
	data.Set("client_secret", a.config.OAuth.ClientSecret)

	if codeVerifier != "" {
		data.Set("code_verifier", codeVerifier) // Required for PKCE
	}

	req, err := http.NewRequest("POST", a.config.OAuth.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token,omitempty"`
		IDToken      string `json:"id_token,omitempty"`
		Scope        string `json:"scope,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &application.TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		ExpiresIn:    tokenResp.ExpiresIn,
	}, nil
}

// GetUserInfo retrieves user information from OAuth provider
func (a *OAuthAdapter) GetUserInfo(accessToken string) (*application.UserInfo, error) {
	a.logger.Debug("Fetching user info from: " + a.config.OAuth.UserInfoURL)

	// Log token type for debugging (first 20 chars)
	tokenPreview := accessToken
	if len(tokenPreview) > 20 {
		tokenPreview = tokenPreview[:20] + "..."
	}
	a.logger.Debugf("Using token: %s", tokenPreview)

	req, err := http.NewRequest("GET", a.config.OAuth.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		a.logger.WithError(err).Error("Failed to make userinfo request")
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		a.logger.Errorf("UserInfo request failed - Status: %d, URL: %s, Response: %s",
			resp.StatusCode, a.config.OAuth.UserInfoURL, string(body))

		// Common Okta 403 errors and solutions
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("userinfo request forbidden (403). Common causes:\n"+
				"  1) Using ID token instead of access token (check you're sending access_token, not id_token)\n"+
				"  2) Access token missing required scopes (must include 'openid')\n"+
				"  3) Token audience mismatch (token 'aud' must match authorization server)\n"+
				"  4) Token expired or invalid\n"+
				"Okta response: %s", string(body))
		}

		return nil, fmt.Errorf("userinfo request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var rawInfo map[string]interface{}
	if err := json.Unmarshal(body, &rawInfo); err != nil {
		return nil, err
	}

	// Extract user info using configurable field mappings
	userInfo := &application.UserInfo{}

	if sub, ok := rawInfo[a.config.OAuth.UserIDField].(string); ok {
		userInfo.Sub = sub
	}
	if email, ok := rawInfo[a.config.OAuth.EmailField].(string); ok {
		userInfo.Email = email
	}
	if name, ok := rawInfo[a.config.OAuth.NameField].(string); ok {
		userInfo.Name = name
	}
	if picture, ok := rawInfo["picture"].(string); ok {
		userInfo.Picture = picture
	}

	// Extract additional fields
	if firstName, ok := rawInfo["given_name"].(string); ok {
		userInfo.FirstName = firstName
	}
	if lastName, ok := rawInfo["family_name"].(string); ok {
		userInfo.LastName = lastName
	}
	if emailVerified, ok := rawInfo["email_verified"].(bool); ok {
		userInfo.EmailVerified = emailVerified
	}

	// Extract groups and roles
	if groups, ok := rawInfo[a.config.OAuth.GroupsField].([]interface{}); ok {
		for _, group := range groups {
			if groupStr, ok := group.(string); ok {
				userInfo.Groups = append(userInfo.Groups, groupStr)
			}
		}
	}

	if roles, ok := rawInfo[a.config.OAuth.RolesField].([]interface{}); ok {
		for _, role := range roles {
			if roleStr, ok := role.(string); ok {
				userInfo.Roles = append(userInfo.Roles, roleStr)
			}
		}
	}

	// Apply admin user/group overrides from config
	a.applyAdminOverrides(userInfo)

	return userInfo, nil
}

// BuildProviderLogoutURL builds the OAuth provider logout URL
func (a *OAuthAdapter) BuildProviderLogoutURL(idToken string) string {
	if !a.config.OAuth.Enabled {
		return "/auth/login"
	}

	// If no ID token, just redirect to local login
	if idToken == "" {
		return "/auth/login"
	}

	// If a specific logout URL is configured, use it
	if a.config.OAuth.LogoutURL != "" {
		logoutURL := a.config.OAuth.LogoutURL

		// Add ID token hint parameter
		separator := "?"
		if strings.Contains(logoutURL, "?") {
			separator = "&"
		}
		logoutURL += fmt.Sprintf("%sid_token_hint=%s", separator, url.QueryEscape(idToken))

		// Add post-logout redirect URI
		if a.config.OAuth.RedirectURL != "" {
			postLogoutURL := strings.Replace(a.config.OAuth.RedirectURL, "/auth/callback", "/auth/login", 1)
			logoutURL += fmt.Sprintf("&post_logout_redirect_uri=%s", url.QueryEscape(postLogoutURL))
		}

		return logoutURL
	}

	// Fallback: try to construct from issuer URL (common OIDC pattern)
	if a.config.OAuth.IssuerURL != "" {
		logoutURL := strings.TrimSuffix(a.config.OAuth.IssuerURL, "/") + "/protocol/openid-connect/logout"
		logoutURL += fmt.Sprintf("?id_token_hint=%s", url.QueryEscape(idToken))

		if a.config.OAuth.RedirectURL != "" {
			postLogoutURL := strings.Replace(a.config.OAuth.RedirectURL, "/auth/callback", "/auth/login", 1)
			logoutURL += fmt.Sprintf("&post_logout_redirect_uri=%s", url.QueryEscape(postLogoutURL))
		}

		return logoutURL
	}

	// Final fallback to local login page
	return "/auth/login"
}

// applyAdminOverrides applies admin user/group configurations
func (a *OAuthAdapter) applyAdminOverrides(userInfo *application.UserInfo) {
	// Check if user should be admin based on config
	for _, adminUser := range a.config.OAuth.AdminUsers {
		if adminUser == userInfo.Email || adminUser == userInfo.Sub {
			// Ensure admin group is present
			hasAdminGroup := false
			for _, group := range userInfo.Groups {
				if group == "admin" || group == "/admin" {
					hasAdminGroup = true
					break
				}
			}
			if !hasAdminGroup {
				userInfo.Groups = append(userInfo.Groups, "admin")
			}
			return
		}
	}

	// Check if any of user's groups are configured as admin groups
	for _, group := range userInfo.Groups {
		for _, adminGroup := range a.config.OAuth.AdminGroups {
			if group == adminGroup {
				// Add admin group if not present
				hasAdminGroup := false
				for _, g := range userInfo.Groups {
					if g == "admin" || g == "/admin" {
						hasAdminGroup = true
						break
					}
				}
				if !hasAdminGroup {
					userInfo.Groups = append(userInfo.Groups, "admin")
				}
				return
			}
		}
	}
}

// GetTokenScopes validates a token and returns all its scopes.
// This function uses OAuth token introspection (RFC 7662) to validate the token with the authorization server.
func (a *OAuthAdapter) GetTokenScopes(accessToken string) ([]string, error) {
	// Check if introspection URL is configured
	if a.config.OAuth.IntrospectionURL == "" {
		return nil, fmt.Errorf("introspection URL not configured")
	}

	// Build introspection request
	data := url.Values{}
	data.Set("token", accessToken)
	data.Set("token_type_hint", "access_token")

	req, err := http.NewRequest("POST", a.config.OAuth.IntrospectionURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create introspection request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Use introspection-specific client credentials if configured, otherwise fall back to main credentials
	clientID := a.config.OAuth.IntrospectionClientID
	clientSecret := a.config.OAuth.IntrospectionClientSecret
	
	if clientID == "" {
		clientID = a.config.OAuth.ClientID
	}
	if clientSecret == "" {
		clientSecret = a.config.OAuth.ClientSecret
	}

	// Add client authentication
	if clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to introspect token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token introspection failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse introspection response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read introspection response: %w", err)
	}

	var introspectionData map[string]interface{}
	if err := json.Unmarshal(body, &introspectionData); err != nil {
		return nil, fmt.Errorf("failed to parse introspection response: %w", err)
	}

	// Check if token is active
	active, ok := introspectionData["active"].(bool)
	if !ok || !active {
		return nil, fmt.Errorf("token is not active")
	}

	// Extract scopes from introspection response
	var scopes []string

	// Try 'scope' field (space-separated string format per RFC 7662)
	if scope, ok := introspectionData["scope"].(string); ok {
		scopes = strings.Split(scope, " ")
	}

	// Try 'scp' field (array format - some providers use this)
	if scp, ok := introspectionData["scp"].([]interface{}); ok {
		for _, s := range scp {
			if scopeStr, ok := s.(string); ok {
				scopes = append(scopes, scopeStr)
			}
		}
	}

	return scopes, nil
}

// ValidateAndCheckScope validates a service account token and checks if it has a specific scope.
// This function assumes the token is from a service account (not a user account).
// It uses OAuth token introspection (RFC 7662) to validate the token with the authorization server.
func (a *OAuthAdapter) ValidateAndCheckScope(accessToken string, requiredScope string) (bool, error) {
	scopes, err := a.GetTokenScopes(accessToken)
	if err != nil {
		return false, err
	}

	// Check if required scope is present
	for _, scope := range scopes {
		if scope == requiredScope {
			return true, nil
		}
	}

	return false, nil
}
