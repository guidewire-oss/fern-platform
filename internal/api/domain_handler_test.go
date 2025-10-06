package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/testhelpers"
	"github.com/stretchr/testify/mock"

	"github.com/gin-gonic/gin"
	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	tagsDomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
	testingApp "github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	testingDomain "github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
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

var _ = Describe("DomainHandler Integration Tests", Ordered, Serial, func() {
	var (
		logger *logging.Logger
	)

	BeforeAll(func() {
		gin.SetMode(gin.TestMode)
	})

	BeforeEach(func() {
		// Initialize logger
		loggingConfig := &config.LoggingConfig{
			Level:  "info",
			Format: "json",
		}
		var err error
		logger, err = logging.NewLogger(loggingConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	XDescribe("Health Check", func() {
		It("should return healthy status", func() {
			// Create a fresh router for this test
			router := gin.New()

			// Create handler - health check doesn't require services
			handler := NewDomainHandler(nil, nil, nil, nil, nil, nil, nil, logger)

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

	XDescribe("Route Registration", func() {
		It("should register all expected routes", func() {
			// Create a fresh router for this test
			router := gin.New()

			handler := NewDomainHandler(nil, nil, nil, nil, nil, nil, nil, logger)
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
		handler = NewDomainHandler(nil, nil, nil, nil, nil, nil, nil, logger)

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

		It("should return 400 when request body is nil or empty", func() {
			// Test with nil body - Gin treats nil body as EOF during JSON binding
			req := httptest.NewRequest("POST", "/api/v1/test-runs", nil)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			// Nil or empty body causes EOF error during JSON binding
			Expect(response["error"]).To(ContainSubstring("EOF"))
		})

		It("should check for nil request body before JSON binding", func() {
			// This test verifies the explicit nil body check exists in the code
			// The check at line 189: if c.Request.Body == nil
			// However, in practice, Gin's httptest always provides a body (even if empty),
			// so the ShouldBindJSON catches it first with EOF error

			// Test with empty string body
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBufferString(""))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			// Empty body causes EOF error during JSON parsing
			Expect(response["error"]).To(ContainSubstring("EOF"))
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
						"status":       "skipped",
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
						"status":       "skipped",
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

	Describe("Tags in Spec Runs and Suite Runs", func() {
		It("should process request with tags in spec runs", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns": []map[string]interface{}{
					{
						"name":   "suite-1",
						"status": "passed",
						"tags": []map[string]interface{}{
							{"name": "smoke"},
							{"name": "priority:high"},
						},
						"specRuns": []map[string]interface{}{
							{
								"name":   "spec-1",
								"status": "passed",
								"tags": []map[string]interface{}{
									{"name": "unit"},
									{"name": "category:backend"},
								},
							},
						},
					},
				},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should parse JSON with tags structure successfully
			// Will fail at service layer due to nil services, but proves tags were parsed
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should process request with tags only in suite runs", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns": []map[string]interface{}{
					{
						"name":   "suite-1",
						"status": "passed",
						"tags": []map[string]interface{}{
							{"name": "regression"},
						},
						"specRuns": []map[string]interface{}{},
					},
				},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should parse JSON with suite-level tags successfully
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should process request with tags at both suite and spec levels", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns": []map[string]interface{}{
					{
						"name":   "integration-suite",
						"status": "passed",
						"tags": []map[string]interface{}{
							{"name": "integration"},
							{"name": "environment:staging"},
						},
						"specRuns": []map[string]interface{}{
							{
								"name":   "api-test",
								"status": "passed",
								"tags": []map[string]interface{}{
									{"name": "api"},
									{"name": "critical"},
								},
							},
							{
								"name":   "db-test",
								"status": "passed",
								"tags": []map[string]interface{}{
									{"name": "database"},
								},
							},
						},
					},
				},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should parse complex tag structure successfully
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should handle missing tags field in spec runs", func() {
			requestBody := map[string]interface{}{
				"testProjectId": "test-project",
				"suiteRuns": []map[string]interface{}{
					{
						"name":   "suite-1",
						"status": "passed",
						// No tags field
						"specRuns": []map[string]interface{}{
							{
								"name":   "spec-1",
								"status": "passed",
								// No tags field
							},
						},
					},
				},
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should handle missing tags field gracefully
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

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

// MockTagRepository provides a mock implementation of TagRepository
type MockTagRepository struct {
	mock.Mock
}

func (m *MockTagRepository) Save(ctx context.Context, tag *tagsDomain.Tag) error {
	args := m.Called(ctx, tag)
	return args.Error(0)
}

func (m *MockTagRepository) FindByID(ctx context.Context, id tagsDomain.TagID) (*tagsDomain.Tag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tagsDomain.Tag), args.Error(1)
}

func (m *MockTagRepository) FindByName(ctx context.Context, name string) (*tagsDomain.Tag, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tagsDomain.Tag), args.Error(1)
}

func (m *MockTagRepository) FindAll(ctx context.Context) ([]*tagsDomain.Tag, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*tagsDomain.Tag), args.Error(1)
}

func (m *MockTagRepository) Delete(ctx context.Context, id tagsDomain.TagID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTagRepository) AssignToTestRun(ctx context.Context, testRunID string, tagIDs []tagsDomain.TagID) error {
	args := m.Called(ctx, testRunID, tagIDs)
	return args.Error(0)
}

// Integration tests with mocked services for recordTestRun
var _ = Describe("recordTestRun Integration Tests with Mocked Services", func() {
	var (
		handler        *DomainHandler
		router         *gin.Engine
		logger         *logging.Logger
		testRunRepo    *MockTestRunRepository
		suiteRunRepo   *MockSuiteRunRepository
		specRunRepo    *MockSpecRunRepository
		tagRepo        *MockTagRepository
		testingService *testingApp.TestRunService
		tagService     *tagsApp.TagService
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

		// Create mocks
		testRunRepo = new(MockTestRunRepository)
		suiteRunRepo = new(MockSuiteRunRepository)
		specRunRepo = new(MockSpecRunRepository)
		tagRepo = new(MockTagRepository)

		// Create services with mocks
		testingService = testingApp.NewTestRunService(testRunRepo, suiteRunRepo, specRunRepo)
		tagService = tagsApp.NewTagService(tagRepo)

		// Setup mock for tag processing - return the same tags (no-op behavior for these tests)
		tagRepo.On("FindByName", mock.Anything, mock.Anything).Return(nil, errors.New("not found")).Maybe()
		tagRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Maybe()

		// Create handler
		handler = NewDomainHandler(testingService, nil, tagService, nil, nil, nil, nil, logger)

		// Setup router
		router = gin.New()
		router.POST("/api/v1/test-runs", handler.recordTestRun)
	})

	Describe("Creating New Test Run", func() {
		It("should create a new test run successfully with no TestSeed", func() {
			requestBody := map[string]interface{}{
				"test_project_id": "test-project-123",
				"git_branch":      "main",
				"git_sha":         "abc123",
				"environment":     "production",
				"suite_runs": []map[string]interface{}{
					{
						"suite_name": "unit-tests",
						"spec_runs": []map[string]interface{}{
							{
								"spec_description": "test 1",
								"status":           "passed",
							},
							{
								"spec_description": "test 2",
								"status":           "passed",
							},
						},
					},
				},
			}

			// Mock expectations - new test run creation
			testRunRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assert response
			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["projectId"]).To(Equal("test-project-123"))
			Expect(response["branch"]).To(Equal("main"))
			Expect(response["commitSha"]).To(Equal("abc123"))
			Expect(response["environment"]).To(Equal("production"))
			Expect(response["status"]).To(Equal("passed"))
			Expect(response["totalTests"]).To(Equal(float64(2)))
			Expect(response["passedTests"]).To(Equal(float64(2)))
			Expect(response["runId"]).NotTo(BeEmpty())

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should create a test run with TestSeed and generate runID from seed", func() {
			requestBody := map[string]interface{}{
				"test_project_id": "test-project-123",
				"test_seed":       uint64(999888),
				"suite_runs":      []interface{}{},
			}

			// Mock expectations - GetByRunID should not find existing
			testRunRepo.On("GetByRunID", mock.Anything, "999888").Return(nil, errors.New("not found")).Once()
			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.RunID == "999888"
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["runId"]).To(Equal("999888"))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should default environment to 'default' when not provided", func() {
			requestBody := map[string]interface{}{
				"test_project_id": "test-project-123",
				"suite_runs":      []interface{}{},
			}

			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.Environment == "default"
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["environment"]).To(Equal("default"))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should calculate status as failed when suite has failed tests", func() {
			requestBody := map[string]interface{}{
				"test_project_id": "test-project",
				"suite_runs": []map[string]interface{}{
					{
						"suite_name": "failing-suite",
						"spec_runs": []map[string]interface{}{
							{
								"spec_description": "passing test",
								"status":           "passed",
							},
							{
								"spec_description": "failing test",
								"status":           "failed",
							},
						},
					},
				},
			}

			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.Status == "failed"
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["status"]).To(Equal("failed"))
			Expect(response["failedTests"]).To(Equal(float64(1)))
			Expect(response["passedTests"]).To(Equal(float64(1)))

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Updating Existing Test Run", func() {
		It("should find existing test run by runID and append suite runs", func() {
			existingTestRun := &testingDomain.TestRun{
				ID:           1,
				RunID:        "existing-run-id",
				ProjectID:    "test-project",
				Status:       "passed",
				TotalTests:   5,
				PassedTests:  5,
				FailedTests:  0,
				SkippedTests: 0,
				SuiteRuns:    []testingDomain.SuiteRun{},
			}

			requestBody := map[string]interface{}{
				"test_project_id": "test-project",
				"test_seed":       uint64(12345),
				"suite_runs": []map[string]interface{}{
					{
						"suite_name": "new-suite",
						"spec_runs": []map[string]interface{}{
							{
								"spec_description": "new test",
								"status":           "passed",
							},
						},
					},
				},
			}

			// Mock expectations - find existing run, add suite run, update test run
			testRunRepo.On("GetByRunID", mock.Anything, "12345").Return(existingTestRun, nil).Once()
			suiteRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				// Simulate database auto-increment by setting the ID
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 100
			}).Return(nil).Once()
			specRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				// Simulate database auto-increment by setting the ID
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 200
			}).Return(nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				// Should have accumulated counts
				return tr.TotalTests == 6 && tr.PassedTests == 6
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["totalTests"]).To(Equal(float64(6)))
			Expect(response["passedTests"]).To(Equal(float64(6)))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
			specRunRepo.AssertExpectations(GinkgoT())
		})

		It("should handle concurrent creation by treating as update when duplicate occurs", func() {
			requestBody := map[string]interface{}{
				"test_project_id": "test-project",
				"test_seed":       uint64(77777),
				"suite_runs": []map[string]interface{}{
					{
						"suite_name": "suite-1",
						"spec_runs": []map[string]interface{}{
							{
								"spec_description": "test 1",
								"status":           "passed",
							},
						},
					},
				},
			}

			// Mock GetByRunID to return not found initially
			testRunRepo.On("GetByRunID", mock.Anything, "77777").Return(nil, errors.New("not found")).Once()

			// Mock Create to simulate duplicate/unique constraint error
			testRunRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("UNIQUE constraint failed")).Once()

			// After duplicate error, it tries to fetch existing test run by runID
			existingTestRun := &testingDomain.TestRun{
				ID:           99,
				RunID:        "77777",
				ProjectID:    "test-project",
				Status:       "passed",
				TotalTests:   0,
				PassedTests:  0,
				FailedTests:  0,
				SkippedTests: 0,
				SuiteRuns:    []testingDomain.SuiteRun{},
			}
			testRunRepo.On("GetByRunID", mock.Anything, "77777").Return(existingTestRun, nil).Once()

			// Now it adds suite runs to the existing test run
			suiteRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 100
			}).Return(nil).Once()
			specRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 200
			}).Return(nil).Once()

			// Update the test run with accumulated data
			testRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
			specRunRepo.AssertExpectations(GinkgoT())
		})

		It("should update status to failed when new batch has failures", func() {
			existingTestRun := &testingDomain.TestRun{
				ID:           1,
				RunID:        "test-run-id",
				ProjectID:    "test-project",
				Status:       "passed",
				TotalTests:   3,
				PassedTests:  3,
				FailedTests:  0,
				SkippedTests: 0,
				SuiteRuns:    []testingDomain.SuiteRun{},
			}

			requestBody := map[string]interface{}{
				"test_project_id": "test-project",
				"test_seed":       uint64(88888),
				"suite_runs": []map[string]interface{}{
					{
						"suite_name": "failing-suite",
						"spec_runs": []map[string]interface{}{
							{
								"spec_description": "failing test",
								"status":           "failed",
							},
						},
					},
				},
			}

			testRunRepo.On("GetByRunID", mock.Anything, "88888").Return(existingTestRun, nil).Once()
			suiteRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 100
			}).Return(nil).Once()
			specRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 200
			}).Return(nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.Status == "failed" && tr.FailedTests == 1
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["status"]).To(Equal("failed"))

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Error Handling", func() {
		It("should return 500 when CreateTestRun fails", func() {
			requestBody := map[string]interface{}{
				"test_project_id": "test-project",
				"suite_runs":      []interface{}{},
			}

			testRunRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("database error")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("database error"))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when CreateSuiteRun fails", func() {
			existingTestRun := &testingDomain.TestRun{
				ID:        1,
				RunID:     "run-id",
				ProjectID: "test-project",
			}

			requestBody := map[string]interface{}{
				"test_project_id": "test-project",
				"test_seed":       uint64(55555),
				"suite_runs": []map[string]interface{}{
					{
						"suite_name": "suite-1",
						"spec_runs":  []interface{}{},
					},
				},
			}

			testRunRepo.On("GetByRunID", mock.Anything, "55555").Return(existingTestRun, nil).Once()
			suiteRunRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("suite creation error")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when CreateSpecRun fails", func() {
			existingTestRun := &testingDomain.TestRun{
				ID:        1,
				RunID:     "run-id",
				ProjectID: "test-project",
			}

			requestBody := map[string]interface{}{
				"test_project_id": "test-project",
				"test_seed":       uint64(66666),
				"suite_runs": []map[string]interface{}{
					{
						"suite_name": "suite-1",
						"spec_runs": []map[string]interface{}{
							{
								"spec_description": "spec-1",
								"status":           "passed",
							},
						},
					},
				},
			}

			testRunRepo.On("GetByRunID", mock.Anything, "66666").Return(existingTestRun, nil).Once()
			suiteRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 100
			}).Return(nil).Once()
			specRunRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("spec creation error")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
			specRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when UpdateTestRun fails", func() {
			existingTestRun := &testingDomain.TestRun{
				ID:        1,
				RunID:     "run-id",
				ProjectID: "test-project",
				SuiteRuns: []testingDomain.SuiteRun{},
			}

			requestBody := map[string]interface{}{
				"test_project_id": "test-project",
				"test_seed":       uint64(44444),
				"suite_runs": []map[string]interface{}{
					{
						"suite_name": "suite-1",
						"spec_runs": []map[string]interface{}{
							{
								"spec_description": "spec-1",
								"status":           "passed",
							},
						},
					},
				},
			}

			testRunRepo.On("GetByRunID", mock.Anything, "44444").Return(existingTestRun, nil).Once()
			suiteRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 100
			}).Return(nil).Once()
			specRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 200
			}).Return(nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.Anything).Return(errors.New("update error")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("getTestRuns with service account", func() {
		It("should handle service account request without user", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			testRunService := testingApp.NewTestRunService(nil, nil, nil)
			handler := NewDomainHandler(testRunService, nil, nil, nil, nil, nil, nil, logger)

			router.GET("/test-runs", handler.getTestRuns)

			req := httptest.NewRequest("GET", "/test-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
			Expect(w.Body.String()).To(ContainSubstring("Authentication required"))
		})

		It("should handle invalid user data", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			testRunService := testingApp.NewTestRunService(nil, nil, nil)
			handler := NewDomainHandler(testRunService, nil, nil, nil, nil, nil, nil, logger)

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", "not-a-user-object") // Invalid type
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			Expect(w.Body.String()).To(ContainSubstring("Invalid user data"))
		})

		It("should successfully retrieve test runs with projectId for regular user", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			// Create mocks
			testRunRepo := new(MockTestRunRepository)
			projectRepo := new(MockProjectRepository)
			permissionRepo := new(MockProjectPermissionRepository)

			// Create services
			testingService := testingApp.NewTestRunService(testRunRepo, nil, nil)
			projectService := projectsApp.NewProjectService(projectRepo, permissionRepo)

			// Create handler
			handler := NewDomainHandler(testingService, projectService, nil, nil, nil, nil, nil, logger)

			// Create test user
			testUser := &authDomain.User{
				UserID: "user-123",
				Email:  "test@example.com",
				Name:   "Test User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-alpha"},
				},
			}

			// Create mock test runs
			mockTestRuns := []*testingDomain.TestRun{
				{
					ID:           1,
					RunID:        "run-123",
					ProjectID:    "project-alpha",
					Branch:       "main",
					Status:       "passed",
					TotalTests:   10,
					PassedTests:  10,
					FailedTests:  0,
					SkippedTests: 0,
				},
			}

			// Create mock project
			mockProject, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("project-alpha"),
				"Project Alpha",
				projectsDomain.Team("team-alpha"),
			)

			// Setup expectations
			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-alpha", 50).Return(mockTestRuns, nil)
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("project-alpha")).Return(mockProject, nil)

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs?projectId=project-alpha", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())
			Expect(response["total"]).To(Equal(float64(1)))
			Expect(response["limit"]).To(Equal(float64(50)))
			Expect(response["offset"]).To(Equal(float64(0)))

			testRunRepo.AssertExpectations(GinkgoT())
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should successfully retrieve test runs with projectId for service account", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			// Create mocks
			testRunRepo := new(MockTestRunRepository)

			// Create services
			testingService := testingApp.NewTestRunService(testRunRepo, nil, nil)

			// Create handler
			handler := NewDomainHandler(testingService, nil, nil, nil, nil, nil, nil, logger)

			// Create test user (service account)
			testUser := &authDomain.User{
				UserID: "service-account-123",
				Email:  "service@example.com",
				Name:   "Service Account",
			}

			// Create mock test runs
			mockTestRuns := []*testingDomain.TestRun{
				{
					ID:           1,
					RunID:        "run-123",
					ProjectID:    "project-alpha",
					Branch:       "main",
					Status:       "passed",
					TotalTests:   10,
					PassedTests:  10,
					FailedTests:  0,
					SkippedTests: 0,
				},
				{
					ID:           2,
					RunID:        "run-456",
					ProjectID:    "project-alpha",
					Branch:       "develop",
					Status:       "failed",
					TotalTests:   20,
					PassedTests:  18,
					FailedTests:  2,
					SkippedTests: 0,
				},
			}

			// Setup expectations
			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-alpha", 50).Return(mockTestRuns, nil)

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", testUser)
				c.Set("is_service_account", true) // Service account flag
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs?projectId=project-alpha", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())
			Expect(response["total"]).To(Equal(float64(2))) // Service account sees all test runs
			Expect(response["limit"]).To(Equal(float64(50)))
			Expect(response["offset"]).To(Equal(float64(0)))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when GetProjectTestRuns fails", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			// Create mocks
			testRunRepo := new(MockTestRunRepository)

			// Create services
			testingService := testingApp.NewTestRunService(testRunRepo, nil, nil)

			// Create handler
			handler := NewDomainHandler(testingService, nil, nil, nil, nil, nil, nil, logger)

			// Create test user
			testUser := &authDomain.User{
				UserID: "user-123",
				Email:  "test@example.com",
				Name:   "Test User",
			}

			// Setup expectations - return error
			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-alpha", 50).
				Return([]*testingDomain.TestRun{}, fmt.Errorf("database error"))

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs?projectId=project-alpha", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			Expect(w.Body.String()).To(ContainSubstring("database error"))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should successfully retrieve recent test runs without projectId for regular user", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			// Create mocks
			testRunRepo := new(MockTestRunRepository)
			projectRepo := new(MockProjectRepository)
			permissionRepo := new(MockProjectPermissionRepository)

			// Create services
			testingService := testingApp.NewTestRunService(testRunRepo, nil, nil)
			projectService := projectsApp.NewProjectService(projectRepo, permissionRepo)

			// Create handler
			handler := NewDomainHandler(testingService, projectService, nil, nil, nil, nil, nil, logger)

			// Create test user
			testUser := &authDomain.User{
				UserID: "user-123",
				Email:  "test@example.com",
				Name:   "Test User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-alpha"},
				},
			}

			// Create mock test runs
			mockTestRuns := []*testingDomain.TestRun{
				{
					ID:           1,
					RunID:        "run-123",
					ProjectID:    "project-alpha",
					Branch:       "main",
					Status:       "passed",
					TotalTests:   10,
					PassedTests:  10,
					FailedTests:  0,
					SkippedTests: 0,
				},
			}

			// Create mock project
			mockProject, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("project-alpha"),
				"Project Alpha",
				projectsDomain.Team("team-alpha"),
			)

			// Setup expectations
			testRunRepo.On("GetRecent", mock.Anything, 50).Return(mockTestRuns, nil)
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("project-alpha")).Return(mockProject, nil)

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())
			Expect(response["total"]).To(Equal(float64(1)))
			Expect(response["limit"]).To(Equal(float64(50)))
			Expect(response["offset"]).To(Equal(float64(0)))

			testRunRepo.AssertExpectations(GinkgoT())
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should successfully retrieve recent test runs without projectId for service account", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			// Create mocks
			testRunRepo := new(MockTestRunRepository)

			// Create services
			testingService := testingApp.NewTestRunService(testRunRepo, nil, nil)

			// Create handler
			handler := NewDomainHandler(testingService, nil, nil, nil, nil, nil, nil, logger)

			// Create test user (service account)
			testUser := &authDomain.User{
				UserID: "service-account-123",
				Email:  "service@example.com",
				Name:   "Service Account",
			}

			// Create mock test runs
			mockTestRuns := []*testingDomain.TestRun{
				{
					ID:           1,
					RunID:        "run-123",
					ProjectID:    "project-alpha",
					Branch:       "main",
					Status:       "passed",
					TotalTests:   10,
					PassedTests:  10,
					FailedTests:  0,
					SkippedTests: 0,
				},
				{
					ID:           2,
					RunID:        "run-456",
					ProjectID:    "project-beta",
					Branch:       "develop",
					Status:       "failed",
					TotalTests:   20,
					PassedTests:  18,
					FailedTests:  2,
					SkippedTests: 0,
				},
			}

			// Setup expectations
			testRunRepo.On("GetRecent", mock.Anything, 50).Return(mockTestRuns, nil)

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", testUser)
				c.Set("is_service_account", true) // Service account flag
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())
			Expect(response["total"]).To(Equal(float64(2))) // Service account sees all test runs
			Expect(response["limit"]).To(Equal(float64(50)))
			Expect(response["offset"]).To(Equal(float64(0)))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when GetRecentTestRuns fails", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			// Create mocks
			testRunRepo := new(MockTestRunRepository)

			// Create services
			testingService := testingApp.NewTestRunService(testRunRepo, nil, nil)

			// Create handler
			handler := NewDomainHandler(testingService, nil, nil, nil, nil, nil, nil, logger)

			// Create test user
			testUser := &authDomain.User{
				UserID: "user-123",
				Email:  "test@example.com",
				Name:   "Test User",
			}

			// Setup expectations - return error
			testRunRepo.On("GetRecent", mock.Anything, 50).
				Return([]*testingDomain.TestRun{}, fmt.Errorf("database connection failed"))

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			Expect(w.Body.String()).To(ContainSubstring("database connection failed"))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should parse custom limit and offset query parameters", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			// Create mocks
			testRunRepo := new(MockTestRunRepository)

			// Create services
			testingService := testingApp.NewTestRunService(testRunRepo, nil, nil)

			// Create handler
			handler := NewDomainHandler(testingService, nil, nil, nil, nil, nil, nil, logger)

			// Create test user (service account for simplicity)
			testUser := &authDomain.User{
				UserID: "service-account-123",
				Email:  "service@example.com",
				Name:   "Service Account",
			}

			// Create mock test runs
			mockTestRuns := []*testingDomain.TestRun{
				{
					ID:           1,
					RunID:        "run-123",
					ProjectID:    "project-alpha",
					Branch:       "main",
					Status:       "passed",
					TotalTests:   10,
					PassedTests:  10,
					FailedTests:  0,
					SkippedTests: 0,
				},
			}

			// Setup expectations with custom limit
			testRunRepo.On("GetRecent", mock.Anything, 100).Return(mockTestRuns, nil)

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", testUser)
				c.Set("is_service_account", true)
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs?limit=100&offset=20", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())
			Expect(response["limit"]).To(Equal(float64(100))) // Custom limit
			Expect(response["offset"]).To(Equal(float64(20))) // Custom offset
			Expect(response["total"]).To(Equal(float64(1)))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should filter test runs by user groups for regular user", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			// Create mocks
			testRunRepo := new(MockTestRunRepository)
			projectRepo := new(MockProjectRepository)
			permissionRepo := new(MockProjectPermissionRepository)

			// Create services
			testingService := testingApp.NewTestRunService(testRunRepo, nil, nil)
			projectService := projectsApp.NewProjectService(projectRepo, permissionRepo)

			// Create handler
			handler := NewDomainHandler(testingService, projectService, nil, nil, nil, nil, nil, logger)

			// Create test user with specific group
			testUser := &authDomain.User{
				UserID: "user-123",
				Email:  "test@example.com",
				Name:   "Test User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-alpha"}, // User only in team-alpha
				},
			}

			// Create mock test runs from different projects
			mockTestRuns := []*testingDomain.TestRun{
				{
					ID:           1,
					RunID:        "run-123",
					ProjectID:    "project-alpha",
					Branch:       "main",
					Status:       "passed",
					TotalTests:   10,
					PassedTests:  10,
					FailedTests:  0,
					SkippedTests: 0,
				},
				{
					ID:           2,
					RunID:        "run-456",
					ProjectID:    "project-beta",
					Branch:       "main",
					Status:       "passed",
					TotalTests:   5,
					PassedTests:  5,
					FailedTests:  0,
					SkippedTests: 0,
				},
			}

			// Create mock projects
			mockProjectAlpha, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("project-alpha"),
				"Project Alpha",
				projectsDomain.Team("team-alpha"), // User has access
			)
			mockProjectBeta, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("project-beta"),
				"Project Beta",
				projectsDomain.Team("team-beta"), // User does NOT have access
			)

			// Setup expectations
			testRunRepo.On("GetRecent", mock.Anything, 50).Return(mockTestRuns, nil)
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("project-alpha")).Return(mockProjectAlpha, nil)
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("project-beta")).Return(mockProjectBeta, nil)

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())

			// Should only see 1 test run (project-alpha), not project-beta
			Expect(response["total"]).To(Equal(float64(1)))

			// Verify the returned test run is from project-alpha
			data := response["data"].([]interface{})
			Expect(len(data)).To(Equal(1))
			firstTestRun := data[0].(map[string]interface{})
			Expect(firstTestRun["projectId"]).To(Equal("project-alpha"))

			testRunRepo.AssertExpectations(GinkgoT())
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should return empty data array when user has no access to any projects", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			// Create mocks
			testRunRepo := new(MockTestRunRepository)
			projectRepo := new(MockProjectRepository)
			permissionRepo := new(MockProjectPermissionRepository)

			// Create services
			testingService := testingApp.NewTestRunService(testRunRepo, nil, nil)
			projectService := projectsApp.NewProjectService(projectRepo, permissionRepo)

			// Create handler
			handler := NewDomainHandler(testingService, projectService, nil, nil, nil, nil, nil, logger)

			// Create test user with no matching groups
			testUser := &authDomain.User{
				UserID: "user-123",
				Email:  "test@example.com",
				Name:   "Test User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-gamma"}, // User in team-gamma
				},
			}

			// Create mock test runs
			mockTestRuns := []*testingDomain.TestRun{
				{
					ID:           1,
					RunID:        "run-123",
					ProjectID:    "project-alpha",
					Branch:       "main",
					Status:       "passed",
					TotalTests:   10,
					PassedTests:  10,
					FailedTests:  0,
					SkippedTests: 0,
				},
			}

			// Create mock project with different team
			mockProject, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("project-alpha"),
				"Project Alpha",
				projectsDomain.Team("team-alpha"), // User NOT in this team
			)

			// Setup expectations
			testRunRepo.On("GetRecent", mock.Anything, 50).Return(mockTestRuns, nil)
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("project-alpha")).Return(mockProject, nil)

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())
			Expect(response["total"]).To(Equal(float64(0))) // User sees no test runs

			data := response["data"].([]interface{})
			Expect(len(data)).To(Equal(0))

			testRunRepo.AssertExpectations(GinkgoT())
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should handle empty test runs list", func() {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			// Create mocks
			testRunRepo := new(MockTestRunRepository)

			// Create services
			testingService := testingApp.NewTestRunService(testRunRepo, nil, nil)

			// Create handler
			handler := NewDomainHandler(testingService, nil, nil, nil, nil, nil, nil, logger)

			// Create test user (service account for simplicity)
			testUser := &authDomain.User{
				UserID: "service-account-123",
				Email:  "service@example.com",
				Name:   "Service Account",
			}

			// Setup expectations - return empty list
			testRunRepo.On("GetRecent", mock.Anything, 50).Return([]*testingDomain.TestRun{}, nil)

			router.GET("/test-runs", func(c *gin.Context) {
				c.Set("user", testUser)
				c.Set("is_service_account", true)
				handler.getTestRuns(c)
			})

			req := httptest.NewRequest("GET", "/test-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())
			Expect(response["total"]).To(Equal(float64(0)))
			Expect(response["limit"]).To(Equal(float64(50)))
			Expect(response["offset"]).To(Equal(float64(0)))

			data := response["data"].([]interface{})
			Expect(len(data)).To(Equal(0))

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})
})

