package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

var _ = Describe("AuthHandler", func() {
	var (
		handler *AuthHandler
		logger  *logging.Logger
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		var err error
		logger, err = logging.NewLogger(&config.LoggingConfig{Level: "info", Format: "json"})
		Expect(err).NotTo(HaveOccurred())
		handler = NewAuthHandler(nil, logger)
	})

	Describe("getCurrentUser", func() {
		It("returns 200 with user data when context has user info", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_id", "user-123")
			c.Set("user_name", "John Doe")
			c.Set("user_email", "john@example.com")
			c.Set("role", "admin")
			c.Set("team_id", "team-456")
			c.Set("team_name", "Engineering")

			handler.getCurrentUser(c)

			Expect(w.Code).To(Equal(http.StatusOK))
			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["id"]).To(Equal("user-123"))
			Expect(body["name"]).To(Equal("John Doe"))
			Expect(body["email"]).To(Equal("john@example.com"))
			Expect(body["role"]).To(Equal("admin"))

			team, ok := body["team"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(team["id"]).To(Equal("team-456"))
			Expect(team["name"]).To(Equal("Engineering"))
		})
	})

	Describe("isUserAuthenticated", func() {
		It("returns true when user_id is set in context", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_id", "user-123")

			Expect(handler.isUserAuthenticated(c)).To(BeTrue())
		})

		It("returns false when user_id is not set", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			Expect(handler.isUserAuthenticated(c)).To(BeFalse())
		})

		It("returns false when user_id is empty string", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_id", "")

			Expect(handler.isUserAuthenticated(c)).To(BeFalse())
		})

		It("returns false when user_id is nil", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_id", nil)

			Expect(handler.isUserAuthenticated(c)).To(BeFalse())
		})
	})

	Describe("showLoginPage", func() {
		It("returns 200 with HTML content when not authenticated", func() {
			router := gin.New()
			router.GET("/auth/login", handler.showLoginPage)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("text/html"))
			Expect(w.Body.String()).To(ContainSubstring("Fern Platform"))
		})

		It("redirects to / when user is already authenticated", func() {
			router := gin.New()
			router.GET("/auth/login", func(c *gin.Context) {
				c.Set("user_id", "user-123")
				handler.showLoginPage(c)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusFound))
			Expect(w.Header().Get("Location")).To(Equal("/"))
		})

		It("stores return URL in cookie", func() {
			router := gin.New()
			router.GET("/auth/login", handler.showLoginPage)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/auth/login?return=/dashboard", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			cookies := w.Result().Cookies()
			found := false
			for _, cookie := range cookies {
				if cookie.Name == "auth_return_url" {
					decoded, err := url.QueryUnescape(cookie.Value)
					Expect(err).NotTo(HaveOccurred())
					Expect(decoded).To(Equal("/dashboard"))
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})

		It("defaults return URL to / when not specified", func() {
			router := gin.New()
			router.GET("/auth/login", handler.showLoginPage)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			cookies := w.Result().Cookies()
			found := false
			for _, cookie := range cookies {
				if cookie.Name == "auth_return_url" {
					decoded, err := url.QueryUnescape(cookie.Value)
					Expect(err).NotTo(HaveOccurred())
					Expect(decoded).To(Equal("/"))
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Describe("generateOAuthURL", func() {
		It("generates URL with http scheme by default", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/auth/login", nil)
			c.Request.Host = "localhost:8080"

			url := handler.generateOAuthURL(c)
			Expect(url).To(Equal("http://localhost:8080/auth/start"))
		})

		It("generates URL with https when X-Forwarded-Proto is https", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/auth/login", nil)
			c.Request.Host = "example.com"
			c.Request.Header.Set("X-Forwarded-Proto", "https")

			url := handler.generateOAuthURL(c)
			Expect(url).To(Equal("https://example.com/auth/start"))
		})

		It("uses X-Forwarded-Host when present", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/auth/login", nil)
			c.Request.Host = "internal:8080"
			c.Request.Header.Set("X-Forwarded-Host", "public.example.com")

			url := handler.generateOAuthURL(c)
			Expect(url).To(Equal("http://public.example.com/auth/start"))
		})
	})

	Describe("getUserPreferences", func() {
		It("returns 200 with default preferences", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_id", "user-123")

			handler.getUserPreferences(c)

			Expect(w.Code).To(Equal(http.StatusOK))
			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["user_id"]).To(Equal("user-123"))
			Expect(body["theme"]).To(Equal("light"))
			Expect(body).To(HaveKey("notifications"))
			Expect(body).To(HaveKey("dashboard"))
		})
	})

	Describe("updateUserPreferences", func() {
		It("returns 200 with echoed preferences", func() {
			router := gin.New()
			router.PUT("/preferences", handler.updateUserPreferences)

			payload := map[string]interface{}{"theme": "dark"}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/preferences", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["theme"]).To(Equal("dark"))
		})

		It("returns 400 for invalid JSON", func() {
			router := gin.New()
			router.PUT("/preferences", handler.updateUserPreferences)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/preferences", bytes.NewBufferString("{bad"))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("getUserProjects", func() {
		It("returns 200 with empty project list", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_id", "user-123")

			handler.getUserProjects(c)

			Expect(w.Code).To(Equal(http.StatusOK))
			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			items, ok := body["items"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(items).To(BeEmpty())
			Expect(body["total"]).To(BeNumerically("==", 0))
		})
	})

	Describe("Admin user management endpoints", func() {
		Describe("listUsers", func() {
			It("returns 200 with empty user list", func() {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)

				handler.listUsers(c)

				Expect(w.Code).To(Equal(http.StatusOK))
				var body map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
				Expect(body["total"]).To(BeNumerically("==", 0))
			})
		})

		Describe("getUser", func() {
			It("returns 200 with user ID", func() {
				router := gin.New()
				router.GET("/users/:userId", handler.getUser)

				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/users/user-789", nil)
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				var body map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
				Expect(body["id"]).To(Equal("user-789"))
			})
		})

		Describe("updateUserRole", func() {
			It("returns 200 with success message", func() {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)

				handler.updateUserRole(c)

				Expect(w.Code).To(Equal(http.StatusOK))
				var body map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
				Expect(body["message"]).To(Equal("Role updated successfully"))
			})
		})

		Describe("suspendUser", func() {
			It("returns 200 with success message", func() {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)

				handler.suspendUser(c)

				Expect(w.Code).To(Equal(http.StatusOK))
				var body map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
				Expect(body["message"]).To(Equal("User suspended successfully"))
			})
		})

		Describe("activateUser", func() {
			It("returns 200 with success message", func() {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)

				handler.activateUser(c)

				Expect(w.Code).To(Equal(http.StatusOK))
				var body map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
				Expect(body["message"]).To(Equal("User activated successfully"))
			})
		})

		Describe("deleteUser", func() {
			It("returns 200 with success message", func() {
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)

				handler.deleteUser(c)

				Expect(w.Code).To(Equal(http.StatusOK))
				var body map[string]interface{}
				Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
				Expect(body["message"]).To(Equal("User deleted successfully"))
			})
		})
	})
})
