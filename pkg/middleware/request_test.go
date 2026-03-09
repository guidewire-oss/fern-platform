package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger(t *testing.T) *logging.Logger {
	t.Helper()
	logger, err := logging.NewLogger(&config.LoggingConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})
	require.NoError(t, err)
	return logger
}

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.GET("/test", func(c *gin.Context) {
		reqID, _ := c.Get("request_id")
		c.String(http.StatusOK, reqID.(string))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Should have set the X-Request-ID header
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
	// Body should contain the generated UUID
	assert.NotEmpty(t, w.Body.String())
}

func TestRequestIDMiddleware_UsesExistingID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestIDMiddleware())
	router.GET("/test", func(c *gin.Context) {
		reqID, _ := c.Get("request_id")
		c.String(http.StatusOK, reqID.(string))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "custom-id-123")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "custom-id-123", w.Header().Get("X-Request-ID"))
	assert.Equal(t, "custom-id-123", w.Body.String())
}

func TestLoggingMiddleware_DoesntPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestLogger(t)

	router := gin.New()
	router.Use(LoggingMiddleware(logger))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	assert.NotPanics(t, func() {
		router.ServeHTTP(w, req)
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRecoveryMiddleware_CatchesPanics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := newTestLogger(t)

	router := gin.New()
	router.Use(RecoveryMiddleware(logger))
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)

	assert.NotPanics(t, func() {
		router.ServeHTTP(w, req)
	})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal server error")
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
}

func TestHealthCheckMiddleware_HealthPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(HealthCheckMiddleware())
	router.GET("/health", func(c *gin.Context) {
		// This should not be reached because the middleware handles it
		c.String(http.StatusOK, "should not reach")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
	assert.Contains(t, w.Body.String(), "fern-platform")
}

func TestHealthCheckMiddleware_NonHealthPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(HealthCheckMiddleware())
	router.GET("/other", func(c *gin.Context) {
		c.String(http.StatusOK, "other endpoint")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/other", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "other endpoint", w.Body.String())
}

func TestHealthCheckMiddleware_PostMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(HealthCheckMiddleware())
	router.POST("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "post handler")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/health", nil)
	router.ServeHTTP(w, req)

	// POST to /health should pass through to the handler, not the middleware
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "post handler", w.Body.String())
}

func TestRateLimitMiddleware_PassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RateLimitMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
