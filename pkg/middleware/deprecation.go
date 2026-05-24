package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// DeprecationOptions configures the Deprecation middleware.
//
// When Enabled is false the middleware is a no-op, which lets the
// server keep v1 endpoints unmarked until v2 GA without rewiring routes.
type DeprecationOptions struct {
	Enabled bool
	Sunset  time.Time // emitted in IMF-fixdate per RFC 7231 / RFC 8594
	Link    string    // optional migration-guide URL
}

// DeprecationOnPrefix returns middleware that emits the deprecation
// headers only when the request path starts with prefix. Useful for
// router-level mounting where the v1 group is owned by another
// package and cannot be wrapped directly.
func DeprecationOnPrefix(prefix string, opts DeprecationOptions) gin.HandlerFunc {
	inner := Deprecation(opts)
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, prefix) {
			inner(c)
			return
		}
		c.Next()
	}
}

// Deprecation emits RFC 8594 deprecation signaling headers on every
// response that flows through it:
//
//	Deprecation: true
//	Sunset: <IMF-fixdate>
//	Link: <url>; rel="deprecation"
//
// Mount on the v1 route group once v2 is GA. Existing client libraries
// that surface response headers will then warn their users automatically.
func Deprecation(opts DeprecationOptions) gin.HandlerFunc {
	if !opts.Enabled {
		return func(c *gin.Context) { c.Next() }
	}
	sunset := opts.Sunset.UTC().Format(http.TimeFormat)
	var link string
	if opts.Link != "" {
		link = fmt.Sprintf(`<%s>; rel="deprecation"`, opts.Link)
	}
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Deprecation", "true")
		h.Set("Sunset", sunset)
		if link != "" {
			h.Set("Link", link)
		}
		c.Next()
	}
}
