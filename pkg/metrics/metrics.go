// Package metrics provides HTTP request instrumentation with histogram
// buckets aligned to the platform's SLOs.
//
// The package defines a thin Recorder interface that production wires
// to a real Prometheus client. The in-memory Recorder shipped here is
// useful for tests and as a fallback in single-instance development.
package metrics

import (
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Observation is a single HTTP request measurement. Production
// recorders translate this into histogram counters + gauges; the
// in-memory recorder retains the slice for inspection.
type Observation struct {
	Method      string
	Route       string
	Status      int
	StatusClass string // "2xx", "3xx", "4xx", "5xx"
	Duration    time.Duration
}

// Recorder accepts request observations. Implementations must be
// safe for concurrent use.
type Recorder interface {
	Observe(Observation)
}

// SLOBuckets returns the histogram bucket boundaries aligned to the
// platform SLOs in perf-budgets.json. Keeping the SLO thresholds as
// bucket edges lets dashboards read P95 burn-rate off the count
// columns without estimation error at the threshold.
func SLOBuckets() []time.Duration {
	return []time.Duration{
		10 * time.Millisecond,
		25 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		250 * time.Millisecond, // ingest P95 SLO
		500 * time.Millisecond, // list P95 SLO
		750 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
	}
}

// HTTPMiddleware records one Observation per Gin request.
//
// The Route label uses Gin's matched route pattern (e.g.
// "/api/v2/test-runs/:id") rather than the raw URL, which keeps
// cardinality bounded — critical for Prometheus.
func HTTPMiddleware(rec Recorder) gin.HandlerFunc {
	if rec == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		rec.Observe(Observation{
			Method:      c.Request.Method,
			Route:       fullPathOrURL(c),
			Status:      status,
			StatusClass: statusClass(status),
			Duration:    time.Since(start),
		})
	}
}

func fullPathOrURL(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	// Unmatched route — record once under a fixed label so unknown
	// 404s don't blow up cardinality.
	return "unmatched"
}

func statusClass(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "u" + strconv.Itoa(status)
	}
}

// InMemoryRecorder retains observations in a slice. Useful for tests
// and as a default that lets the rest of the pipeline boot before a
// real metrics backend is wired.
type InMemoryRecorder struct {
	mu  sync.Mutex
	obs []Observation
}

// NewInMemoryRecorder returns an empty recorder.
func NewInMemoryRecorder() *InMemoryRecorder { return &InMemoryRecorder{} }

func (r *InMemoryRecorder) Observe(o Observation) {
	r.mu.Lock()
	r.obs = append(r.obs, o)
	r.mu.Unlock()
}

// Observations returns a snapshot copy of recorded observations.
func (r *InMemoryRecorder) Observations() []Observation {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Observation, len(r.obs))
	copy(out, r.obs)
	return out
}