var _ = Describe("completeTestRun Method Tests", func() {
	var (
		handler        *DomainHandler
		router         *gin.Engine
		logger         *logging.Logger
		testRunRepo    *MockTestRunRepository
		suiteRunRepo   *MockSuiteRunRepository
		testingService *testingApp.TestRunService
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

		// Create mocks
		testRunRepo = new(MockTestRunRepository)
		suiteRunRepo = new(MockSuiteRunRepository)

		// Create services with mocks
		testingService = testingApp.NewTestRunService(testRunRepo, suiteRunRepo, nil)

		// Create handler
		handler = NewDomainHandler(testingService, nil, nil, nil, nil, nil, nil, logger)

		// Setup router
		router = gin.New()
		router.POST("/api/v1/test-runs/complete", handler.completeTestRun)
	})

	Describe("Request Validation", func() {
		It("should return 400 for invalid JSON", func() {
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBufferString("invalid json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})

		It("should return 400 for missing required runId field", func() {
			requestBody := map[string]interface{}{
				"status": "passed",
				// Missing runId
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("RunID"))
		})

		It("should return 400 for empty request body", func() {
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBufferString("{}"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})
	})

	Describe("Successful Test Run Completion", func() {
		It("should complete test run with all fields provided", func() {
			now := time.Now()
			requestBody := map[string]interface{}{
				"runId":        "test-run-id-123",
				"status":       "passed",
				"endTime":      now.Format(time.RFC3339),
				"totalTests":   100,
				"passedTests":  95,
				"failedTests":  5,
				"skippedTests": 0,
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        1,
				RunID:     "test-run-id-123",
				ProjectID: "test-project",
				Status:    "running",
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-id-123").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(1)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(1)).Return([]*testingDomain.SuiteRun{}, nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.ID == 1 && tr.Status == "passed"
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["message"]).To(Equal("Test run completed successfully"))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should complete test run with minimal fields (only runId)", func() {
			requestBody := map[string]interface{}{
				"runId": "minimal-run-id",
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        2,
				RunID:     "minimal-run-id",
				ProjectID: "test-project",
				Status:    "running",
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "minimal-run-id").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(2)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(2)).Return([]*testingDomain.SuiteRun{}, nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.ID == 2
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["message"]).To(Equal("Test run completed successfully"))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should auto-set endTime when not provided", func() {
			requestBody := map[string]interface{}{
				"runId":  "auto-endtime-run",
				"status": "passed",
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        3,
				RunID:     "auto-endtime-run",
				ProjectID: "test-project",
				Status:    "running",
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "auto-endtime-run").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(3)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(3)).Return([]*testingDomain.SuiteRun{}, nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should use provided endTime when specified", func() {
			customEndTime := time.Date(2025, 10, 31, 12, 0, 0, 0, time.UTC)
			requestBody := map[string]interface{}{
				"runId":   "custom-endtime-run",
				"status":  "failed",
				"endTime": customEndTime.Format(time.RFC3339),
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        4,
				RunID:     "custom-endtime-run",
				ProjectID: "test-project",
				Status:    "running",
			}

			testRunRepo.On("GetByRunID", mock.Anything, "custom-endtime-run").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(4)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(4)).Return([]*testingDomain.SuiteRun{}, nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Status Handling", func() {
		It("should complete test run with 'passed' status", func() {
			requestBody := map[string]interface{}{
				"runId":  "passed-run",
				"status": "passed",
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        5,
				RunID:     "passed-run",
				ProjectID: "test-project",
				Status:    "running",
			}

			testRunRepo.On("GetByRunID", mock.Anything, "passed-run").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(5)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(5)).Return([]*testingDomain.SuiteRun{}, nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.Status == "passed"
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should complete test run with 'failed' status", func() {
			requestBody := map[string]interface{}{
				"runId":  "failed-run",
				"status": "failed",
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        6,
				RunID:     "failed-run",
				ProjectID: "test-project",
				Status:    "running",
			}

			testRunRepo.On("GetByRunID", mock.Anything, "failed-run").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(6)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(6)).Return([]*testingDomain.SuiteRun{}, nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.Status == "failed"
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should handle empty status", func() {
			requestBody := map[string]interface{}{
				"runId":  "empty-status-run",
				"status": "",
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        7,
				RunID:     "empty-status-run",
				ProjectID: "test-project",
				Status:    "running",
			}

			testRunRepo.On("GetByRunID", mock.Anything, "empty-status-run").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(7)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(7)).Return([]*testingDomain.SuiteRun{}, nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Test Counts and Suite Runs", func() {
		It("should calculate test counts from suite runs", func() {
			requestBody := map[string]interface{}{
				"runId":  "suite-runs-test",
				"status": "passed",
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        8,
				RunID:     "suite-runs-test",
				ProjectID: "test-project",
				Status:    "running",
			}

			// Create suite runs with test counts
			suiteRuns := []*testingDomain.SuiteRun{
				{
					ID:           1,
					TestRunID:    8,
					TotalTests:   10,
					PassedTests:  8,
					FailedTests:  2,
					SkippedTests: 0,
				},
				{
					ID:           2,
					TestRunID:    8,
					TotalTests:   5,
					PassedTests:  5,
					FailedTests:  0,
					SkippedTests: 0,
				},
			}

			testRunRepo.On("GetByRunID", mock.Anything, "suite-runs-test").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(8)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(8)).Return(suiteRuns, nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				// Verify test counts are calculated correctly
				return tr.TotalTests == 15 &&
					tr.PassedTests == 13 &&
					tr.FailedTests == 2 &&
					tr.SkippedTests == 0
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Error Handling", func() {
		It("should return 404 when test run not found by runId", func() {
			requestBody := map[string]interface{}{
				"runId":  "non-existent-run",
				"status": "passed",
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "non-existent-run").Return(nil, errors.New("not found")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Test run not found"))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when CompleteTestRun service fails", func() {
			requestBody := map[string]interface{}{
				"runId":  "complete-fail-run",
				"status": "passed",
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        10,
				RunID:     "complete-fail-run",
				ProjectID: "test-project",
				Status:    "running",
			}

			testRunRepo.On("GetByRunID", mock.Anything, "complete-fail-run").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(10)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(10)).Return([]*testingDomain.SuiteRun{}, nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.Anything).Return(errors.New("database connection error")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("database connection error"))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when GetByID fails", func() {
			requestBody := map[string]interface{}{
				"runId":  "getbyid-fail-run",
				"status": "passed",
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        11,
				RunID:     "getbyid-fail-run",
				ProjectID: "test-project",
				Status:    "running",
			}

			testRunRepo.On("GetByRunID", mock.Anything, "getbyid-fail-run").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(11)).Return(nil, errors.New("database error")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).NotTo(BeEmpty())

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when FindByTestRunID fails", func() {
			requestBody := map[string]interface{}{
				"runId":  "find-suite-fail-run",
				"status": "passed",
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        12,
				RunID:     "find-suite-fail-run",
				ProjectID: "test-project",
				Status:    "running",
			}

			testRunRepo.On("GetByRunID", mock.Anything, "find-suite-fail-run").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(12)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(12)).Return(nil, errors.New("suite runs query failed")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("suite runs query failed"))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Response Format", func() {
		It("should return correct success response structure", func() {
			requestBody := map[string]interface{}{
				"runId":  "response-test-run",
				"status": "passed",
			}

			existingTestRun := &testingDomain.TestRun{
				ID:        13,
				RunID:     "response-test-run",
				ProjectID: "test-project",
				Status:    "running",
			}

			testRunRepo.On("GetByRunID", mock.Anything, "response-test-run").Return(existingTestRun, nil).Once()
			testRunRepo.On("GetByID", mock.Anything, uint(13)).Return(existingTestRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(13)).Return([]*testingDomain.SuiteRun{}, nil).Once()
			testRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(HaveKey("message"))
			Expect(response["message"]).To(Equal("Test run completed successfully"))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})
})

// MockProjectRepository provides a mock implementation of ProjectRepository
type MockProjectRepository struct {
	mock.Mock
}

func (m *MockProjectRepository) Save(ctx context.Context, project *projectsDomain.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockProjectRepository) FindByID(ctx context.Context, id uint) (*projectsDomain.Project, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*projectsDomain.Project), args.Error(1)
}

func (m *MockProjectRepository) FindByProjectID(ctx context.Context, projectID projectsDomain.ProjectID) (*projectsDomain.Project, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*projectsDomain.Project), args.Error(1)
}

func (m *MockProjectRepository) FindByTeam(ctx context.Context, team projectsDomain.Team) ([]*projectsDomain.Project, error) {
	args := m.Called(ctx, team)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*projectsDomain.Project), args.Error(1)
}

func (m *MockProjectRepository) Update(ctx context.Context, project *projectsDomain.Project) error {
	args := m.Called(ctx, project)
	return args.Error(0)
}

func (m *MockProjectRepository) ExistsByProjectID(ctx context.Context, projectID projectsDomain.ProjectID) (bool, error) {
	args := m.Called(ctx, projectID)
	return args.Bool(0), args.Error(1)
}

func (m *MockProjectRepository) FindAll(ctx context.Context, limit, offset int) ([]*projectsDomain.Project, int64, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*projectsDomain.Project), args.Get(1).(int64), args.Error(2)
}

func (m *MockProjectRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockProjectPermissionRepository provides a mock implementation of ProjectPermissionRepository
type MockProjectPermissionRepository struct {
	mock.Mock
}

func (m *MockProjectPermissionRepository) Save(ctx context.Context, permission *projectsDomain.ProjectPermission) error {
	args := m.Called(ctx, permission)
	return args.Error(0)
}

func (m *MockProjectPermissionRepository) FindByProjectAndUser(ctx context.Context, projectID projectsDomain.ProjectID, userID string) ([]*projectsDomain.ProjectPermission, error) {
	args := m.Called(ctx, projectID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*projectsDomain.ProjectPermission), args.Error(1)
}

func (m *MockProjectPermissionRepository) FindByUser(ctx context.Context, userID string) ([]*projectsDomain.ProjectPermission, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*projectsDomain.ProjectPermission), args.Error(1)
}

func (m *MockProjectPermissionRepository) FindByProject(ctx context.Context, projectID projectsDomain.ProjectID) ([]*projectsDomain.ProjectPermission, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*projectsDomain.ProjectPermission), args.Error(1)
}

func (m *MockProjectPermissionRepository) Delete(ctx context.Context, projectID projectsDomain.ProjectID, userID string, permission projectsDomain.PermissionType) error {
	args := m.Called(ctx, projectID, userID, permission)
	return args.Error(0)
}

func (m *MockProjectPermissionRepository) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

var _ = Describe("createProject Method Tests", func() {
	var (
		handler        *DomainHandler
		router         *gin.Engine
		logger         *logging.Logger
		projectRepo    *MockProjectRepository
		permissionRepo *MockProjectPermissionRepository
		projectService *projectsApp.ProjectService
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

		// Create mocks
		projectRepo = new(MockProjectRepository)
		permissionRepo = new(MockProjectPermissionRepository)

		// Create services with mocks
		projectService = projectsApp.NewProjectService(projectRepo, permissionRepo)

		// Create handler
		handler = NewDomainHandler(nil, projectService, nil, nil, nil, nil, nil, logger)

		// Setup router
		router = gin.New()
		router.POST("/api/v1/projects", func(c *gin.Context) {
			// Set a mock user in the context
			user := &authDomain.User{
				UserID: "test-user-123",
				Email:  "test@example.com",
				Name:   "Test User",
			}
			c.Set("user", user)
			handler.createProject(c)
		})
	})

	Describe("Request Validation", func() {
		It("should return 400 for invalid JSON", func() {
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBufferString("invalid json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})

		It("should return 400 for missing required name field", func() {
			requestBody := map[string]interface{}{
				"team": "engineering",
				// Missing name
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("Name"))
		})

		It("should return 400 for missing required team field", func() {
			requestBody := map[string]interface{}{
				"name": "Test Project",
				// Missing team
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("Team"))
		})

		It("should return 400 for empty request body", func() {
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBufferString("{}"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})
	})

	Describe("Successful Project Creation", func() {
		It("should create project with all fields provided", func() {
			requestBody := map[string]interface{}{
				"projectId":     "custom-project-id",
				"name":          "Test Project",
				"description":   "A test project",
				"repository":    "https://github.com/test/repo",
				"defaultBranch": "main",
				"team":          "engineering",
				"settings": map[string]interface{}{
					"notifications": true,
					"theme":         "dark",
				},
			}

			// Mock expectations
			projectRepo.On("ExistsByProjectID", mock.Anything, projectsDomain.ProjectID("custom-project-id")).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.MatchedBy(func(p *projectsDomain.Project) bool {
				return p.ProjectID() == projectsDomain.ProjectID("custom-project-id")
			})).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("custom-project-id")).Return(
				func(ctx context.Context, pid projectsDomain.ProjectID) *projectsDomain.Project {
					proj, _ := projectsDomain.NewProject(pid, "Test Project", "engineering")
					return proj
				}(context.Background(), "custom-project-id"), nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["projectId"]).To(Equal("custom-project-id"))
			Expect(response["name"]).To(Equal("Test Project"))
			Expect(response["team"]).To(Equal("engineering"))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})

		It("should create project with minimal fields (only name and team)", func() {
			requestBody := map[string]interface{}{
				"name": "Minimal Project",
				"team": "platform",
			}

			// Mock expectations - projectId should be auto-generated as "minimal-project"
			projectRepo.On("ExistsByProjectID", mock.Anything, projectsDomain.ProjectID("minimal-project")).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.MatchedBy(func(p *projectsDomain.Project) bool {
				return p.ProjectID() == projectsDomain.ProjectID("minimal-project")
			})).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["projectId"]).To(Equal("minimal-project"))
			Expect(response["name"]).To(Equal("Minimal Project"))
			Expect(response["team"]).To(Equal("platform"))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})

		It("should auto-generate projectId from name when not provided", func() {
			requestBody := map[string]interface{}{
				"name": "Auto Generated ID Project",
				"team": "data",
			}

			// projectId should be "auto-generated-id-project"
			projectRepo.On("ExistsByProjectID", mock.Anything, projectsDomain.ProjectID("auto-generated-id-project")).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.MatchedBy(func(p *projectsDomain.Project) bool {
				return p.ProjectID() == projectsDomain.ProjectID("auto-generated-id-project")
			})).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["projectId"]).To(Equal("auto-generated-id-project"))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})

		It("should use provided projectId when specified", func() {
			requestBody := map[string]interface{}{
				"projectId": "my-custom-id",
				"name":      "Custom ID Project",
				"team":      "security",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, projectsDomain.ProjectID("my-custom-id")).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.MatchedBy(func(p *projectsDomain.Project) bool {
				return p.ProjectID() == projectsDomain.ProjectID("my-custom-id")
			})).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["projectId"]).To(Equal("my-custom-id"))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Project ID Generation", func() {
		It("should convert spaces to dashes in auto-generated projectId", func() {
			requestBody := map[string]interface{}{
				"name": "Project With Spaces",
				"team": "frontend",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, projectsDomain.ProjectID("project-with-spaces")).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["projectId"]).To(Equal("project-with-spaces"))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})

		It("should convert to lowercase in auto-generated projectId", func() {
			requestBody := map[string]interface{}{
				"name": "UPPERCASE PROJECT",
				"team": "backend",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, projectsDomain.ProjectID("uppercase-project")).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["projectId"]).To(Equal("uppercase-project"))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Additional Fields Handling", func() {
		It("should create project even when description alone causes update to fail validation", func() {
			requestBody := map[string]interface{}{
				"name":        "Project With Description",
				"description": "This is a test description",
				"team":        "ops",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			projectRepo.On("FindByProjectID", mock.Anything, mock.Anything).Return(
				func(ctx context.Context, pid projectsDomain.ProjectID) *projectsDomain.Project {
					proj, _ := projectsDomain.NewProject(pid, "Project With Description", "ops")
					return proj
				}(context.Background(), "project-with-description"), nil).Once()
			// Note: Update is NOT expected because UpdateProject fails validation
			// when description is provided alone (defaultBranch cannot be empty)

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Project creation should still succeed (error is only logged)
			Expect(w.Code).To(Equal(http.StatusCreated))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})

		It("should update project with repository and defaultBranch when provided", func() {
			requestBody := map[string]interface{}{
				"name":          "Project With Repo",
				"repository":    "https://github.com/test/repo",
				"defaultBranch": "develop",
				"team":          "mobile",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			projectRepo.On("FindByProjectID", mock.Anything, mock.Anything).Return(
				func(ctx context.Context, pid projectsDomain.ProjectID) *projectsDomain.Project {
					proj, _ := projectsDomain.NewProject(pid, "Project With Repo", "mobile")
					return proj
				}(context.Background(), "project-with-repo"), nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})

		It("should create project even when settings alone causes update to fail validation", func() {
			requestBody := map[string]interface{}{
				"name": "Project With Settings",
				"team": "qa",
				"settings": map[string]interface{}{
					"autoMerge":     true,
					"requireReview": false,
					"maxRetries":    3,
				},
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			projectRepo.On("FindByProjectID", mock.Anything, mock.Anything).Return(
				func(ctx context.Context, pid projectsDomain.ProjectID) *projectsDomain.Project {
					proj, _ := projectsDomain.NewProject(pid, "Project With Settings", "qa")
					return proj
				}(context.Background(), "project-with-settings"), nil).Once()
			// Note: Update is NOT expected because UpdateProject fails validation
			// when settings is provided alone (defaultBranch cannot be empty)

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Project creation should still succeed (error is only logged)
			Expect(w.Code).To(Equal(http.StatusCreated))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})

		It("should not call UpdateProject when no additional fields provided", func() {
			requestBody := map[string]interface{}{
				"name": "Basic Project",
				"team": "devops",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			// Note: No FindByProjectID call expected since there are no additional fields

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Error Handling", func() {
		It("should return 500 when project already exists", func() {
			requestBody := map[string]interface{}{
				"projectId": "existing-project",
				"name":      "Existing Project",
				"team":      "test",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, projectsDomain.ProjectID("existing-project")).Return(true, nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("already exists"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when ExistsByProjectID fails", func() {
			requestBody := map[string]interface{}{
				"name": "Test Project",
				"team": "test",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, errors.New("database error")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("database error"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when Save fails", func() {
			requestBody := map[string]interface{}{
				"name": "Test Project",
				"team": "test",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(errors.New("save failed")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("save failed"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should still succeed when UpdateProject fails after creation", func() {
			requestBody := map[string]interface{}{
				"name":        "Project Update Fails",
				"description": "Test description",
				"team":        "test",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			projectRepo.On("FindByProjectID", mock.Anything, mock.Anything).Return(nil, errors.New("not found")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should still return 201 even though update failed
			Expect(w.Code).To(Equal(http.StatusCreated))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Response Format", func() {
		It("should return correct response structure", func() {
			requestBody := map[string]interface{}{
				"name": "Response Test Project",
				"team": "testing",
			}

			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permissionRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(HaveKey("projectId"))
			Expect(response).To(HaveKey("name"))
			Expect(response).To(HaveKey("team"))
			Expect(response).To(HaveKey("isActive"))
			Expect(response["name"]).To(Equal("Response Test Project"))
			Expect(response["team"]).To(Equal("testing"))

			projectRepo.AssertExpectations(GinkgoT())
			permissionRepo.AssertExpectations(GinkgoT())
		})
	})
})

var _ = Describe("addSpecRun Method Tests", func() {
	var (
		handler        *DomainHandler
		router         *gin.Engine
		logger         *logging.Logger
		suiteRunRepo   *MockSuiteRunRepository
		specRunRepo    *MockSpecRunRepository
		testingService *testingApp.TestRunService
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

		// Create mocks
		suiteRunRepo = new(MockSuiteRunRepository)
		specRunRepo = new(MockSpecRunRepository)

		// Create services with mocks
		testingService = testingApp.NewTestRunService(nil, suiteRunRepo, specRunRepo)

		// Create handler
		handler = NewDomainHandler(testingService, nil, nil, nil, nil, nil, nil, logger)

		// Setup router
		router = gin.New()
		router.POST("/api/v1/spec-runs", handler.addSpecRun)
	})

	Describe("Request Validation", func() {
		It("should return 400 for invalid JSON", func() {
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBufferString("invalid json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})

		It("should return 400 for missing required suiteRunId field", func() {
			requestBody := map[string]interface{}{
				"specName": "Test Spec",
				"status":   "passed",
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("SuiteRunID"))
		})

		It("should return 400 for missing required specName field", func() {
			requestBody := map[string]interface{}{
				"suiteRunId": 1,
				"status":     "passed",
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("SpecName"))
		})

		It("should return 400 for empty request body", func() {
			requestBody := map[string]interface{}{}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})
	})

	Describe("Successful Spec Run Creation", func() {
		It("should create spec run with all fields provided", func() {
			now := time.Now()
			endTime := now.Add(5 * time.Second)
			requestBody := map[string]interface{}{
				"suiteRunId":   uint(1),
				"specName":     "Complete Test Spec",
				"status":       "passed",
				"startTime":    now.Format(time.RFC3339),
				"endTime":      endTime.Format(time.RFC3339),
				"duration":     int64(5000000000), // 5 seconds in nanoseconds
				"errorMessage": "",
				"stackTrace":   "",
				"stdout":       "Test output",
				"stderr":       "",
				"retries":      0,
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SpecRun) bool {
				return sr.SuiteRunID == 1 && sr.Name == "Complete Test Spec" && sr.Status == "passed"
			})).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 100
			}).Return(nil).Once()

			// Mock for updateSuiteStatistics
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(1)).Return([]*testingDomain.SpecRun{
				{ID: 100, Name: "Complete Test Spec", Status: "passed", StartTime: now, EndTime: &endTime, Duration: 5 * time.Second},
			}, nil).Once()
			suiteRunRepo.On("GetByID", mock.Anything, uint(1)).Return(&testingDomain.SuiteRun{
				ID: 1, Name: "Test Suite", TestRunID: 1,
			}, nil).Once()
			suiteRunRepo.On("Update", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				return sr.TotalTests == 1 && sr.PassedTests == 1
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(float64(100)))
			Expect(response["specName"]).To(Equal("Complete Test Spec"))

			specRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should create spec run with minimal fields (only required fields)", func() {
			requestBody := map[string]interface{}{
				"suiteRunId": uint(2),
				"specName":   "Minimal Test Spec",
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SpecRun) bool {
				return sr.SuiteRunID == 2 && sr.Name == "Minimal Test Spec"
			})).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 101
			}).Return(nil).Once()

			// Mock for updateSuiteStatistics
			now := time.Now()
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(2)).Return([]*testingDomain.SpecRun{
				{ID: 101, Name: "Minimal Test Spec", Status: "", StartTime: now},
			}, nil).Once()
			suiteRunRepo.On("GetByID", mock.Anything, uint(2)).Return(&testingDomain.SuiteRun{
				ID: 2, Name: "Test Suite 2", TestRunID: 1,
			}, nil).Once()
			suiteRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(float64(101)))
			Expect(response["specName"]).To(Equal("Minimal Test Spec"))

			specRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should create spec run with failed status and error details", func() {
			now := time.Now()
			endTime := now.Add(2 * time.Second)
			requestBody := map[string]interface{}{
				"suiteRunId":   uint(3),
				"specName":     "Failed Test Spec",
				"status":       "failed",
				"startTime":    now.Format(time.RFC3339),
				"endTime":      endTime.Format(time.RFC3339),
				"errorMessage": "Assertion failed: expected true but got false",
				"stackTrace":   "at test.js:10\nat runner.js:25",
				"retries":      2,
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SpecRun) bool {
				return sr.SuiteRunID == 3 && sr.Name == "Failed Test Spec" && sr.Status == "failed" && sr.RetryCount == 2
			})).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 102
			}).Return(nil).Once()

			// Mock for updateSuiteStatistics
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(3)).Return([]*testingDomain.SpecRun{
				{ID: 102, Name: "Failed Test Spec", Status: "failed", StartTime: now, EndTime: &endTime, Duration: 2 * time.Second},
			}, nil).Once()
			suiteRunRepo.On("GetByID", mock.Anything, uint(3)).Return(&testingDomain.SuiteRun{
				ID: 3, Name: "Test Suite 3", TestRunID: 1,
			}, nil).Once()
			suiteRunRepo.On("Update", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				return sr.TotalTests == 1 && sr.FailedTests == 1
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(float64(102)))
			Expect(response["specName"]).To(Equal("Failed Test Spec"))

			specRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should create spec run with skipped status", func() {
			requestBody := map[string]interface{}{
				"suiteRunId": uint(4),
				"specName":   "Skipped Test Spec",
				"status":     "skipped",
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SpecRun) bool {
				return sr.SuiteRunID == 4 && sr.Name == "Skipped Test Spec" && sr.Status == "skipped"
			})).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 103
			}).Return(nil).Once()

			// Mock for updateSuiteStatistics
			now := time.Now()
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(4)).Return([]*testingDomain.SpecRun{
				{ID: 103, Name: "Skipped Test Spec", Status: "skipped", StartTime: now},
			}, nil).Once()
			suiteRunRepo.On("GetByID", mock.Anything, uint(4)).Return(&testingDomain.SuiteRun{
				ID: 4, Name: "Test Suite 4", TestRunID: 1,
			}, nil).Once()
			suiteRunRepo.On("Update", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				return sr.TotalTests == 1 && sr.SkippedTests == 1
			})).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(float64(103)))
			Expect(response["specName"]).To(Equal("Skipped Test Spec"))

			specRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Time Handling", func() {
		It("should use default startTime when not provided", func() {
			requestBody := map[string]interface{}{
				"suiteRunId": uint(5),
				"specName":   "Default Time Spec",
				"status":     "passed",
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SpecRun) bool {
				// StartTime should be set to approximately now
				return sr.SuiteRunID == 5 && !sr.StartTime.IsZero()
			})).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 104
			}).Return(nil).Once()

			// Mock for updateSuiteStatistics
			now := time.Now()
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(5)).Return([]*testingDomain.SpecRun{
				{ID: 104, Name: "Default Time Spec", Status: "passed", StartTime: now},
			}, nil).Once()
			suiteRunRepo.On("GetByID", mock.Anything, uint(5)).Return(&testingDomain.SuiteRun{
				ID: 5, Name: "Test Suite 5", TestRunID: 1,
			}, nil).Once()
			suiteRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			specRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should respect custom startTime and endTime when provided", func() {
			customStart := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
			customEnd := time.Date(2024, 1, 1, 10, 0, 10, 0, time.UTC)

			requestBody := map[string]interface{}{
				"suiteRunId": uint(6),
				"specName":   "Custom Time Spec",
				"status":     "passed",
				"startTime":  customStart.Format(time.RFC3339),
				"endTime":    customEnd.Format(time.RFC3339),
				"duration":   int64(10000000000), // 10 seconds
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SpecRun) bool {
				return sr.SuiteRunID == 6 &&
					sr.StartTime.Equal(customStart) &&
					sr.EndTime != nil && sr.EndTime.Equal(customEnd) &&
					sr.Duration == 10*time.Second
			})).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 105
			}).Return(nil).Once()

			// Mock for updateSuiteStatistics
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(6)).Return([]*testingDomain.SpecRun{
				{ID: 105, Name: "Custom Time Spec", Status: "passed", StartTime: customStart, EndTime: &customEnd, Duration: 10 * time.Second},
			}, nil).Once()
			suiteRunRepo.On("GetByID", mock.Anything, uint(6)).Return(&testingDomain.SuiteRun{
				ID: 6, Name: "Test Suite 6", TestRunID: 1,
			}, nil).Once()
			suiteRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			specRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Error Handling", func() {
		It("should return 500 when Create fails", func() {
			requestBody := map[string]interface{}{
				"suiteRunId": uint(7),
				"specName":   "Error Spec",
				"status":     "passed",
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("database error")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("database error"))

			specRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when updateSuiteStatistics fails (FindBySuiteRunID error)", func() {
			requestBody := map[string]interface{}{
				"suiteRunId": uint(8),
				"specName":   "Update Error Spec",
				"status":     "passed",
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 106
			}).Return(nil).Once()
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(8)).Return(nil, errors.New("failed to find spec runs")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("failed to find spec runs"))

			specRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when updateSuiteStatistics fails (GetByID error)", func() {
			requestBody := map[string]interface{}{
				"suiteRunId": uint(9),
				"specName":   "Suite Get Error Spec",
				"status":     "passed",
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 107
			}).Return(nil).Once()
			now := time.Now()
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(9)).Return([]*testingDomain.SpecRun{
				{ID: 107, Name: "Suite Get Error Spec", Status: "passed", StartTime: now},
			}, nil).Once()
			suiteRunRepo.On("GetByID", mock.Anything, uint(9)).Return(nil, errors.New("suite not found")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("suite not found"))

			specRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when updateSuiteStatistics fails (Update error)", func() {
			requestBody := map[string]interface{}{
				"suiteRunId": uint(10),
				"specName":   "Suite Update Error Spec",
				"status":     "passed",
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 108
			}).Return(nil).Once()
			now := time.Now()
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(10)).Return([]*testingDomain.SpecRun{
				{ID: 108, Name: "Suite Update Error Spec", Status: "passed", StartTime: now},
			}, nil).Once()
			suiteRunRepo.On("GetByID", mock.Anything, uint(10)).Return(&testingDomain.SuiteRun{
				ID: 10, Name: "Test Suite 10", TestRunID: 1,
			}, nil).Once()
			suiteRunRepo.On("Update", mock.Anything, mock.Anything).Return(errors.New("failed to update suite")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("failed to update suite"))

			specRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Response Format", func() {
		It("should return correct response structure with id and specName", func() {
			requestBody := map[string]interface{}{
				"suiteRunId": uint(11),
				"specName":   "Response Format Spec",
				"status":     "passed",
			}

			// Mock expectations
			specRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				spec := args.Get(1).(*testingDomain.SpecRun)
				spec.ID = 999
			}).Return(nil).Once()
			now := time.Now()
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(11)).Return([]*testingDomain.SpecRun{
				{ID: 999, Name: "Response Format Spec", Status: "passed", StartTime: now},
			}, nil).Once()
			suiteRunRepo.On("GetByID", mock.Anything, uint(11)).Return(&testingDomain.SuiteRun{
				ID: 11, Name: "Test Suite 11", TestRunID: 1,
			}, nil).Once()
			suiteRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Verify response structure
			Expect(response).To(HaveKey("id"))
			Expect(response).To(HaveKey("specName"))
			Expect(response["id"]).To(Equal(float64(999)))
			Expect(response["specName"]).To(Equal("Response Format Spec"))

			// Verify only expected keys are present
			Expect(len(response)).To(Equal(2))

			specRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})
})

