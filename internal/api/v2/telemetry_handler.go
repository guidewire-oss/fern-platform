package v2

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebVital is one Core Web Vitals measurement reported by the browser.
//
// Field set mirrors what `web-vitals` (the Google library) emits in
// its onLCP/onINP/onCLS callbacks; we accept only what we plan to
// store. Anything beyond is ignored.
type WebVital struct {
	Name   string  `json:"name"`            // "LCP" | "INP" | "CLS" | "FCP" | "TTFB"
	Value  float64 `json:"value"`           // ms for time metrics, unitless for CLS
	Rating string  `json:"rating,omitempty"` // "good" | "needs-improvement" | "poor"
	Route  string  `json:"route"`           // SPA route pattern (e.g. "/test-runs")
}

// WebVitalSink consumes recorded vitals. Production wires this to a
// Prometheus histogram + a low-rate log line; tests use a fake.
type WebVitalSink interface {
	Record(WebVital)
}

// TelemetryHandler serves POST /api/v2/telemetry/vitals.
type TelemetryHandler struct {
	sink WebVitalSink
}

// NewTelemetryHandler constructs a handler. A nil sink means
// "accept-and-drop" — the endpoint still 202s so a metrics-backend
// outage doesn't surface to users.
func NewTelemetryHandler(sink WebVitalSink) *TelemetryHandler {
	return &TelemetryHandler{sink: sink}
}

// Register mounts /telemetry/vitals on the given v2 group.
func (h *TelemetryHandler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/telemetry")
	g.POST("/vitals", h.vitals)
}

// allowedMetrics caps what we accept to bound Prometheus label
// cardinality. Adding a metric requires a code change, which is the
// point — labels in metrics are not user-controllable surface.
var allowedMetrics = map[string]struct{}{
	"LCP": {}, "INP": {}, "CLS": {}, "FCP": {}, "TTFB": {},
}

const maxRouteLen = 256

func (h *TelemetryHandler) vitals(c *gin.Context) {
	var v WebVital
	if err := c.ShouldBindJSON(&v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	if _, ok := allowedMetrics[v.Name]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown metric name"})
		return
	}
	if v.Route == "" || len(v.Route) > maxRouteLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "route is required and at most 256 chars"})
		return
	}
	if !validValue(v.Name, v.Value) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value out of expected range"})
		return
	}
	if h.sink != nil {
		h.sink.Record(v)
	}
	c.Status(http.StatusAccepted)
}

// validValue applies a permissive range check. The intent is to
// drop obvious garbage (negative timings, CLS > 1) without policing
// the long tail of real-but-bad measurements — that is the metric
// backend's job.
func validValue(name string, value float64) bool {
	switch name {
	case "CLS":
		return value >= 0 && value <= 1
	default: // time-based metrics: 0..120000 ms (2 min upper bound)
		return value >= 0 && value <= 120_000
	}
}
