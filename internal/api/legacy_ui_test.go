package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/pkg/config"
)

// An unauthenticated visit to /legacy must send the user through login with
// a return path back to /legacy, otherwise the callback drops them on the v2
// SPA at / and the rollback UI is unreachable behind auth.
func TestServeLegacyUI_UnauthenticatedKeepsLegacyReturnPath(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("JWT_SECRET", "test-only-not-a-real-secret")
	if err := config.NewManager().Load(""); err != nil {
		t.Fatalf("config could not be loaded: %v", err)
	}
	if !config.GetConfig().Auth.Enabled {
		t.Fatal("AUTH_ENABLED=true did not enable auth in the loaded config")
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DomainHandlerV2{}
	r.GET("/legacy", h.serveLegacyUI)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/legacy", nil))

	if w.Code != http.StatusFound {
		t.Fatalf("status=%d, want 302", w.Code)
	}
	if got, want := w.Header().Get("Location"), "/auth/login?return=%2Flegacy"; got != want {
		t.Errorf("Location=%q, want %q", got, want)
	}
}

// Old /web URLs (e.g. bookmarked /web/#/test-runs) must redirect to /legacy
// rather than keep serving the legacy UI, while /web assets still load (#237).
func TestLegacyUIRoutes_WebRedirectsToLegacyAndAssetsStillServe(t *testing.T) {
	t.Chdir("../..") // ./web is resolved relative to the repo root

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &DomainHandlerV2{}
	h.registerLegacyUIRoutes(r)

	for _, path := range []string{"/web", "/web/", "/web/index.html"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusFound {
			t.Errorf("%s: status=%d, want 302", path, w.Code)
		}
		if got := w.Header().Get("Location"); got != "/legacy" {
			t.Errorf("%s: Location=%q, want /legacy", path, got)
		}
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/web/js/graphql-client.js", nil))
	if w.Code != http.StatusOK {
		t.Errorf("/web/js/graphql-client.js: status=%d, want 200", w.Code)
	}
}
