package v2_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	apiv2 "github.com/guidewire-oss/fern-platform/internal/api/v2"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(_ context.Context) error { return f.err }

func newHealthRouter(t *testing.T, p apiv2.Pinger) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	apiv2.RegisterHealthRoutes(r, p)
	return r
}

func TestHealthz_AlwaysOK(t *testing.T) {
	// /healthz is for liveness — should not check downstream deps.
	// Even with a failing pinger it returns 200.
	r := newHealthRouter(t, fakePinger{err: errors.New("db down")})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if w.Code != http.StatusOK {
		t.Errorf("liveness should always 200, got %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("expected status:ok, got %+v", body)
	}
}

func TestReadyz_OKWhenDBReachable(t *testing.T) {
	r := newHealthRouter(t, fakePinger{err: nil})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if w.Code != http.StatusOK {
		t.Errorf("readyz should 200 with healthy DB, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestReadyz_503WhenDBUnreachable(t *testing.T) {
	r := newHealthRouter(t, fakePinger{err: errors.New("conn refused")})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz should 503 with unhealthy DB, got %d", w.Code)
	}
	// Response should explain the failure so k8s events surface a useful message.
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "unhealthy" {
		t.Errorf("expected status:unhealthy, got %+v", body)
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("expected error field, got %+v", body)
	}
}

func TestReadyz_NilPingerStillReadyEnough(t *testing.T) {
	// A nil pinger means "no DB to check" — degenerate but possible
	// in test deployments. Reply 200 so the pod stays in service.
	r := newHealthRouter(t, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if w.Code != http.StatusOK {
		t.Errorf("readyz with nil pinger should 200, got %d", w.Code)
	}
}
