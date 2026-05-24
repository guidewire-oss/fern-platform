package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/pkg/metrics"
)

func newRouter(t *testing.T, rec metrics.Recorder) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(metrics.HTTPMiddleware(rec))
	r.GET("/api/v2/test-runs", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v2/test-runs/:id", func(c *gin.Context) { c.Status(http.StatusNotFound) })
	r.GET("/api/v1/test-runs", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	return r
}

func TestHTTPMiddleware_RecordsObservation(t *testing.T) {
	rec := metrics.NewInMemoryRecorder()
	r := newRouter(t, rec)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs", nil))

	obs := rec.Observations()
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	o := obs[0]
	if o.Method != "GET" || o.Route != "/api/v2/test-runs" || o.Status != 200 {
		t.Errorf("labels wrong: %+v", o)
	}
	if o.Duration <= 0 {
		t.Errorf("duration not recorded: %v", o.Duration)
	}
}

func TestHTTPMiddleware_UsesRoutePatternNotURL(t *testing.T) {
	rec := metrics.NewInMemoryRecorder()
	r := newRouter(t, rec)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs/abc-123", nil))

	obs := rec.Observations()
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	// Route label must collapse path params to keep cardinality bounded.
	if obs[0].Route != "/api/v2/test-runs/:id" {
		t.Errorf("Route should be the pattern, got %q", obs[0].Route)
	}
}

func TestHTTPMiddleware_RecordsErrorStatuses(t *testing.T) {
	rec := metrics.NewInMemoryRecorder()
	r := newRouter(t, rec)

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/test-runs", nil))
	obs := rec.Observations()
	if obs[0].Status != 500 {
		t.Errorf("expected status 500, got %d", obs[0].Status)
	}
	if obs[0].StatusClass != "5xx" {
		t.Errorf("expected StatusClass 5xx, got %q", obs[0].StatusClass)
	}
}

func TestSLOBuckets_AreSortedAndCoverSLOs(t *testing.T) {
	buckets := metrics.SLOBuckets()
	for i := 1; i < len(buckets); i++ {
		if buckets[i-1] >= buckets[i] {
			t.Errorf("bucket %d (%v) >= bucket %d (%v); must be strictly increasing",
				i-1, buckets[i-1], i, buckets[i])
		}
	}
	// SLOs: list P95 < 500ms; detail P95 < 300ms; ingest P95 < 250ms.
	// Bucket set must include the SLO thresholds so we can read off
	// the burn rate from histogram counts.
	required := []time.Duration{
		250 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		5 * time.Second,
	}
	for _, want := range required {
		if !containsDuration(buckets, want) {
			t.Errorf("bucket set missing required SLO bucket %v: %v", want, buckets)
		}
	}
}

func containsDuration(s []time.Duration, want time.Duration) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func TestInMemoryRecorder_ConcurrentSafe(t *testing.T) {
	rec := metrics.NewInMemoryRecorder()
	done := make(chan struct{})
	const n = 200
	for i := 0; i < n; i++ {
		go func() {
			rec.Observe(metrics.Observation{
				Method: "GET", Route: "/x", Status: 200, StatusClass: "2xx", Duration: time.Millisecond,
			})
			done <- struct{}{}
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}
	if len(rec.Observations()) != n {
		t.Errorf("lost observations under concurrency: got %d, want %d", len(rec.Observations()), n)
	}
}
