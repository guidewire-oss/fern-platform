package infrastructure_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/internal/domains/summary/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

func TestGormSummaryRepository(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Summary Infrastructure Suite")
}

var _ = Describe("GormSummaryRepository", Label("integration", "infrastructure", "summary"), func() {
	var (
		db   *gorm.DB
		repo *infrastructure.GormSummaryRepository
	)

	BeforeEach(func() {
		var err error
		// Create in-memory SQLite database for testing
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		Expect(err).NotTo(HaveOccurred())

		// Auto-migrate the schema
		err = db.AutoMigrate(
			&database.TestRun{},
			&database.SuiteRun{},
			&database.SpecRun{},
			&database.Tag{},
		)
		Expect(err).NotTo(HaveOccurred())

		repo = infrastructure.NewGormSummaryRepository(db)
	})

	AfterEach(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})

	Describe("NewGormSummaryRepository", func() {
		It("should create a new repository", func() {
			newRepo := infrastructure.NewGormSummaryRepository(db)
			Expect(newRepo).NotTo(BeNil())
		})
	})

	Describe("GetTestRunsBySeed", func() {
		Context("when no test runs exist", func() {
			It("should return empty slice", func() {
				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(0))
			})
		})

		Context("when test runs exist for different project", func() {
			It("should return empty slice", func() {
				// Create test run for different project
				testRun := &database.TestRun{
					ProjectID: "other-proj",
					RunID:     "seed-456",
					Branch:    "main",
					CommitSHA: "abc123",
					StartTime: time.Now(),
				}
				err := db.Create(testRun).Error
				Expect(err).NotTo(HaveOccurred())

				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(0))
			})
		})

		Context("when test runs exist for different seed", func() {
			It("should return empty slice", func() {
				// Create test run for different seed
				testRun := &database.TestRun{
					ProjectID: "proj-123",
					RunID:     "other-seed",
					Branch:    "main",
					CommitSHA: "abc123",
					StartTime: time.Now(),
				}
				err := db.Create(testRun).Error
				Expect(err).NotTo(HaveOccurred())

				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(0))
			})
		})

		Context("when test runs exist", func() {
			It("should return test runs with correct data", func() {
				startTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
				endTime := time.Date(2025, 1, 1, 10, 30, 0, 0, time.UTC)

				// Create test run
				testRun := &database.TestRun{
					ProjectID: "proj-123",
					RunID:     "seed-456",
					Branch:    "main",
					CommitSHA: "abc123",
					StartTime: startTime,
					EndTime:   &endTime,
				}
				err := db.Create(testRun).Error
				Expect(err).NotTo(HaveOccurred())

				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].GitBranch).To(Equal("main"))
				Expect(result[0].GitSHA).To(Equal("abc123"))
				Expect(result[0].StartTime).To(Equal(startTime))
				Expect(result[0].EndTime).To(Equal(endTime))
			})

			It("should return test runs without end time", func() {
				startTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

				// Create test run without end time
				testRun := &database.TestRun{
					ProjectID: "proj-123",
					RunID:     "seed-456",
					Branch:    "main",
					CommitSHA: "abc123",
					StartTime: startTime,
					EndTime:   nil,
				}
				err := db.Create(testRun).Error
				Expect(err).NotTo(HaveOccurred())

				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].EndTime.IsZero()).To(BeTrue())
			})

			It("should preload suite runs and spec runs", func() {
				startTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

				// Create test run
				testRun := &database.TestRun{
					ProjectID: "proj-123",
					RunID:     "seed-456",
					Branch:    "main",
					CommitSHA: "abc123",
					StartTime: startTime,
				}
				err := db.Create(testRun).Error
				Expect(err).NotTo(HaveOccurred())

				// Create suite run
				suiteRun := &database.SuiteRun{
					TestRunID: testRun.ID,
					SuiteName: "Test Suite",
				}
				err = db.Create(suiteRun).Error
				Expect(err).NotTo(HaveOccurred())

				// Create spec run
				specRun := &database.SpecRun{
					SuiteRunID: suiteRun.ID,
					SpecName:   "Test Spec",
					Status:     "passed",
				}
				err = db.Create(specRun).Error
				Expect(err).NotTo(HaveOccurred())

				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].SuiteRuns).To(HaveLen(1))
				Expect(result[0].SuiteRuns[0].SpecRuns).To(HaveLen(1))
				Expect(result[0].SuiteRuns[0].SpecRuns[0].Status).To(Equal("passed"))
			})

			It("should preload tags on spec runs", func() {
				startTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

				// Create test run
				testRun := &database.TestRun{
					ProjectID: "proj-123",
					RunID:     "seed-456",
					Branch:    "main",
					CommitSHA: "abc123",
					StartTime: startTime,
				}
				err := db.Create(testRun).Error
				Expect(err).NotTo(HaveOccurred())

				// Create suite run
				suiteRun := &database.SuiteRun{
					TestRunID: testRun.ID,
					SuiteName: "Test Suite",
				}
				err = db.Create(suiteRun).Error
				Expect(err).NotTo(HaveOccurred())

				// Create tag
				tag := &database.Tag{
					Name:     "component:auth",
					Category: "component",
					Value:    "auth",
				}
				err = db.Create(tag).Error
				Expect(err).NotTo(HaveOccurred())

				// Create spec run with tag
				specRun := &database.SpecRun{
					SuiteRunID: suiteRun.ID,
					SpecName:   "Test Spec",
					Status:     "passed",
					Tags:       []database.Tag{*tag},
				}
				err = db.Create(specRun).Error
				Expect(err).NotTo(HaveOccurred())

				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].SuiteRuns[0].SpecRuns[0].Tags).To(HaveLen(1))
				Expect(result[0].SuiteRuns[0].SpecRuns[0].Tags[0].Category).To(Equal("component"))
				Expect(result[0].SuiteRuns[0].SpecRuns[0].Tags[0].Value).To(Equal("auth"))
			})

			It("should preload tags on suite runs", func() {
				startTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

				// Create test run
				testRun := &database.TestRun{
					ProjectID: "proj-123",
					RunID:     "seed-456",
					Branch:    "main",
					CommitSHA: "abc123",
					StartTime: startTime,
				}
				err := db.Create(testRun).Error
				Expect(err).NotTo(HaveOccurred())

				// Create tag
				tag := &database.Tag{
					Name:     "priority:high",
					Category: "priority",
					Value:    "high",
				}
				err = db.Create(tag).Error
				Expect(err).NotTo(HaveOccurred())

				// Create suite run with tag
				suiteRun := &database.SuiteRun{
					TestRunID: testRun.ID,
					SuiteName: "Test Suite",
					Tags:      []database.Tag{*tag},
				}
				err = db.Create(suiteRun).Error
				Expect(err).NotTo(HaveOccurred())

				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].SuiteRuns[0].Tags).To(HaveLen(1))
				Expect(result[0].SuiteRuns[0].Tags[0].Category).To(Equal("priority"))
				Expect(result[0].SuiteRuns[0].Tags[0].Value).To(Equal("high"))
			})

			It("should handle multiple suite runs within a single test run", func() {
				startTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

				// Create test run
				testRun := &database.TestRun{
					ProjectID: "proj-123",
					RunID:     "seed-456",
					Branch:    "main",
					CommitSHA: "abc123",
					StartTime: startTime,
				}
				err := db.Create(testRun).Error
				Expect(err).NotTo(HaveOccurred())

				// Create multiple suite runs
				suiteRun1 := &database.SuiteRun{
					TestRunID: testRun.ID,
					SuiteName: "Suite 1",
				}
				suiteRun2 := &database.SuiteRun{
					TestRunID: testRun.ID,
					SuiteName: "Suite 2",
				}
				err = db.Create(suiteRun1).Error
				Expect(err).NotTo(HaveOccurred())
				err = db.Create(suiteRun2).Error
				Expect(err).NotTo(HaveOccurred())

				// Create spec runs in each suite
				specRun1 := &database.SpecRun{
					SuiteRunID: suiteRun1.ID,
					SpecName:   "Spec 1",
					Status:     "passed",
				}
				specRun2 := &database.SpecRun{
					SuiteRunID: suiteRun2.ID,
					SpecName:   "Spec 2",
					Status:     "failed",
				}
				err = db.Create(specRun1).Error
				Expect(err).NotTo(HaveOccurred())
				err = db.Create(specRun2).Error
				Expect(err).NotTo(HaveOccurred())

				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].SuiteRuns).To(HaveLen(2))
				Expect(result[0].SuiteRuns[0].SpecRuns).To(HaveLen(1))
				Expect(result[0].SuiteRuns[1].SpecRuns).To(HaveLen(1))
			})
		})

		Context("when database error occurs", func() {
			It("should return error", func() {
				// Close the database connection to simulate error
				sqlDB, _ := db.DB()
				sqlDB.Close()

				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get test runs"))
				Expect(result).To(BeNil())
			})
		})

		Context("when spec runs have multiple tags", func() {
			It("should convert all tags correctly", func() {
				startTime := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)

				// Create test run
				testRun := &database.TestRun{
					ProjectID: "proj-123",
					RunID:     "seed-456",
					Branch:    "main",
					CommitSHA: "abc123",
					StartTime: startTime,
				}
				err := db.Create(testRun).Error
				Expect(err).NotTo(HaveOccurred())

				// Create suite run
				suiteRun := &database.SuiteRun{
					TestRunID: testRun.ID,
					SuiteName: "Test Suite",
				}
				err = db.Create(suiteRun).Error
				Expect(err).NotTo(HaveOccurred())

				// Create multiple tags
				tag1 := &database.Tag{
					Name:     "component:auth",
					Category: "component",
					Value:    "auth",
				}
				tag2 := &database.Tag{
					Name:     "priority:high",
					Category: "priority",
					Value:    "high",
				}
				err = db.Create(tag1).Error
				Expect(err).NotTo(HaveOccurred())
				err = db.Create(tag2).Error
				Expect(err).NotTo(HaveOccurred())

				// Create spec run with multiple tags
				specRun := &database.SpecRun{
					SuiteRunID: suiteRun.ID,
					SpecName:   "Test Spec",
					Status:     "passed",
					Tags:       []database.Tag{*tag1, *tag2},
				}
				err = db.Create(specRun).Error
				Expect(err).NotTo(HaveOccurred())

				result, err := repo.GetTestRunsBySeed("proj-123", "seed-456")

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(1))
				Expect(result[0].SuiteRuns[0].SpecRuns[0].Tags).To(HaveLen(2))

				// Verify both tags are present
				tagMap := make(map[string]string)
				for _, tag := range result[0].SuiteRuns[0].SpecRuns[0].Tags {
					tagMap[tag.Category] = tag.Value
				}
				Expect(tagMap["component"]).To(Equal("auth"))
				Expect(tagMap["priority"]).To(Equal("high"))
			})
		})
	})
})
