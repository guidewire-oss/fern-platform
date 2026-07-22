package domain

import (
	"context"
	"time"
)

// UserRepository defines the interface for user persistence
type UserRepository interface {
	// User operations
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	FindByID(ctx context.Context, userID string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByIDOrEmail(ctx context.Context, userID, email string) (*User, error)
	UpdateLastLogin(ctx context.Context, userID string, loginTime time.Time) error
	// List returns a page of users + the total row count, ordered by
	// name. Used by the admin user-management UI; the listing endpoint
	// is intentionally separate from the query-by-ID path so callers
	// don't accidentally bulk-load via FindByID().
	List(ctx context.Context, limit, offset int) ([]*User, int64, error)
	// UpdateRole writes the role column for a given user. Returns
	// ErrNotFound (the testing-domain sentinel; auth uses its own
	// equivalent surfacing) when no row matches.
	UpdateRole(ctx context.Context, userID string, role UserRole) error

	// UpdateStatus writes the status column for admin suspend/activate
	// operations. Returns "user not found" when no row matches.
	UpdateStatus(ctx context.Context, userID string, status UserStatus) error

	// SoftDelete sets deleted_at on a user (gorm soft-delete pattern).
	// The row stays in the DB but is excluded from default queries.
	// Returns "user not found" when no row matches.
	SoftDelete(ctx context.Context, userID string) error

	// Group operations
	SetUserGroups(ctx context.Context, userID string, groups []string) error
	GetUserGroups(ctx context.Context, userID string) ([]UserGroup, error)

	// Scope operations
	GrantScope(ctx context.Context, scope UserScope) error
	RevokeScope(ctx context.Context, userID, scope string) error
	GetUserScopes(ctx context.Context, userID string) ([]UserScope, error)
}

// SessionRepository defines the interface for session persistence
type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	FindByID(ctx context.Context, sessionID string) (*Session, error)
	FindActiveByID(ctx context.Context, sessionID string) (*Session, error)
	UpdateActivity(ctx context.Context, sessionID string) error
	Invalidate(ctx context.Context, sessionID string) error
	InvalidateAllForUser(ctx context.Context, userID string) error
	CleanupExpired(ctx context.Context) error
}
