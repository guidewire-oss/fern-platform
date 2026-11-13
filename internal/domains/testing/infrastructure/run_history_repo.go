package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

// RunHistoryRepo reads the horizontal view for the run-detail history
// strip: each suite and spec of one run, across the project's recent
// runs on the same branch.
type RunHistoryRepo struct {
	db *gorm.DB
}

// NewRunHistoryRepo constructs the repository.
func NewRunHistoryRepo(db *gorm.DB) *RunHistoryRepo {
	return &RunHistoryRepo{db: db}
}

// historyWindow is the set of runs a history covers.
type historyWindow struct {
	projectID string
	runs      []domain.HistoryRun // oldest first, ending with the viewed run
	ids       []uint              // runs' ids, same order
}

// loadWindow finds the window of up to limit runs that ends at the run
// with database id runID: same project, same branch, ordered by start
// time (id breaks ties). It returns domain.ErrNotFound when the run does
// not exist. Two queries: the viewed run and the window.
func (r *RunHistoryRepo) loadWindow(db *gorm.DB, runID uint, limit int) (historyWindow, error) {
	if limit <= 0 {
		limit = domain.DefaultRunHistoryLimit
	}
	if limit > domain.MaxRunHistoryLimit {
		limit = domain.MaxRunHistoryLimit
	}

	var viewed database.TestRun
	if err := db.Select("id, project_id, branch, start_time").First(&viewed, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return historyWindow{}, domain.ErrNotFound
		}
		return historyWindow{}, fmt.Errorf("load run %d: %w", runID, err)
	}

	var window []database.TestRun
	if err := db.Model(&database.TestRun{}).
		Select("id, run_id, branch, status, start_time, duration_ms").
		Where("project_id = ? AND branch = ?", viewed.ProjectID, viewed.Branch).
		Where("start_time < ? OR (start_time = ? AND id <= ?)", viewed.StartTime, viewed.StartTime, viewed.ID).
		Order("start_time DESC, id DESC").
		Limit(limit).
		Find(&window).Error; err != nil {
		return historyWindow{}, fmt.Errorf("load history window: %w", err)
	}

	w := historyWindow{
		projectID: viewed.ProjectID,
		runs:      make([]domain.HistoryRun, len(window)),
		ids:       make([]uint, len(window)),
	}
	for i, row := range window {
		// window is newest first; histories read oldest first.
		j := len(window) - 1 - i
		w.runs[j] = domain.HistoryRun{
			ID:         row.ID,
			RunID:      row.RunID,
			Branch:     row.Branch,
			Status:     row.Status,
			StartTime:  row.StartTime,
			DurationMs: row.Duration,
		}
		w.ids[j] = row.ID
	}
	return w, nil
}

// RunHistory returns every suite and spec of the run with database id
// runID across its history window (see loadWindow). Four queries
// regardless of the run's size: the viewed run, the window, and the
// suite and spec results in the window.
func (r *RunHistoryRepo) RunHistory(ctx context.Context, runID uint, limit int) (*domain.RunHistory, error) {
	db := r.db.WithContext(ctx)
	w, err := r.loadWindow(db, runID, limit)
	if err != nil {
		return nil, err
	}
	runs, ids := w.runs, w.ids

	var suites []database.SuiteRun
	if err := db.Model(&database.SuiteRun{}).
		Select("test_run_id, suite_name, status, passed_specs, failed_specs, skipped_specs, duration_ms").
		Where("test_run_id IN ?", ids).
		Order("id").
		Find(&suites).Error; err != nil {
		return nil, fmt.Errorf("load suite history: %w", err)
	}
	rows := make([]domain.HistoryRow, len(suites))
	for i, s := range suites {
		rows[i] = domain.HistoryRow{
			TestRunID:  s.TestRunID,
			SuiteName:  s.SuiteName,
			Status:     domain.SuiteOutcome(s.Status, s.PassedSpecs, s.FailedSpecs, s.SkippedSpecs),
			DurationMs: s.Duration,
		}
	}
	var specRows []domain.HistoryRow
	if err := db.Table("spec_runs AS sr").
		Select("sur.test_run_id, sur.suite_name, sr.spec_name, sr.status, sr.duration_ms").
		Joins("JOIN suite_runs AS sur ON sur.id = sr.suite_run_id").
		Where("sur.test_run_id IN ?", ids).
		Where("sr.deleted_at IS NULL AND sur.deleted_at IS NULL").
		Order("sr.id").
		Scan(&specRows).Error; err != nil {
		return nil, fmt.Errorf("load spec history: %w", err)
	}

	h := domain.BuildRunHistory(runs, append(rows, specRows...))
	h.ProjectID = w.projectID
	return &h, nil
}

// SpecHistory returns one spec, identified by suite and spec name, across
// the same window RunHistory uses for that run, with each result's error
// message and stack trace. Three queries: the viewed run, the window, and
// the spec's results.
func (r *RunHistoryRepo) SpecHistory(ctx context.Context, runID uint, limit int, suiteName, specName string) (*domain.SpecRunHistory, error) {
	db := r.db.WithContext(ctx)
	w, err := r.loadWindow(db, runID, limit)
	if err != nil {
		return nil, err
	}

	var rows []domain.SpecHistoryRow
	if err := db.Table("spec_runs AS sr").
		Select("sur.test_run_id, sr.status, sr.duration_ms, sr.error_message, sr.stack_trace").
		Joins("JOIN suite_runs AS sur ON sur.id = sr.suite_run_id").
		Where("sur.test_run_id IN ?", w.ids).
		Where("sur.suite_name = ? AND sr.spec_name = ?", suiteName, specName).
		Where("sr.deleted_at IS NULL AND sur.deleted_at IS NULL").
		Order("sr.id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load spec history: %w", err)
	}

	h := domain.BuildSpecRunHistory(w.runs, suiteName, specName, rows)
	h.ProjectID = w.projectID
	return &h, nil
}
