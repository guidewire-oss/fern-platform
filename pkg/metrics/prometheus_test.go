package metrics_test

import (
	"strings"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/pkg/metrics"
)

func TestPromExposition_EmitsHelpAndType(t *testing.T) {
	rec := metrics.NewInMemoryRecorder()
	rec.Observe(metrics.Observation{
		Method: "GET", Route: "/api/v2/test-runs", Status: 200,
		StatusClass: "2xx", Duration: 100 * time.Millisecond,
	})

	out := metrics.PrometheusExposition(rec)

	for _, want := range []string{
		"# HELP fern_http_requests_total",
		"# TYPE fern_http_requests_total counter",
		"# HELP fern_http_request_duration_seconds",
		"# TYPE fern_http_request_duration_seconds histogram",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestPromExposition_CounterIncrementsPerLabelTuple(t *testing.T) {
	rec := metrics.NewInMemoryRecorder()
	for i := 0; i < 3; i++ {
		rec.Observe(metrics.Observation{
			Method: "GET", Route: "/x", Status: 200, StatusClass: "2xx",
			Duration: 50 * time.Millisecond,
		})
	}
	rec.Observe(metrics.Observation{
		Method: "GET", Route: "/x", Status: 500, StatusClass: "5xx",
		Duration: 200 * time.Millisecond,
	})

	out := metrics.PrometheusExposition(rec)
	mustContain(t, out, `fern_http_requests_total{method="GET",route="/x",status_class="2xx"} 3`)
	mustContain(t, out, `fern_http_requests_total{method="GET",route="/x",status_class="5xx"} 1`)
}

func TestPromExposition_HistogramIncludesAllSLOBuckets(t *testing.T) {
	rec := metrics.NewInMemoryRecorder()
	rec.Observe(metrics.Observation{
		Method: "GET", Route: "/x", Status: 200, StatusClass: "2xx",
		Duration: 150 * time.Millisecond,
	})

	out := metrics.PrometheusExposition(rec)
	// Every SLO bucket boundary must appear as a histogram bucket.
	for _, want := range []string{
		`le="0.25"`,
		`le="0.5"`,
		`le="1"`,
		`le="2"`,
		`le="5"`,
		`le="+Inf"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("histogram missing bucket %s\n%s", want, out)
		}
	}
	// 150ms falls into the 0.25s bucket and all larger ones.
	mustContain(t, out, `fern_http_request_duration_seconds_bucket{method="GET",route="/x",le="0.25"} 1`)
	mustContain(t, out, `fern_http_request_duration_seconds_bucket{method="GET",route="/x",le="+Inf"} 1`)
	mustContain(t, out, `fern_http_request_duration_seconds_count{method="GET",route="/x"} 1`)
}

func TestPromExposition_EscapesLabelValues(t *testing.T) {
	rec := metrics.NewInMemoryRecorder()
	rec.Observe(metrics.Observation{
		Method: "GET", Route: `/x"with\quotes`, Status: 200, StatusClass: "2xx",
		Duration: 10 * time.Millisecond,
	})
	out := metrics.PrometheusExposition(rec)
	// Quotes and backslashes in label values must be escaped per spec.
	mustContain(t, out, `route="/x\"with\\quotes"`)
}

func TestPromExposition_EmptyRecorderProducesValidOutput(t *testing.T) {
	rec := metrics.NewInMemoryRecorder()
	out := metrics.PrometheusExposition(rec)
	// Still emits HELP/TYPE lines and ends with newline per spec.
	if !strings.Contains(out, "# HELP fern_http_requests_total") {
		t.Errorf("empty recorder should still emit metadata\n%s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("exposition must end with newline")
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("output missing line:\n  want: %s\n  got:\n%s", needle, haystack)
	}
}
