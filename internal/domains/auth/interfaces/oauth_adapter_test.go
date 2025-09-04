package interfaces

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/auth/application"
	_ "github.com/guidewire-oss/fern-platform/internal/domains/auth/application"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("OAuthAdapter", Label("auth"), func() {
	var (
		authConfig *config.AuthConfig
		logger     *logging.Logger
		adapter    *OAuthAdapter
	)

	BeforeEach(func() {
		authConfig = &config.AuthConfig{
			OAuth: config.OAuthConfig{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				RedirectURL:  "http://localhost/auth/callback",
				AuthURL:      "http://authserver/authorize",
				TokenURL:     "http://authserver/token",
				UserInfoURL:  "http://authserver/userinfo",
				Scopes:       []string{"openid", "profile", "email"},
				UserIDField:  "sub",
				EmailField:   "email",
				NameField:    "name",
				GroupsField:  "groups",
				RolesField:   "roles",
				Enabled:      true,
			},
		}
		logger, _ = logging.NewLogger(&config.LoggingConfig{
			Level: "debug",
		})
		adapter = NewOAuthAdapter(authConfig, logger)
	})

	Describe("GenerateState", func() {
		It("generates a random state string", func() {
			state, err := adapter.GenerateState()
			Expect(err).To(BeNil())
			Expect(state).NotTo(BeEmpty())
		})

		It("generates different states on subsequent calls", func() {
			state1, err1 := adapter.GenerateState()
			state2, err2 := adapter.GenerateState()
			Expect(err1).To(BeNil())
			Expect(err2).To(BeNil())
			Expect(state1).NotTo(Equal(state2))
		})
	})

	Describe("BuildAuthURL", func() {
		It("builds URL without PKCE", func() {
			url := adapter.BuildAuthURL("xyz")
			Expect(url).To(ContainSubstring("response_type=code"))
			Expect(url).To(ContainSubstring("state=xyz"))
			Expect(url).To(ContainSubstring("client_id=client-id"))
			Expect(url).To(ContainSubstring("redirect_uri=http%3A%2F%2Flocalhost%2Fauth%2Fcallback"))
			Expect(url).To(ContainSubstring("scope=openid+profile+email"))
			Expect(url).NotTo(ContainSubstring("code_challenge"))
		})

		It("builds URL with PKCE", func() {
			url := adapter.BuildAuthURL("xyz", "challenge123")
			Expect(url).To(ContainSubstring("code_challenge=challenge123"))
			Expect(url).To(ContainSubstring("code_challenge_method=S256"))
		})

		It("builds URL with empty scopes", func() {
			authConfig.OAuth.Scopes = []string{}
			adapter = NewOAuthAdapter(authConfig, logger)
			url := adapter.BuildAuthURL("state123")
			Expect(url).To(ContainSubstring("scope="))
			Expect(url).To(ContainSubstring("state=state123"))
		})

		It("builds URL with single scope", func() {
			authConfig.OAuth.Scopes = []string{"openid"}
			adapter = NewOAuthAdapter(authConfig, logger)
			url := adapter.BuildAuthURL("state456")
			Expect(url).To(ContainSubstring("scope=openid"))
		})

		It("ignores empty code challenge", func() {
			url := adapter.BuildAuthURL("xyz", "")
			Expect(url).NotTo(ContainSubstring("code_challenge"))
			Expect(url).NotTo(ContainSubstring("code_challenge_method"))
		})
	})

	Describe("ExchangeCodeForToken", func() {
		var server *httptest.Server

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		It("successfully exchanges code for token", func() {
			tokenResp := map[string]interface{}{
				"access_token":  "access123",
				"refresh_token": "refresh123",
				"id_token":      "id123",
				"expires_in":    3600,
			}
			body, _ := json.Marshal(tokenResp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write(body)
			}))
			authConfig.OAuth.TokenURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			token, err := adapter.ExchangeCodeForToken("code123", "")
			Expect(err).To(BeNil())
			Expect(token.AccessToken).To(Equal("access123"))
			Expect(token.RefreshToken).To(Equal("refresh123"))
			Expect(token.IDToken).To(Equal("id123"))
		})

		It("successfully exchanges code with PKCE verifier", func() {
			var receivedVerifier string
			tokenResp := map[string]interface{}{
				"access_token": "access-pkce",
				"expires_in":   7200,
			}
			body, _ := json.Marshal(tokenResp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()
				receivedVerifier = r.Form.Get("code_verifier")
				Expect(r.Form.Get("grant_type")).To(Equal("authorization_code"))
				Expect(r.Form.Get("code")).To(Equal("auth-code"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))
				Expect(r.Header.Get("Accept")).To(Equal("application/json"))
				w.WriteHeader(http.StatusOK)
				w.Write(body)
			}))
			authConfig.OAuth.TokenURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			token, err := adapter.ExchangeCodeForToken("auth-code", "verifier123")
			Expect(err).To(BeNil())
			Expect(receivedVerifier).To(Equal("verifier123"))
			Expect(token.AccessToken).To(Equal("access-pkce"))
			Expect(token.ExpiresIn).To(Equal(7200))
		})

		It("handles token response without optional fields", func() {
			tokenResp := map[string]interface{}{
				"access_token": "minimal-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}
			body, _ := json.Marshal(tokenResp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write(body)
			}))
			authConfig.OAuth.TokenURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			token, err := adapter.ExchangeCodeForToken("code", "")
			Expect(err).To(BeNil())
			Expect(token.AccessToken).To(Equal("minimal-token"))
			Expect(token.RefreshToken).To(BeEmpty())
			Expect(token.IDToken).To(BeEmpty())
		})

		It("returns error for non-200 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "bad request", http.StatusBadRequest)
			}))
			authConfig.OAuth.TokenURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			token, err := adapter.ExchangeCodeForToken("code", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("token exchange failed"))
			Expect(token).To(BeNil())
		})

		It("returns error for 401 unauthorized", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
			}))
			authConfig.OAuth.TokenURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			token, err := adapter.ExchangeCodeForToken("code", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("token exchange failed"))
			Expect(token).To(BeNil())
		})

		It("returns error for invalid JSON", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("not-json"))
			}))
			authConfig.OAuth.TokenURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			token, err := adapter.ExchangeCodeForToken("code", "")
			Expect(err).To(HaveOccurred())
			Expect(token).To(BeNil())
		})

		It("handles network timeout", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(11 * time.Second) // Longer than client timeout
			}))
			authConfig.OAuth.TokenURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			token, err := adapter.ExchangeCodeForToken("code", "")
			Expect(err).To(HaveOccurred())
			Expect(token).To(BeNil())
		})

		It("handles invalid URL", func() {
			authConfig.OAuth.TokenURL = "http://[invalid-url"
			adapter = NewOAuthAdapter(authConfig, logger)

			token, err := adapter.ExchangeCodeForToken("code", "")
			Expect(err).To(HaveOccurred())
			Expect(token).To(BeNil())
		})
	})

	Describe("GetUserInfo", func() {
		var server *httptest.Server

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		It("returns parsed user info with all fields", func() {
			resp := map[string]interface{}{
				"sub":            "123",
				"email":          "user@example.com",
				"name":           "User One",
				"picture":        "http://pic",
				"given_name":     "User",
				"family_name":    "One",
				"email_verified": true,
				"groups":         []interface{}{"dev", "ops"},
				"roles":          []interface{}{"admin"},
			}
			body, _ := json.Marshal(resp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Header.Get("Authorization")).To(Equal("Bearer test-token"))
				Expect(r.Header.Get("Accept")).To(Equal("application/json"))
				w.Write(body)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			authConfig.OAuth.AdminUsers = []string{"user@example.com"}
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("test-token")
			Expect(err).To(BeNil())
			Expect(info.Email).To(Equal("user@example.com"))
			Expect(info.Sub).To(Equal("123"))
			Expect(info.Name).To(Equal("User One"))
			Expect(info.Picture).To(Equal("http://pic"))
			Expect(info.FirstName).To(Equal("User"))
			Expect(info.LastName).To(Equal("One"))
			Expect(info.EmailVerified).To(BeTrue())
			Expect(info.Groups).To(ContainElement("dev"))
			Expect(info.Groups).To(ContainElement("ops"))
			Expect(info.Groups).To(ContainElement("admin"))
			Expect(info.Roles).To(ContainElement("admin"))
		})

		It("handles missing optional fields", func() {
			resp := map[string]interface{}{
				"sub":   "456",
				"email": "minimal@example.com",
			}
			body, _ := json.Marshal(resp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write(body)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(BeNil())
			Expect(info.Sub).To(Equal("456"))
			Expect(info.Email).To(Equal("minimal@example.com"))
			Expect(info.Name).To(BeEmpty())
			Expect(info.Picture).To(BeEmpty())
			Expect(info.Groups).To(BeEmpty())
			Expect(info.Roles).To(BeEmpty())
		})

		It("handles fields with wrong types", func() {
			resp := map[string]interface{}{
				"sub":            123,  // wrong type (number instead of string)
				"email":          true, // wrong type (bool instead of string)
				"name":           "Valid Name",
				"email_verified": "yes",          // wrong type (string instead of bool)
				"groups":         "not-an-array", // wrong type (string instead of array)
				"roles":          123,            // wrong type (number instead of array)
			}
			body, _ := json.Marshal(resp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write(body)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(BeNil())
			Expect(info.Sub).To(BeEmpty())   // Wrong type, should be empty
			Expect(info.Email).To(BeEmpty()) // Wrong type, should be empty
			Expect(info.Name).To(Equal("Valid Name"))
			Expect(info.EmailVerified).To(BeFalse()) // Wrong type, defaults to false
			Expect(info.Groups).To(BeEmpty())        // Wrong type, should be empty
			Expect(info.Roles).To(BeEmpty())         // Wrong type, should be empty
		})

		It("handles groups array with mixed types", func() {
			resp := map[string]interface{}{
				"sub":    "789",
				"groups": []interface{}{"valid-group", 123, true, "another-group"},
				"roles":  []interface{}{456, "valid-role", false},
			}
			body, _ := json.Marshal(resp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write(body)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(BeNil())
			Expect(info.Groups).To(HaveLen(2))
			Expect(info.Groups).To(ContainElement("valid-group"))
			Expect(info.Groups).To(ContainElement("another-group"))
			Expect(info.Roles).To(HaveLen(1))
			Expect(info.Roles).To(ContainElement("valid-role"))
		})

		It("applies admin override based on sub field", func() {
			resp := map[string]interface{}{
				"sub":    "admin-sub-123",
				"email":  "other@example.com",
				"groups": []interface{}{"users"},
			}
			body, _ := json.Marshal(resp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write(body)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			authConfig.OAuth.AdminUsers = []string{"admin-sub-123"} // Match by sub
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(BeNil())
			Expect(info.Groups).To(ContainElement("admin"))
		})

		It("does not duplicate admin group if already present", func() {
			resp := map[string]interface{}{
				"sub":    "sub123",
				"email":  "admin@example.com",
				"groups": []interface{}{"users", "admin"},
			}
			body, _ := json.Marshal(resp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write(body)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			authConfig.OAuth.AdminUsers = []string{"admin@example.com"}
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(BeNil())
			adminCount := 0
			for _, g := range info.Groups {
				if g == "admin" {
					adminCount++
				}
			}
			Expect(adminCount).To(Equal(1))
		})

		It("recognizes /admin group as admin", func() {
			resp := map[string]interface{}{
				"sub":    "sub456",
				"email":  "user@example.com",
				"groups": []interface{}{"/admin", "users"},
			}
			body, _ := json.Marshal(resp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write(body)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			authConfig.OAuth.AdminUsers = []string{"user@example.com"}
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(BeNil())
			// Should not add another admin group since /admin is already present
			adminCount := 0
			for _, g := range info.Groups {
				if g == "admin" || g == "/admin" {
					adminCount++
				}
			}
			Expect(adminCount).To(Equal(1))
		})

		It("applies admin override based on AdminGroups", func() {
			resp := map[string]interface{}{
				"sub":    "sub123",
				"email":  "x@example.com",
				"groups": []interface{}{"managers"},
			}
			body, _ := json.Marshal(resp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write(body)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			authConfig.OAuth.AdminGroups = []string{"managers"}
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("access")
			Expect(err).To(BeNil())
			Expect(info.Groups).To(ContainElement("admin"))
		})

		It("applies admin override for multiple admin groups", func() {
			resp := map[string]interface{}{
				"sub":    "sub789",
				"email":  "user@example.com",
				"groups": []interface{}{"developers", "team-leads"},
			}
			body, _ := json.Marshal(resp)
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write(body)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			authConfig.OAuth.AdminGroups = []string{"managers", "team-leads", "executives"}
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(BeNil())
			Expect(info.Groups).To(ContainElement("admin"))
		})

		It("returns error on non-200 response", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "fail", http.StatusUnauthorized)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("userinfo request failed with status 401"))
			Expect(info).To(BeNil())
		})

		It("returns error for 403 forbidden", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "forbidden", http.StatusForbidden)
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("userinfo request failed with status 403"))
			Expect(info).To(BeNil())
		})

		It("returns error on invalid JSON", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("bad-json"))
			}))
			authConfig.OAuth.UserInfoURL = server.URL
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(HaveOccurred())
			Expect(info).To(BeNil())
		})

		It("handles network error", func() {
			authConfig.OAuth.UserInfoURL = "http://non-existent-server-12345.invalid"
			adapter = NewOAuthAdapter(authConfig, logger)

			info, err := adapter.GetUserInfo("token")
			Expect(err).To(HaveOccurred())
			Expect(info).To(BeNil())
		})
	})

	Describe("BuildProviderLogoutURL", func() {
		It("returns local login if disabled", func() {
			authConfig.OAuth.Enabled = false
			adapter = NewOAuthAdapter(authConfig, logger)
			Expect(adapter.BuildProviderLogoutURL("id")).To(Equal("/auth/login"))
		})

		It("returns local login if no ID token", func() {
			Expect(adapter.BuildProviderLogoutURL("")).To(Equal("/auth/login"))
		})

		It("uses configured LogoutURL", func() {
			authConfig.OAuth.LogoutURL = "http://auth/logout"
			url := adapter.BuildProviderLogoutURL("id123")
			Expect(url).To(ContainSubstring("id_token_hint=id123"))
			Expect(url).To(ContainSubstring("post_logout_redirect_uri="))
			Expect(url).To(ContainSubstring("%2Fauth%2Flogin"))
		})

		It("handles LogoutURL with existing query params", func() {
			authConfig.OAuth.LogoutURL = "http://auth/logout?param=value"
			url := adapter.BuildProviderLogoutURL("id456")
			Expect(url).To(ContainSubstring("param=value"))
			Expect(url).To(ContainSubstring("&id_token_hint=id456"))
			Expect(url).NotTo(ContainSubstring("?id_token_hint"))
		})

		It("properly URL-encodes the ID token", func() {
			authConfig.OAuth.LogoutURL = "http://auth/logout"
			url := adapter.BuildProviderLogoutURL("id/with+special=chars")
			Expect(url).To(ContainSubstring("id_token_hint=id%2Fwith%2Bspecial%3Dchars"))
		})

		It("uses IssuerURL fallback", func() {
			authConfig.OAuth.LogoutURL = ""
			authConfig.OAuth.IssuerURL = "http://issuer"
			url := adapter.BuildProviderLogoutURL("id123")
			Expect(url).To(ContainSubstring("protocol/openid-connect/logout"))
			Expect(url).To(ContainSubstring("id_token_hint=id123"))
		})

		It("handles IssuerURL with trailing slash", func() {
			authConfig.OAuth.LogoutURL = ""
			authConfig.OAuth.IssuerURL = "http://issuer/"
			url := adapter.BuildProviderLogoutURL("id789")
			Expect(url).To(Equal("http://issuer/protocol/openid-connect/logout?id_token_hint=id789&post_logout_redirect_uri=http%3A%2F%2Flocalhost%2Fauth%2Flogin"))
			Expect(url).NotTo(ContainSubstring("//protocol"))
		})

		It("handles empty RedirectURL", func() {
			authConfig.OAuth.LogoutURL = "http://auth/logout"
			authConfig.OAuth.RedirectURL = ""
			url := adapter.BuildProviderLogoutURL("id123")
			Expect(url).To(ContainSubstring("id_token_hint=id123"))
			Expect(url).NotTo(ContainSubstring("post_logout_redirect_uri"))
		})

		It("falls back to /auth/login when no logout URL configured", func() {
			authConfig.OAuth.LogoutURL = ""
			authConfig.OAuth.IssuerURL = ""
			Expect(adapter.BuildProviderLogoutURL("id123")).To(Equal("/auth/login"))
		})
	})

	Describe("applyAdminOverrides", func() {
		var userInfo *application.UserInfo

		BeforeEach(func() {
			userInfo = &application.UserInfo{
				Sub:    "user123",
				Email:  "test@example.com",
				Groups: []string{"users"},
			}
		})

		It("adds admin group for AdminUsers email match", func() {
			authConfig.OAuth.AdminUsers = []string{"test@example.com"}
			adapter = NewOAuthAdapter(authConfig, logger)
			adapter.applyAdminOverrides(userInfo)
			Expect(userInfo.Groups).To(ContainElement("admin"))
		})

		It("adds admin group for AdminUsers sub match", func() {
			authConfig.OAuth.AdminUsers = []string{"user123"}
			adapter = NewOAuthAdapter(authConfig, logger)
			adapter.applyAdminOverrides(userInfo)
			Expect(userInfo.Groups).To(ContainElement("admin"))
		})

		It("does not add admin if not in AdminUsers", func() {
			authConfig.OAuth.AdminUsers = []string{"other@example.com"}
			adapter = NewOAuthAdapter(authConfig, logger)
			originalGroups := len(userInfo.Groups)
			adapter.applyAdminOverrides(userInfo)
			Expect(userInfo.Groups).To(HaveLen(originalGroups))
			Expect(userInfo.Groups).NotTo(ContainElement("admin"))
		})

		It("adds admin group for AdminGroups match", func() {
			userInfo.Groups = append(userInfo.Groups, "managers")
			authConfig.OAuth.AdminGroups = []string{"managers"}
			adapter = NewOAuthAdapter(authConfig, logger)
			adapter.applyAdminOverrides(userInfo)
			Expect(userInfo.Groups).To(ContainElement("admin"))
		})

		It("prioritizes AdminUsers over AdminGroups", func() {
			userInfo.Groups = append(userInfo.Groups, "blocked-group")
			authConfig.OAuth.AdminUsers = []string{"test@example.com"}
			authConfig.OAuth.AdminGroups = []string{"blocked-group"}
			adapter = NewOAuthAdapter(authConfig, logger)
			adapter.applyAdminOverrides(userInfo)
			Expect(userInfo.Groups).To(ContainElement("admin"))
		})

		It("handles empty admin configurations", func() {
			authConfig.OAuth.AdminUsers = []string{}
			authConfig.OAuth.AdminGroups = []string{}
			adapter = NewOAuthAdapter(authConfig, logger)
			originalGroups := len(userInfo.Groups)
			adapter.applyAdminOverrides(userInfo)
			Expect(userInfo.Groups).To(HaveLen(originalGroups))
			Expect(userInfo.Groups).NotTo(ContainElement("admin"))
		})

		It("handles nil groups", func() {
			userInfo.Groups = nil
			authConfig.OAuth.AdminUsers = []string{"test@example.com"}
			adapter = NewOAuthAdapter(authConfig, logger)
			adapter.applyAdminOverrides(userInfo)
			Expect(userInfo.Groups).To(HaveLen(1))
			Expect(userInfo.Groups).To(ContainElement("admin"))
		})
	})

	Describe("NewOAuthAdapter", func() {
		It("creates adapter with correct configuration", func() {
			newAdapter := NewOAuthAdapter(authConfig, logger)
			Expect(newAdapter).NotTo(BeNil())
			Expect(newAdapter.config).To(Equal(authConfig))
			Expect(newAdapter.logger).To(Equal(logger))
			Expect(newAdapter.client).NotTo(BeNil())
			Expect(newAdapter.client.Timeout).To(Equal(10 * time.Second))
		})
	})

	Describe("Edge Cases", func() {
		It("handles special characters in scopes", func() {
			authConfig.OAuth.Scopes = []string{"openid", "profile email", "custom:scope"}
			adapter = NewOAuthAdapter(authConfig, logger)
			url := adapter.BuildAuthURL("state")
			Expect(url).To(ContainSubstring("scope=openid+profile+email+custom%3Ascope"))
		})

		It("handles malformed UserInfoURL", func() {
			authConfig.OAuth.UserInfoURL = "://invalid-url"
			adapter = NewOAuthAdapter(authConfig, logger)
			info, err := adapter.GetUserInfo("token")
			Expect(err).To(HaveOccurred())
			Expect(info).To(BeNil())
		})

		It("handles very long state parameter", func() {
			longState := strings.Repeat("a", 1000)
			url := adapter.BuildAuthURL(longState)
			Expect(url).To(ContainSubstring("state=" + longState))
		})
	})
})
