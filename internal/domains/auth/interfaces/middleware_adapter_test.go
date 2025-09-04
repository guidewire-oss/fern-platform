package interfaces_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	_ "net/http"
	"net/http/httptest"
	_ "strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/auth/interfaces"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

type fakeAuthService struct {
	validateFn func(ctx context.Context, sid string) (*domain.Session, error)
	authFn     func(ctx context.Context, u application.UserInfo, t application.TokenInfo, ip, ua string) (*application.AuthenticateResult, error)
	logoutFn   func(ctx context.Context, sid string) error
}

func (f *fakeAuthService) ValidateSession(ctx context.Context, sid string) (*domain.Session, error) {
	return f.validateFn(ctx, sid)
}
func (f *fakeAuthService) AuthenticateWithOAuth(ctx context.Context, u application.UserInfo, t application.TokenInfo, ip, ua string) (*application.AuthenticateResult, error) {
	return f.authFn(ctx, u, t, ip, ua)
}
func (f *fakeAuthService) Logout(ctx context.Context, sid string) error {
	return f.logoutFn(ctx, sid)
}

type fakeOAuthAdapter struct {
	state     string
	stateErr  error
	authURL   string
	token     *application.TokenInfo
	tokenErr  error
	user      *application.UserInfo
	userErr   error
	logoutURL string
}

func (f *fakeOAuthAdapter) GenerateState() (string, error) { return f.state, f.stateErr }
func (f *fakeOAuthAdapter) BuildAuthURL(state string, codeChallenge ...string) string {
	if len(codeChallenge) > 0 {
		return f.authURL + "?cc=" + codeChallenge[0]
	}
	return f.authURL
}
func (f *fakeOAuthAdapter) ExchangeCodeForToken(code, verifier string) (*application.TokenInfo, error) {
	return f.token, f.tokenErr
}
func (f *fakeOAuthAdapter) GetUserInfo(accessToken string) (*application.UserInfo, error) {
	return f.user, f.userErr
}
func (f *fakeOAuthAdapter) BuildProviderLogoutURL(idToken string) string { return f.logoutURL }

