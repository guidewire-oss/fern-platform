package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/api"
	"github.com/guidewire-oss/fern-platform/internal/testhelpers"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// Test entry point
func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Domain Handler Suite")
}

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
			// Create a handler with all services as nil for health check
			// Health check doesn't use any services
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

	// Note: These tests are focused on verifying that the handler correctly routes
	// requests and returns proper HTTP responses. We're not testing the business
	// logic which should be tested in the service layer tests.
	
	Describe("Authentication Requirement", func() {
		It("should require authentication for protected endpoints", func() {
			// Create a mock auth middleware that rejects unauthenticated requests
			mockAuth := &MockAuthMiddleware{}
			
			// Create handler with auth middleware
			handler := api.NewDomainHandler(nil, nil, nil, nil, mockAuth, logger)
			handler.RegisterRoutes(router)
			
			// Try to access a protected endpoint without authentication
			w := testhelpers.PerformRequest(router, "GET", "/api/v1/test-runs", nil)
			
			// Should be rejected by auth middleware
			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})
	})
})

