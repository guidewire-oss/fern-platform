package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestOAuthMiddleware(t *testing.T, authEnabled, oauthEnabled bool) *OAuthMiddleware {
	t.Helper()
	logger, err := logging.NewLogger(&config.LoggingConfig{
		Level:  "error",
		Format: "json",
	})
	require.NoError(t, err)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate required tables
	err = db.AutoMigrate(&database.User{}, &database.UserSession{}, &database.UserGroup{}, &database.UserScope{}, &database.ProjectAccess{}, &database.ProjectPermission{})
	require.NoError(t, err)

	return NewOAuthMiddleware(&config.AuthConfig{
		Enabled: authEnabled,
		OAuth: config.OAuthConfig{
			Enabled:      oauthEnabled,
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURL:  "http://localhost:8080/auth/callback",
			AuthURL:      "http://localhost:9090/auth",
			TokenURL:     "http://localhost:9090/token",
			UserInfoURL:  "http://localhost:9090/userinfo",
			Scopes:       []string{"openid", "profile", "email"},
		},
	}, db, logger)
}

func TestNewOAuthMiddleware(t *testing.T) {
	m := newTestOAuthMiddleware(t, true, true)
	require.NotNil(t, m)
	assert.NotNil(t, m.config)
	assert.NotNil(t, m.db)
	assert.NotNil(t, m.logger)
}

func TestRequireOAuth_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, false, false)

	router := gin.New()
	router.Use(m.RequireOAuth())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireOAuth_NoSession_APIRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, true)

	router := gin.New()
	router.Use(m.RequireOAuth())
	router.GET("/api/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Accept", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authentication required")
}

func TestRequireOAuth_NoSession_BrowserRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, true)

	router := gin.New()
	router.Use(m.RequireOAuth())
	router.GET("/dashboard", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("Accept", "text/html")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/auth/login")
}

func TestStartOAuthFlow_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, false)

	router := gin.New()
	router.GET("/auth/login", m.StartOAuthFlow())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/login", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "OAuth not enabled")
}

func TestStartOAuthFlow_Enabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, true)

	router := gin.New()
	router.GET("/auth/login", m.StartOAuthFlow())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/login", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	assert.Contains(t, location, "http://localhost:9090/auth")
	assert.Contains(t, location, "client_id=test-client")
	assert.Contains(t, location, "response_type=code")
}

func TestLogout_NoCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, true)

	router := gin.New()
	router.GET("/auth/logout", m.Logout())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/logout", nil)
	router.ServeHTTP(w, req)

	// Without a session cookie, should still redirect to login
	assert.Equal(t, http.StatusFound, w.Code)
}

func TestLogout_AJAXRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, true)

	router := gin.New()
	router.GET("/auth/logout", m.Logout())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/logout", nil)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Logged out successfully")
}

func TestHandleOAuthCallback_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, false)

	router := gin.New()
	router.GET("/auth/callback", m.HandleOAuthCallback())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=xyz", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "OAuth not enabled")
}

func TestHandleOAuthCallback_MissingState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, true)

	router := gin.New()
	router.GET("/auth/callback", m.HandleOAuthCallback())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/callback?code=abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing state parameter")
}

func TestHandleOAuthCallback_NoStateCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, true)

	router := gin.New()
	router.GET("/auth/callback", m.HandleOAuthCallback())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=xyz", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Session expired")
}

func TestHandleOAuthCallback_StateMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, true)

	router := gin.New()
	router.GET("/auth/callback", m.HandleOAuthCallback())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=wrong-state", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "expected-state"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid state parameter")
}

func TestHandleOAuthCallback_MissingCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestOAuthMiddleware(t, true, true)

	router := gin.New()
	router.GET("/auth/callback", m.HandleOAuthCallback())

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/callback?state=my-state", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "my-state"})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization code required")
}

// Test helper functions

func TestGetOAuthUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user present", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		user := &database.User{UserID: "u1", Email: "u1@test.com"}
		c.Set("user", user)

		result, ok := GetOAuthUser(c)
		assert.True(t, ok)
		assert.Equal(t, "u1", result.UserID)
	})

	t.Run("user not present", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		result, ok := GetOAuthUser(c)
		assert.False(t, ok)
		assert.Nil(t, result)
	})
}

func TestGetOAuthSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("session present", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		session := &database.UserSession{SessionID: "sess-1"}
		c.Set("session", session)

		result, ok := GetOAuthSession(c)
		assert.True(t, ok)
		assert.Equal(t, "sess-1", result.SessionID)
	})

	t.Run("session not present", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		result, ok := GetOAuthSession(c)
		assert.False(t, ok)
		assert.Nil(t, result)
	})
}

func TestIsAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("admin user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{Role: string(database.RoleAdmin)})

		assert.True(t, IsAdmin(c))
	})

	t.Run("regular user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{Role: string(database.RoleUser)})

		assert.False(t, IsAdmin(c))
	})

	t.Run("no user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		assert.False(t, IsAdmin(c))
	})
}

