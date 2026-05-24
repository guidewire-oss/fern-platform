package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
)

// DevAuthOptions configures the local-dev auth bypass middleware.
//
// When Enabled is true, every request is annotated with a synthetic
// admin user and the same context value the OAuth middleware would
// set. This lets the GraphQL resolvers (which short-circuit on
// "user not authenticated") and the REST handlers (which key off
// gin context user_id/role) work end-to-end during a smoke run that
// has AUTH_ENABLED=false.
//
// Production deployments do NOT enable this — Auth.Enabled stays true
// and a real OAuth middleware runs instead. The middleware is a no-op
// when Enabled is false, so the same wiring is safe to leave in place.
type DevAuthOptions struct {
	Enabled bool
}

// DevAdminUserID is the synthetic user injected by the bypass. Kept
// constant so saved-views and other user-scoped tables tie back to a
// recognizable identity in the DB.
const DevAdminUserID = "dev-admin"

// DevAuth returns middleware that installs a synthetic admin user
// when Enabled is true. When false, it is a pass-through.
func DevAuth(opts DevAuthOptions) gin.HandlerFunc {
	if !opts.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	user := &authDomain.User{
		UserID:        DevAdminUserID,
		Email:         "dev-admin@local",
		Name:          "Local Dev Admin",
		FirstName:     "Dev",
		LastName:      "Admin",
		Role:          authDomain.RoleAdmin,
		Status:        authDomain.StatusActive,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	return func(c *gin.Context) {
		// Gin context keys — what REST handlers read.
		c.Set("user_id", user.UserID)
		c.Set("user_name", user.Name)
		c.Set("user_email", user.Email)
		c.Set("role", string(user.Role))
		c.Set("user", user)

		// Request context keys — what GraphQL resolvers read via
		// ctx.Value("user") in helpers.go.
		//nolint:revive,staticcheck // string keys match existing graphql helpers
		ctx := context.WithValue(c.Request.Context(), "user", user)
		ctx = context.WithValue(ctx, "user_id", user.UserID)
		ctx = context.WithValue(ctx, "role", string(user.Role))
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
