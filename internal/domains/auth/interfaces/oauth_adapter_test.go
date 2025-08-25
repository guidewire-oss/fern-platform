package interfaces

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/application"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestOAuthAdapter() *OAuthAdapter {
	cfg := &config.AuthConfig{
		OAuth: config.OAuthConfig{
			Enabled:      true,
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "http://localhost:8080/auth/callback",
			AuthURL:      "https://provider.com/auth",
			TokenURL:     "https://provider.com/token",
			UserInfoURL:  "https://provider.com/userinfo",
			LogoutURL:    "https://provider.com/logout",
			IssuerURL:    "https://provider.com",
			Scopes:       []string{"openid", "profile", "email"},
			UserIDField:  "sub",
			EmailField:   "email",
			NameField:    "name",
			GroupsField:  "groups",
			RolesField:   "roles",
			AdminUsers:   []string{"admin@example.com"},
			AdminGroups:  []string{"/admin-group"},
		},
	}

	loggingConfig := &config.LoggingConfig{
		Level:  "info",
		Format: "json",
	}

	logger, _ := logging.NewLogger(loggingConfig)
	return NewOAuthAdapter(cfg, logger)
}

func TestNewOAuthAdapter(t *testing.T) {
	cfg := &config.AuthConfig{}

	loggingConfig := &config.LoggingConfig{
		Level:  "info",
		Format: "json",
	}
	logger, _ := logging.NewLogger(loggingConfig)

	adapter := NewOAuthAdapter(cfg, logger)

	assert.NotNil(t, adapter)
	assert.Equal(t, cfg, adapter.config)
	assert.Equal(t, logger, adapter.logger)
	assert.NotNil(t, adapter.client)
	assert.Equal(t, 10*time.Second, adapter.client.Timeout)
}

func TestGenerateState(t *testing.T) {
	adapter := createTestOAuthAdapter()

	state1, err := adapter.GenerateState()
	require.NoError(t, err)
	assert.NotEmpty(t, state1)
	assert.Greater(t, len(state1), 40) // Base64 encoded 32 bytes should be longer

	state2, err := adapter.GenerateState()
	require.NoError(t, err)
	assert.NotEmpty(t, state2)
	assert.NotEqual(t, state1, state2) // Should generate different states
}

func TestBuildAuthURL(t *testing.T) {
	adapter := createTestOAuthAdapter()
	state := "test-state-123"

	authURL := adapter.BuildAuthURL(state)

	parsedURL, err := url.Parse(authURL)
	require.NoError(t, err)

	assert.Equal(t, "https", parsedURL.Scheme)
	assert.Equal(t, "provider.com", parsedURL.Host)
	assert.Equal(t, "/auth", parsedURL.Path)

	params := parsedURL.Query()
	assert.Equal(t, "code", params.Get("response_type"))
	assert.Equal(t, "test-client-id", params.Get("client_id"))
	assert.Equal(t, "http://localhost:8080/auth/callback", params.Get("redirect_uri"))
	assert.Equal(t, "openid profile email", params.Get("scope"))
	assert.Equal(t, state, params.Get("state"))
}

func TestBuildAuthURLWithPKCE(t *testing.T) {
	adapter := createTestOAuthAdapter()
	state := "test-state-123"
	codeChallenge := "test-code-challenge"

	authURL := adapter.BuildAuthURL(state, codeChallenge)

	parsedURL, err := url.Parse(authURL)
	require.NoError(t, err)

	params := parsedURL.Query()
	assert.Equal(t, "code", params.Get("response_type"))
	assert.Equal(t, "test-client-id", params.Get("client_id"))
	assert.Equal(t, "http://localhost:8080/auth/callback", params.Get("redirect_uri"))
	assert.Equal(t, "openid profile email", params.Get("scope"))
	assert.Equal(t, state, params.Get("state"))
	assert.Equal(t, codeChallenge, params.Get("code_challenge"))
	assert.Equal(t, "S256", params.Get("code_challenge_method"))
}

func TestExchangeCodeForToken_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		// Parse form data
		err := r.ParseForm()
		require.NoError(t, err)

		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "test-auth-code", r.Form.Get("code"))
		assert.Equal(t, "http://localhost:8080/auth/callback", r.Form.Get("redirect_uri"))
		assert.Equal(t, "test-client-id", r.Form.Get("client_id"))
		assert.Equal(t, "test-client-secret", r.Form.Get("client_secret"))

		// Send successful response
		response := map[string]interface{}{
			"access_token":  "test-access-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "test-refresh-token",
			"id_token":      "test-id-token",
			"scope":         "openid profile email",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := createTestOAuthAdapter()
	adapter.config.OAuth.TokenURL = server.URL

	tokenInfo, err := adapter.ExchangeCodeForToken("test-auth-code", "")

	require.NoError(t, err)
	assert.Equal(t, "test-access-token", tokenInfo.AccessToken)
	assert.Equal(t, "test-refresh-token", tokenInfo.RefreshToken)
	assert.Equal(t, "test-id-token", tokenInfo.IDToken)
	assert.Equal(t, 3600, tokenInfo.ExpiresIn)
}