func TestIsTeamManager(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("admin is manager", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{
			Role: string(database.RoleAdmin),
		})
		assert.True(t, IsTeamManager(c))
	})

	t.Run("user with manager group", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{
			Role: string(database.RoleUser),
			UserGroups: []database.UserGroup{
				{GroupName: "team-a-managers"},
			},
		})
		assert.True(t, IsTeamManager(c))
	})

	t.Run("regular user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{
			Role:       string(database.RoleUser),
			UserGroups: []database.UserGroup{{GroupName: "team-a-users"}},
		})
		assert.False(t, IsTeamManager(c))
	})

	t.Run("no user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		assert.False(t, IsTeamManager(c))
	})
}

func TestIsManagerForTeam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("admin manages all", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{Role: string(database.RoleAdmin)})

		assert.True(t, IsManagerForTeam(c, "any-team"))
	})

	t.Run("manager for specific team", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{
			Role:       string(database.RoleUser),
			UserGroups: []database.UserGroup{{GroupName: "fern-managers"}},
		})

		assert.True(t, IsManagerForTeam(c, "fern"))
		assert.False(t, IsManagerForTeam(c, "other-team"))
	})
}

func TestGetUserTeams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user with team groups", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{
			UserGroups: []database.UserGroup{
				{GroupName: "/fern-managers"},
				{GroupName: "core-users"},
				{GroupName: "admin"}, // not a team pattern
			},
		})

		teams := GetUserTeams(c)
		assert.Contains(t, teams, "fern")
		assert.Contains(t, teams, "core")
	})

	t.Run("no user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		teams := GetUserTeams(c)
		assert.Nil(t, teams)
	})
}

func TestCanAccessTeamProjects(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("admin accesses all", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{Role: string(database.RoleAdmin)})

		assert.True(t, CanAccessTeamProjects(c, "any-team"))
	})

	t.Run("user in team group", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{
			Role:       string(database.RoleUser),
			UserGroups: []database.UserGroup{{GroupName: "fern-users"}},
		})

		assert.True(t, CanAccessTeamProjects(c, "fern"))
		assert.False(t, CanAccessTeamProjects(c, "other"))
	})

	t.Run("no user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		assert.False(t, CanAccessTeamProjects(c, "fern"))
	})
}

func TestGetUserScopes_OAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user with valid scopes", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user", &database.User{
			UserScopes: []database.UserScope{
				{Scope: "read:projects"},
				{Scope: "write:projects"},
			},
		})

		scopes := GetUserScopes(c)
		assert.Contains(t, scopes, "read:projects")
		assert.Contains(t, scopes, "write:projects")
	})

	t.Run("no user", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		scopes := GetUserScopes(c)
		assert.Nil(t, scopes)
	})
}

func TestHasScope_OAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", &database.User{
		UserScopes: []database.UserScope{
			{Scope: "read:projects"},
			{Scope: "write:*"},
		},
	})

	assert.True(t, HasScope(c, "read:projects"))
	assert.True(t, HasScope(c, "write:anything"))
	assert.False(t, HasScope(c, "delete:projects"))
}

func TestMatchScope_OAuth(t *testing.T) {
	// matchScope in oauth.go has same logic as helpers.go
	tests := []struct {
		name     string
		user     string
		required string
		want     bool
	}{
		{"exact", "a:b", "a:b", true},
		{"wildcard", "a:*", "a:b", true},
		{"mismatch", "a:b", "a:c", false},
		{"different lengths", "a:b:c", "a:b", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchScope(tt.user, tt.required))
		})
	}
}

func TestBuildProviderLogoutURL(t *testing.T) {
	t.Run("oauth disabled", func(t *testing.T) {
		m := newTestOAuthMiddleware(t, true, false)
		url := m.buildProviderLogoutURL("some-token")
		assert.Equal(t, "/auth/login", url)
	})

	t.Run("no id token", func(t *testing.T) {
		m := newTestOAuthMiddleware(t, true, true)
		url := m.buildProviderLogoutURL("")
		assert.Equal(t, "/auth/login", url)
	})

	t.Run("with configured logout URL", func(t *testing.T) {
		logger, _ := logging.NewLogger(&config.LoggingConfig{Level: "error", Format: "json"})
		db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})

		m := NewOAuthMiddleware(&config.AuthConfig{
			Enabled: true,
			OAuth: config.OAuthConfig{
				Enabled:     true,
				LogoutURL:   "https://provider.com/logout",
				RedirectURL: "http://localhost:8080/auth/callback",
			},
		}, db, logger)

		url := m.buildProviderLogoutURL("test-id-token")
		assert.Contains(t, url, "https://provider.com/logout")
		assert.Contains(t, url, "id_token_hint=test-id-token")
		assert.Contains(t, url, "post_logout_redirect_uri")
	})

	t.Run("with issuer URL fallback", func(t *testing.T) {
		logger, _ := logging.NewLogger(&config.LoggingConfig{Level: "error", Format: "json"})
		db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})

		m := NewOAuthMiddleware(&config.AuthConfig{
			Enabled: true,
			OAuth: config.OAuthConfig{
				Enabled:     true,
				IssuerURL:   "https://keycloak.local/realms/test",
				RedirectURL: "http://localhost:8080/auth/callback",
			},
		}, db, logger)

		url := m.buildProviderLogoutURL("test-id-token")
		assert.Contains(t, url, "keycloak.local")
		assert.Contains(t, url, "openid-connect/logout")
		assert.Contains(t, url, "id_token_hint=test-id-token")
	})
}

