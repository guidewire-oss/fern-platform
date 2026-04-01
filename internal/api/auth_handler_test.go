package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// authContextMiddleware injects user values into gin context to simulate authenticated requests
func authContextMiddleware(userID, userName, userEmail, role, teamID, teamName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_name", userName)
		c.Set("user_email", userEmail)
		c.Set("role", role)
		c.Set("team_id", teamID)
		c.Set("team_name", teamName)
		c.Next()
	}
}

var _ = Describe("AuthHandler", func() {
	var (
		handler *AuthHandler
		router  *gin.Engine
		logger  *logging.Logger
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		loggingConfig := &config.LoggingConfig{Level: "info", Format: "json"}
		var err error
		logger, err = logging.NewLogger(loggingConfig)
		Expect(err).NotTo(HaveOccurred())
		// authMiddleware is nil — handler methods don't call it; only RegisterRoutes does
		handler = NewAuthHandler(nil, logger)
		router = gin.New()
	})

	Describe("showLoginPage", func() {
		BeforeEach(func() {
			router.GET("/auth/login", handler.showLoginPage)
		})

		It("should serve the login page HTML when not authenticated", func() {
			req := httptest.NewRequest("GET", "/auth/login", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("text/html"))
			Expect(w.Body.String()).To(ContainSubstring("Fern Platform"))
			Expect(w.Body.String()).To(ContainSubstring("/auth/start"))
		})

		It("should redirect to / when already authenticated", func() {
			router2 := gin.New()
			router2.Use(authContextMiddleware("user-1", "Alice", "alice@example.com", "user", "team-1", "Team A"))
			router2.GET("/auth/login", handler.showLoginPage)

			req := httptest.NewRequest("GET", "/auth/login", nil)
			w := httptest.NewRecorder()
			router2.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusFound))
			Expect(w.Header().Get("Location")).To(Equal("/"))
		})

		It("should use return query param as cookie", func() {
			req := httptest.NewRequest("GET", "/auth/login?return=/dashboard", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Set-Cookie")).To(ContainSubstring("auth_return_url"))
		})

		It("should use https scheme when X-Forwarded-Proto is https", func() {
			req := httptest.NewRequest("GET", "/auth/login", nil)
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Host = "example.com"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("https://example.com/auth/start"))
		})

		It("should use X-Forwarded-Host when present", func() {
			req := httptest.NewRequest("GET", "/auth/login", nil)
			req.Header.Set("X-Forwarded-Host", "proxy.example.com")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("proxy.example.com/auth/start"))
		})
	})

	Describe("getCurrentUser", func() {
		BeforeEach(func() {
			router.Use(authContextMiddleware("user-42", "Bob", "bob@example.com", "admin", "team-7", "Platform"))
			router.GET("/auth/user", handler.getCurrentUser)
		})

		It("should return the current user details", func() {
			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["id"]).To(Equal("user-42"))
			Expect(body["name"]).To(Equal("Bob"))
			Expect(body["email"]).To(Equal("bob@example.com"))
			Expect(body["role"]).To(Equal("admin"))

			team := body["team"].(map[string]interface{})
			Expect(team["id"]).To(Equal("team-7"))
			Expect(team["name"]).To(Equal("Platform"))
		})
	})

	Describe("getUserPreferences", func() {
		BeforeEach(func() {
			router.Use(authContextMiddleware("user-1", "Alice", "alice@example.com", "user", "team-1", "Team A"))
			router.GET("/api/v1/user/preferences", handler.getUserPreferences)
		})

		It("should return default preferences", func() {
			req := httptest.NewRequest("GET", "/api/v1/user/preferences", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["user_id"]).To(Equal("user-1"))
			Expect(body["theme"]).To(Equal("light"))

			notifications := body["notifications"].(map[string]interface{})
			Expect(notifications["email"]).To(BeTrue())

			dashboard := body["dashboard"].(map[string]interface{})
			Expect(dashboard["default_view"]).To(Equal("grid"))
		})
	})

	Describe("updateUserPreferences", func() {
		BeforeEach(func() {
			router.Use(authContextMiddleware("user-1", "Alice", "alice@example.com", "user", "team-1", "Team A"))
			router.PUT("/api/v1/user/preferences", handler.updateUserPreferences)
		})

		It("should echo back updated preferences", func() {
			prefs := map[string]interface{}{"theme": "dark", "notifications": false}
			body, _ := json.Marshal(prefs)

			req := httptest.NewRequest("PUT", "/api/v1/user/preferences", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["theme"]).To(Equal("dark"))
		})

		It("should return bad request for invalid JSON", func() {
			req := httptest.NewRequest("PUT", "/api/v1/user/preferences", bytes.NewBufferString("{invalid"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("getUserProjects", func() {
		BeforeEach(func() {
			router.Use(authContextMiddleware("user-1", "Alice", "alice@example.com", "user", "team-1", "Team A"))
			router.GET("/api/v1/user/projects", handler.getUserProjects)
		})

		It("should return empty project list", func() {
			req := httptest.NewRequest("GET", "/api/v1/user/projects", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["total"]).To(BeNumerically("==", 0))
			Expect(resp["items"]).To(BeEmpty())
		})
	})

	Describe("Admin user management", func() {
		BeforeEach(func() {
			router.Use(authContextMiddleware("admin-1", "Admin", "admin@example.com", "admin", "team-1", "Admin Team"))
			router.GET("/admin/users", handler.listUsers)
			router.GET("/admin/users/:userId", handler.getUser)
			router.PUT("/admin/users/:userId/role", handler.updateUserRole)
			router.POST("/admin/users/:userId/suspend", handler.suspendUser)
			router.POST("/admin/users/:userId/activate", handler.activateUser)
			router.DELETE("/admin/users/:userId", handler.deleteUser)
		})

		It("listUsers should return empty list", func() {
			req := httptest.NewRequest("GET", "/admin/users", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["total"]).To(BeNumerically("==", 0))
		})

		It("getUser should return user with the requested ID", func() {
			req := httptest.NewRequest("GET", "/admin/users/user-99", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["id"]).To(Equal("user-99"))
		})

		It("updateUserRole should return success message", func() {
			req := httptest.NewRequest("PUT", "/admin/users/user-1/role", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["message"]).To(ContainSubstring("Role updated"))
		})

		It("suspendUser should return success message", func() {
			req := httptest.NewRequest("POST", "/admin/users/user-1/suspend", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["message"]).To(ContainSubstring("suspended"))
		})

		It("activateUser should return success message", func() {
			req := httptest.NewRequest("POST", "/admin/users/user-1/activate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["message"]).To(ContainSubstring("activated"))
		})

		It("deleteUser should return success message", func() {
			req := httptest.NewRequest("DELETE", "/admin/users/user-1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["message"]).To(ContainSubstring("deleted"))
		})
	})

	Describe("isUserAuthenticated", func() {
		It("should return false when user_id is not in context", func() {
			router.GET("/test", func(c *gin.Context) {
				result := handler.isUserAuthenticated(c)
				c.JSON(http.StatusOK, gin.H{"authenticated": result})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["authenticated"]).To(BeFalse())
		})

		It("should return true when user_id is set in context", func() {
			router.Use(func(c *gin.Context) {
				c.Set("user_id", "user-1")
				c.Next()
			})
			router.GET("/test", func(c *gin.Context) {
				result := handler.isUserAuthenticated(c)
				c.JSON(http.StatusOK, gin.H{"authenticated": result})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["authenticated"]).To(BeTrue())
		})
	})
})
