package v2_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	apiv2 "github.com/guidewire-oss/fern-platform/internal/api/v2"
)

type fakeVitalSink struct {
	mu    sync.Mutex
	calls []apiv2.WebVital
}

func (f *fakeVitalSink) Record(v apiv2.WebVital) {
	f.mu.Lock()
	f.calls = append(f.calls, v)
	f.mu.Unlock()
}

func newTelemetryRouter(t *testing.T, sink apiv2.WebVitalSink) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	apiv2.NewTelemetryHandler(sink).Register(r.Group("/api/v2"))
	return r
}

func TestTelemetry_AcceptsValidLCP(t *testing.T) {
	sink := &fakeVitalSink{}
	r := newTelemetryRouter(t, sink)
	body := strings.NewReader(`{"name":"LCP","value":1850,"rating":"good","route":"/test-runs"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/telemetry/vitals", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(sink.calls) != 1 {
		t.Fatalf("expected one sink call, got %d", len(sink.calls))
	}
	got := sink.calls[0]
	if got.Name != "LCP" || got.Value != 1850 || got.Route != "/test-runs" {
		t.Errorf("recorded vital wrong: %+v", got)
	}
}

func TestTelemetry_RejectsUnknownMetric(t *testing.T) {
	r := newTelemetryRouter(t, &fakeVitalSink{})
	body := strings.NewReader(`{"name":"BANANA","value":1,"route":"/x"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/telemetry/vitals", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown metric, got %d", w.Code)
	}
}

func TestTelemetry_RejectsOutOfRangeValue(t *testing.T) {
	cases := []string{
		`{"name":"LCP","value":-1,"route":"/x"}`,
		`{"name":"LCP","value":9999999,"route":"/x"}`,
		`{"name":"CLS","value":1.5,"route":"/x"}`, // CLS in [0,1]
	}
	for _, body := range cases {
		r := newTelemetryRouter(t, &fakeVitalSink{})
		req := httptest.NewRequest(http.MethodPost, "/api/v2/telemetry/vitals",
			bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", body, w.Code)
		}
	}
}

func TestTelemetry_RejectsMissingRoute(t *testing.T) {
	r := newTelemetryRouter(t, &fakeVitalSink{})
	body := strings.NewReader(`{"name":"LCP","value":1000,"route":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/telemetry/vitals", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTelemetry_RejectsHugeRouteToCapCardinality(t *testing.T) {
	r := newTelemetryRouter(t, &fakeVitalSink{})
	huge := strings.Repeat("a", 1024)
	body := strings.NewReader(`{"name":"LCP","value":1000,"route":"` + huge + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/telemetry/vitals", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for oversized route, got %d", w.Code)
	}
}

func TestTelemetry_DropsWhenSinkUnset(t *testing.T) {
	// Nil sink should not panic; endpoint still 202s so the client
	// doesn't see errors during a metrics-backend outage.
	r := newTelemetryRouter(t, nil)
	body := strings.NewReader(`{"name":"INP","value":120,"route":"/x"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/telemetry/vitals", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202 with nil sink, got %d", w.Code)
	}
}