func TestExchangeCodeForToken_WithPKCE(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		require.NoError(t, err)

		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "test-auth-code", r.Form.Get("code"))
		assert.Equal(t, "test-code-verifier", r.Form.Get("code_verifier"))

		response := map[string]interface{}{
			"access_token": "test-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := createTestOAuthAdapter()
	adapter.config.OAuth.TokenURL = server.URL

	tokenInfo, err := adapter.ExchangeCodeForToken("test-auth-code", "test-code-verifier")

	require.NoError(t, err)
	assert.Equal(t, "test-access-token", tokenInfo.AccessToken)
}

func TestExchangeCodeForToken_Error(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid_grant", "error_description": "The provided authorization grant is invalid"}`))
	}))
	defer server.Close()

	adapter := createTestOAuthAdapter()
	adapter.config.OAuth.TokenURL = server.URL

	tokenInfo, err := adapter.ExchangeCodeForToken("invalid-code", "")

	require.Error(t, err)
	assert.Nil(t, tokenInfo)
	assert.Contains(t, err.Error(), "token exchange failed")
}

func TestGetUserInfo_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		response := map[string]interface{}{
			"sub":            "user123",
			"email":          "user@example.com",
			"name":           "Test User",
			"picture":        "https://example.com/avatar.jpg",
			"given_name":     "Test",
			"family_name":    "User",
			"email_verified": true,
			"groups":         []interface{}{"group1", "group2"},
			"roles":          []interface{}{"user", "viewer"},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := createTestOAuthAdapter()
	adapter.config.OAuth.UserInfoURL = server.URL

	userInfo, err := adapter.GetUserInfo("test-access-token")

	require.NoError(t, err)
	assert.Equal(t, "user123", userInfo.Sub)
	assert.Equal(t, "user@example.com", userInfo.Email)
	assert.Equal(t, "Test User", userInfo.Name)
	assert.Equal(t, "https://example.com/avatar.jpg", userInfo.Picture)
	assert.Equal(t, "Test", userInfo.FirstName)
	assert.Equal(t, "User", userInfo.LastName)
	assert.True(t, userInfo.EmailVerified)
	assert.Equal(t, []string{"group1", "group2"}, userInfo.Groups)
	assert.Equal(t, []string{"user", "viewer"}, userInfo.Roles)
}

func TestGetUserInfo_AdminUser(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"sub":    "admin123",
			"email":  "admin@example.com",
			"name":   "Admin User",
			"groups": []interface{}{"regular-group"},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := createTestOAuthAdapter()
	adapter.config.OAuth.UserInfoURL = server.URL

	userInfo, err := adapter.GetUserInfo("test-access-token")

	require.NoError(t, err)
	assert.Equal(t, "admin@example.com", userInfo.Email)
	assert.Contains(t, userInfo.Groups, "admin")         // Should be added due to AdminUsers config
	assert.Contains(t, userInfo.Groups, "regular-group") // Original group should remain
}

func TestGetUserInfo_AdminGroup(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"sub":    "user456",
			"email":  "user@example.com",
			"name":   "Regular User",
			"groups": []interface{}{"/admin-group", "other-group"},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := createTestOAuthAdapter()
	adapter.config.OAuth.UserInfoURL = server.URL

	userInfo, err := adapter.GetUserInfo("test-access-token")

	require.NoError(t, err)
	assert.Contains(t, userInfo.Groups, "admin")        // Should be added due to AdminGroups config
	assert.Contains(t, userInfo.Groups, "/admin-group") // Original admin group should remain
	assert.Contains(t, userInfo.Groups, "other-group")  // Other groups should remain
}

func TestGetUserInfo_Error(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid_token"}`))
	}))
	defer server.Close()

	adapter := createTestOAuthAdapter()
	adapter.config.OAuth.UserInfoURL = server.URL

	userInfo, err := adapter.GetUserInfo("invalid-token")

	require.Error(t, err)
	assert.Nil(t, userInfo)
	assert.Contains(t, err.Error(), "userinfo request failed with status 401")
}

