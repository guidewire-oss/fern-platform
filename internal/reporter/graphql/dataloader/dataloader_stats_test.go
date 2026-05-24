package dataloader

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/pkg/database"
)

// TestProjectStatsLoader_HarmonizedMetric verifies that the project stats
// loader exposes per-test totals (TotalTests / PassedTests). The Projects
// list and the treemap both derive their displayed pass rate from these
// fields — if they were ever to diverge the two views would show different
// numbers for the same project, which is the bug this test guards against.
func TestProjectStatsLoader_HarmonizedMetric(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.TestRun{}))

	now := time.Now().UTC()
	runs := []database.TestRun{
		{
			// A fully green run — counts toward PassedRuns.
			ProjectID: "proj-a", RunID: "r1", Status: "passed",
			Branch: "main", StartTime: now,
			TotalTests: 100, PassedTests: 100, FailedTests: 0,
			Duration: 5000,
		},
		{
			// A run with at least one failure — does NOT count toward
			// PassedRuns, but its passed_tests/total_tests still feed
			// the harmonized success rate.
			ProjectID: "proj-a", RunID: "r2", Status: "failed",
			Branch: "main", StartTime: now,
			TotalTests: 80, PassedTests: 67, FailedTests: 13,
			Duration: 4000,
		},
	}
	for i := range runs {
		require.NoError(t, db.Create(&runs[i]).Error)
	}

	loaders := NewLoaders(db)
	thunk := loaders.ProjectStatsByProjectID.Load(context.Background(), "proj-a")
	data, err := thunk()
	require.NoError(t, err)
	require.NotNil(t, data)

	require.Equal(t, int64(2), data.TotalRuns)
	require.Equal(t, int64(1), data.PassedRuns, "only the run with status=passed and failed_tests=0 counts as a passed run")
	require.Equal(t, int64(180), data.TotalTests, "total_tests is summed across runs (100 + 80)")
	require.Equal(t, int64(167), data.PassedTests, "passed_tests is summed across runs (100 + 67)")
}

// TestProjectStatsLoader_MissingProject covers the path where a project ID
// is requested but has no test_runs. Previously this returned a cached
// empty struct from a process-wide loader; with request-scoped loaders the
// behaviour is the same per request, but the cache no longer survives the
// request, so a re-issued query after data lands returns the real numbers.
func TestProjectStatsLoader_MissingProject(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&database.TestRun{}))

	loaders := NewLoaders(db)
	data, err := loaders.ProjectStatsByProjectID.Load(context.Background(), "ghost")()
	require.NoError(t, err)
	require.NotNil(t, data)
	require.Equal(t, int64(0), data.TotalRuns)
	require.Equal(t, int64(0), data.TotalTests)
	require.Equal(t, int64(0), data.PassedTests)
}
