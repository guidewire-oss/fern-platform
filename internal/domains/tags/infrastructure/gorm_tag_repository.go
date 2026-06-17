package infrastructure

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

type coverageRow struct {
	Value        string
	Total        int
	Passed       int
	Failed       int
	Skipped      int
	LastRunAtStr string `gorm:"column:last_run_at"` // SQLite returns datetime as string; parsed below
}

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

// GetJiraTagCoverageByProject returns per-JIRA-issue-key coverage counts for all test runs in a project.
func (r *GormTagRepository) GetJiraTagCoverageByProject(ctx context.Context, projectID string) (map[string]domain.CoverageCount, error) {
	var rows []coverageRow
	// UNION spec-run-level and test-run-level JIRA tags so both tagging
	// granularities contribute to coverage counts.
	err := r.db.WithContext(ctx).Raw(`
		SELECT UPPER(t.value)                                              AS value,
		       COUNT(*)                                                    AS total,
		       SUM(CASE WHEN tagged.status = 'passed'  THEN 1 ELSE 0 END) AS passed,
		       SUM(CASE WHEN tagged.status = 'failed'  THEN 1 ELSE 0 END) AS failed,
		       SUM(CASE WHEN tagged.status = 'skipped' THEN 1 ELSE 0 END) AS skipped,
		       MAX(tagged.run_at)                                          AS last_run_at
		FROM tags t
		JOIN (
		    SELECT srt.tag_id, sr.status, sr.start_time AS run_at
		    FROM   spec_run_tags srt
		    JOIN   spec_runs  sr ON sr.id  = srt.spec_run_id
		    JOIN   suite_runs su ON su.id  = sr.suite_run_id
		    JOIN   test_runs  tr ON tr.id  = su.test_run_id
		    WHERE  tr.project_id = ? AND sr.deleted_at IS NULL AND su.deleted_at IS NULL AND tr.deleted_at IS NULL

		    UNION ALL

		    SELECT trt.tag_id, tr.status, tr.start_time AS run_at
		    FROM   test_run_tags trt
		    JOIN   test_runs tr ON tr.id = trt.test_run_id
		    WHERE  tr.project_id = ? AND tr.deleted_at IS NULL
		) tagged ON tagged.tag_id = t.id
		WHERE t.category = 'jira'
		GROUP BY UPPER(t.value)
		LIMIT 1000
	`, projectID, projectID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query jira tag coverage: %w", err)
	}

	result := make(map[string]domain.CoverageCount, len(rows))
	for _, row := range rows {
		var lastRunAt *time.Time
		if row.LastRunAtStr != "" {
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, row.LastRunAtStr); err == nil {
					lastRunAt = &t
					break
				}
			}
		}
		result[row.Value] = domain.CoverageCount{
			Total:     row.Total,
			Passed:    row.Passed,
			Failed:    row.Failed,
			Skipped:   row.Skipped,
			LastRunAt: lastRunAt,
		}
	}
	return result, nil
}

// GetSpecRunsByJiraTag returns the runs tagged with the given JIRA issue key within a project.
// It UNIONs spec-run-level tags (spec_run_tags) and test-run-level tags (test_run_tags) so the
// drill-down detail matches GetJiraTagCoverageByProject, which counts both granularities. A
// test-run-level tag has no associated spec/suite, so those columns come back empty for it.
func (r *GormTagRepository) GetSpecRunsByJiraTag(ctx context.Context, projectID, issueKey string) ([]domain.CoveredSpecRun, error) {
	var rows []domain.CoveredSpecRun
	err := r.db.WithContext(ctx).Raw(`
		SELECT spec_name, status, suite_name, test_run_id, branch, start_time, duration
		FROM (
		    SELECT sr.spec_name  AS spec_name,
		           sr.status     AS status,
		           su.suite_name AS suite_name,
		           tr.run_id     AS test_run_id,
		           COALESCE(tr.branch, '') AS branch,
		           sr.start_time AS start_time,
		           sr.duration_ms AS duration
		    FROM   spec_run_tags srt
		    JOIN   spec_runs  sr ON sr.id  = srt.spec_run_id
		    JOIN   suite_runs su ON su.id  = sr.suite_run_id
		    JOIN   test_runs  tr ON tr.id  = su.test_run_id
		    JOIN   tags        t ON t.id   = srt.tag_id
		    WHERE  tr.project_id = ?
		      AND  t.category    = 'jira'
		      AND  UPPER(t.value) = UPPER(?)
		      AND  sr.deleted_at IS NULL
		      AND  su.deleted_at IS NULL
		      AND  tr.deleted_at IS NULL

		    UNION ALL

		    SELECT ''            AS spec_name,
		           tr.status     AS status,
		           ''            AS suite_name,
		           tr.run_id     AS test_run_id,
		           COALESCE(tr.branch, '') AS branch,
		           tr.start_time AS start_time,
		           tr.duration_ms AS duration
		    FROM   test_run_tags trt
		    JOIN   test_runs tr ON tr.id = trt.test_run_id
		    JOIN   tags       t  ON t.id = trt.tag_id
		    WHERE  tr.project_id = ?
		      AND  t.category    = 'jira'
		      AND  UPPER(t.value) = UPPER(?)
		      AND  tr.deleted_at IS NULL
		) combined
		ORDER  BY start_time DESC
		LIMIT  200
	`, projectID, issueKey, projectID, issueKey).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query spec runs by jira tag: %w", err)
	}
	return rows, nil
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
