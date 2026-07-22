package infrastructure

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// savedViewRow mirrors the saved_views migration. We define the model
// inline here rather than in pkg/database to avoid pulling a v2-only
// concept into the legacy shared package.
type savedViewRow struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     string    `gorm:"column:user_id;index"`
	Page       string    `gorm:"column:page"`
	Name       string    `gorm:"column:name"`
	FilterJSON []byte    `gorm:"column:filter_json"`
	// The DB columns are TIMESTAMP WITH TIME ZONE — using int64 here
	// caused pgx to fail with "unable to encode <unix> into timestamptz".
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (savedViewRow) TableName() string { return "saved_views" }

// GormSavedViewRepository persists SavedView rows via GORM. It maps
// the unique-constraint violation on (user_id, page, name) to
// domain.ErrSavedViewConflict so handlers can return 409.
type GormSavedViewRepository struct {
	db *gorm.DB
}

// NewGormSavedViewRepository constructs a repo on an existing GORM handle.
func NewGormSavedViewRepository(db *gorm.DB) *GormSavedViewRepository {
	return &GormSavedViewRepository{db: db}
}

func (r *GormSavedViewRepository) Create(ctx context.Context, v *domain.SavedView) error {
	row := savedViewRow{
		UserID:     v.UserID,
		Page:       v.Page,
		Name:       v.Name,
		FilterJSON: v.FilterJSON,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrSavedViewConflict
		}
		return err
	}
	v.ID = row.ID
	return nil
}

func (r *GormSavedViewRepository) List(ctx context.Context, userID, page string) ([]*domain.SavedView, error) {
	var rows []savedViewRow
	q := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if page != "" {
		q = q.Where("page = ?", page)
	}
	if err := q.Order("name").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.SavedView, len(rows))
	for i, row := range rows {
		out[i] = &domain.SavedView{
			ID:         row.ID,
			UserID:     row.UserID,
			Page:       row.Page,
			Name:       row.Name,
			FilterJSON: row.FilterJSON,
		}
	}
	return out, nil
}

func (r *GormSavedViewRepository) Delete(ctx context.Context, userID string, id uint) error {
	res := r.db.WithContext(ctx).
		Where("user_id = ? AND id = ?", userID, id).
		Delete(&savedViewRow{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// isUniqueViolation looks for both Postgres ("duplicate key") and
// SQLite ("UNIQUE constraint failed") surfacings of the unique
// constraint on saved_views. Driver-agnostic so tests can use SQLite.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "UNIQUE constraint failed")
}
