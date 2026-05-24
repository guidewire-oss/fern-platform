package tracing_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/pkg/tracing"
)

// recordingTracer captures spans for assertion.
type recordingTracer struct {
	spans []recordedSpan
}

type recordedSpan struct {
	name string
	attrs map[string]any
	endedWithError bool
}

func (r *recordingTracer) Start(_ context.Context, name string) (context.Context, tracing.Span) {
	s := &recordingSpan{name: name, attrs: map[string]any{}}
	r.spans = append(r.spans, recordedSpan{name: name, attrs: s.attrs})
	return context.Background(), s
}

type recordingSpan struct {
	name  string
	attrs map[string]any
	ended bool
	err   error
}

func (s *recordingSpan) SetAttribute(key string, value any) {
	s.attrs[key] = value
}
func (s *recordingSpan) End()                  { s.ended = true }
func (s *recordingSpan) RecordError(err error) { s.err = err }

func TestNoopTracer_NeverPanics(t *testing.T) {
	tr := tracing.NoopTracer{}
	ctx, span := tr.Start(context.Background(), "test")
	if ctx == nil {
		t.Fatal("Start must not return nil context")
	}
	if span == nil {
		t.Fatal("Start must return a span (even if noop)")
	}
	span.SetAttribute("k", "v")
	span.RecordError(http.ErrNoCookie)
	span.End()
}

func TestHTTPMiddleware_StartsSpanPerRequest(t *testing.T) {
	rec := &recordingTracer{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(tracing.HTTPMiddleware(rec))
	r.GET("/api/v2/test-runs/:id", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs/abc", nil))

	if len(rec.spans) != 1 {
		t.Fatalf("expected one span, got %d", len(rec.spans))
	}
	s := rec.spans[0]
	if s.name != "GET /api/v2/test-runs/:id" {
		t.Errorf("span name should use route pattern, got %q", s.name)
	}
	if s.attrs["http.method"] != "GET" {
		t.Errorf("http.method attr missing/wrong: %+v", s.attrs)
	}
	if s.attrs["http.route"] != "/api/v2/test-runs/:id" {
		t.Errorf("http.route attr missing/wrong: %+v", s.attrs)
	}
	if s.attrs["http.status_code"] != 200 {
		t.Errorf("http.status_code attr missing/wrong: %+v", s.attrs)
	}
}

func TestHTTPMiddleware_NilTracerIsNoop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(tracing.HTTPMiddleware(nil))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Code != http.StatusOK {
		t.Errorf("nil tracer should be a no-op pass-through, got %d", w.Code)
	}
}

func TestHTTPMiddleware_UnmatchedRouteUsesLiteralPath(t *testing.T) {
	rec := &recordingTracer{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(tracing.HTTPMiddleware(rec))
	// no routes registered

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if len(rec.spans) != 1 {
		t.Fatalf("expected one span, got %d", len(rec.spans))
	}
	// Unmatched route: span name should still be meaningful but bounded
	// — using a constant literal keeps cardinality from blowing up.
	if rec.spans[0].name != "GET (unmatched)" {
		t.Errorf("unmatched route span name unexpected: %q", rec.spans[0].name)
	}
}