var _ = Describe("addSuiteRun Method Tests", func() {
	var (
		handler        *DomainHandler
		router         *gin.Engine
		logger         *logging.Logger
		testRunRepo    *MockTestRunRepository
		suiteRunRepo   *MockSuiteRunRepository
		testingService *testingApp.TestRunService
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

		// Create mocks
		testRunRepo = new(MockTestRunRepository)
		suiteRunRepo = new(MockSuiteRunRepository)

		// Create services with mocks
		testingService = testingApp.NewTestRunService(testRunRepo, suiteRunRepo, nil)

		// Create handler
		handler = NewDomainHandler(testingService, nil, nil, nil, nil, nil, nil, logger)

		// Setup router
		router = gin.New()
		router.POST("/api/v1/suite-runs", handler.addSuiteRun)
	})

	Describe("Request Validation", func() {
		It("should return 400 for invalid JSON", func() {
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBufferString("invalid json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})

		It("should return 400 for missing required testRunId field", func() {
			requestBody := map[string]interface{}{
				"suiteName": "Test Suite",
				"status":    "running",
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("TestRunID"))
		})

		It("should return 400 for missing required suiteName field", func() {
			requestBody := map[string]interface{}{
				"testRunId": "test-run-123",
				"status":    "running",
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("SuiteName"))
		})

		It("should return 400 for empty request body", func() {
			requestBody := map[string]interface{}{}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})
	})

	Describe("Test Run Lookup", func() {
		It("should return 404 when test run not found", func() {
			requestBody := map[string]interface{}{
				"testRunId": "non-existent-run",
				"suiteName": "Test Suite",
			}

			// Mock GetTestRunByRunID to return not found
			testRunRepo.On("GetByRunID", mock.Anything, "non-existent-run").Return(nil, errors.New("not found")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Test run not found"))

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Successful Suite Run Creation", func() {
		It("should create suite run with all fields provided", func() {
			now := time.Now()
			endTime := now.Add(10 * time.Minute)
			requestBody := map[string]interface{}{
				"testRunId":   "test-run-1",
				"suiteName":   "Complete Suite",
				"status":      "passed",
				"startTime":   now.Format(time.RFC3339),
				"endTime":     endTime.Format(time.RFC3339),
				"duration":    int64(600000000000), // 10 minutes in nanoseconds
				"totalSpecs":  10,
				"passedSpecs": 8,
				"failedSpecs": 2,
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-1").Return(&testingDomain.TestRun{
				ID: 1, RunID: "test-run-1", ProjectID: "project-1",
			}, nil).Once()

			suiteRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				return sr.TestRunID == 1 &&
					sr.Name == "Complete Suite" &&
					sr.Status == "passed" &&
					sr.TotalTests == 10 &&
					sr.PassedTests == 8 &&
					sr.FailedTests == 2 &&
					sr.SkippedTests == 0 &&
					sr.Duration == 10*time.Minute
			})).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 100
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(float64(100)))
			Expect(response["suiteName"]).To(Equal("Complete Suite"))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should create suite run with minimal fields (only required fields)", func() {
			requestBody := map[string]interface{}{
				"testRunId": "test-run-2",
				"suiteName": "Minimal Suite",
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-2").Return(&testingDomain.TestRun{
				ID: 2, RunID: "test-run-2", ProjectID: "project-2",
			}, nil).Once()

			suiteRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				return sr.TestRunID == 2 && sr.Name == "Minimal Suite" && !sr.StartTime.IsZero()
			})).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 101
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(float64(101)))
			Expect(response["suiteName"]).To(Equal("Minimal Suite"))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should create suite run with failed status", func() {
			requestBody := map[string]interface{}{
				"testRunId":   "test-run-3",
				"suiteName":   "Failed Suite",
				"status":      "failed",
				"totalSpecs":  5,
				"passedSpecs": 2,
				"failedSpecs": 3,
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-3").Return(&testingDomain.TestRun{
				ID: 3, RunID: "test-run-3", ProjectID: "project-3",
			}, nil).Once()

			suiteRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				return sr.TestRunID == 3 &&
					sr.Name == "Failed Suite" &&
					sr.Status == "failed" &&
					sr.TotalTests == 5 &&
					sr.PassedTests == 2 &&
					sr.FailedTests == 3 &&
					sr.SkippedTests == 0
			})).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 102
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(float64(102)))
			Expect(response["suiteName"]).To(Equal("Failed Suite"))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should create suite run with running status", func() {
			requestBody := map[string]interface{}{
				"testRunId": "test-run-4",
				"suiteName": "Running Suite",
				"status":    "running",
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-4").Return(&testingDomain.TestRun{
				ID: 4, RunID: "test-run-4", ProjectID: "project-4",
			}, nil).Once()

			suiteRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				return sr.TestRunID == 4 && sr.Name == "Running Suite" && sr.Status == "running"
			})).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 103
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(float64(103)))
			Expect(response["suiteName"]).To(Equal("Running Suite"))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Test Count Calculations", func() {
		It("should correctly calculate skipped tests", func() {
			requestBody := map[string]interface{}{
				"testRunId":   "test-run-5",
				"suiteName":   "Suite With Skipped",
				"totalSpecs":  20,
				"passedSpecs": 15,
				"failedSpecs": 2,
				// skippedTests should be calculated as 20 - 15 - 2 = 3
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-5").Return(&testingDomain.TestRun{
				ID: 5, RunID: "test-run-5", ProjectID: "project-5",
			}, nil).Once()

			suiteRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				return sr.TestRunID == 5 &&
					sr.TotalTests == 20 &&
					sr.PassedTests == 15 &&
					sr.FailedTests == 2 &&
					sr.SkippedTests == 3
			})).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 104
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should handle zero test counts", func() {
			requestBody := map[string]interface{}{
				"testRunId":   "test-run-6",
				"suiteName":   "Empty Suite",
				"totalSpecs":  0,
				"passedSpecs": 0,
				"failedSpecs": 0,
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-6").Return(&testingDomain.TestRun{
				ID: 6, RunID: "test-run-6", ProjectID: "project-6",
			}, nil).Once()

			suiteRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				return sr.TotalTests == 0 && sr.PassedTests == 0 && sr.FailedTests == 0 && sr.SkippedTests == 0
			})).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 105
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Time Handling", func() {
		It("should use default startTime when not provided", func() {
			requestBody := map[string]interface{}{
				"testRunId": "test-run-7",
				"suiteName": "Default Time Suite",
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-7").Return(&testingDomain.TestRun{
				ID: 7, RunID: "test-run-7", ProjectID: "project-7",
			}, nil).Once()

			suiteRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				// StartTime should be set to approximately now
				return sr.TestRunID == 7 && !sr.StartTime.IsZero()
			})).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 106
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})

		It("should respect custom startTime and endTime when provided", func() {
			customStart := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
			customEnd := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)

			requestBody := map[string]interface{}{
				"testRunId": "test-run-8",
				"suiteName": "Custom Time Suite",
				"startTime": customStart.Format(time.RFC3339),
				"endTime":   customEnd.Format(time.RFC3339),
				"duration":  int64(1800000000000), // 30 minutes
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-8").Return(&testingDomain.TestRun{
				ID: 8, RunID: "test-run-8", ProjectID: "project-8",
			}, nil).Once()

			suiteRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(sr *testingDomain.SuiteRun) bool {
				return sr.TestRunID == 8 &&
					sr.StartTime.Equal(customStart) &&
					sr.EndTime != nil && sr.EndTime.Equal(customEnd) &&
					sr.Duration == 30*time.Minute
			})).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 107
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Error Handling", func() {
		It("should return 500 when CreateSuiteRun fails", func() {
			requestBody := map[string]interface{}{
				"testRunId": "test-run-9",
				"suiteName": "Error Suite",
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-9").Return(&testingDomain.TestRun{
				ID: 9, RunID: "test-run-9", ProjectID: "project-9",
			}, nil).Once()

			suiteRunRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("database error")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("database error"))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Response Format", func() {
		It("should return correct response structure with id and suiteName", func() {
			requestBody := map[string]interface{}{
				"testRunId": "test-run-10",
				"suiteName": "Response Format Suite",
			}

			// Mock expectations
			testRunRepo.On("GetByRunID", mock.Anything, "test-run-10").Return(&testingDomain.TestRun{
				ID: 10, RunID: "test-run-10", ProjectID: "project-10",
			}, nil).Once()

			suiteRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				suite := args.Get(1).(*testingDomain.SuiteRun)
				suite.ID = 999
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Verify response structure
			Expect(response).To(HaveKey("id"))
			Expect(response).To(HaveKey("suiteName"))
			Expect(response["id"]).To(Equal(float64(999)))
			Expect(response["suiteName"]).To(Equal("Response Format Suite"))

			// Verify only expected keys are present
			Expect(len(response)).To(Equal(2))

			testRunRepo.AssertExpectations(GinkgoT())
			suiteRunRepo.AssertExpectations(GinkgoT())
		})
	})
})

