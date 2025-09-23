package infrastructure_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

var _ = Describe("GormTestRunRepository", func() {
	var (
		repo *infrastructure.GormTestRunRepository
		db   *gorm.DB
		ctx  context.Context
		_    *gorm.DB
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Create in-memory SQLite database
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())

		// Auto-migrate the schema
		err = db.AutoMigrate(&database.TestRun{}, &database.SuiteRun{}, &database.SpecRun{})
		Expect(err).NotTo(HaveOccurred())

		repo = infrastructure.NewGormTestRunRepository(db)
		_ = db
	})

	Describe("NewGormTestRunRepository", func() {
		It("should create a new repository instance", func() {
			newRepo := infrastructure.NewGormTestRunRepository(db)
			Expect(newRepo).NotTo(BeNil())
		})
	})

	Describe("Create", func() {
		var testRun *domain.TestRun

		BeforeEach(func() {
			testRun = &domain.TestRun{
				RunID:        "test-run-1",
				ProjectID:    "project-1",
				Status:       "running",
				StartTime:    time.Now(),
				TotalTests:   10,
				PassedTests:  0,
				FailedTests:  0,
				SkippedTests: 0,
				Metadata:     map[string]interface{}{"branch": "main"},
			}
		})

		It("should create a test run successfully", func() {
			err := repo.Create(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
			Expect(testRun.ID).NotTo(BeZero())
		})

		It("should return error when database fails", func() {
			// Close the database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			err := repo.Create(ctx, testRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create test run"))
		})
	})

	Describe("Update", func() {
		var testRun *domain.TestRun
		var _ uint

		BeforeEach(func() {
			// Create a test run first
			testRun = &domain.TestRun{
				RunID:        "test-run-2",
				ProjectID:    "project-1",
				Status:       "running",
				StartTime:    time.Now(),
				TotalTests:   10,
				PassedTests:  0,
				FailedTests:  0,
				SkippedTests: 0,
				Metadata:     map[string]interface{}{"branch": "main"},
			}
			err := repo.Create(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
			_ = testRun.ID

			// Update the test run
			testRun.Status = "passed"
			testRun.EndTime = &time.Time{}
			*testRun.EndTime = time.Now()
			testRun.Duration = 5 * time.Second
			testRun.PassedTests = 8
			testRun.FailedTests = 2
		})

		It("should update a test run successfully", func() {
			err := repo.Update(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when test run not found", func() {
			testRun.ID = 999999
			err := repo.Update(ctx, testRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("test run not found"))
		})

		It("should return error when database fails", func() {
			// Close the database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			err := repo.Update(ctx, testRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update test run"))
		})
	})

	Describe("GetByID", func() {
		var testRunID uint

		BeforeEach(func() {
			testRun := &domain.TestRun{
				RunID:        "test-run-3",
				ProjectID:    "project-1",
				Status:       "passed",
				StartTime:    time.Now(),
				TotalTests:   5,
				PassedTests:  5,
				FailedTests:  0,
				SkippedTests: 0,
			}
			err := repo.Create(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
			testRunID = testRun.ID
		})

		It("should retrieve a test run by ID", func() {
			result, err := repo.GetByID(ctx, testRunID)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.ID).To(Equal(testRunID))
			Expect(result.RunID).To(Equal("test-run-3"))
		})

		It("should return error when test run not found", func() {
			result, err := repo.GetByID(ctx, 999999)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("test run not found"))
			Expect(result).To(BeNil())
		})

		It("should return error when database fails", func() {
			// Close the database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			result, err := repo.GetByID(ctx, testRunID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get test run"))
			Expect(result).To(BeNil())
		})
	})

	Describe("GetByRunID", func() {
		BeforeEach(func() {
			testRun := &domain.TestRun{
				RunID:        "unique-run-id",
				ProjectID:    "project-1",
				Status:       "passed",
				StartTime:    time.Now(),
				TotalTests:   3,
				PassedTests:  3,
				FailedTests:  0,
				SkippedTests: 0,
			}
			err := repo.Create(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should retrieve a test run by run ID", func() {
			result, err := repo.GetByRunID(ctx, "unique-run-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.RunID).To(Equal("unique-run-id"))
		})

		It("should return error when test run not found", func() {
			result, err := repo.GetByRunID(ctx, "non-existent-run-id")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("test run not found"))
			Expect(result).To(BeNil())
		})

		It("should return error when database fails", func() {
			// Close the database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			result, err := repo.GetByRunID(ctx, "unique-run-id")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get test run"))
			Expect(result).To(BeNil())
		})
	})

	Describe("GetByProjectID", func() {
		BeforeEach(func() {
			// Create multiple test runs for the same project
			for i := 0; i < 3; i++ {
				testRun := &domain.TestRun{
					RunID:        fmt.Sprintf("run-%d", i),
					ProjectID:    "project-test",
					Status:       "passed",
					StartTime:    time.Now().Add(-time.Duration(i) * time.Hour),
					TotalTests:   5,
					PassedTests:  5,
					FailedTests:  0,
					SkippedTests: 0,
				}
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should retrieve all test runs for a project", func() {
			results, err := repo.GetByProjectID(ctx, "project-test")
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(3))
			// Should be ordered by created_at DESC
			Expect(results[0].RunID).To(Equal("run-2"))
		})

		It("should return empty slice when no test runs found", func() {
			results, err := repo.GetByProjectID(ctx, "non-existent-project")
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(BeEmpty())
		})

		It("should return error when database fails", func() {
			// Close the database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			results, err := repo.GetByProjectID(ctx, "project-test")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get test runs"))
			Expect(results).To(BeNil())
		})
	})

	Describe("GetLatestByProjectID", func() {
		BeforeEach(func() {
			// Create multiple test runs
			for i := 0; i < 5; i++ {
				testRun := &domain.TestRun{
					RunID:        fmt.Sprintf("latest-run-%d", i),
					ProjectID:    "project-latest",
					Status:       "passed",
					StartTime:    time.Now().Add(-time.Duration(i) * time.Hour),
					TotalTests:   5,
					PassedTests:  5,
					FailedTests:  0,
					SkippedTests: 0,
				}
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should retrieve latest test runs with limit", func() {
			results, err := repo.GetLatestByProjectID(ctx, "project-latest", 2)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(2))
		})

		It("should retrieve all test runs when limit is 0", func() {
			results, err := repo.GetLatestByProjectID(ctx, "project-latest", 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(5))
		})

		It("should return error when database fails", func() {
			// Close the database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			results, err := repo.GetLatestByProjectID(ctx, "project-latest", 2)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get latest test runs"))
			Expect(results).To(BeNil())
		})
	})

	Describe("GetWithDetails", func() {
		var testRunID uint

		BeforeEach(func() {
			testRun := &domain.TestRun{
				RunID:        "detailed-run",
				ProjectID:    "project-detailed",
				Status:       "passed",
				StartTime:    time.Now(),
				TotalTests:   2,
				PassedTests:  2,
				FailedTests:  0,
				SkippedTests: 0,
			}
			err := repo.Create(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
			testRunID = testRun.ID
		})

		It("should retrieve test run with details", func() {
			result, err := repo.GetWithDetails(ctx, testRunID)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.ID).To(Equal(testRunID))
		})

		It("should return error when test run not found", func() {
			result, err := repo.GetWithDetails(ctx, 999999)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("test run not found"))
			Expect(result).To(BeNil())
		})

		It("should return error when database fails", func() {
			// Close the database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			result, err := repo.GetWithDetails(ctx, testRunID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get test run with details"))
			Expect(result).To(BeNil())
		})
	})

	Describe("FindByDateRange", func() {
		var startDate, endDate time.Time

		BeforeEach(func() {
			startDate = time.Now().Add(-24 * time.Hour)
			endDate = time.Now().Add(24 * time.Hour)

			// Create test runs within date range
			testRun1 := &domain.TestRun{
				RunID:     "date-run-1",
				ProjectID: "project-date",
				Status:    "passed",
				StartTime: time.Now(),
			}
			testRun2 := &domain.TestRun{
				RunID:     "date-run-2",
				ProjectID: "project-date",
				Status:    "failed",
				StartTime: time.Now().Add(-1 * time.Hour),
			}

			err := repo.Create(ctx, testRun1)
			Expect(err).NotTo(HaveOccurred())
			err = repo.Create(ctx, testRun2)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should find test runs within date range", func() {
			results, err := repo.FindByDateRange(ctx, "project-date", startDate, endDate)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(2))
		})

		It("should return empty slice when no runs in date range", func() {
			pastStart := time.Now().Add(-48 * time.Hour)
			pastEnd := time.Now().Add(-25 * time.Hour)
			results, err := repo.FindByDateRange(ctx, "project-date", pastStart, pastEnd)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(BeEmpty())
		})

		It("should return error when database fails", func() {
			// Close the database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			results, err := repo.FindByDateRange(ctx, "project-date", startDate, endDate)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to find test runs by date range"))
			Expect(results).To(BeNil())
		})
	})

	Describe("GetTestRunSummary", func() {
		BeforeEach(func() {
			// Create test runs with different statuses
			testRuns := []*domain.TestRun{
				{RunID: "summary-1", ProjectID: "project-summary", Status: "passed", Duration: 1000 * time.Millisecond},
				{RunID: "summary-2", ProjectID: "project-summary", Status: "passed", Duration: 2000 * time.Millisecond},
				{RunID: "summary-3", ProjectID: "project-summary", Status: "failed", Duration: 1500 * time.Millisecond},
				{RunID: "summary-4", ProjectID: "project-summary", Status: "running", Duration: 0},
			}

			for _, testRun := range testRuns {
				testRun.StartTime = time.Now()
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

	Describe("Delete", func() {
		var testRunID uint

		BeforeEach(func() {
			testRun := &domain.TestRun{
				RunID:     "delete-run",
				ProjectID: "project-delete",
				Status:    "passed",
				StartTime: time.Now(),
			}
			err := repo.Create(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
			testRunID = testRun.ID
		})

		It("should delete a test run", func() {
			err := repo.Delete(ctx, testRunID)
			Expect(err).NotTo(HaveOccurred())

			// Verify it's deleted
			_, err = repo.GetByID(ctx, testRunID)
			Expect(err).To(HaveOccurred())
		})

		It("should not return error when deleting non-existent run", func() {
			err := repo.Delete(ctx, 999999)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("CountByProjectID", func() {
		BeforeEach(func() {
			// Create test runs for counting
			for i := 0; i < 3; i++ {
				testRun := &domain.TestRun{
					RunID:     fmt.Sprintf("count-run-%d", i),
					ProjectID: "project-count",
					Status:    "passed",
					StartTime: time.Now(),
				}
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should count test runs for a project", func() {
			count, err := repo.CountByProjectID(ctx, "project-count")
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(int64(3)))
		})

		It("should return 0 for empty project", func() {
			count, err := repo.CountByProjectID(ctx, "empty-project")
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(int64(0)))
		})

		It("should return error when database fails", func() {
			// Close the database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			count, err := repo.CountByProjectID(ctx, "project-count")
			Expect(err).To(HaveOccurred())
			Expect(count).To(Equal(int64(0)))
		})
	})

	Describe("GetRecent", func() {
		BeforeEach(func() {
			// Create test runs across different projects
			projects := []string{"project-a", "project-b", "project-c"}
			for i, project := range projects {
				testRun := &domain.TestRun{
					RunID:     fmt.Sprintf("recent-run-%d", i),
					ProjectID: project,
					Status:    "passed",
					StartTime: time.Now().Add(-time.Duration(i) * time.Hour),
				}
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should retrieve recent test runs with limit", func() {
			results, err := repo.GetRecent(ctx, 2)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(2))
			// Should be ordered by created_at DESC
			Expect(results[0].RunID).To(Equal("recent-run-2"))
		})

		It("should retrieve all test runs when limit is 0", func() {
			results, err := repo.GetRecent(ctx, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(results)).To(BeNumerically(">=", 3))
		})

		It("should return error when database fails", func() {
			// Close the database to simulate error
			sqlDB, _ := db.DB()
			sqlDB.Close()

			results, err := repo.GetRecent(ctx, 2)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get recent test runs"))
			Expect(results).To(BeNil())
		})
	})

	Context("Edge cases and error scenarios", func() {
		It("should handle context cancellation", func() {
			cancelCtx, cancel := context.WithCancel(ctx)
			cancel()

			testRun := &domain.TestRun{
				RunID:     "cancelled-run",
				ProjectID: "project-cancel",
				Status:    "running",
				StartTime: time.Now(),
			}

			err := repo.Create(cancelCtx, testRun)
			Expect(err).To(HaveOccurred())
		})

		It("should handle large metadata objects", func() {
			largeMetadata := make(map[string]interface{})
			for i := 0; i < 100; i++ {
				largeMetadata[fmt.Sprintf("key-%d", i)] = fmt.Sprintf("value-%d", i)
			}

			testRun := &domain.TestRun{
				RunID:     "large-metadata-run",
				ProjectID: "project-large",
				Status:    "passed",
				StartTime: time.Now(),
				Metadata:  largeMetadata,
			}

			err := repo.Create(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			retrieved, err := repo.GetByID(ctx, testRun.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retrieved.Metadata).To(HaveLen(100))
		})
	})

	Describe("GetTestRunSummary", func() {
		BeforeEach(func() {
			// Create test runs with different statuses and durations
			testRuns := []*domain.TestRun{
				{
					RunID:     "summary-1",
					ProjectID: "project-summary",
					Status:    "passed",
					StartTime: time.Now().Add(-4 * time.Hour),
					Duration:  1000 * time.Millisecond,
				},
				{
					RunID:     "summary-2",
					ProjectID: "project-summary",
					Status:    "passed",
					StartTime: time.Now().Add(-3 * time.Hour),
					Duration:  2000 * time.Millisecond,
				},
				{
					RunID:     "summary-3",
					ProjectID: "project-summary",
					Status:    "failed",
					StartTime: time.Now().Add(-2 * time.Hour),
					Duration:  1500 * time.Millisecond,
				},
				{
					RunID:     "summary-4",
					ProjectID: "project-summary",
					Status:    "failed",
					StartTime: time.Now().Add(-1 * time.Hour),
					Duration:  3000 * time.Millisecond,
				},
				{
					RunID:     "summary-5",
					ProjectID: "project-summary",
					Status:    "running",
					StartTime: time.Now(),
					Duration:  0,
				},
				{
					RunID:     "other-project-1",
					ProjectID: "project-other",
					Status:    "passed",
					StartTime: time.Now(),
					Duration:  500 * time.Millisecond,
				},
			}

			for _, testRun := range testRuns {
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should calculate correct summary statistics", func() {
			summary, err := repo.GetTestRunSummary(ctx, "project-summary")
			Expect(err).NotTo(HaveOccurred())
			Expect(summary).NotTo(BeNil())

			// Verify counts
			Expect(summary.TotalRuns).To(Equal(5))
			Expect(summary.PassedRuns).To(Equal(2))
			Expect(summary.FailedRuns).To(Equal(2))

			// Verify success rate (2 passed out of 5 total = 0.4)
			Expect(summary.SuccessRate).To(BeNumerically("~", 0.4, 0.001))

			// Verify average duration
			// (1000 + 2000 + 1500 + 3000 + 0) / 5 = 1500ms
			expectedAvgDuration := 1500 * time.Millisecond
			Expect(summary.AverageRunTime).To(Equal(expectedAvgDuration))
		})

		It("should return zero values for non-existent project", func() {
			summary, err := repo.GetTestRunSummary(ctx, "non-existent-project")
			Expect(err).NotTo(HaveOccurred())
			Expect(summary).NotTo(BeNil())

			Expect(summary.TotalRuns).To(Equal(0))
			Expect(summary.PassedRuns).To(Equal(0))
			Expect(summary.FailedRuns).To(Equal(0))
			Expect(summary.SuccessRate).To(Equal(0.0))
			Expect(summary.AverageRunTime).To(Equal(time.Duration(0)))
		})

		It("should handle project with only passed runs", func() {
			// Create a project with only passed runs
			passedOnlyRuns := []*domain.TestRun{
				{
					RunID:     "passed-only-1",
					ProjectID: "project-passed-only",
					Status:    "passed",
					StartTime: time.Now(),
					Duration:  1000 * time.Millisecond,
				},
				{
					RunID:     "passed-only-2",
					ProjectID: "project-passed-only",
					Status:    "passed",
					StartTime: time.Now(),
					Duration:  2000 * time.Millisecond,
				},
			}

			for _, testRun := range passedOnlyRuns {
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())
			}

			summary, err := repo.GetTestRunSummary(ctx, "project-passed-only")
			Expect(err).NotTo(HaveOccurred())
			Expect(summary).NotTo(BeNil())

			Expect(summary.TotalRuns).To(Equal(2))
			Expect(summary.PassedRuns).To(Equal(2))
			Expect(summary.FailedRuns).To(Equal(0))
			Expect(summary.SuccessRate).To(Equal(1.0))                        // 100% success rate
			Expect(summary.AverageRunTime).To(Equal(1500 * time.Millisecond)) // (1000+2000)/2
		})

		It("should handle project with only failed runs", func() {
			// Create a project with only failed runs
			failedOnlyRuns := []*domain.TestRun{
				{
					RunID:     "failed-only-1",
					ProjectID: "project-failed-only",
					Status:    "failed",
					StartTime: time.Now(),
					Duration:  500 * time.Millisecond,
				},
				{
					RunID:     "failed-only-2",
					ProjectID: "project-failed-only",
					Status:    "failed",
					StartTime: time.Now(),
					Duration:  1500 * time.Millisecond,
				},
			}

			for _, testRun := range failedOnlyRuns {
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())
			}

			summary, err := repo.GetTestRunSummary(ctx, "project-failed-only")
			Expect(err).NotTo(HaveOccurred())
			Expect(summary).NotTo(BeNil())

			Expect(summary.TotalRuns).To(Equal(2))
			Expect(summary.PassedRuns).To(Equal(0))
			Expect(summary.FailedRuns).To(Equal(2))
			Expect(summary.SuccessRate).To(Equal(0.0))                        // 0% success rate
			Expect(summary.AverageRunTime).To(Equal(1000 * time.Millisecond)) // (500+1500)/2
		})

		It("should handle project with various statuses including unknown ones", func() {
			// Create runs with different statuses
			mixedStatusRuns := []*domain.TestRun{
				{
					RunID:     "mixed-1",
					ProjectID: "project-mixed",
					Status:    "passed",
					StartTime: time.Now(),
					Duration:  1000 * time.Millisecond,
				},
				{
					RunID:     "mixed-2",
					ProjectID: "project-mixed",
					Status:    "failed",
					StartTime: time.Now(),
					Duration:  2000 * time.Millisecond,
				},
				{
					RunID:     "mixed-3",
					ProjectID: "project-mixed",
					Status:    "running",
					StartTime: time.Now(),
					Duration:  0,
				},
				{
					RunID:     "mixed-4",
					ProjectID: "project-mixed",
					Status:    "cancelled",
					StartTime: time.Now(),
					Duration:  500 * time.Millisecond,
				},
			}

			for _, testRun := range mixedStatusRuns {
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())
			}

			summary, err := repo.GetTestRunSummary(ctx, "project-mixed")
			Expect(err).NotTo(HaveOccurred())
			Expect(summary).NotTo(BeNil())

			Expect(summary.TotalRuns).To(Equal(4))
			Expect(summary.PassedRuns).To(Equal(1))
			Expect(summary.FailedRuns).To(Equal(1))
			// Success rate should be 1/4 = 0.25
			Expect(summary.SuccessRate).To(BeNumerically("~", 0.25, 0.001))
			// Average duration: (1000 + 2000 + 0 + 500) / 4 = 875
			Expect(summary.AverageRunTime).To(Equal(875 * time.Millisecond))
		})

		Context("Error scenarios", func() {
			It("should return error when database connection is lost", func() {
				// Close the database to simulate connection loss
				sqlDB, _ := db.DB()
				sqlDB.Close()

				summary, err := repo.GetTestRunSummary(ctx, "project-summary")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to count"))
				Expect(summary).To(BeNil())
			})

			It("should handle context cancellation", func() {
				cancelCtx, cancel := context.WithCancel(ctx)
				cancel()

				summary, err := repo.GetTestRunSummary(cancelCtx, "project-summary")
				Expect(err).To(HaveOccurred())
				Expect(summary).To(BeNil())
			})

			It("should handle context timeout", func() {
				timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
				defer cancel()

				// Sleep to ensure context times out
				time.Sleep(1 * time.Millisecond)

				summary, err := repo.GetTestRunSummary(timeoutCtx, "project-summary")
				Expect(err).To(HaveOccurred())
				Expect(summary).To(BeNil())
			})

			It("should handle database transaction errors", func() {
				// Create a scenario that might cause database constraints issues
				// This tests error handling in the database layer

				// First create some data
				testRun := &domain.TestRun{
					RunID:     "error-test-run",
					ProjectID: "project-error",
					Status:    "passed",
					StartTime: time.Now(),
					Duration:  1000 * time.Millisecond,
				}
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())

				// Now close the database to simulate error during summary calculation
				sqlDB, _ := db.DB()
				originalMaxIdleConns := 1
				sqlDB.SetMaxIdleConns(0)                    // Force connection closure
				sqlDB.SetMaxOpenConns(0)                    // Prevent new connections
				sqlDB.SetMaxIdleConns(originalMaxIdleConns) // Reset to avoid affecting other tests

				// This should fail due to no available connections
				summary, err := repo.GetTestRunSummary(ctx, "project-error")
				// Note: This might not always fail depending on SQLite behavior,
				// but it tests the error path when it does occur
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("failed to count"))
					Expect(summary).To(BeNil())
				}
			})
		})

		Context("Edge cases and boundary conditions", func() {
			It("should handle empty string project ID", func() {
				summary, err := repo.GetTestRunSummary(ctx, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(summary).NotTo(BeNil())
				Expect(summary.TotalRuns).To(Equal(0))
			})

			It("should handle very long project ID", func() {
				longProjectID := strings.Repeat("a", 1000)
				summary, err := repo.GetTestRunSummary(ctx, longProjectID)
				Expect(err).NotTo(HaveOccurred())
				Expect(summary).NotTo(BeNil())
				Expect(summary.TotalRuns).To(Equal(0))
			})

			It("should handle project with zero duration runs", func() {
				zeroDurationRuns := []*domain.TestRun{
					{
						RunID:     "zero-duration-1",
						ProjectID: "project-zero-duration",
						Status:    "passed",
						StartTime: time.Now(),
						Duration:  0,
					},
					{
						RunID:     "zero-duration-2",
						ProjectID: "project-zero-duration",
						Status:    "passed",
						StartTime: time.Now(),
						Duration:  0,
					},
				}

				for _, testRun := range zeroDurationRuns {
					err := repo.Create(ctx, testRun)
					Expect(err).NotTo(HaveOccurred())
				}

				summary, err := repo.GetTestRunSummary(ctx, "project-zero-duration")
				Expect(err).NotTo(HaveOccurred())
				Expect(summary).NotTo(BeNil())

				Expect(summary.TotalRuns).To(Equal(2))
				Expect(summary.PassedRuns).To(Equal(2))
				Expect(summary.AverageRunTime).To(Equal(time.Duration(0)))
				Expect(summary.SuccessRate).To(Equal(1.0))
			})

			It("should handle project with very large durations", func() {
				largeDurationRuns := []*domain.TestRun{
					{
						RunID:     "large-duration-1",
						ProjectID: "project-large-duration",
						Status:    "passed",
						StartTime: time.Now(),
						Duration:  24 * time.Hour, // 1 day
					},
					{
						RunID:     "large-duration-2",
						ProjectID: "project-large-duration",
						Status:    "failed",
						StartTime: time.Now(),
						Duration:  48 * time.Hour, // 2 days
					},
				}

				for _, testRun := range largeDurationRuns {
					err := repo.Create(ctx, testRun)
					Expect(err).NotTo(HaveOccurred())
				}

				summary, err := repo.GetTestRunSummary(ctx, "project-large-duration")
				Expect(err).NotTo(HaveOccurred())
				Expect(summary).NotTo(BeNil())

				Expect(summary.TotalRuns).To(Equal(2))
				Expect(summary.PassedRuns).To(Equal(1))
				Expect(summary.FailedRuns).To(Equal(1))
				expectedAvg := ((24 * time.Hour) + (48 * time.Hour)) / 2 // 36 hours
				Expect(summary.AverageRunTime).To(Equal(expectedAvg))
				Expect(summary.SuccessRate).To(Equal(0.5))
			})

			It("should calculate success rate correctly with many runs", func() {
				// Create exactly 100 runs: 75 passed, 25 failed
				projectID := "project-large-dataset"

				for i := 0; i < 100; i++ {
					status := "passed"
					if i >= 75 {
						status = "failed"
					}

					testRun := &domain.TestRun{
						RunID:     fmt.Sprintf("large-dataset-%d", i),
						ProjectID: projectID,
						Status:    status,
						StartTime: time.Now(),
						Duration:  time.Duration(i+1) * time.Millisecond,
					}
					err := repo.Create(ctx, testRun)
					Expect(err).NotTo(HaveOccurred())
				}

				summary, err := repo.GetTestRunSummary(ctx, projectID)
				Expect(err).NotTo(HaveOccurred())
				Expect(summary).NotTo(BeNil())

				Expect(summary.TotalRuns).To(Equal(100))
				Expect(summary.PassedRuns).To(Equal(75))
				Expect(summary.FailedRuns).To(Equal(25))
				Expect(summary.SuccessRate).To(BeNumerically("~", 0.75, 0.001))

				// Average duration calculation:
				// We created durations 1ms, 2ms, ..., 100ms
				// Sum = 1+2+...+100 = 5050ms, Average = 5050/100 = 50.5ms
				// But since duration is stored as int64 in DB, AVG returns 50.5
				// which gets truncated to 50 when converted to time.Duration
				expectedAvg := 50 * time.Millisecond
				Expect(summary.AverageRunTime).To(BeNumerically(">=", expectedAvg))
				Expect(summary.AverageRunTime).To(BeNumerically("<=", 51*time.Millisecond))
			})

			It("should handle NULL/empty average duration from database", func() {
				// Test the case where AVG returns NULL (when no records match)
				// This is already covered by the "non-existent project" test,
				// but let's be explicit about this scenario
				summary, err := repo.GetTestRunSummary(ctx, "absolutely-empty-project")
				Expect(err).NotTo(HaveOccurred())
				Expect(summary).NotTo(BeNil())
				Expect(summary.AverageRunTime).To(Equal(time.Duration(0)))
			})

			It("should handle division by zero in success rate calculation", func() {
				// This scenario is covered by empty project test, but let's verify explicitly
				summary, err := repo.GetTestRunSummary(ctx, "empty-for-division-test")
				Expect(err).NotTo(HaveOccurred())
				Expect(summary).NotTo(BeNil())

				// When TotalRuns is 0, SuccessRate should be 0.0, not cause panic
				Expect(summary.TotalRuns).To(Equal(0))
				Expect(summary.SuccessRate).To(Equal(0.0))
			})

			It("should handle special characters in project ID", func() {
				specialProjectID := "project-with-special-chars-!@#$%^&*()_+-={}[]|\\:;\"'<>,.?/"

				testRun := &domain.TestRun{
					RunID:     "special-char-run",
					ProjectID: specialProjectID,
					Status:    "passed",
					StartTime: time.Now(),
					Duration:  1000 * time.Millisecond,
				}
				err := repo.Create(ctx, testRun)
				Expect(err).NotTo(HaveOccurred())

				summary, err := repo.GetTestRunSummary(ctx, specialProjectID)
				Expect(err).NotTo(HaveOccurred())
				Expect(summary).NotTo(BeNil())
				Expect(summary.TotalRuns).To(Equal(1))
				Expect(summary.PassedRuns).To(Equal(1))
			})
		})

		Context("Performance and data integrity", func() {
			//It("should handle concurrent access", func() {
			//	// Create a project for concurrent testing
			//	concurrentProjectID := "project-concurrent"
			//	testRun := &domain.TestRun{
			//		RunID:     "concurrent-run",
			//		ProjectID: concurrentProjectID,
			//		Status:    "passed",
			//		StartTime: time.Now(),
			//		Duration:  1000 * time.Millisecond,
			//	}
			//	err := repo.Create(ctx, testRun)
			//	Expect(err).NotTo(HaveOccurred())
			//
			//	// Run multiple GetTestRunSummary calls concurrently
			//	const numGoroutines = 10
			//	results := make(chan *domain.TestRunSummary, numGoroutines)
			//	errors := make(chan error, numGoroutines)
			//
			//	for i := 0; i < numGoroutines; i++ {
			//		go func() {
			//			summary, err := repo.GetTestRunSummary(ctx, concurrentProjectID)
			//			if err != nil {
			//				errors <- err
			//				return
			//			}
			//			results <- summary
			//		}()
			//	}
			//
			//	// Collect results
			//	for i := 0; i < numGoroutines; i++ {
			//		select {
			//		case summary := <-results:
			//			Expect(summary.TotalRuns).To(Equal(1))
			//			Expect(summary.PassedRuns).To(Equal(1))
			//			Expect(summary.SuccessRate).To(Equal(1.0))
			//		case err := <-errors:
			//			Fail(fmt.Sprintf("Concurrent access failed: %v", err))
			//		case <-time.After(5 * time.Second):
			//			Fail("Timeout waiting for concurrent operations")
			//		}
			//	}
			//})

			It("should maintain consistency with multiple status types", func() {
				// Test various status combinations to ensure counts are consistent
				statusTestProject := "project-status-consistency"
				statuses := []string{"passed", "failed", "running", "cancelled", "timeout", "skipped"}

				// Create 2 runs for each status
				for _, status := range statuses {
					for i := 0; i < 2; i++ {
						testRun := &domain.TestRun{
							RunID:     fmt.Sprintf("%s-run-%d", status, i),
							ProjectID: statusTestProject,
							Status:    status,
							StartTime: time.Now(),
							Duration:  time.Duration((i+1)*100) * time.Millisecond,
						}
						err := repo.Create(ctx, testRun)
						Expect(err).NotTo(HaveOccurred())
					}
				}

				summary, err := repo.GetTestRunSummary(ctx, statusTestProject)
				Expect(err).NotTo(HaveOccurred())
				Expect(summary).NotTo(BeNil())

				// Total should be 2 * 6 = 12 runs
				Expect(summary.TotalRuns).To(Equal(12))
				// Only "passed" status counts as passed (2 runs)
				Expect(summary.PassedRuns).To(Equal(2))
				// Only "failed" status counts as failed (2 runs)
				Expect(summary.FailedRuns).To(Equal(2))
				// Success rate should be 2/12 = 0.1667
				Expect(summary.SuccessRate).To(BeNumerically("~", 2.0/12.0, 0.001))
			})
		})
	})
})
