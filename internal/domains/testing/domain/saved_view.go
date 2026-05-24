package domain

import (
	"context"
	"errors"
	"time"
)

// ErrSavedViewConflict is returned when a user already has a view
// with the same (page, name).
var ErrSavedViewConflict = errors.New("saved view: name already taken on this page")

// SavedView is a user-named filter preset for a given list page.
//
// FilterJSON stays as []byte at this layer; the application service
// is responsible for marshalling concrete filter types in and out.
// This keeps the saved-views surface agnostic to the shape of any
// one page's filter.
type SavedView struct {
	ID         uint
	UserID     string
	Page       string
	Name       string
	FilterJSON []byte
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// SavedViewRepository persists user-scoped filter presets.
type SavedViewRepository interface {
	Create(ctx context.Context, v *SavedView) error
	List(ctx context.Context, userID, page string) ([]*SavedView, error)
	Delete(ctx context.Context, userID string, id uint) error
}