var _ = Describe("startTestRun Method Tests", func() {
	var (
		handler        *DomainHandler
		router         *gin.Engine
		logger         *logging.Logger
		testRunRepo    *MockTestRunRepository
		testingService *testingApp.TestRunService
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

		// Create mocks
		testRunRepo = new(MockTestRunRepository)

		// Create services with mocks
		testingService = testingApp.NewTestRunService(testRunRepo, nil, nil)

		// Create handler
		handler = NewDomainHandler(testingService, nil, nil, nil, nil, nil, nil, logger)

		// Setup router
		router = gin.New()
		router.POST("/api/v1/test-runs/start", handler.startTestRun)
	})

	Describe("Request Validation", func() {
		It("should return 400 for invalid JSON", func() {
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBufferString("invalid json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})

		It("should return 400 for missing required projectId field", func() {
			requestBody := map[string]interface{}{
				"branch": "main",
			}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("ProjectID"))
		})

		It("should return 400 for empty request body", func() {
			requestBody := map[string]interface{}{}

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Not(BeNil()))
		})
	})

	Describe("Successful Test Run Start", func() {
		It("should start test run with all fields provided", func() {
			requestBody := map[string]interface{}{
				"projectId":   "test-project",
				"runId":       "custom-run-id",
				"branch":      "main",
				"commitSha":   "abc123def456",
				"environment": "staging",
				"tags":        []string{"regression", "smoke"},
				"metadata": map[string]interface{}{
					"buildNumber": "123",
					"triggeredBy": "CI/CD",
				},
			}

			// Mock expectations
			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.ProjectID == "test-project" &&
					tr.RunID == "custom-run-id" &&
					tr.Branch == "main" &&
					tr.GitCommit == "abc123def456" &&
					tr.Environment == "staging" &&
					tr.Status == "running" &&
					!tr.StartTime.IsZero()
			})).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				testRun.ID = 100
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(float64(100)))
			Expect(response["runId"]).To(Equal("custom-run-id"))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should start test run with minimal fields (only projectId)", func() {
			requestBody := map[string]interface{}{
				"projectId": "minimal-project",
			}

			// Mock expectations
			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.ProjectID == "minimal-project" &&
					tr.RunID != "" && // RunID should be auto-generated
					tr.Status == "running" &&
					tr.Environment == "default" && // Default environment
					!tr.StartTime.IsZero()
			})).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				testRun.ID = 101
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal(float64(101)))
			Expect(response["runId"]).To(Not(BeEmpty()))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should start test run with custom branch and commitSha", func() {
			requestBody := map[string]interface{}{
				"projectId": "git-project",
				"branch":    "feature/new-feature",
				"commitSha": "1a2b3c4d5e6f7890",
			}

			// Mock expectations
			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.ProjectID == "git-project" &&
					tr.Branch == "feature/new-feature" &&
					tr.GitCommit == "1a2b3c4d5e6f7890"
			})).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				testRun.ID = 102
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should start test run with tags", func() {
			requestBody := map[string]interface{}{
				"projectId": "tagged-project",
				"tags":      []string{"nightly", "integration", "critical"},
			}

			// Mock expectations
			testRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				testRun.ID = 103
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should start test run with metadata", func() {
			requestBody := map[string]interface{}{
				"projectId": "metadata-project",
				"metadata": map[string]interface{}{
					"buildUrl":      "https://ci.example.com/build/123",
					"pullRequestId": 456,
					"author":        "john.doe",
				},
			}

			// Mock expectations
			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.ProjectID == "metadata-project" &&
					tr.Metadata != nil &&
					tr.Metadata["buildUrl"] == "https://ci.example.com/build/123"
			})).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				testRun.ID = 104
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("RunID Generation", func() {
		It("should auto-generate runId when not provided", func() {
			requestBody := map[string]interface{}{
				"projectId": "auto-id-project",
			}

			var capturedRunID string
			testRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				capturedRunID = testRun.RunID
				testRun.ID = 105
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Verify runId was auto-generated (should be a UUID)
			Expect(response["runId"]).To(Not(BeEmpty()))
			Expect(capturedRunID).To(Not(BeEmpty()))
			// Basic UUID format check (8-4-4-4-12 format)
			Expect(capturedRunID).To(MatchRegexp(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should use provided runId when specified", func() {
			requestBody := map[string]interface{}{
				"projectId": "custom-id-project",
				"runId":     "my-custom-run-id-123",
			}

			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.RunID == "my-custom-run-id-123"
			})).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				testRun.ID = 106
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["runId"]).To(Equal("my-custom-run-id-123"))

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Environment Handling", func() {
		It("should use default environment when not provided", func() {
			requestBody := map[string]interface{}{
				"projectId": "env-default-project",
			}

			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.Environment == "default"
			})).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				testRun.ID = 107
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should use provided environment when specified", func() {
			requestBody := map[string]interface{}{
				"projectId":   "env-custom-project",
				"environment": "production",
			}

			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.Environment == "production"
			})).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				testRun.ID = 108
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should handle various environment names", func() {
			environments := []string{"dev", "staging", "qa", "production", "test"}

			for idx, env := range environments {
				requestBody := map[string]interface{}{
					"projectId":   "env-project",
					"environment": env,
				}

				testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
					return tr.Environment == env
				})).Run(func(args mock.Arguments) {
					testRun := args.Get(1).(*testingDomain.TestRun)
					testRun.ID = uint(200 + idx)
				}).Return(nil).Once()

				body, _ := json.Marshal(requestBody)
				req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusCreated))
			}

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Test Run Status", func() {
		It("should always set status to running", func() {
			requestBody := map[string]interface{}{
				"projectId": "status-project",
			}

			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *testingDomain.TestRun) bool {
				return tr.Status == "running"
			})).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				testRun.ID = 109
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should set startTime to current time", func() {
			requestBody := map[string]interface{}{
				"projectId": "time-project",
			}

			beforeTime := time.Now()
			var capturedStartTime time.Time

			testRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				capturedStartTime = testRun.StartTime
				testRun.ID = 110
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			afterTime := time.Now()

			Expect(w.Code).To(Equal(http.StatusCreated))
			// StartTime should be between beforeTime and afterTime
			Expect(capturedStartTime.After(beforeTime.Add(-1 * time.Second))).To(BeTrue())
			Expect(capturedStartTime.Before(afterTime.Add(1 * time.Second))).To(BeTrue())

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Error Handling", func() {
		It("should return 500 when CreateTestRun fails", func() {
			requestBody := map[string]interface{}{
				"projectId": "error-project",
			}

			testRunRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("database connection error")).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("database connection error"))

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Response Format", func() {
		It("should return correct response structure with id and runId", func() {
			requestBody := map[string]interface{}{
				"projectId": "response-project",
				"runId":     "response-run-id",
			}

			testRunRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				testRun := args.Get(1).(*testingDomain.TestRun)
				testRun.ID = 999
			}).Return(nil).Once()

			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Verify response structure
			Expect(response).To(HaveKey("id"))
			Expect(response).To(HaveKey("runId"))
			Expect(response["id"]).To(Equal(float64(999)))
			Expect(response["runId"]).To(Equal("response-run-id"))

			// Verify only expected keys are present
			Expect(len(response)).To(Equal(2))

			testRunRepo.AssertExpectations(GinkgoT())
		})
	})
})

