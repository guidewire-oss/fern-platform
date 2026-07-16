package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/pkg/middleware"
)

func newCSPRouter(t *testing.T, opts middleware.CSPOptions) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.CSP(opts))
	r.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("<html></html>"))
	})
	r.GET("/api/v2/test-runs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/assets/script.js", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/javascript", []byte("/* js */"))
	})
	return r
}

func TestCSP_AppliesToHTMLResponses(t *testing.T) {
	r := newCSPRouter(t, middleware.CSPOptions{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	got := w.Header().Get("Content-Security-Policy")
	if got == "" {
		t.Fatal("CSP header missing on HTML response")
	}
	for _, want := range []string{
		"default-src 'self'",
		"script-src 'self'",
		"img-src 'self' data:",
		"connect-src 'self'",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("CSP %q missing directive %q", got, want)
		}
	}
}

func TestCSP_SkipsJSONResponses(t *testing.T) {
	r := newCSPRouter(t, middleware.CSPOptions{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs", nil))

	// JSON API responses do not render in a browsing context. CSP on
	// them adds noise without adding safety.
	if w.Header().Get("Content-Security-Policy") != "" {
		t.Error("CSP should not be emitted on JSON responses")
	}
}

func TestCSP_ExtraConnectSrcAppended(t *testing.T) {
	r := newCSPRouter(t, middleware.CSPOptions{
		ExtraConnectSrc: []string{"https://datadog.example.com", "wss://stream.example.com"},
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	got := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(got, "connect-src 'self' https://datadog.example.com wss://stream.example.com") {
		t.Errorf("extra connect-src not appended: %q", got)
	}
}

func TestCSP_AppliesToAssetsByContentType(t *testing.T) {
	// JS bundles served at /assets should not get CSP — they are not
	// rendered top-level, and adding CSP would tag every script
	// response with a policy header that browsers ignore for non-doc
	// loads.
	r := newCSPRouter(t, middleware.CSPOptions{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/script.js", nil))
	if w.Header().Get("Content-Security-Policy") != "" {
		t.Error("CSP should not be set on JS bundles")
	}
}
