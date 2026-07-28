// Fern Platform - Unified platform entry point
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/guidewire-oss/fern-platform/internal/api"
	apiv2 "github.com/guidewire-oss/fern-platform/internal/api/v2"
	"github.com/guidewire-oss/fern-platform/internal/domains"
	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	authInterfaces "github.com/guidewire-oss/fern-platform/internal/domains/auth/interfaces"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	testingapp "github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	testinginfra "github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql"
	"github.com/guidewire-oss/fern-platform/internal/web"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"github.com/guidewire-oss/fern-platform/pkg/metrics"
	"github.com/guidewire-oss/fern-platform/pkg/middleware"
	"github.com/guidewire-oss/fern-platform/pkg/tracing"
	"gorm.io/gorm"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	configManager := config.NewManager()
	if err := configManager.Load(*configPath); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	cfg := config.GetConfig()

	// Initialize logging
	if err := logging.Initialize(&cfg.Logging); err != nil {
		log.Fatalf("Failed to initialize logging: %v", err)
	}

	logger := logging.GetLogger()
	logger.WithService("fern-platform").Info("Starting Fern Platform")

	// Initialize database
	db, err := database.NewDatabase(&cfg.Database)
	if err != nil {
		logger.WithService("fern-platform").WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Run migrations
	logger.WithService("fern-platform").Info("Starting database migrations from path: migrations")
	if err := db.Migrate("migrations"); err != nil {
		logger.WithService("fern-platform").WithError(err).Fatal("Failed to run database migrations")
	}
	logger.WithService("fern-platform").Info("Database migrations completed successfully")

	// Initialize domain factory for DDD architecture
	domainFactory := domains.NewDomainFactory(db.DB, logger, &cfg.Auth)

	// Get domain services directly
	testingService := domainFactory.GetTestingService()
	projectService := domainFactory.GetProjectDomainService()
	tagService := domainFactory.GetTagDomainService()
	flakyDetectionService := domainFactory.GetFlakyDetectionService()
	jiraConnectionService := domainFactory.GetJiraConnectionService()
	jiraFieldMappingService := domainFactory.GetJiraFieldMappingService()
	summaryHandler := domainFactory.GetSummaryHandler()
	authMiddleware := domainFactory.GetAuthMiddleware()

	// Initialize HTTP server
	if cfg.Server.Host == "0.0.0.0" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.LoggingMiddleware(logger))
	router.Use(middleware.RecoveryMiddleware(logger))
	router.Use(middleware.SecurityHeadersMiddleware())
	router.Use(middleware.HealthCheckMiddleware())

	// Local-dev auth bypass. When AUTH_ENABLED=false (docker-compose
	// smoke, single-user dev), inject a synthetic admin user so the
	// GraphQL resolvers and REST handlers that require an authenticated
	// principal can serve seeded data without an OAuth flow. No effect
	// when auth is enabled (production).
	if !cfg.Auth.Enabled {
		// Make sure the synthetic admin exists in the users table —
		// FKs like saved_views.user_id require a real row.
		if err := ensureDevAdminUser(domainFactory.GetUserRepository(), logger); err != nil {
			logger.WithService("fern-platform").WithError(err).
				Warn("failed to upsert dev-admin user; saved-view writes may fail")
		}
	}
	router.Use(middleware.DevAuth(middleware.DevAuthOptions{
		Enabled: !cfg.Auth.Enabled,
	}))

	// HTTP metrics recorder. In-memory by default; production wires a
	// Prometheus-backed Recorder via build tags or a small adapter.
	metricsRecorder := metrics.NewInMemoryRecorder()
	router.Use(metrics.HTTPMiddleware(metricsRecorder))

	// Distributed tracing scaffold. Default is a no-op tracer; a
	// production deploy wires the OpenTelemetry SDK via an adapter
	// behind the Tracer interface.
	router.Use(tracing.HTTPMiddleware(tracing.NoopTracer{}))

	// Strict CSP on HTML responses. Self-hosters with extra connect-src
	// origins (Datadog, custom CDN) set FERN_CSP_EXTRA_CONNECT_SRC as a
	// space-separated list.
	router.Use(middleware.CSP(middleware.CSPOptions{
		ExtraConnectSrc: splitSpace(os.Getenv("FERN_CSP_EXTRA_CONNECT_SRC")),
	}))

	// v1 deprecation signaling (RFC 8594). Off by default; operators
	// flip FERN_V1_DEPRECATED=true once v2 has GA'd. Sunset date comes
	// from FERN_V1_SUNSET_DATE (RFC 3339) or defaults to 12 months out.
	if strings.EqualFold(os.Getenv("FERN_V1_DEPRECATED"), "true") {
		sunset := time.Now().AddDate(1, 0, 0)
		if v := os.Getenv("FERN_V1_SUNSET_DATE"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				sunset = t
			}
		}
		link := os.Getenv("FERN_V1_MIGRATION_GUIDE_URL")
		router.Use(middleware.DeprecationOnPrefix("/api/v1", middleware.DeprecationOptions{
			Enabled: true,
			Sunset:  sunset,
			Link:    link,
		}))
	}

	// CORS configuration
	if gin.Mode() == gin.DebugMode {
		router.Use(middleware.DevCORSMiddleware())
	} else {
		corsConfig := middleware.DefaultCORSConfig()
		router.Use(middleware.NewCORSMiddleware(corsConfig))
	}

	// Initialize domain-based API handler (V2 split handler architecture)
	domainHandler := api.NewDomainHandlerV2(
		testingService,
		projectService,
		tagService,
		flakyDetectionService,
		jiraConnectionService,
		summaryHandler,
		authMiddleware,
		domainFactory.GetUserRepository(),
		db.DB,
		logger,
	)
	domainHandler.RegisterRoutes(router)
	logger.WithService("fern-platform").Info("Using split handler architecture (V2)")

	// GraphQL routes with role group names from config
	// Initialize GraphQL resolver with domain services
	resolver := graphql.NewResolver(testingService, projectService, tagService, flakyDetectionService, jiraConnectionService, jiraFieldMappingService, db.DB, logger)
	resolver.SetCoverageService(domainFactory.GetCoverageService())

	roleGroupNames := &graphql.RoleGroupNames{
		AdminGroup:   cfg.Auth.OAuth.AdminGroupName,
		ManagerGroup: cfg.Auth.OAuth.ManagerGroupName,
		UserGroup:    cfg.Auth.OAuth.UserGroupName,
	}

	gqlHandler := graphql.NewHandler(resolver, roleGroupNames)
	gqlHandler.RegisterRoutes(router, authMiddleware)

	// v2 surface (RFC-004). Off by default; users opt in with
	// FERN_V2_UI_ENABLED=true. See docs/specs/frontend-modernization.
	if strings.EqualFold(os.Getenv("FERN_V2_UI_ENABLED"), "true") {
		queryRepo := testinginfra.NewTestRunQueryRepo(db.DB)
		// Facet aggregates over millions of suite_runs are expensive
		// (large GROUP BYs). Cache aggressively — 5 min TTL.
		facetCache := testingapp.NewMemoryFacetCache(5 * time.Minute)
		// Test runs store only a project_id; the resolver batches the
		// project_details lookup so the list and its project facet can
		// show display names.
		queryService := testingapp.NewTestRunQueryService(queryRepo, facetCache).
			WithProjectNames(testinginfra.NewProjectNameRepo(db.DB))
		savedViewRepo := testinginfra.NewGormSavedViewRepository(db.DB)

		apiv2.MountV2(apiv2.MountOptions{
			Engine:        router,
			TestRunSvc:    queryService,
			TrendsSvc:     testingService, // shares the TestRunService — implements AggregateDailyByProjects
			SavedViewRepo: savedViewRepo,
			// VitalSink left nil: the telemetry endpoint accepts and
			// drops vitals until a Prometheus-backed sink is wired.

			// Auth: when AUTH_ENABLED=true, apply the same middleware
			// that protects /api/v1. When AUTH_ENABLED=false (local
			// smoke), authMiddleware.RequireAuth() is a permissive
			// passthrough that injects a synthetic "dev-admin" user,
			// so the group is still safe to leave wired.
			Auth: authMiddleware.RequireAuth(),
			// Team-based authorization for the read endpoints, mirroring
			// the v1 GraphQL rule. Without it a non-admin could read any
			// team's runs via /api/v2/test-runs?project=<id>.
			Scope: v2ProjectScope{
				projects:  projectService,
				adminName: cfg.Auth.OAuth.AdminGroupName,
				mgrName:   cfg.Auth.OAuth.ManagerGroupName,
				userName:  cfg.Auth.OAuth.UserGroupName,
			},
		})
		logger.WithService("fern-platform").Info("v2 API surface mounted at /api/v2")
	}

	// Prometheus text exposition for scrapers. The format is
	// version 0.0.4; PrometheusContentType matches what scrapers
	// expect by default, so no scrape_config tweaks are needed.
	router.GET("/metrics", func(c *gin.Context) {
		body := metrics.PrometheusExposition(metricsRecorder)
		c.Data(http.StatusOK, metrics.PrometheusContentType, []byte(body))
	})

	// k8s-style health endpoints. /healthz is liveness (cheap),
	// /readyz is readiness (pings DB). The legacy /health route
	// above continues to work for any existing probes.
	apiv2.RegisterHealthRoutes(router, dbPinger{db: db.DB})

	// v2 SPA mounted at /v2/* (RFC-004 FR-25: legacy UI remains at /
	// until parity is verified). Gated on the same FERN_V2_UI_ENABLED
	// flag as the /api/v2 surface — mounting the SPA without its API
	// produces a half-broken page where every fetch 404s, so the two
	// must flip together.
	if strings.EqualFold(os.Getenv("FERN_V2_UI_ENABLED"), "true") {
		if err := web.RegisterAtPrefix(router, "/v2"); err != nil {
			logger.WithService("fern-platform").WithError(err).Fatal("failed to mount v2 SPA")
		}
		logger.WithService("fern-platform").Info("v2 SPA mounted at /v2/")
	}

	// /favicon.ico — every browser requests this even when the page
	// sets a data-URI link. Serving an inline 🌿 SVG silences the
	// 404 warning and avoids a wasted round-trip on every page load.
	router.GET("/favicon.ico", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
		c.Data(http.StatusOK, "image/svg+xml",
			[]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">🌿</text></svg>`))
	})

	// Note: Static file serving is handled by the API handler

	// Create HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		logger.WithService("fern-platform").
			WithFields(map[string]interface{}{
				"host": cfg.Server.Host,
				"port": cfg.Server.Port,
			}).Info("Starting Fern Platform HTTP server")

		if cfg.Server.TLS.Enabled {
			if err := srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile); err != nil && err != http.ErrServerClosed {
				logger.WithService("fern-platform").WithError(err).Fatal("Failed to start HTTPS server")
			}
		} else {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.WithService("fern-platform").WithError(err).Fatal("Failed to start HTTP server")
			}
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.WithService("fern-platform").Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithService("fern-platform").WithError(err).Fatal("Server forced to shutdown")
	}

	logger.WithService("fern-platform").Info("Server exited")
}