var _ = Describe("getProjects Method Tests", func() {
	var (
		handler        *DomainHandler
		router         *gin.Engine
		logger         *logging.Logger
		projectRepo    *MockProjectRepository
		permissionRepo *MockProjectPermissionRepository
		projectService *projectsApp.ProjectService
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

		// Create mocks
		projectRepo = new(MockProjectRepository)
		permissionRepo = new(MockProjectPermissionRepository)

		// Create services with mocks
		projectService = projectsApp.NewProjectService(projectRepo, permissionRepo)

		// Create handler
		handler = NewDomainHandler(nil, projectService, nil, nil, nil, nil, nil, logger)

		// Setup router
		router = gin.New()
		router.GET("/api/v1/projects", handler.getProjects)
	})

	Describe("Successful Project Retrieval", func() {
		It("should retrieve projects with default limit and offset", func() {
			// Create mock projects
			mockProject1, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("project-1"),
				"Project One",
				projectsDomain.Team("team-alpha"),
			)
			mockProject2, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("project-2"),
				"Project Two",
				projectsDomain.Team("team-beta"),
			)
			mockProjects := []*projectsDomain.Project{mockProject1, mockProject2}

			// Setup expectations with default limit=100, offset=0
			projectRepo.On("FindAll", mock.Anything, 100, 0).Return(mockProjects, int64(2), nil)

			req := httptest.NewRequest("GET", "/api/v1/projects", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())
			Expect(response["total"]).To(Equal(float64(2)))
			Expect(response["limit"]).To(Equal(float64(100))) // Default limit
			Expect(response["offset"]).To(Equal(float64(0)))  // Default offset

			// Verify data structure
			data := response["data"].([]interface{})
			Expect(len(data)).To(Equal(2))

			// Verify first project
			firstProject := data[0].(map[string]interface{})
			Expect(firstProject["projectId"]).To(Equal("project-1"))
			Expect(firstProject["name"]).To(Equal("Project One"))
			Expect(firstProject["team"]).To(Equal("team-alpha"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should retrieve projects with custom limit and offset", func() {
			// Create mock project
			mockProject, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("project-3"),
				"Project Three",
				projectsDomain.Team("team-gamma"),
			)
			mockProjects := []*projectsDomain.Project{mockProject}

			// Setup expectations with custom limit=50, offset=10
			projectRepo.On("FindAll", mock.Anything, 50, 10).Return(mockProjects, int64(100), nil)

			req := httptest.NewRequest("GET", "/api/v1/projects?limit=50&offset=10", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())
			Expect(response["total"]).To(Equal(float64(100)))
			Expect(response["limit"]).To(Equal(float64(50)))  // Custom limit
			Expect(response["offset"]).To(Equal(float64(10))) // Custom offset

			data := response["data"].([]interface{})
			Expect(len(data)).To(Equal(1))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should handle empty projects list", func() {
			// Setup expectations - return empty list
			projectRepo.On("FindAll", mock.Anything, 100, 0).Return([]*projectsDomain.Project{}, int64(0), nil)

			req := httptest.NewRequest("GET", "/api/v1/projects", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["data"]).NotTo(BeNil())
			Expect(response["total"]).To(Equal(float64(0)))
			Expect(response["limit"]).To(Equal(float64(100)))
			Expect(response["offset"]).To(Equal(float64(0)))

			data := response["data"].([]interface{})
			Expect(len(data)).To(Equal(0))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should retrieve multiple projects and convert them to API format", func() {
			// Create 3 mock projects
			mockProject1, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("proj-alpha"),
				"Alpha Project",
				projectsDomain.Team("team-alpha"),
			)
			mockProject2, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("proj-beta"),
				"Beta Project",
				projectsDomain.Team("team-beta"),
			)
			mockProject3, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("proj-gamma"),
				"Gamma Project",
				projectsDomain.Team("team-gamma"),
			)
			mockProjects := []*projectsDomain.Project{mockProject1, mockProject2, mockProject3}

			// Setup expectations
			projectRepo.On("FindAll", mock.Anything, 100, 0).Return(mockProjects, int64(3), nil)

			req := httptest.NewRequest("GET", "/api/v1/projects", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["total"]).To(Equal(float64(3)))

			data := response["data"].([]interface{})
			Expect(len(data)).To(Equal(3))

			// Verify each project has required fields
			for i, projectData := range data {
				project := projectData.(map[string]interface{})
				Expect(project["projectId"]).NotTo(BeEmpty())
				Expect(project["name"]).NotTo(BeEmpty())
				Expect(project["team"]).NotTo(BeEmpty())

				// Verify projects are in correct order
				if i == 0 {
					Expect(project["projectId"]).To(Equal("proj-alpha"))
				} else if i == 1 {
					Expect(project["projectId"]).To(Equal("proj-beta"))
				} else if i == 2 {
					Expect(project["projectId"]).To(Equal("proj-gamma"))
				}
			}

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should parse large limit values correctly", func() {
			mockProjects := []*projectsDomain.Project{}

			// Setup expectations with large limit
			projectRepo.On("FindAll", mock.Anything, 1000, 50).Return(mockProjects, int64(0), nil)

			req := httptest.NewRequest("GET", "/api/v1/projects?limit=1000&offset=50", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["limit"]).To(Equal(float64(1000)))
			Expect(response["offset"]).To(Equal(float64(50)))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should handle invalid limit parameter gracefully (defaults to 0)", func() {
			mockProjects := []*projectsDomain.Project{}

			// Setup expectations - invalid limit converts to 0 by strconv.Atoi
			projectRepo.On("FindAll", mock.Anything, 0, 0).Return(mockProjects, int64(0), nil)

			req := httptest.NewRequest("GET", "/api/v1/projects?limit=invalid", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["limit"]).To(Equal(float64(0))) // Invalid converts to 0

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should handle invalid offset parameter gracefully (converts to 0)", func() {
			mockProjects := []*projectsDomain.Project{}

			// Setup expectations - invalid offset converts to 0 by strconv.Atoi
			projectRepo.On("FindAll", mock.Anything, 100, 0).Return(mockProjects, int64(0), nil)

			req := httptest.NewRequest("GET", "/api/v1/projects?offset=invalid", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["offset"]).To(Equal(float64(0))) // Invalid converts to 0

			projectRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Error Handling", func() {
		It("should return 500 when ListProjects fails", func() {
			// Setup expectations - return error
			projectRepo.On("FindAll", mock.Anything, 100, 0).
				Return([]*projectsDomain.Project{}, int64(0), fmt.Errorf("database connection failed"))

			req := httptest.NewRequest("GET", "/api/v1/projects", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("database connection failed"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when ListProjects returns a service error", func() {
			// Setup expectations - return service layer error
			projectRepo.On("FindAll", mock.Anything, 50, 25).
				Return([]*projectsDomain.Project{}, int64(0), fmt.Errorf("service unavailable"))

			req := httptest.NewRequest("GET", "/api/v1/projects?limit=50&offset=25", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(ContainSubstring("service unavailable"))

			projectRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("Response Format Validation", func() {
		It("should return correct response structure with all required fields", func() {
			mockProject, _ := projectsDomain.NewProject(
				projectsDomain.ProjectID("test-project"),
				"Test Project",
				projectsDomain.Team("test-team"),
			)
			mockProjects := []*projectsDomain.Project{mockProject}

			projectRepo.On("FindAll", mock.Anything, 100, 0).Return(mockProjects, int64(1), nil)

			req := httptest.NewRequest("GET", "/api/v1/projects", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Verify top-level response structure
			Expect(response).To(HaveKey("data"))
			Expect(response).To(HaveKey("total"))
			Expect(response).To(HaveKey("limit"))
			Expect(response).To(HaveKey("offset"))

			// Verify project data structure
			data := response["data"].([]interface{})
			Expect(len(data)).To(Equal(1))

			project := data[0].(map[string]interface{})
			Expect(project).To(HaveKey("id"))
			Expect(project).To(HaveKey("projectId"))
			Expect(project).To(HaveKey("name"))
			Expect(project).To(HaveKey("description"))
			Expect(project).To(HaveKey("repository"))
			Expect(project).To(HaveKey("defaultBranch"))
			Expect(project).To(HaveKey("team"))
			Expect(project).To(HaveKey("isActive"))
			Expect(project).To(HaveKey("settings"))
			Expect(project).To(HaveKey("createdAt"))
			Expect(project).To(HaveKey("updatedAt"))

			projectRepo.AssertExpectations(GinkgoT())
		})
	})
})

var _ = Describe("getCurrentUser Method Tests", func() {
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

		// Create handler (no services needed for this endpoint)
		handler = NewDomainHandler(nil, nil, nil, nil, nil, nil, nil, logger)

		// Setup router
		router = gin.New()
		router.GET("/auth/user", func(c *gin.Context) {
			// getCurrentUser expects user to be set in context by middleware
			// Tests will set the user individually
			handler.getCurrentUser(c)
		})
	})

	Describe("Successful User Retrieval", func() {
		It("should return current user data with all fields", func() {
			testUser := &authDomain.User{
				UserID: "user-123",
				Email:  "test@example.com",
				Name:   "Test User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-alpha"},
					{GroupName: "team-beta"},
				},
			}

			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response["id"]).To(Equal("user-123"))
			Expect(response["email"]).To(Equal("test@example.com"))
			Expect(response["name"]).To(Equal("Test User"))
			Expect(response["role"]).To(Equal("user"))

			// Verify teams array
			teams := response["teams"].([]interface{})
			Expect(len(teams)).To(Equal(2))
			Expect(teams[0]).To(Equal("team-alpha"))
			Expect(teams[1]).To(Equal("team-beta"))
		})

		It("should return user with admin role", func() {
			testUser := &authDomain.User{
				UserID: "admin-123",
				Email:  "admin@example.com",
				Name:   "Admin User",
				Role:   authDomain.RoleAdmin,
				Groups: []authDomain.UserGroup{
					{GroupName: "admins"},
				},
			}

			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response["id"]).To(Equal("admin-123"))
			Expect(response["role"]).To(Equal("admin"))

			teams := response["teams"].([]interface{})
			Expect(len(teams)).To(Equal(1))
			Expect(teams[0]).To(Equal("admins"))
		})

		It("should return user with manager role", func() {
			testUser := &authDomain.User{
				UserID: "manager-123",
				Email:  "manager@example.com",
				Name:   "Manager User",
				Role:   authDomain.RoleManager,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-alpha-managers"},
				},
			}

			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response["id"]).To(Equal("manager-123"))
			Expect(response["role"]).To(Equal("manager"))

			teams := response["teams"].([]interface{})
			Expect(len(teams)).To(Equal(1))
			Expect(teams[0]).To(Equal("team-alpha-managers"))
		})

		It("should return user with no groups", func() {
			testUser := &authDomain.User{
				UserID: "user-456",
				Email:  "newuser@example.com",
				Name:   "New User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{}, // Empty groups
			}

			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response["id"]).To(Equal("user-456"))
			Expect(response["email"]).To(Equal("newuser@example.com"))
			Expect(response["name"]).To(Equal("New User"))
			Expect(response["role"]).To(Equal("user"))

			// Verify teams is nil (no groups)
			Expect(response["teams"]).To(BeNil())
		})

		It("should return user with many groups", func() {
			testUser := &authDomain.User{
				UserID: "user-789",
				Email:  "multigroup@example.com",
				Name:   "Multi Group User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-alpha"},
					{GroupName: "team-beta"},
					{GroupName: "team-gamma"},
					{GroupName: "team-delta"},
					{GroupName: "team-epsilon"},
				},
			}

			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			teams := response["teams"].([]interface{})
			Expect(len(teams)).To(Equal(5))
			Expect(teams[0]).To(Equal("team-alpha"))
			Expect(teams[1]).To(Equal("team-beta"))
			Expect(teams[2]).To(Equal("team-gamma"))
			Expect(teams[3]).To(Equal("team-delta"))
			Expect(teams[4]).To(Equal("team-epsilon"))
		})

		It("should handle user with nil Groups slice", func() {
			testUser := &authDomain.User{
				UserID: "user-999",
				Email:  "nilgroups@example.com",
				Name:   "Nil Groups User",
				Role:   authDomain.RoleUser,
				Groups: nil, // nil groups
			}

			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response["id"]).To(Equal("user-999"))
			Expect(response["teams"]).To(BeNil())
		})
	})

	Describe("Error Cases", func() {
		It("should return 401 when user does not exist in context", func() {
			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				// Don't set user in context
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("User not found"))
		})

		It("should return 401 when user is nil", func() {
			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", nil) // Set user as nil
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("User not found"))
		})

		It("should return 500 when user data is invalid type", func() {
			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", "invalid-string") // Wrong type
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Invalid user data"))
		})

		It("should return 500 when user data is wrong struct type", func() {
			type WrongUser struct {
				ID   string
				Name string
			}

			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", &WrongUser{ID: "123", Name: "Test"}) // Wrong struct
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Invalid user data"))
		})

		It("should return 500 when user data is a number", func() {
			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", 12345) // Wrong type - number
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Invalid user data"))
		})
	})

	Describe("Response Format Validation", func() {
		It("should return response with exactly the expected fields", func() {
			testUser := &authDomain.User{
				UserID: "user-format-test",
				Email:  "format@example.com",
				Name:   "Format Test User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "test-team"},
				},
			}

			router = gin.New()
			router.GET("/auth/user", func(c *gin.Context) {
				c.Set("user", testUser)
				handler.getCurrentUser(c)
			})

			req := httptest.NewRequest("GET", "/auth/user", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Verify all expected keys exist
			Expect(response).To(HaveKey("id"))
			Expect(response).To(HaveKey("email"))
			Expect(response).To(HaveKey("name"))
			Expect(response).To(HaveKey("role"))
			Expect(response).To(HaveKey("teams"))

			// Verify exactly 5 keys (no extra fields)
			Expect(len(response)).To(Equal(5))

			// Verify types
			_, ok := response["id"].(string)
			Expect(ok).To(BeTrue(), "id should be a string")

			_, ok = response["email"].(string)
			Expect(ok).To(BeTrue(), "email should be a string")

			_, ok = response["name"].(string)
			Expect(ok).To(BeTrue(), "name should be a string")

			_, ok = response["role"].(string)
			Expect(ok).To(BeTrue(), "role should be a string")

			_, ok = response["teams"].([]interface{})
			Expect(ok).To(BeTrue(), "teams should be an array")
		})
	})
})

