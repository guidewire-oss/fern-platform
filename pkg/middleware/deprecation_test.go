package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/pkg/middleware"
)

func newDeprecatedRouter(t *testing.T, opts middleware.DeprecationOptions) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Deprecation(opts))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestDeprecation_EmitsHeadersWhenEnabled(t *testing.T) {
	sunset := time.Date(2027, 5, 14, 0, 0, 0, 0, time.UTC)
	r := newDeprecatedRouter(t, middleware.DeprecationOptions{
		Enabled: true,
		Sunset:  sunset,
		Link:    "https://docs.fern/migrate-v2",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Deprecation"); got != "true" {
		t.Errorf(`Deprecation header: got %q want "true"`, got)
	}
	wantSunset := sunset.UTC().Format(http.TimeFormat)
	if got := w.Header().Get("Sunset"); got != wantSunset {
		t.Errorf("Sunset header: got %q want %q", got, wantSunset)
	}
	if got := w.Header().Get("Link"); got != `<https://docs.fern/migrate-v2>; rel="deprecation"` {
		t.Errorf("Link header: got %q", got)
	}
}

func TestDeprecation_NoHeadersWhenDisabled(t *testing.T) {
	r := newDeprecatedRouter(t, middleware.DeprecationOptions{Enabled: false})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Deprecation"); got != "" {
		t.Errorf("expected no Deprecation header when disabled, got %q", got)
	}
	if got := w.Header().Get("Sunset"); got != "" {
		t.Errorf("expected no Sunset header when disabled, got %q", got)
	}
}

func TestDeprecation_OmitsLinkWhenEmpty(t *testing.T) {
	r := newDeprecatedRouter(t, middleware.DeprecationOptions{
		Enabled: true,
		Sunset:  time.Now().Add(24 * time.Hour),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if got := w.Header().Get("Link"); got != "" {
		t.Errorf("expected no Link header without configured URL, got %q", got)
	}
}
