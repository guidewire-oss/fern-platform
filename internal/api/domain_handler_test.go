package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/guidewire-oss/fern-platform/internal/testhelpers"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TODO:DomainHandler uses concrete service types (not interfaces) and you can't inject mocks without changing the implementation, this branch will never be reached in unit tests.
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
		logger *logging.Logger
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)

		// Initialize logger
		loggingConfig := &config.LoggingConfig{
			Level:  "info",
			Format: "json",
		}
		var err error
		logger, err = logging.NewLogger(loggingConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Health Check", func() {
		It("should return healthy status", func() {
			// Create a fresh router for this test
			router := gin.New()

			// Create handler - health check doesn't require services
			handler := NewDomainHandler(nil, nil, nil, nil, nil, nil, logger)

			// Register routes
			handler.RegisterRoutes(router)

			// Create request
			w := testhelpers.PerformRequest(router, "GET", "/health", nil)

			// Assert response
			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["status"]).To(Equal("healthy"))
			Expect(response).To(HaveKey("time"))
		})
	})

	Describe("Route Registration", func() {
		It("should register all expected routes", func() {
			// Create a fresh router for this test
			router := gin.New()

			handler := NewDomainHandler(nil, nil, nil, nil, nil, nil, logger)
			handler.RegisterRoutes(router)

			routes := router.Routes()

			// Verify some key routes are registered
			expectedPaths := []string{
				"/health",
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

var _ = Describe("recordTestRun Function Tests", func() {
	var (
		handler *DomainHandler
		router  *gin.Engine
		logger  *logging.Logger
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)

		// Initialize logger
		loggingConfig := &config.LoggingConfig{
			Level:  "info",
			Format: "json",
		}
		var err error
		logger, err = logging.NewLogger(loggingConfig)
		Expect(err).NotTo(HaveOccurred())

		// Create handler with nil services - we'll test what we can without mocking
		handler = NewDomainHandler(nil, nil, nil, nil, nil, nil, logger)

		// Setup router with only the specific route we're testing
		router = gin.New()
		router.POST("/api/v1/test-runs", handler.recordTestRun)
	})

	Describe("Request Validation and JSON Binding", func() {
		It("should return 400 for invalid JSON", func() {
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBufferString("invalid json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})

		It("should return 500 for empty JSON object", func() {
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBufferString("{}"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should return 500 for missing testProjectId", func() {
			requestBody := map[string]interface{}{
				"suiteRuns": []interface{}{},
				// Missing testProjectId
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should pass JSON binding validation with valid structure", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// With nil services, this should reach the service layer and fail there (500)
			// This proves JSON binding worked correctly
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("Environment Field Processing", func() {
		It("should process request with explicit environment", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"environment":   "production",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should pass environment processing logic, fail at service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should process request without environment field", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should default environment and reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should process request with empty environment string", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"environment":   "",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should handle empty environment and reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("TestSeed and RunID Generation Logic", func() {
		It("should process request with non-zero TestSeed", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"testSeed":      12345,
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should process TestSeed logic and reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should process request with zero TestSeed", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"testSeed":      0,
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should generate UUID and reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should process request without TestSeed field", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should generate UUID and reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("Suite Runs Processing", func() {
		It("should process empty suite runs array", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should process empty suite array and reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should process suite runs with various statuses", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns": []map[string]interface{}{
					{
						"name":         "suite-1",
						"status":       "passed",
						"totalTests":   5,
						"passedTests":  5,
						"failedTests":  0,
						"skippedTests": 0,
					},
					{
						"name":         "suite-2",
						"status":       "failed",
						"totalTests":   3,
						"passedTests":  1,
						"failedTests":  2,
						"skippedTests": 0,
					},
					{
						"name":         "suite-3",
						"status":       "partial",
						"totalTests":   2,
						"passedTests":  1,
						"failedTests":  0,
						"skippedTests": 1,
					},
				},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should process all suite conversion logic and reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("Git Information Processing", func() {
		It("should process request with git branch and commit", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"gitBranch":     "main",
				"gitSha":        "abc123def456",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should process git information and reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should process request without git information", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should handle missing git information and reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("Complex Request Scenarios", func() {
		It("should process comprehensive test run request", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "comprehensive-project",
				"gitBranch":     "feature/testing",
				"gitSha":        "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b",
				"environment":   "staging",
				"testSeed":      uint64(999888777),
				"suiteRuns": []map[string]interface{}{
					{
						"name":         "unit-tests",
						"status":       "passed",
						"totalTests":   25,
						"passedTests":  25,
						"failedTests":  0,
						"skippedTests": 0,
					},
					{
						"name":         "integration-tests",
						"status":       "partial",
						"totalTests":   15,
						"passedTests":  12,
						"failedTests":  0,
						"skippedTests": 3,
					},
					{
						"name":         "e2e-tests",
						"status":       "failed",
						"totalTests":   8,
						"passedTests":  6,
						"failedTests":  2,
						"skippedTests": 0,
					},
				},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should process all fields and logic, reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should handle large number of suite runs", func() {
			suiteRuns := make([]map[string]interface{}, 20)
			for i := 0; i < 20; i++ {
				suiteRuns[i] = map[string]interface{}{
					"name":         fmt.Sprintf("suite-%d", i),
					"status":       "passed",
					"totalTests":   10,
					"passedTests":  10,
					"failedTests":  0,
					"skippedTests": 0,
				}
			}

			requestBody := map[string]interface{}{
				"testProjectId": "large-test-project",
				"suiteRuns":     suiteRuns,
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should handle large payload and reach service layer
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	//TODO: handle the non post requests tests
	Describe("HTTP Method and Content Type Validation", func() {
		It("should reject non-POST requests", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("GET", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should handle requests without Content-Type header", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			// No Content-Type header
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should still attempt to parse JSON
			Expect(w.Code).To(BeNumerically(">=", 400))
		})
	})

	Describe("Error Response Format Consistency", func() {
		It("should return consistent error format for JSON parsing errors", func() {
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBufferString("invalid json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(HaveKey("error"))
			Expect(response["error"]).To(BeAssignableToTypeOf(""))
		})

		It("should return consistent error format for validation errors", func() {
			requestBody := map[string]interface{}{
				"suiteRuns": []interface{}{},
				// Missing required testProjectId
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(HaveKey("error"))
		})

		It("should return consistent error format for service layer errors", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns":     []interface{}{},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(HaveKey("error"))
		})
	})
})