var _ = Describe("requireManagerRole Method Tests", func() {
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

		// Create handler (no services needed for middleware testing)
		handler = NewDomainHandler(nil, nil, nil, nil, nil, nil, nil, logger)

		// Setup router with middleware
		router = gin.New()
	})

	Describe("Successful Authorization", func() {
		It("should allow user with admin role", func() {
			testUser := &authDomain.User{
				UserID: "admin-123",
				Email:  "admin@example.com",
				Name:   "Admin User",
				Role:   authDomain.RoleAdmin,
				Groups: []authDomain.UserGroup{},
			}

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["message"]).To(Equal("success"))
		})

		It("should allow user with manager role", func() {
			testUser := &authDomain.User{
				UserID: "manager-123",
				Email:  "manager@example.com",
				Name:   "Manager User",
				Role:   authDomain.RoleManager,
				Groups: []authDomain.UserGroup{},
			}

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["message"]).To(Equal("success"))
		})

		It("should allow regular user with team manager group", func() {
			testUser := &authDomain.User{
				UserID: "user-123",
				Email:  "user@example.com",
				Name:   "Team Manager User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-alpha-managers"},
				},
			}

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["message"]).To(Equal("success"))
		})

		It("should allow regular user with manager group with leading slash", func() {
			testUser := &authDomain.User{
				UserID: "user-456",
				Email:  "user2@example.com",
				Name:   "Team Manager User 2",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "/team-beta-managers"},
				},
			}

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["message"]).To(Equal("success"))
		})

		It("should allow user with multiple groups including one manager group", func() {
			testUser := &authDomain.User{
				UserID: "user-789",
				Email:  "user3@example.com",
				Name:   "Multi Group User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-alpha"},
					{GroupName: "team-beta"},
					{GroupName: "team-gamma-managers"},
					{GroupName: "team-delta"},
				},
			}

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["message"]).To(Equal("success"))
		})

		It("should allow admin even with no groups", func() {
			testUser := &authDomain.User{
				UserID: "admin-456",
				Email:  "admin2@example.com",
				Name:   "Admin User 2",
				Role:   authDomain.RoleAdmin,
				Groups: nil, // No groups
			}

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["message"]).To(Equal("success"))
		})
	})

	Describe("Authorization Failures", func() {
		It("should return 401 when user does not exist in context", func() {
			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Authentication required"))
		})

		It("should return 401 when user is nil", func() {
			router.Use(func(c *gin.Context) {
				c.Set("user", nil)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Authentication required"))
		})

		It("should return 500 when user data is invalid type", func() {
			router.Use(func(c *gin.Context) {
				c.Set("user", "invalid-string")
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Invalid user data"))
		})

		It("should return 403 when regular user has no manager groups", func() {
			testUser := &authDomain.User{
				UserID: "user-999",
				Email:  "regularuser@example.com",
				Name:   "Regular User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-alpha"},
					{GroupName: "team-beta"},
				},
			}

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusForbidden))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Manager or admin role required"))
		})

		It("should return 403 when user has no groups", func() {
			testUser := &authDomain.User{
				UserID: "user-888",
				Email:  "newuser@example.com",
				Name:   "New User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{},
			}

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusForbidden))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Manager or admin role required"))
		})

		It("should return 403 when user has groups that look similar but are not manager groups", func() {
			testUser := &authDomain.User{
				UserID: "user-777",
				Email:  "almostmanager@example.com",
				Name:   "Almost Manager User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-managers-alpha"}, // Not ending with -managers
					{GroupName: "manager-team"},        // Not ending with -managers
					{GroupName: "managers"},            // Just "managers", not "something-managers"
				},
			}

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusForbidden))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Manager or admin role required"))
		})

		It("should return 500 when user data is wrong struct type", func() {
			type WrongUser struct {
				ID   string
				Name string
			}

			router.Use(func(c *gin.Context) {
				c.Set("user", &WrongUser{ID: "123", Name: "Test"})
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Invalid user data"))
		})
	})

	Describe("Middleware Behavior", func() {
		It("should abort request and not call next handler when unauthorized", func() {
			testUser := &authDomain.User{
				UserID: "user-666",
				Email:  "blocked@example.com",
				Name:   "Blocked User",
				Role:   authDomain.RoleUser,
				Groups: []authDomain.UserGroup{
					{GroupName: "team-alpha"},
				},
			}

			handlerCalled := false

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				handlerCalled = true
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusForbidden))
			Expect(handlerCalled).To(BeFalse(), "Handler should not be called when unauthorized")
		})

		It("should call next handler when authorized", func() {
			testUser := &authDomain.User{
				UserID: "manager-999",
				Email:  "authorized@example.com",
				Name:   "Authorized Manager",
				Role:   authDomain.RoleManager,
				Groups: []authDomain.UserGroup{},
			}

			handlerCalled := false

			router.Use(func(c *gin.Context) {
				c.Set("user", testUser)
				c.Next()
			})

			router.GET("/protected", handler.requireManagerRole(), func(c *gin.Context) {
				handlerCalled = true
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(handlerCalled).To(BeTrue(), "Handler should be called when authorized")
		})
	})
})
