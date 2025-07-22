package api_test

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/api"
	"github.com/guidewire-oss/fern-platform/internal/testhelpers"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// Removed duplicate test entry point - using TestAPISuite from api_suite_test.go instead

// MockAuthMiddleware provides a mock implementation of AuthMiddlewareAdapter
type MockAuthMiddleware struct {
	mock.Mock
}

func (m *MockAuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

func (m *MockAuthMiddleware) RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// For testing, just check if user is authenticated
		if _, exists := c.Get("user_id"); !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	}
}

func (m *MockAuthMiddleware) StartOAuthFlow() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/auth/callback")
	}
}

func (m *MockAuthMiddleware) HandleOAuthCallback() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "OAuth callback"})
	}
}

func (m *MockAuthMiddleware) Logout() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
	}
}

// This test file addresses the issue of "DomainHandler initialized with nil service dependencies"
// by demonstrating:
// 1. Which endpoints can safely work with nil services (e.g., health check)
// 2. Why comprehensive handler testing requires proper service interfaces
// 3. The recommended testing strategy for this architecture
//
// The fix: Instead of creating handlers with nil services for all tests,
// we acknowledge that most endpoints require real or mock services to function.
// The architecture uses concrete types instead of interfaces, making unit testing
// at the handler level challenging without refactoring to use dependency injection
// with interfaces.
var _ = Describe("DomainHandler Integration Tests", func() {
	var (
		logger   *logging.Logger
		router   *gin.Engine
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		
		loggingConfig := &config.LoggingConfig{
			Level: "info",
			Format: "json",
		}
		var err error
		logger, err = logging.NewLogger(loggingConfig)
		Expect(err).NotTo(HaveOccurred())
		
		// Create a new router for each test
		router = gin.New()
	})

	Describe("Health Check", func() {
		It("should return healthy status", func() {
			// Create a handler - health check doesn't require services
			// This is one of the few endpoints that works with nil services
			handler := api.NewDomainHandler(nil, nil, nil, nil, nil, logger)
			
			// Register routes
			handler.RegisterRoutes(router)
			
			// Create request
			w := testhelpers.PerformRequest(router, "GET", "/api/v1/health", nil)
			
			// Assert response
			Expect(w.Code).To(Equal(http.StatusOK))
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["status"]).To(Equal("healthy"))
			Expect(response).To(HaveKey("timestamp"))
		})
	})

	// Note: Comprehensive handler tests with actual service calls would require:
	// 1. Creating interfaces for all services (TestRunService, ProjectService, etc.)
	// 2. Creating mock implementations of those interfaces
	// 3. Injecting mocks and setting expectations
	//
	// The current architecture uses concrete service types, not interfaces,
	// which makes unit testing at the handler level challenging.
	//
	// Best practices:
	// - Test business logic at the service layer (where it's easier to mock repositories)
	// - Test HTTP handling, routing, and middleware at the handler layer
	// - Use integration tests with a real database for end-to-end testing
	
	Describe("Route Registration", func() {
		It("should register all expected routes", func() {
			handler := api.NewDomainHandler(nil, nil, nil, nil, nil, logger)
			handler.RegisterRoutes(router)
			
			routes := router.Routes()
			
			// Verify some key routes are registered
			expectedPaths := []string{
				"/api/v1/health",
				"/auth/login", 
				"/auth/logout",
				"/auth/callback",
			}
			
			for _, expectedPath := range expectedPaths {
				found := false
				for _, route := range routes {
					if route.Path == expectedPath {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Expected route %s to be registered", expectedPath)
			}
		})
	})
})