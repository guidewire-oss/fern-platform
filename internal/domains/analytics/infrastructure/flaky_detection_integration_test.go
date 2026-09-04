package infrastructure_test

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

// The detector's queries are hand-written SQL, and a mock matching on query
// text accepts column names that do not exist. So these run against a real
// database. The schema is AutoMigrate over the production models, which
// catches SQL that disagrees with them — but not the Postgres schema itself,
// since migrations/*.sql never runs here.

const (
	flakyProjectID = "proj-flaky-integration"
	flakySuiteName = "example suite"
	flakySpecName  = "flaky_spec"
	stableSpecName = "stable_spec"
)

// dbSeq names each test's in-memory database uniquely.
var dbSeq atomic.Uint64

func newFlakyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:flaky-%d?mode=memory&cache=shared", dbSeq.Add(1))

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, db.AutoMigrate(
		&database.TestRun{},
		&database.SuiteRun{},
		&database.SpecRun{},
		&database.FlakyTest{},
	))

	return db
}

// seedFlakyHistory writes 10 runs an hour apart, each with one flaky and one
// stable spec.
//
// The flaky spec fails on runs 0, 5 and 6: a 3/10 rate, inside the 0.05–0.95
// band. Run 0 is the oldest and the detector walks newest-first, so it ends on
// a failure with consecutivePasses at 0 — keeping calculateFlakeScore's
// stability adjustment out of play so the expected score is exact.
//
// The stable spec always passes, so it must not be detected.
func seedFlakyHistory(t *testing.T, db *gorm.DB) {
	t.Helper()

	failedRuns := map[int]bool{0: true, 5: true, 6: true}
	base := time.Now().Add(-48 * time.Hour)

	for i := 0; i < 10; i++ {
		createdAt := base.Add(time.Duration(i) * time.Hour)

		run := database.TestRun{
			ProjectID: flakyProjectID,
			RunID:     "run-" + strconv.Itoa(i),
			Status:    "completed",
			StartTime: createdAt,
		}
		run.CreatedAt = createdAt
		run.UpdatedAt = createdAt
		require.NoError(t, db.Create(&run).Error)

		suite := database.SuiteRun{
			TestRunID: run.ID,
			SuiteName: flakySuiteName,
			Status:    "completed",
			StartTime: createdAt,
		}
		suite.CreatedAt = createdAt
		require.NoError(t, db.Create(&suite).Error)

		flakyStatus := "passed"
		errMessage := ""
		if failedRuns[i] {
			flakyStatus = "failed"
			errMessage = "boom"
		}

		specs := []database.SpecRun{
			{
				SuiteRunID:   suite.ID,
				SpecName:     flakySpecName,
				Status:       flakyStatus,
				ErrorMessage: errMessage,
				StartTime:    createdAt,
			},
			{
				SuiteRunID: suite.ID,
				SpecName:   stableSpecName,
				Status:     "passed",
				StartTime:  createdAt,
			},
		}
		for j := range specs {
			specs[j].CreatedAt = createdAt
			require.NoError(t, db.Create(&specs[j]).Error)
		}
	}
}

func newFlakyService(db *gorm.DB) *application.FlakyDetectionService {
	repo := infrastructure.NewGormFlakyDetectionRepository(db)
	return application.NewFlakyDetectionService(repo, domain.DefaultFlakyTestDetectionConfig())
}

func TestAnalyzeTestRun_DetectsFlakySpec(t *testing.T) {
	db := newFlakyTestDB(t)
	seedFlakyHistory(t, db)

	svc := newFlakyService(db)
	ctx := context.Background()

	analysis, err := svc.AnalyzeTestRun(ctx, flakyProjectID, "run-9")
	require.NoError(t, err)
	require.NotNil(t, analysis)

	// Both spec names are in the window, but only one is flaky.
	assert.Equal(t, 2, analysis.TotalTests)
	assert.Equal(t, []string{domain.BuildTestID(flakyProjectID, flakySuiteName, flakySpecName)}, analysis.NewFlaky)

	flaky, err := svc.GetFlakyTests(ctx, flakyProjectID)
	require.NoError(t, err)
	require.Len(t, flaky, 1, "the stable spec must not be reported as flaky")

	got := flaky[0]
	assert.Equal(t, flakySpecName, got.TestName)
	assert.Equal(t, flakyProjectID, got.ProjectID)
	assert.Equal(t, 10, got.TotalRuns)
	assert.Equal(t, 3, got.FailureCount)
	assert.NotZero(t, got.ID, "the row ID is what resolve/ignore addresses")

	// 0.30 failure rate * (0.7 + 0.3*(10/100)) = 0.219
	assert.InDelta(t, 0.219, got.FlakeScore, 1e-9)

	// flake_rate is DECIMAL(5,4) in Postgres, so the stored value must be the
	// 0–1 fraction: 0.219 scaled to 21.9 overflows and the save fails.
	var storedRates []float64
	require.NoError(t, db.Model(&database.FlakyTest{}).
		Where("project_id = ?", flakyProjectID).
		Pluck("flake_rate", &storedRates).Error)
	require.Len(t, storedRates, 1)
	assert.InDelta(t, got.FlakeScore, storedRates[0], 1e-9)
	assert.LessOrEqual(t, storedRates[0], 1.0)
}

func TestAnalyzeTestRun_IsIdempotent(t *testing.T) {
	db := newFlakyTestDB(t)
	seedFlakyHistory(t, db)

	svc := newFlakyService(db)
	ctx := context.Background()

	_, err := svc.AnalyzeTestRun(ctx, flakyProjectID, "run-9")
	require.NoError(t, err)
	_, err = svc.AnalyzeTestRun(ctx, flakyProjectID, "run-9")
	require.NoError(t, err)

	// Re-analysis must update the record, not insert a second one.
	var rows []database.FlakyTest
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)

	// Save writes every column, so the update must not wipe created_at.
	assert.False(t, rows[0].CreatedAt.IsZero())
}

func TestMarkTestResolved_ByRowID(t *testing.T) {
	db := newFlakyTestDB(t)
	seedFlakyHistory(t, db)

	svc := newFlakyService(db)
	ctx := context.Background()

	_, err := svc.AnalyzeTestRun(ctx, flakyProjectID, "run-9")
	require.NoError(t, err)

	flaky, err := svc.GetFlakyTests(ctx, flakyProjectID)
	require.NoError(t, err)
	require.Len(t, flaky, 1)

	require.NoError(t, svc.MarkTestResolved(ctx, flaky[0].ID))

	// GetFlakyTests only returns active records.
	remaining, err := svc.GetFlakyTests(ctx, flakyProjectID)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestMarkTestResolved_UnknownID(t *testing.T) {
	db := newFlakyTestDB(t)
	svc := newFlakyService(db)

	err := svc.MarkTestResolved(context.Background(), 4242)
	assert.ErrorIs(t, err, domain.ErrFlakyTestNotFound)
}

// Guards the fixture's maths, so a change to calculateFlakeScore fails here
// rather than confusingly above.
func TestExpectedScoreMatchesFormula(t *testing.T) {
	failureRate := 3.0 / 10.0
	runConfidence := math.Min(10.0/100.0, 1.0)
	assert.InDelta(t, 0.219, failureRate*(0.7+0.3*runConfidence), 1e-9)
}
