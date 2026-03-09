package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultCORSConfig(t *testing.T) {
	cfg := DefaultCORSConfig()

	assert.NotEmpty(t, cfg.AllowOrigins)
	assert.Contains(t, cfg.AllowOrigins, "http://localhost:3000")
	assert.Contains(t, cfg.AllowOrigins, "https://localhost:3000")

	assert.Contains(t, cfg.AllowMethods, "GET")
	assert.Contains(t, cfg.AllowMethods, "POST")
	assert.Contains(t, cfg.AllowMethods, "PUT")
	assert.Contains(t, cfg.AllowMethods, "DELETE")
	assert.Contains(t, cfg.AllowMethods, "OPTIONS")

	assert.Contains(t, cfg.AllowHeaders, "Authorization")
	assert.Contains(t, cfg.AllowHeaders, "Content-Type")
	assert.Contains(t, cfg.AllowHeaders, "X-Request-ID")

	assert.Contains(t, cfg.ExposeHeaders, "X-Request-ID")
	assert.Contains(t, cfg.ExposeHeaders, "X-Total-Count")

	assert.True(t, cfg.AllowCredentials)
	assert.Equal(t, 12*time.Hour, cfg.MaxAge)
}

func TestProductionCORSConfig(t *testing.T) {
	t.Run("with custom origins", func(t *testing.T) {
		origins := []string{"https://example.com"}
		cfg := ProductionCORSConfig(origins)
		assert.Equal(t, origins, cfg.AllowOrigins)
		assert.True(t, cfg.AllowCredentials)
	})

	t.Run("with empty origins uses defaults", func(t *testing.T) {
		cfg := ProductionCORSConfig(nil)
		defaultCfg := DefaultCORSConfig()
		assert.Equal(t, defaultCfg.AllowOrigins, cfg.AllowOrigins)
	})
}

func TestNewCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := DefaultCORSConfig()
	handler := NewCORSMiddleware(cfg)
	require.NotNil(t, handler)

	router := gin.New()
	router.Use(handler)
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Test preflight OPTIONS request
	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	router.ServeHTTP(w, req)

	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestNewCORSMiddleware_DisallowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := DefaultCORSConfig()
	handler := NewCORSMiddleware(cfg)

	router := gin.New()
	router.Use(handler)
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	router.ServeHTTP(w, req)

	// The cors middleware should not set the Access-Control-Allow-Origin for disallowed origins
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestDevCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := DevCORSMiddleware()
	require.NotNil(t, handler)

	router := gin.New()
	router.Use(handler)
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	router.ServeHTTP(w, req)

	// Dev CORS allows all origins
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}