func TestDetermineUserRole(t *testing.T) {
	logger, _ := logging.NewLogger(&config.LoggingConfig{Level: "error", Format: "json"})
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})

	t.Run("admin user by email", func(t *testing.T) {
		m := NewOAuthMiddleware(&config.AuthConfig{
			Enabled: true,
			OAuth: config.OAuthConfig{
				Enabled:    true,
				AdminUsers: []string{"admin@test.com"},
			},
		}, db, logger)

		role := m.determineUserRole(&UserInfo{Email: "admin@test.com"})
		assert.Equal(t, "admin", role)
	})

	t.Run("admin user by sub", func(t *testing.T) {
		m := NewOAuthMiddleware(&config.AuthConfig{
			Enabled: true,
			OAuth: config.OAuthConfig{
				Enabled:    true,
				AdminUsers: []string{"admin-sub-id"},
			},
		}, db, logger)

		role := m.determineUserRole(&UserInfo{Sub: "admin-sub-id"})
		assert.Equal(t, "admin", role)
	})

	t.Run("admin group", func(t *testing.T) {
		m := NewOAuthMiddleware(&config.AuthConfig{
			Enabled: true,
			OAuth: config.OAuthConfig{
				Enabled: true,
			},
		}, db, logger)

		role := m.determineUserRole(&UserInfo{Groups: []string{"admin"}})
		assert.Equal(t, "admin", role)
	})

	t.Run("configured admin group", func(t *testing.T) {
		m := NewOAuthMiddleware(&config.AuthConfig{
			Enabled: true,
			OAuth: config.OAuthConfig{
				Enabled:     true,
				AdminGroups: []string{"super-admins"},
			},
		}, db, logger)

		role := m.determineUserRole(&UserInfo{Groups: []string{"super-admins"}})
		assert.Equal(t, "admin", role)
	})

	t.Run("user role mapping", func(t *testing.T) {
		m := NewOAuthMiddleware(&config.AuthConfig{
			Enabled: true,
			OAuth: config.OAuthConfig{
				Enabled:         true,
				UserRoleMapping: map[string]string{"special@test.com": "admin"},
			},
		}, db, logger)

		role := m.determineUserRole(&UserInfo{Email: "special@test.com"})
		assert.Equal(t, "admin", role)
	})

	t.Run("group role mapping", func(t *testing.T) {
		m := NewOAuthMiddleware(&config.AuthConfig{
			Enabled: true,
			OAuth: config.OAuthConfig{
				Enabled:          true,
				GroupRoleMapping: map[string]string{"devs": "admin"},
			},
		}, db, logger)

		role := m.determineUserRole(&UserInfo{Groups: []string{"devs"}})
		assert.Equal(t, "admin", role)
	})

	t.Run("default role", func(t *testing.T) {
		m := NewOAuthMiddleware(&config.AuthConfig{
			Enabled: true,
			OAuth: config.OAuthConfig{
				Enabled: true,
			},
		}, db, logger)

		role := m.determineUserRole(&UserInfo{Email: "regular@test.com"})
		assert.Equal(t, "user", role)
	})
}

func TestIsAPIRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger, _ := logging.NewLogger(&config.LoggingConfig{Level: "error", Format: "json"})
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	m := NewOAuthMiddleware(&config.AuthConfig{}, db, logger)

	tests := []struct {
		name   string
		path   string
		accept string
		ct     string
		want   bool
	}{
		{"api path", "/api/v1/test", "", "", true},
		{"json accept", "/dashboard", "application/json", "", true},
		{"json content-type", "/dashboard", "", "application/json", true},
		{"browser request", "/dashboard", "text/html", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", tt.path, nil)
			if tt.accept != "" {
				c.Request.Header.Set("Accept", tt.accept)
			}
			if tt.ct != "" {
				c.Request.Header.Set("Content-Type", tt.ct)
			}
			assert.Equal(t, tt.want, m.isAPIRequest(c))
		})
	}
}

func TestBuildAuthURL(t *testing.T) {
	m := newTestOAuthMiddleware(t, true, true)

	url := m.buildAuthURL("test-state-123")
	assert.Contains(t, url, "http://localhost:9090/auth")
	assert.Contains(t, url, "client_id=test-client")
	assert.Contains(t, url, "state=test-state-123")
	assert.Contains(t, url, "response_type=code")
	assert.Contains(t, url, "scope=openid+profile+email")
}

func TestGenerateState(t *testing.T) {
	m := newTestOAuthMiddleware(t, true, true)

	state1, err := m.generateState()
	require.NoError(t, err)
	assert.NotEmpty(t, state1)

	state2, err := m.generateState()
	require.NoError(t, err)
	assert.NotEmpty(t, state2)

	assert.NotEqual(t, state1, state2, "states should be unique")
}
