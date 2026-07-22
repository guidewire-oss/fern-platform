// Package tracing provides a small abstraction over distributed
// tracing. Production wires this to the OpenTelemetry SDK; tests and
// dep-light environments use NoopTracer.
//
// The interface is deliberately tiny — just what the codebase needs.
// We don't try to wrap every OTel feature; for richer use, code can
// reach for go.opentelemetry.io/otel directly once it's wired into
// the platform's module.
package tracing

import (
	"context"

	"github.com/gin-gonic/gin"
)

// Tracer creates spans. Implementations must be safe for concurrent use.
type Tracer interface {
	// Start returns a new span and a context carrying it. The caller
	// must End() the span to flush attributes to the backend.
	Start(ctx context.Context, name string) (context.Context, Span)
}

// Span is the per-operation handle. End must be called exactly once.
type Span interface {
	SetAttribute(key string, value any)
	RecordError(err error)
	End()
}

// NoopTracer satisfies Tracer without producing any output. Useful
// when no tracing backend is configured (the default).
type NoopTracer struct{}

func (NoopTracer) Start(ctx context.Context, _ string) (context.Context, Span) {
	return ctx, noopSpan{}
}

type noopSpan struct{}

func (noopSpan) SetAttribute(string, any) {}
func (noopSpan) RecordError(error)        {}
func (noopSpan) End()                     {}

// HTTPMiddleware starts a span per Gin request, tagged with the
// matched route pattern (not the literal URL) so trace cardinality
// stays bounded.
//
// Passing nil tracer makes the middleware a no-op, which lets the
// rest of the pipeline boot before tracing is configured.
func HTTPMiddleware(tr Tracer) gin.HandlerFunc {
	if tr == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		route := c.FullPath()
		if route == "" {
			route = "(unmatched)"
		}
		ctx, span := tr.Start(c.Request.Context(), c.Request.Method+" "+route)
		defer span.End()

		c.Request = c.Request.WithContext(ctx)

		span.SetAttribute("http.method", c.Request.Method)
		span.SetAttribute("http.route", route)

		c.Next()

		span.SetAttribute("http.status_code", c.Writer.Status())
		if len(c.Errors) > 0 {
			span.RecordError(c.Errors.Last())
		}
	}
}