func TestBuildProviderLogoutURL(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func(*config.AuthConfig)
		idToken     string
		expected    string
	}{
		{
			name: "OAuth disabled",
			setupConfig: func(cfg *config.AuthConfig) {
				cfg.OAuth.Enabled = false
			},
			idToken:  "test-id-token",
			expected: "/auth/login",
		},
		{
			name: "Empty ID token",
			setupConfig: func(cfg *config.AuthConfig) {
				cfg.OAuth.Enabled = true
			},
			idToken:  "",
			expected: "/auth/login",
		},
		{
			name: "With configured logout URL",
			setupConfig: func(cfg *config.AuthConfig) {
				cfg.OAuth.Enabled = true
				cfg.OAuth.LogoutURL = "https://provider.com/logout"
				cfg.OAuth.RedirectURL = "http://localhost:8080/auth/callback"
			},
			idToken:  "test-id-token",
			expected: "https://provider.com/logout?id_token_hint=test-id-token&post_logout_redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fauth%2Flogin",
		},
		{
			name: "With logout URL containing query params",
			setupConfig: func(cfg *config.AuthConfig) {
				cfg.OAuth.Enabled = true
				cfg.OAuth.LogoutURL = "https://provider.com/logout?param=value"
				cfg.OAuth.RedirectURL = "http://localhost:8080/auth/callback"
			},
			idToken:  "test-id-token",
			expected: "https://provider.com/logout?param=value&id_token_hint=test-id-token&post_logout_redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fauth%2Flogin",
		},
		{
			name: "No logout URL or issuer URL",
			setupConfig: func(cfg *config.AuthConfig) {
				cfg.OAuth.Enabled = true
				cfg.OAuth.LogoutURL = ""
				cfg.OAuth.IssuerURL = ""
			},
			idToken:  "test-id-token",
			expected: "/auth/login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := createTestOAuthAdapter()
			tt.setupConfig(adapter.config)

			result := adapter.BuildProviderLogoutURL(tt.idToken)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyAdminOverrides(t *testing.T) {
	tests := []struct {
		name        string
		userInfo    *application.UserInfo
		adminUsers  []string
		adminGroups []string
		expectAdmin bool
	}{
		{
			name: "Admin user by email",
			userInfo: &application.UserInfo{
				Email:  "admin@example.com",
				Groups: []string{"regular-group"},
			},
			adminUsers:  []string{"admin@example.com"},
			adminGroups: []string{},
			expectAdmin: true,
		},
		{
			name: "Admin user by sub",
			userInfo: &application.UserInfo{
				Sub:    "admin123",
				Email:  "user@example.com",
				Groups: []string{"regular-group"},
			},
			adminUsers:  []string{"admin123"},
			adminGroups: []string{},
			expectAdmin: true,
		},
		{
			name: "Admin user by group",
			userInfo: &application.UserInfo{
				Email:  "user@example.com",
				Groups: []string{"regular-group", "/admin-group"},
			},
			adminUsers:  []string{},
			adminGroups: []string{"/admin-group"},
			expectAdmin: true,
		},
		{
			name: "Regular user",
			userInfo: &application.UserInfo{
				Email:  "user@example.com",
				Groups: []string{"regular-group"},
			},
			adminUsers:  []string{"admin@example.com"},
			adminGroups: []string{"/admin-group"},
			expectAdmin: false,
		},
		{
			name: "User already has admin group",
			userInfo: &application.UserInfo{
				Email:  "admin@example.com",
				Groups: []string{"admin", "regular-group"},
			},
			adminUsers:  []string{"admin@example.com"},
			adminGroups: []string{},
			expectAdmin: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := createTestOAuthAdapter()
			adapter.config.OAuth.AdminUsers = tt.adminUsers
			adapter.config.OAuth.AdminGroups = tt.adminGroups

			adapter.applyAdminOverrides(tt.userInfo)

			hasAdmin := false
			for _, group := range tt.userInfo.Groups {
				if group == "admin" || group == "/admin" {
					hasAdmin = true
					break
				}
			}

			assert.Equal(t, tt.expectAdmin, hasAdmin)
		})
	}
}

func TestGetUserInfo_CustomFieldMappings(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"user_id":       "custom123",
			"email_address": "custom@example.com",
			"display_name":  "Custom User",
			"user_groups":   []interface{}{"custom-group"},
			"user_roles":    []interface{}{"custom-role"},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter := createTestOAuthAdapter()
	adapter.config.OAuth.UserInfoURL = server.URL
	// Override field mappings
	adapter.config.OAuth.UserIDField = "user_id"
	adapter.config.OAuth.EmailField = "email_address"
	adapter.config.OAuth.NameField = "display_name"
	adapter.config.OAuth.GroupsField = "user_groups"
	adapter.config.OAuth.RolesField = "user_roles"

	userInfo, err := adapter.GetUserInfo("test-access-token")

	require.NoError(t, err)
	assert.Equal(t, "custom123", userInfo.Sub)
	assert.Equal(t, "custom@example.com", userInfo.Email)
	assert.Equal(t, "Custom User", userInfo.Name)
	assert.Equal(t, []string{"custom-group"}, userInfo.Groups)
	assert.Equal(t, []string{"custom-role"}, userInfo.Roles)
}

// Benchmark tests
func BenchmarkGenerateState(b *testing.B) {
	adapter := createTestOAuthAdapter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := adapter.GenerateState()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildAuthURL(b *testing.B) {
	adapter := createTestOAuthAdapter()
	state := "test-state"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = adapter.BuildAuthURL(state)
	}
}
