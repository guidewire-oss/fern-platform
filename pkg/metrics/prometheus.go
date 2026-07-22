package metrics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type counterKey struct{ method, route, statusClass string }
type histKey struct{ method, route string }

// PrometheusExposition formats the recorded observations as a
// Prometheus text exposition document (version 0.0.4).
//
// We emit it directly rather than depending on prometheus/client_golang
// to keep this package dep-free. The format is small and stable; the
// upstream client is needed only if you want push-gateway support or
// exemplar/OpenMetrics features, neither of which we use today.
//
// Output ends with a newline as required by the spec.
func PrometheusExposition(r *InMemoryRecorder) string {
	obs := r.Observations()

	counters := map[counterKey]int{}
	histCounts := map[histKey]int{}
	histSums := map[histKey]float64{}
	histBuckets := map[histKey]map[time.Duration]int{}

	buckets := SLOBuckets()

	for _, o := range obs {
		ck := counterKey{o.Method, o.Route, o.StatusClass}
		counters[ck]++

		hk := histKey{o.Method, o.Route}
		histCounts[hk]++
		histSums[hk] += o.Duration.Seconds()
		if histBuckets[hk] == nil {
			histBuckets[hk] = map[time.Duration]int{}
		}
		for _, b := range buckets {
			if o.Duration <= b {
				histBuckets[hk][b]++
			}
		}
	}

	var b strings.Builder

	b.WriteString("# HELP fern_http_requests_total Total HTTP requests handled by Fern.\n")
	b.WriteString("# TYPE fern_http_requests_total counter\n")
	for _, k := range sortedCounterKeys(counters) {
		fmt.Fprintf(&b, `fern_http_requests_total{method="%s",route="%s",status_class="%s"} %d`+"\n",
			escapeLabel(k.method), escapeLabel(k.route), escapeLabel(k.statusClass), counters[k])
	}

	b.WriteString("# HELP fern_http_request_duration_seconds HTTP request latency.\n")
	b.WriteString("# TYPE fern_http_request_duration_seconds histogram\n")
	for _, k := range sortedHistKeys(histCounts) {
		for _, bucket := range buckets {
			fmt.Fprintf(&b, `fern_http_request_duration_seconds_bucket{method="%s",route="%s",le="%s"} %d`+"\n",
				escapeLabel(k.method), escapeLabel(k.route),
				formatBucket(bucket), histBuckets[k][bucket])
		}
		fmt.Fprintf(&b, `fern_http_request_duration_seconds_bucket{method="%s",route="%s",le="+Inf"} %d`+"\n",
			escapeLabel(k.method), escapeLabel(k.route), histCounts[k])
		fmt.Fprintf(&b, `fern_http_request_duration_seconds_sum{method="%s",route="%s"} %s`+"\n",
			escapeLabel(k.method), escapeLabel(k.route), formatFloat(histSums[k]))
		fmt.Fprintf(&b, `fern_http_request_duration_seconds_count{method="%s",route="%s"} %d`+"\n",
			escapeLabel(k.method), escapeLabel(k.route), histCounts[k])
	}

	return b.String()
}

// PrometheusContentType is the MIME type Prometheus scrapers expect.
const PrometheusContentType = "text/plain; version=0.0.4; charset=utf-8"

func sortedCounterKeys(m map[counterKey]int) []counterKey {
	keys := make([]counterKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		return keys[i].statusClass < keys[j].statusClass
	})
	return keys
}

func sortedHistKeys(m map[histKey]int) []histKey {
	keys := make([]histKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		return keys[i].route < keys[j].route
	})
	return keys
}

// escapeLabel applies the two escapes the Prometheus exposition
// format requires for label values: backslash and double-quote.
// Newlines also need escaping but our label sources never contain them.
func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func formatBucket(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'g', -1, 64)
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