var _ = Describe("AuthMiddlewareAdapter", Label("auth"), func() {
	var (
		authSvc  *fakeAuthService
		oauth    *fakeOAuthAdapter
		cfg      *config.AuthConfig
		logger   *logging.Logger
		adapter  *interfaces.AuthMiddlewareAdapter
		recorder *httptest.ResponseRecorder
		c        *gin.Context
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		authSvc = &fakeAuthService{}
		oauth = &fakeOAuthAdapter{state: "state123", authURL: "http://auth", logoutURL: "http://logout"}
		cfg = &config.AuthConfig{Enabled: true, OAuth: config.OAuthConfig{Enabled: true}}
		logger, _ = logging.NewLogger(&config.LoggingConfig{
			Level: "debug",
		})
		adapter = interfaces.NewAuthMiddlewareAdapter(authSvc, nil, oauth, cfg, logger)
		recorder = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest("GET", "/", nil)
	})

	Describe("GenerateCodeVerifier/Challenge", func() {
		It("generates verifier and challenge", func() {
			verifier, err := interfaces.GenerateCodeVerifier()
			Expect(err).To(BeNil())
			Expect(len(verifier)).To(BeNumerically(">", 10))

			challenge := interfaces.GenerateCodeChallenge(verifier)
			h := sha256.Sum256([]byte(verifier))
			Expect(challenge).To(Equal(base64.RawURLEncoding.EncodeToString(h[:])))
		})
	})

	Describe("RequireAuth", func() {
		It("authenticates with bearer token", func() {
			oauth.user = &application.UserInfo{Sub: "u1"}
			authSvc.authFn = func(ctx context.Context, u application.UserInfo, t application.TokenInfo, ip, ua string) (*application.AuthenticateResult, error) {
				return &application.AuthenticateResult{Session: &domain.Session{SessionID: "sid1"}, User: &domain.User{UserID: "u1"}}, nil
			}
			authSvc.validateFn = func(ctx context.Context, sid string) (*domain.Session, error) {
				return &domain.Session{SessionID: sid, User: &domain.User{UserID: "u1"}}, nil
			}

			c.Request.Header.Set("Authorization", "Bearer abc")
			mw := adapter.RequireAuth()
			mw(c)

			Expect(c.IsAborted()).To(BeFalse())
			Expect(c.Keys["user_id"]).To(Equal("u1"))
		})

		It("fails with invalid token", func() {
			oauth.userErr = errors.New("bad token")
			c.Request.Header.Set("Authorization", "Bearer bad")
			mw := adapter.RequireAuth()
			mw(c)

			Expect(recorder.Code).To(Equal(400))
		})

		It("redirects to login if no session", func() {
			authSvc.validateFn = func(ctx context.Context, sid string) (*domain.Session, error) { return nil, errors.New("bad session") }
			mw := adapter.RequireAuth()
			mw(c)

			Expect(recorder.Code).To(Equal(302))
			Expect(recorder.Header().Get("Location")).To(Equal("/auth/login"))
		})
	})

	Describe("StartOAuthFlow", func() {
		It("redirects with PKCE flow when no client secret", func() {
			cfg.OAuth.ClientSecret = ""
			mw := adapter.StartOAuthFlow()
			mw(c)

			Expect(recorder.Code).To(Equal(302))
			Expect(recorder.Header().Get("Location")).To(ContainSubstring("http://auth"))
		})

		It("returns error if state generation fails", func() {
			oauth.stateErr = errors.New("fail")
			mw := adapter.StartOAuthFlow()
			mw(c)
			Expect(recorder.Code).To(Equal(500))
		})
	})

	Describe("HandleOAuthCallback", func() {
		BeforeEach(func() {
			c.Request = httptest.NewRequest("GET", "/callback?state=state123&code=abc", nil)
			c.SetCookie("oauth_state", "state123", 600, "/", "", false, true)
		})

		It("completes happy path", func() {
			// attach cookie to *incoming request*
			req := httptest.NewRequest("GET", "/callback?state=state123&code=abc", nil)
			req.AddCookie(&http.Cookie{
				Name:  "oauth_state",
				Value: "state123",
			})
			c.Request = req

			oauth.token = &application.TokenInfo{AccessToken: "tok"}
			oauth.user = &application.UserInfo{Sub: "u1"}
			authSvc.authFn = func(ctx context.Context, u application.UserInfo, t application.TokenInfo, ip, ua string) (*application.AuthenticateResult, error) {
				return &application.AuthenticateResult{
					Session: &domain.Session{SessionID: "s1"},
					User:    &domain.User{UserID: "u1"},
				}, nil
			}

			mw := adapter.HandleOAuthCallback()
			mw(c)

			Expect(recorder.Code).To(Equal(302))
			Expect(recorder.Header().Get("Location")).To(Equal("/"))
		})

		It("fails with invalid state", func() {
			c.Request = httptest.NewRequest("GET", "/callback?state=wrong", nil)
			mw := adapter.HandleOAuthCallback()
			mw(c)
			Expect(recorder.Code).To(Equal(400))
		})

		It("fails with missing code", func() {
			c.Request = httptest.NewRequest("GET", "/callback?state=state123", nil)
			c.SetCookie("oauth_state", "state123", 600, "/", "", false, true)
			mw := adapter.HandleOAuthCallback()
			mw(c)
			Expect(recorder.Code).To(Equal(400))
		})
	})

	Describe("Logout", func() {
		It("clears session and redirects", func() {
			authSvc.validateFn = func(ctx context.Context, sid string) (*domain.Session, error) {
				return &domain.Session{SessionID: "s1", IDToken: "idtok"}, nil
			}
			authSvc.logoutFn = func(ctx context.Context, sid string) error { return nil }
			c.SetCookie("session_id", "s1", 600, "/", "", false, true)

			mw := adapter.Logout()
			mw(c)

			Expect(recorder.Code).To(Equal(302))
			Expect(recorder.Header().Get("Location")).To(Equal("/auth/login"))
		})

		It("redirects to login if no session", func() {
			mw := adapter.Logout()
			mw(c)
			Expect(recorder.Code).To(Equal(302))
			Expect(recorder.Header().Get("Location")).To(Equal("/auth/login"))
		})
	})

	// New tests for extra cases
	Describe("Logout extra cases", func() {
		It("returns JSON for AJAX request", func() {
			authSvc.validateFn = func(ctx context.Context, sid string) (*domain.Session, error) {
				return &domain.Session{SessionID: "s1", IDToken: "idtok"}, nil
			}
			authSvc.logoutFn = func(ctx context.Context, sid string) error { return nil }

			req := httptest.NewRequest("POST", "/logout", nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: "s1"})
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			c.Request = req

			mw := adapter.Logout()
			mw(c)

			Expect(recorder.Code).To(Equal(200))
			Expect(recorder.Body.String()).To(ContainSubstring("Logged out successfully"))
		})

		It("redirects even if ValidateSession returns nil", func() {
			authSvc.validateFn = func(ctx context.Context, sid string) (*domain.Session, error) {
				return nil, nil
			}
			authSvc.logoutFn = func(ctx context.Context, sid string) error { return nil }

			req := httptest.NewRequest("POST", "/logout", nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: "s1"})

			w := httptest.NewRecorder()
			r := gin.New()
			r.POST("/logout", adapter.Logout()) // attach middleware as route handler

			r.ServeHTTP(w, req) // actually run HTTP request through Gin

			Expect(w.Code).To(Equal(302))
			Expect(w.Header().Get("Location")).To(Equal("http://logout"))
		})
	})

	Describe("RequireAdmin", func() {
		It("denies non-admin user", func() {
			u := &domain.User{UserID: "u1", Role: domain.RoleUser}
			s := &domain.Session{SessionID: "s1", User: u}
			authSvc.validateFn = func(ctx context.Context, sid string) (*domain.Session, error) { return s, nil }

			req := httptest.NewRequest("GET", "/api/admin", nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: "s1"})
			w := httptest.NewRecorder()
			r := gin.New()
			r.GET("/api/admin", adapter.RequireAdmin())

			r.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(403))
			Expect(w.Body.String()).To(ContainSubstring("Admin privileges required"))
		})

		It("allows admin user", func() {
			u := &domain.User{UserID: "admin1", Role: domain.RoleAdmin}
			s := &domain.Session{SessionID: "s1", User: u}
			authSvc.validateFn = func(ctx context.Context, sid string) (*domain.Session, error) { return s, nil }

			req := httptest.NewRequest("GET", "/api/admin", nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: "s1"})

			w := httptest.NewRecorder()
			r := gin.New()
			r.Use(adapter.RequireAdmin())
			r.GET("/api/admin", func(c *gin.Context) {
				c.String(200, "ok")
			})

			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(200))
			Expect(w.Body.String()).To(Equal("ok"))
		})
	})

	Describe("RequireManager", func() {
		It("denies non-manager user", func() {
			u := &domain.User{UserID: "u1", Role: domain.RoleUser}
			s := &domain.Session{SessionID: "s1", User: u}
			authSvc.validateFn = func(ctx context.Context, sid string) (*domain.Session, error) {
				return s, nil
			}

			// Create request with session cookie
			req := httptest.NewRequest("GET", "/api/teams", nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: "s1"})
			w := httptest.NewRecorder()

			// Only mount RequireManager — no final handler
			r := gin.New()
			r.GET("/api/teams", adapter.RequireManager())

			// Run request
			r.ServeHTTP(w, req)

			// Assert response
			Expect(w.Code).To(Equal(403))
			Expect(w.Body.String()).To(ContainSubstring("Manager privileges required"))
		})

		It("allows manager user", func() {
			u := &domain.User{
				UserID: "u2",
				Role:   domain.RoleUser, // Role doesn't matter for IsTeamManager
				Groups: []domain.UserGroup{
					{GroupName: "team-managers"}, // manager group
				},
			}
			s := &domain.Session{SessionID: "s2", User: u}
			authSvc.validateFn = func(ctx context.Context, sid string) (*domain.Session, error) { return s, nil }

			req := httptest.NewRequest("GET", "/api/teams", nil)
			req.AddCookie(&http.Cookie{Name: "session_id", Value: "s2"})
			w := httptest.NewRecorder()

			r := gin.New()
			r.GET("/api/teams", adapter.RequireManager(), func(c *gin.Context) {
				c.String(200, "ok")
			})

			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(200))
			Expect(w.Body.String()).To(Equal("ok"))
		})
	})

	// -----------------Nimish -----------------

	Describe("CanAccessTeamProjects", func() {
		var c *gin.Context

		BeforeEach(func() {
			w := httptest.NewRecorder()
			c, _ = gin.CreateTestContext(w)
		})

		It("returns false if no user in context", func() {
			Expect(interfaces.CanAccessTeamProjects(c, "team1")).To(BeFalse())
		})

		It("returns true if user is admin", func() {
			u := &domain.User{
				UserID: "admin1",
				Role:   domain.RoleAdmin,
			}
			c.Set("user", u)
			Expect(interfaces.CanAccessTeamProjects(c, "team1")).To(BeTrue())
		})

		It("returns true if user is in team-managers group", func() {
			u := &domain.User{
				UserID: "u1",
				Role:   domain.RoleUser,
				Groups: []domain.UserGroup{
					{GroupName: "team1-managers"},
				},
			}
			c.Set("user", u)
			Expect(interfaces.CanAccessTeamProjects(c, "team1")).To(BeTrue())
		})

		It("returns true if user is in team-users group", func() {
			u := &domain.User{
				UserID: "u2",
				Role:   domain.RoleUser,
				Groups: []domain.UserGroup{
					{GroupName: "team1-users"},
				},
			}
			c.Set("user", u)
			Expect(interfaces.CanAccessTeamProjects(c, "team1")).To(BeTrue())
		})

		It("returns false if user is not in any relevant groups", func() {
			u := &domain.User{
				UserID: "u3",
				Role:   domain.RoleUser,
				Groups: []domain.UserGroup{
					{GroupName: "other-team-managers"},
				},
			}
			c.Set("user", u)
			Expect(interfaces.CanAccessTeamProjects(c, "team1")).To(BeFalse())
		})

		It("handles group names with leading slash", func() {
			u := &domain.User{
				UserID: "u4",
				Role:   domain.RoleUser,
				Groups: []domain.UserGroup{
					{GroupName: "/team1-users"},
				},
			}
			c.Set("user", u)
			Expect(interfaces.CanAccessTeamProjects(c, "team1")).To(BeTrue())
		})
	})
})

