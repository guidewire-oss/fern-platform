// Fern Platform - Unified platform entry point

// @title           Fern Platform API
// @version         1.0
// @description     Unified test intelligence platform — aggregate, analyze and act on test results from any CI/CD pipeline.
// @termsOfService  https://github.com/guidewire-oss/fern-platform

// @contact.name   Fern Platform Team
// @contact.url    https://github.com/guidewire-oss/fern-platform/issues

// @license.name  Apache 2.0
// @license.url   https://www.apache.org/licenses/LICENSE-2.0.html

// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT bearer token. Format: "Bearer <token>"

// @tag.name health
// @tag.description Health and readiness checks

// @tag.name test-runs
// @tag.description Test run submission and retrieval

// @tag.name projects
// @tag.description Project management

// @tag.name tags
// @tag.description Test tagging

// @tag.name flaky-tests
// @tag.description Flaky test detection

// @tag.name jira
// @tag.description JIRA integration

// @tag.name auth
// @tag.description Authentication and user management

// @tag.name admin
// @tag.description Admin-only operations

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	_ "github.com/guidewire-oss/fern-platform/docs"
	api "github.com/guidewire-oss/fern-platform/internal/api"
	"github.com/guidewire-oss/fern-platform/internal/domains"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"github.com/guidewire-oss/fern-platform/pkg/middleware"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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
	defer func(db *database.DB) {
		err := db.Close()
		if err != nil {
			logger.WithService("fern-platform").WithError(err).Fatal("Failed to close database connection")
		}
	}(db)

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

	// Swagger UI is intentionally unauthenticated so that developers and
	// API consumers can explore the API without needing a token. The UI
	// only serves generated documentation — no data is exposed through it.
	// If this instance needs to be locked down, wrap the handler with
	// authMiddleware.RequireAuth() before deploying to a restricted environment.
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
