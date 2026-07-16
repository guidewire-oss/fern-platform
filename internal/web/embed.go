// Package web embeds the Vite-built single-page application into the
// Go binary and registers Gin routes that serve it.
//
// Build pipeline: `cd web && pnpm build` writes to web/dist/, which
// the Makefile copies to internal/web/dist/ (or, in CI, a multi-stage
// Dockerfile drops it there directly). The //go:embed directive
// captures whatever is in dist/ at compile time.
package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

// reservedPrefixes are the URL prefixes the SPA must never swallow.
// A request to one of these that does not match a registered route
// returns 404, not the index page — otherwise broken API clients
// would receive a 200 with HTML and never notice their bug.
var reservedPrefixes = []string{
	"/api/",
	"/graphql",
	"/auth/",
	"/health",
	"/metrics",
}

// Register mounts asset serving and a SPA index fallback on r.
//
// Call this last, after all API/GraphQL/auth routes are registered.
// Gin's NoRoute handler is shared, so registering web last lets the
// other handlers win path matches.
func Register(r *gin.Engine) error {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return fmt.Errorf("web: %w", err)
	}

	indexBytes, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return fmt.Errorf("web: read index.html: %w", err)
	}

	// Hashed assets are served via http.FileServer over the embedded
	// filesystem. Vite emits them under /assets/*; matching there keeps
	// the SPA fallback below cheap.
	assetServer := http.StripPrefix("/assets/", http.FileServer(http.FS(mustSub(sub, "assets"))))
	r.GET("/assets/*filepath", gin.WrapH(assetServer))

	// SPA fallback: any unmatched path returns index.html unless it
	// targets a reserved API/auth prefix.
	//
	// index.html must never be cached: it names the content-hashed asset
	// bundle, so a browser holding a stale index keeps loading an old SPA
	// after a deploy (the assets themselves are immutable-cached by hash).
	// Without this header the app appeared "not updated" after deploys.
	serveIndex := func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
	}
	r.GET("/", serveIndex)

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		for _, prefix := range reservedPrefixes {
			if strings.HasPrefix(p, prefix) {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		}
		serveIndex(c)
	})

	return nil
}

func mustSub(parent fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(parent, dir)
	if err != nil {
		// dist/assets must exist as a build invariant; missing means
		// the build was misconfigured and we want it to fail loudly.
		panic(fmt.Sprintf("web: sub %s: %v", dir, err))
	}
	return sub
}

// RegisterAtPrefix mounts the embedded SPA at a non-root prefix
// (e.g. "/v2") so it can coexist with the legacy UI at "/". Vite
// must be built with `base: '<prefix>/'` so the asset URLs in the
// emitted index.html match `<prefix>/assets/...`.
//
// Unlike Register, this function does NOT install a NoRoute handler
// — the existing root handler keeps owning everything outside the
// prefix.
func RegisterAtPrefix(r *gin.Engine, prefix string) error {
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return Register(r)
	}

	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return fmt.Errorf("web: %w", err)
	}
	indexBytes, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return fmt.Errorf("web: read index.html: %w", err)
	}

	// Hashed assets under <prefix>/assets/*. Vite emits these with
	// content-hashed filenames so a year-long Cache-Control is safe.
	assetServer := http.StripPrefix(prefix+"/assets/", http.FileServer(http.FS(mustSub(sub, "assets"))))
	r.GET(prefix+"/assets/*filepath", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		assetServer.ServeHTTP(c.Writer, c.Request)
	})

	// index.html at the bare prefix. The wider "any unmatched path
	// under <prefix>" case is handled by NoRoute below — Gin's
	// radix tree won't accept both /prefix/*subpath and the more
	// specific /prefix/assets/*filepath at the same time.
	serveIndex := func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
	}
	r.GET(prefix, serveIndex)
	r.GET(prefix+"/", serveIndex)

	// SPA-route fallback: client-side routes like /v2/projects/abc
	// fall through to NoRoute. If the unmatched path starts with the
	// prefix (and isn't an API path), serve index.html and let the
	// browser router take it from there.
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if !strings.HasPrefix(p, prefix+"/") && p != prefix {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		for _, rp := range reservedPrefixes {
			if strings.HasPrefix(p, rp) {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		}
		serveIndex(c)
	})

	return nil
}
