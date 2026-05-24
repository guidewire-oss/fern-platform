package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// CSPOptions configures the strict Content-Security-Policy emitted on
// HTML responses. The defaults match RFC-004 § Security:
//
//	default-src 'self'; script-src 'self';
//	style-src 'self' 'unsafe-inline'; img-src 'self' data:;
//	connect-src 'self';
//
// 'unsafe-inline' on style-src is a concession to Tailwind's inline
// generated styles; we accept this trade-off in exchange for dropping
// the legacy 'unsafe-inline' on script-src which the old UI required.
//
// Self-hosters whose deployment fetches metrics or images from another
// origin add to those source lists via the Extra* fields rather than
// editing the policy string directly.
type CSPOptions struct {
	ExtraConnectSrc []string
	ExtraImgSrc     []string
}

// CSP returns middleware that sets a strict Content-Security-Policy
// header on HTML responses. JSON/JS/CSS responses are unaffected;
// browsers don't apply CSP to non-document loads, so emitting the
// header there only adds noise to network panels.
func CSP(opts CSPOptions) gin.HandlerFunc {
	policy := buildPolicy(opts)
	return func(c *gin.Context) {
		c.Next()
		ct := c.Writer.Header().Get("Content-Type")
		if strings.HasPrefix(ct, "text/html") {
			c.Writer.Header().Set("Content-Security-Policy", policy)
		}
	}
}

func buildPolicy(opts CSPOptions) string {
	connect := joinSources([]string{"'self'"}, opts.ExtraConnectSrc)
	img := joinSources([]string{"'self'", "data:"}, opts.ExtraImgSrc)
	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src " + img,
		"connect-src " + connect,
		"font-src 'self' data:",
		"object-src 'none'",
		"base-uri 'self'",
		"frame-ancestors 'none'",
		"form-action 'self'",
	}, "; ")
}

func joinSources(base, extra []string) string {
	out := append([]string{}, base...)
	for _, s := range extra {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return strings.Join(out, " ")
}