var _ = Describe("Auth Middleware Helper Functions", func() {
	var (
		c *gin.Context
		w *httptest.ResponseRecorder
		_ *gin.Engine
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		w = httptest.NewRecorder()
		_ = gin.New()
		c, _ = gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	})

	Describe("GetAuthSession", func() {
		Context("when session is not set in context", func() {
			It("should return nil and false", func() {
				session, exists := interfaces.GetAuthSession(c)
				Expect(session).To(BeNil())
				Expect(exists).To(BeFalse())
			})
		})

		Context("when session is set with wrong type", func() {
			It("should return nil and false", func() {
				c.Set("session", "invalid-session-type")

				session, exists := interfaces.GetAuthSession(c)
				Expect(session).To(BeNil())
				Expect(exists).To(BeFalse())
			})
		})

		Context("when session is set with correct type", func() {
			It("should return session and true", func() {
				mockSession := &domain.Session{
					SessionID: "test-session-id",
					UserID:    "test-user-id",
					IDToken:   "test-id-token",
				}
				c.Set("session", mockSession)

				session, exists := interfaces.GetAuthSession(c)
				Expect(session).To(Equal(mockSession))
				Expect(exists).To(BeTrue())
				Expect(session.SessionID).To(Equal("test-session-id"))
				Expect(session.UserID).To(Equal("test-user-id"))
				Expect(session.IDToken).To(Equal("test-id-token"))
			})
		})
	})

	Describe("IsTeamManager", func() {
		Context("when user is not set in context", func() {
			It("should return false", func() {
				result := interfaces.IsTeamManager(c)
				Expect(result).To(BeFalse())
			})
		})

		Context("when user is set with wrong type", func() {
			It("should return false", func() {
				c.Set("user", "invalid-user-type")

				result := interfaces.IsTeamManager(c)
				Expect(result).To(BeFalse())
			})
		})

		Context("when user exists but is not a team manager", func() {
			It("should return false", func() {
				mockUser := &domain.User{
					UserID: "test-user",
					Email:  "test@example.com",
					Role:   domain.RoleUser, // Regular user role
					Groups: []domain.UserGroup{
						{GroupName: "regular-group"},
					},
				}
				c.Set("user", mockUser)

				result := interfaces.IsTeamManager(c)
				Expect(result).To(BeFalse())
			})
		})

		Context("when user exists and is a team manager", func() {
			It("should return true", func() {
				mockUser := &domain.User{
					UserID: "test-manager",
					Email:  "manager@example.com",
					Role:   domain.RoleUser,
					Groups: []domain.UserGroup{
						{GroupName: "team1-managers"},
						{GroupName: "regular-group"},
					},
				}
				c.Set("user", mockUser)

				result := interfaces.IsTeamManager(c)
				Expect(result).To(BeTrue())
			})
		})

		Context("when user is admin", func() {
			It("should return true", func() {
				mockUser := &domain.User{
					UserID: "test-admin",
					Email:  "admin@example.com",
					Role:   domain.RoleAdmin,
					Groups: []domain.UserGroup{},
				}
				c.Set("user", mockUser)

				result := interfaces.IsTeamManager(c)
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("IsAdmin", func() {
		Context("when user is not set in context", func() {
			It("should return false", func() {
				result := interfaces.IsAdmin(c)
				Expect(result).To(BeFalse())
			})
		})

		Context("when user is set with wrong type", func() {
			It("should return false", func() {
				c.Set("user", "invalid-user-type")

				result := interfaces.IsAdmin(c)
				Expect(result).To(BeFalse())
			})
		})

		Context("when user exists but is not an admin", func() {
			It("should return false", func() {
				mockUser := &domain.User{
					UserID: "test-user",
					Email:  "user@example.com",
					Role:   domain.RoleUser, // Regular user role
					Groups: []domain.UserGroup{
						{GroupName: "regular-group"},
					},
				}
				c.Set("user", mockUser)

				result := interfaces.IsAdmin(c)
				Expect(result).To(BeFalse())
			})
		})

		Context("when user exists and is an admin", func() {
			It("should return true", func() {
				mockUser := &domain.User{
					UserID: "test-admin",
					Email:  "admin@example.com",
					Role:   domain.RoleAdmin, // Admin role
					Groups: []domain.UserGroup{
						{GroupName: "admin-group"},
					},
				}
				c.Set("user", mockUser)

				result := interfaces.IsAdmin(c)
				Expect(result).To(BeTrue())
			})
		})

		Context("when user exists with manager role but not admin", func() {
			It("should return false", func() {
				mockUser := &domain.User{
					UserID: "test-manager",
					Email:  "manager@example.com",
					Role:   domain.RoleUser, // User role with manager groups
					Groups: []domain.UserGroup{
						{GroupName: "team1-managers"},
						{GroupName: "team2-managers"},
					},
				}
				c.Set("user", mockUser)

				result := interfaces.IsAdmin(c)
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("IsManagerForTeam", func() {
		teamName := "engineering"

		Context("when user is not set in context", func() {
			It("should return false", func() {
				result := interfaces.IsManagerForTeam(c, teamName)
				Expect(result).To(BeFalse())
			})
		})

		Context("when user is set with wrong type", func() {
			It("should return false", func() {
				c.Set("user", "invalid-user-type")

				result := interfaces.IsManagerForTeam(c, teamName)
				Expect(result).To(BeFalse())
			})
		})

		Context("when user exists but is not manager for the specific team", func() {
			It("should return false", func() {
				mockUser := &domain.User{
					UserID: "test-user",
					Email:  "user@example.com",
					Role:   domain.RoleUser,
					Groups: []domain.UserGroup{
						{GroupName: "other-team-managers"},
						{GroupName: "engineering-users"}, // User of the team but not manager
					},
				}
				c.Set("user", mockUser)

				result := interfaces.IsManagerForTeam(c, teamName)
				Expect(result).To(BeFalse())
			})
		})

		Context("when user exists and is manager for the specific team", func() {
			It("should return true", func() {
				mockUser := &domain.User{
					UserID: "test-manager",
					Email:  "manager@example.com",
					Role:   domain.RoleUser,
					Groups: []domain.UserGroup{
						{GroupName: "engineering-managers"},
						{GroupName: "other-group"},
					},
				}
				c.Set("user", mockUser)

				result := interfaces.IsManagerForTeam(c, teamName)
				Expect(result).To(BeTrue())
			})
		})

		Context("when user is admin", func() {
			It("should return true regardless of team", func() {
				mockUser := &domain.User{
					UserID: "test-admin",
					Email:  "admin@example.com",
					Role:   domain.RoleAdmin,
					Groups: []domain.UserGroup{}, // Admin doesn't need specific groups
				}
				c.Set("user", mockUser)

				result := interfaces.IsManagerForTeam(c, teamName)
				Expect(result).To(BeTrue())
			})
		})

		Context("when user has manager group with leading slash", func() {
			It("should return false (domain method doesn't handle leading slash)", func() {
				mockUser := &domain.User{
					UserID: "test-manager",
					Email:  "manager@example.com",
					Role:   domain.RoleUser,
					Groups: []domain.UserGroup{
						{GroupName: "/engineering-managers"}, // Group with leading slash
					},
				}
				c.Set("user", mockUser)

				result := interfaces.IsManagerForTeam(c, teamName)
				Expect(result).To(BeFalse())
			})
		})

		Context("when checking empty team name", func() {
			It("should return false for non-admin users", func() {
				mockUser := &domain.User{
					UserID: "test-user",
					Email:  "user@example.com",
					Role:   domain.RoleUser,
					Groups: []domain.UserGroup{
						{GroupName: "some-managers"},
					},
				}
				c.Set("user", mockUser)

				result := interfaces.IsManagerForTeam(c, "")
				Expect(result).To(BeFalse())
			})

			It("should return true for admin users", func() {
				mockUser := &domain.User{
					UserID: "test-admin",
					Email:  "admin@example.com",
					Role:   domain.RoleAdmin,
					Groups: []domain.UserGroup{},
				}
				c.Set("user", mockUser)

				result := interfaces.IsManagerForTeam(c, "")
				Expect(result).To(BeTrue())
			})
		})
	})
})