// splitSpace tokenizes a space-separated env var into a slice with
// empties removed. Used for FERN_CSP_EXTRA_CONNECT_SRC so operators
// can pass a quoted list in their config without YAML overhead.
func splitSpace(s string) []string {
	if s == "" {
		return nil
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// dbPinger adapts gorm.DB to the apiv2.Pinger interface used by
// /readyz. We avoid pulling gorm into the v2 package — the readiness
// probe needs only Ping, and decoupling lets the v2 surface be
// reused in tests without a real DB.
type dbPinger struct {
	db *gorm.DB
}

func (p dbPinger) Ping(ctx context.Context) error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// ensureDevAdminUser upserts the synthetic admin row that DevAuth
// references. Without this, anything that has a foreign key to
// users(user_id) — saved_views, project_access — fails to insert
// when called by the bypass-injected dev-admin.
func ensureDevAdminUser(repo authDomain.UserRepository, logger *logging.Logger) error {
	ctx := context.Background()
	if _, err := repo.FindByID(ctx, middleware.DevAdminUserID); err == nil {
		return nil
	}
	now := time.Now()
	user := &authDomain.User{
		UserID:        middleware.DevAdminUserID,
		Email:         "dev-admin@local",
		Name:          "Local Dev Admin",
		FirstName:     "Dev",
		LastName:      "Admin",
		Role:          authDomain.RoleAdmin,
		Status:        authDomain.StatusActive,
		EmailVerified: true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := repo.Create(ctx, user); err != nil {
		// Race: a parallel boot may have inserted it between FindByID
		// and Create. Re-check rather than treat as fatal.
		if _, ferr := repo.FindByID(ctx, middleware.DevAdminUserID); ferr == nil {
			return nil
		}
		return err
	}
	logger.WithService("fern-platform").Info("seeded synthetic dev-admin user (auth disabled)")
	return nil
}

// v2ProjectScope implements apiv2.ProjectScoper for the /api/v2 read
// surface. It mirrors the team-based rule the v1 GraphQL resolvers enforce
// (see getAccessibleProjectSnapshots / userCanAccessProject): a non-admin
// may read only projects owned by one of their teams, where a team is any
// of the caller's groups that is not a role group.
//
// It lives in main.go (not a sibling file) because the fern-platform image
// is built with `go build cmd/fern-platform/main.go`, a single-file build
// that ignores other files in package main.
type v2ProjectScope struct {
	projects  *projectsApp.ProjectService
	adminName string
	mgrName   string
	userName  string
}

// AccessibleProjects returns the project ids the caller may read.
// unrestricted is true for admins (Role == admin), matching the v1 rule
// where only admins bypass team scoping. Any error fails closed.
func (s v2ProjectScope) AccessibleProjects(c *gin.Context) (map[string]struct{}, bool, error) {
	user, ok := authInterfaces.GetAuthUser(c)
	if !ok || user == nil {
		return nil, false, fmt.Errorf("no authenticated user")
	}
	if user.IsAdmin() {
		return nil, true, nil
	}

	teams := make(map[string]bool)
	for _, g := range user.Groups {
		name := strings.TrimPrefix(g.GroupName, "/")
		if name == "" || name == s.adminName || name == s.mgrName || name == s.userName {
			continue
		}
		teams[name] = true
	}

	// Same 1000-project cap the GraphQL path uses. Acceptable for now;
	// the shared TODO is to push team filtering into the query.
	projects, _, err := s.projects.ListProjects(c.Request.Context(), 1000, 0)
	if err != nil {
		return nil, false, err
	}
	allowed := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		snap := p.ToSnapshot()
		if snap.Team != "" && teams[string(snap.Team)] {
			allowed[string(snap.ProjectID)] = struct{}{}
		}
	}
	return allowed, false, nil
}
