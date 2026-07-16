// Package graphql provides GraphQL HTTP handlers
package graphql

import (
	"context"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	authInterfaces "github.com/guidewire-oss/fern-platform/internal/domains/auth/interfaces"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/dataloader"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/generated"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Handler provides GraphQL HTTP handlers
type Handler struct {
	resolver       *Resolver
	server         *handler.Server
	roleGroupNames *RoleGroupNames
}

// NewHandler creates a new GraphQL handler with performance optimizations
func NewHandler(resolver *Resolver, roleGroupNames *RoleGroupNames) *Handler {
	// Create GraphQL server with optimizations
	srv := handler.NewDefaultServer(
		generated.NewExecutableSchema(
			generated.Config{
				Resolvers: resolver,
			},
		),
	)

	// Add performance extensions
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100), // Cache 100 queries
	})

	// Configure transports
	srv.AddTransport(&transport.Websocket{
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow connections from any origin in development
				// In production, this should be more restrictive
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		KeepAlivePingInterval: 10 * time.Second,
		InitFunc: func(ctx context.Context, initPayload transport.InitPayload) (context.Context, *transport.InitPayload, error) {
			// Extract auth token from connection params if needed
			return ctx, &initPayload, nil
		},
	})

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{
		MaxUploadSize: 32 << 20, // 32MB
		MaxMemory:     32 << 20, // 32MB
	})

	// Set query complexity limit to prevent abuse
	srv.Use(extension.FixedComplexityLimit(1000))

	// Add custom error handler
	srv.SetErrorPresenter(func(ctx context.Context, e error) *gqlerror.Error {
		err := graphql.DefaultErrorPresenter(ctx, e)

		// Log errors
		if resolver.logger != nil {
			resolver.logger.WithError(e).Error("GraphQL error")
		}

		// Don't expose internal errors in production
		if err.Extensions == nil {
			err.Extensions = make(map[string]interface{})
		}

		// Add request ID if available
		if reqID := ctx.Value("request_id"); reqID != nil {
			err.Extensions["request_id"] = reqID
		}

		return err
	})

	// Add panic recovery
	srv.SetRecoverFunc(func(ctx context.Context, err interface{}) error {
		if resolver.logger != nil {
			resolver.logger.WithField("panic", err).Error("GraphQL panic")
		}
		return graphql.DefaultRecover(ctx, err)
	})

	// Enable field middleware for timing and logging
	srv.AroundFields(func(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
		start := time.Now()

		// Get field context
		fc := graphql.GetFieldContext(ctx)

		// Execute resolver
		res, err = next(ctx)

		// Log slow queries
		duration := time.Since(start)
		if duration > 100*time.Millisecond && resolver.logger != nil {
			resolver.logger.WithFields(map[string]interface{}{
				"field":    fc.Field.Name,
				"duration": duration.String(),
				"path":     fc.Path(),
			}).Warn("Slow GraphQL field")
		}

		return res, err
	})

	return &Handler{
		resolver:       resolver,
		server:         srv,
		roleGroupNames: roleGroupNames,
	}
}

// RegisterRoutes registers GraphQL routes with the Gin router
func (h *Handler) RegisterRoutes(router *gin.Engine, authMiddleware *authInterfaces.AuthMiddlewareAdapter) {
	// GraphQL playground (development only)
	router.GET("/graphql", func(c *gin.Context) {
		// Check if we're in development mode
		if gin.Mode() == gin.ReleaseMode {
			c.JSON(http.StatusNotFound, gin.H{"error": "GraphQL playground disabled in production"})
			return
		}
		playground.Handler("Fern Platform GraphQL", "/query")(c.Writer, c.Request)
	})

	// GraphQL endpoint with authentication
	router.POST("/query", authMiddleware.RequireAuth(), h.graphqlHandler())

	// WebSocket endpoint for subscriptions
	router.GET("/query", authMiddleware.RequireAuth(), h.graphqlHandler())
}

// graphqlHandler returns a Gin handler for GraphQL queries
func (h *Handler) graphqlHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Build dataloaders per request so their internal caches are
		// scoped to a single HTTP request. A process-wide loader would
		// cache the first result a key sees and serve it forever — which
		// previously caused project stats to stick at zero whenever the
		// first read happened before all test_runs had been inserted.
		ctx = context.WithValue(ctx, "loaders", dataloader.NewLoaders(h.resolver.db))

		// Add request metadata
		ctx = context.WithValue(ctx, "request_id", c.GetString("request_id"))

		// Add role group names to context
		ctx = context.WithValue(ctx, "roleGroupNames", h.roleGroupNames)

		// Get user from auth context
		if user, exists := authInterfaces.GetAuthUser(c); exists {
			ctx = context.WithValue(ctx, "user", user)
		}

		// Update request with new context
		c.Request = c.Request.WithContext(ctx)

		// Serve GraphQL
		h.server.ServeHTTP(c.Writer, c.Request)
	}
}
