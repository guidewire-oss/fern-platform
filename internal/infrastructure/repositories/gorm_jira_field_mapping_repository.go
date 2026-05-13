package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

type GormJiraFieldMappingRepository struct {
	db *gorm.DB
}

func NewGormJiraFieldMappingRepository(db *gorm.DB) *GormJiraFieldMappingRepository {
	return &GormJiraFieldMappingRepository{db: db}
}

// Get retrieves the active mapping for a project. Returns nil, nil when none exists.
func (r *GormJiraFieldMappingRepository) Get(ctx context.Context, projectID string) (*integrations.JiraFieldMapping, error) {
	var model database.JiraFieldMapping

	err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get JIRA field mapping: %w", err)
	}

	var entries []integrations.FieldMappingEntry
	if len(model.Entries) > 0 {
		if err := json.Unmarshal(model.Entries, &entries); err != nil {
			return nil, fmt.Errorf("failed to deserialize field mapping entries: %w", err)
		}
	}

	return integrations.ReconstructJiraFieldMapping(
		model.ProjectID,
		entries,
		model.UpdatedBy,
		model.CreatedAt,
		model.UpdatedAt,
	), nil
}

// Upsert creates or replaces the mapping for the project using INSERT … ON CONFLICT DO UPDATE.
func (r *GormJiraFieldMappingRepository) Upsert(ctx context.Context, mapping *integrations.JiraFieldMapping) error {
	snap := mapping.Snapshot()

	entriesJSON, err := json.Marshal(snap.Entries)
	if err != nil {
		return fmt.Errorf("failed to serialize field mapping entries: %w", err)
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Exec(
			`INSERT INTO "jira_field_mappings" ("project_id","entries","updated_by","created_at","updated_at","deleted_at") `+
				`VALUES (?,?,?,?,?,?) `+
				`ON CONFLICT ("project_id") WHERE "deleted_at" IS NULL DO UPDATE SET "entries"=excluded."entries","updated_by"=excluded."updated_by","updated_at"=excluded."updated_at"`,
			snap.ProjectID, json.RawMessage(entriesJSON), snap.UpdatedBy, snap.CreatedAt, snap.UpdatedAt, nil,
		).Error
	})
	if err != nil {
		return fmt.Errorf("failed to upsert JIRA field mapping: %w", err)
	}

	return nil
}

// Delete soft-deletes the mapping for the project by setting deleted_at.
// Uses raw SQL because GORM's soft-delete scope is bypassed when calling tx.Exec directly.
func (r *GormJiraFieldMappingRepository) Delete(ctx context.Context, projectID string) error {
	now := time.Now()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Exec(
			`UPDATE "jira_field_mappings" SET "deleted_at"=? WHERE project_id=? AND "jira_field_mappings"."deleted_at" IS NULL`,
			now, projectID,
		).Error
	})
	if err != nil {
		return fmt.Errorf("failed to delete JIRA field mapping: %w", err)
	}
	return nil
}
