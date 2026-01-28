package infrastructure

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

// GormTagRepository is a GORM implementation of TagRepository
type GormTagRepository struct {
	db *gorm.DB
}

// NewGormTagRepository creates a new GORM tag repository
func NewGormTagRepository(db *gorm.DB) *GormTagRepository {
	return &GormTagRepository{db: db}
}

// Save persists a tag
func (r *GormTagRepository) Save(ctx context.Context, tag *domain.Tag) error {
	// Convert domain model to database model
	snapshot := tag.ToSnapshot()
	dbTag := &database.Tag{
		Name:     snapshot.Name,
		Category: snapshot.Category,
		Value:    snapshot.Value,
	}

	if err := r.db.WithContext(ctx).Create(dbTag).Error; err != nil {
		return fmt.Errorf("failed to save tag: %w", err)
	}

	// Note: In a real implementation, we'd need a way to set the ID back on the domain model
	// This is a limitation of the current domain design

	return nil
}

// FindByID retrieves a tag by ID
func (r *GormTagRepository) FindByID(ctx context.Context, id domain.TagID) (*domain.Tag, error) {
	// Convert TagID to uint
	idUint, err := strconv.ParseUint(string(id), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid tag ID format: %w", err)
	}

	var dbTag database.Tag
	if err := r.db.WithContext(ctx).First(&dbTag, uint(idUint)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tag not found")
		}
		return nil, fmt.Errorf("failed to find tag: %w", err)
	}

	return r.toDomainModel(&dbTag)
}

// FindByName retrieves a tag by name
func (r *GormTagRepository) FindByName(ctx context.Context, name string) (*domain.Tag, error) {
	// Normalize the name for search
	normalizedName := strings.TrimSpace(strings.ToLower(name))

	var dbTag database.Tag
	if err := r.db.WithContext(ctx).Where("LOWER(name) = ?", normalizedName).First(&dbTag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("tag not found")
		}
		return nil, fmt.Errorf("failed to find tag: %w", err)
	}

	return r.toDomainModel(&dbTag)
}

// FindAll retrieves all tags
func (r *GormTagRepository) FindAll(ctx context.Context) ([]*domain.Tag, error) {
	var dbTags []database.Tag
	if err := r.db.WithContext(ctx).Order("name").Find(&dbTags).Error; err != nil {
		return nil, fmt.Errorf("failed to find tags: %w", err)
	}

	tags := make([]*domain.Tag, len(dbTags))
	for i, dbTag := range dbTags {
		tag, err := r.toDomainModel(&dbTag)
		if err != nil {
			return nil, err
		}
		tags[i] = tag
	}

	return tags, nil
}

// Delete removes a tag
func (r *GormTagRepository) Delete(ctx context.Context, id domain.TagID) error {
	// Convert TagID to uint
	idUint, err := strconv.ParseUint(string(id), 10, 32)
	if err != nil {
		return fmt.Errorf("invalid tag ID format: %w", err)
	}

	// Delete the tag and its associations
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete test run tag associations
		if err := tx.Where("tag_id = ?", uint(idUint)).Delete(&database.TestRunTag{}).Error; err != nil {
			return fmt.Errorf("failed to delete tag associations: %w", err)
		}

		// Delete the tag
		if err := tx.Delete(&database.Tag{}, uint(idUint)).Error; err != nil {
			return fmt.Errorf("failed to delete tag: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

// AssignToTestRun assigns tags to a test run
func (r *GormTagRepository) AssignToTestRun(ctx context.Context, testRunID string, tagIDs []domain.TagID) error {
	// Convert testRunID to uint
	testRunIDUint, err := strconv.ParseUint(testRunID, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid test run ID format: %w", err)
	}

	// Begin transaction
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove existing tag associations for this test run
		if err := tx.Where("test_run_id = ?", uint(testRunIDUint)).Delete(&database.TestRunTag{}).Error; err != nil {
			return fmt.Errorf("failed to remove existing tag associations: %w", err)
		}

		// Create new associations
		for _, tagID := range tagIDs {
			tagIDUint, err := strconv.ParseUint(string(tagID), 10, 32)
			if err != nil {
				return fmt.Errorf("invalid tag ID format: %w", err)
			}

			testRunTag := &database.TestRunTag{
				TestRunID: uint(testRunIDUint),
				TagID:     uint(tagIDUint),
			}

			if err := tx.Create(testRunTag).Error; err != nil {
				return fmt.Errorf("failed to assign tag to test run: %w", err)
			}
		}

		return nil
	})
}

// toDomainModel converts a database model to a domain model
func (r *GormTagRepository) toDomainModel(dbTag *database.Tag) (*domain.Tag, error) {
	// Use ReconstructTag to create domain model with all fields from database
	return domain.ReconstructTag(
		domain.TagID(fmt.Sprintf("%d", dbTag.ID)),
		dbTag.Name,
		dbTag.Category,
		dbTag.Value,
		dbTag.CreatedAt,
	), nil
}

// GetOrCreateTag gets an existing tag by name or creates a new one
func (r *GormTagRepository) GetOrCreateTag(ctx context.Context, name string) (*domain.Tag, error) {
	// Try to find existing tag
	tag, err := r.FindByName(ctx, name)
	if err == nil {
		return tag, nil
	}

	// Create new tag if not found
	newTag, err := domain.NewTag(name)
	if err != nil {
		return nil, err
	}

	if err := r.Save(ctx, newTag); err != nil {
		// Check if another process created it concurrently
		if strings.Contains(err.Error(), "duplicate key") {
			return r.FindByName(ctx, name)
		}
		return nil, err
	}

	// Retrieve the saved tag to get the ID
	return r.FindByName(ctx, name)
}
