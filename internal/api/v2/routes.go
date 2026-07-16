package v2

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/pkg/middleware"
)

// MountOptions wires the v2 surface into an existing Gin engine.
//
// The current PR introduces the v2 handler in isolation. main.go will
// call MountV2 in a follow-up PR once the env-flag and feature-flagged
// rollout per docs/specs/frontend-modernization/migration-guide.md is
// agreed. Wiring is exposed as a function (not auto-registered) so the
// integration point is explicit and reviewable.
type MountOptions struct {
	Engine        *gin.Engine
	TestRunSvc    TestRunQueryService
	TrendsSvc     TestRunTrendsService
	SavedViewRepo SavedViewRepo
	VitalSink     WebVitalSink
	UserID        UserIDProvider // optional; defaults to DefaultUserIDProvider
	// Scope is the authorization boundary applied to the test-run and
	// trends read endpoints. Nil disables per-project team scoping —
	// appropriate only for local AUTH_ENABLED=false runs (synthetic admin).
	Scope ProjectScoper
	// Auth is applied to every /api/v2 route except telemetry. Pass
	// the same middleware that protects /api/v1 so v1 and v2 behave
	// identically with respect to authentication. Nil leaves the v2
	// group open — appropriate only for local development with
	// AUTH_ENABLED=false (DevAuth injects a synthetic user upstream).
	Auth gin.HandlerFunc
}

// MountV2 registers the /api/v2/* routes on opts.Engine.
//
// MountV2 is safe to call after the v1 routes are registered; v1 and
// v2 live on separate Gin groups and do not compete for paths.
//
// Nil dependencies skip the routes they would have served, so a
// partial wiring during rollout still produces a working server.
func MountV2(opts MountOptions) {
	// Telemetry is mounted without auth so unauthenticated browsers
	// (e.g., on the login page) can still report Web Vitals. The
	// authed surface is everything else.
	publicGroup := opts.Engine.Group("/api/v2")
	NewTelemetryHandler(opts.VitalSink).Register(publicGroup)

	authedGroup := opts.Engine.Group("/api/v2")
	if opts.Auth != nil {
		authedGroup.Use(opts.Auth)
	}
	if opts.TestRunSvc != nil {
		NewTestRunHandler(opts.TestRunSvc).WithScope(opts.Scope).Register(authedGroup)
	}
	if opts.TrendsSvc != nil {
		NewTestRunTrendsHandler(opts.TrendsSvc, opts.UserID).WithScope(opts.Scope).Register(authedGroup)
	}
	if opts.SavedViewRepo != nil {
		NewSavedViewHandler(opts.SavedViewRepo, opts.UserID).Register(authedGroup)
	}
}

// MountV1Deprecation attaches RFC 8594 sunset headers to the existing
// v1 router group. Pass the same group used by the v1 handler
// registration. The middleware is a no-op when opts.Enabled is false,
// which lets v2 ship in dark mode before sunset is announced.
func MountV1Deprecation(g *gin.RouterGroup, sunset time.Time, link string, enabled bool) {
	g.Use(middleware.Deprecation(middleware.DeprecationOptions{
		Enabled: enabled,
		Sunset:  sunset,
		Link:    link,
	}))
}
