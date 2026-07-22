package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/internal/web"
)

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Existing API routes must keep working after web.Register is called.
	r.GET("/api/v1/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/graphql", func(c *gin.Context) { c.String(http.StatusOK, "graphql ok") })
	r.GET("/auth/login", func(c *gin.Context) { c.String(http.StatusOK, "login") })
	if err := web.Register(r); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return r
}

func TestEmbed_ServesIndexAtRoot(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "<div id=\"root\">") {
		t.Errorf("body did not contain SPA root mount point: %s", w.Body.String())
	}
}

func TestEmbed_SPAFallbackForArbitraryPath(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/projects/abc/runs/123", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("SPA fallback should serve index.html, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "<div id=\"root\">") {
		t.Errorf("SPA fallback did not return index.html: %s", w.Body.String())
	}
}

func TestEmbed_DoesNotInterceptAPIRoutes(t *testing.T) {
	r := newTestRouter(t)
	cases := []struct {
		path, want string
	}{
		{"/api/v1/ping", `"ok":true`},
		{"/graphql", "graphql ok"},
		{"/auth/login", "login"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != http.StatusOK {
			t.Errorf("%s: status=%d", tc.path, w.Code)
		}
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Errorf("%s: body=%q want substring %q", tc.path, w.Body.String(), tc.want)
		}
	}
}

func TestEmbed_RegisterAtPrefix_ServesIndexAtPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "legacy root") })
	r.GET("/api/v1/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	if err := web.RegisterAtPrefix(r, "/v2"); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		path string
		want string
	}{
		{"/", "legacy root"},                                        // legacy untouched
		{"/v2", "<div id=\"root\">"},                                // SPA shell
		{"/v2/", "<div id=\"root\">"},                               // trailing slash
		{"/v2/projects/abc", "<div id=\"root\">"},                   // SPA client route
		{"/api/v1/ping", `"ok":true`},                               // API untouched
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != http.StatusOK {
			t.Errorf("%s: status=%d", tc.path, w.Code)
		}
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Errorf("%s: body did not contain %q\n--- got ---\n%s", tc.path, tc.want, w.Body.String())
		}
	}
}

func TestEmbed_RegisterAtPrefix_CachesAssetsAggressively(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := web.RegisterAtPrefix(r, "/v2"); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v2/assets/placeholder.txt", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	cc := w.Header().Get("Cache-Control")
	if !strings.Contains(cc, "max-age=31536000") || !strings.Contains(cc, "immutable") {
		t.Errorf("expected long-immutable cache on hashed assets, got %q", cc)
	}
}

func TestEmbed_RegisterAtPrefix_EmptyPrefixFallsBackToRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := web.RegisterAtPrefix(r, ""); err != nil {
		t.Fatal(err)
	}
	// Empty prefix delegates to Register — index.html served at /
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestEmbed_UnknownAPIRouteReturns404NotIndex(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/does-not-exist", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown API route, got %d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "<div id=\"root\">") {
		t.Errorf("API 404 should not return SPA index")
	}
}
