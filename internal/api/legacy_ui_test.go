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
		t.Skipf("config could not be loaded in this environment: %v", err)
	}
	if !config.GetConfig().Auth.Enabled {
		t.Skip("auth not enabled by AUTH_ENABLED in this environment")
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
