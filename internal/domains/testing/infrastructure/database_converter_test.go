package infrastructure_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

var _ = Describe("DatabaseConverter", func() {
	var (
		converter *infrastructure.DatabaseConverter
		now       time.Time
	)

	BeforeEach(func() {
		converter = infrastructure.NewDatabaseConverter()
		now = time.Now().UTC().Truncate(time.Second) // Truncate for cleaner comparisons
	})

	Describe("NewDatabaseConverter", func() {
		It("should create a new database converter instance", func() {
			converter := infrastructure.NewDatabaseConverter()
			Expect(converter).ToNot(BeNil())
		})
	})

	Describe("ConvertTestRunToDatabase", func() {
		var (
			domainTestRun *domain.TestRun
			expectedDB    *database.TestRun
		)

		BeforeEach(func() {
			metadata := map[string]interface{}{
				"version":     "1.0.0",
				"environment": "test",
			}

			endTime := now.Add(5 * time.Minute)

			domainTestRun = &domain.TestRun{
				ProjectID:    "project-123",
				RunID:        "run-456",
				Status:       "completed",
				Branch:       "main",
				GitCommit:    "abc123def456",
				StartTime:    now,
				EndTime:      &endTime,
				Duration:     5 * time.Minute,
				TotalTests:   100,
				PassedTests:  85,
				FailedTests:  10,
				SkippedTests: 5,
				Environment:  "staging",
				Metadata:     metadata,
				SuiteRuns:    []domain.SuiteRun{},
			}

			expectedDB = &database.TestRun{
				ProjectID:    "project-123",
				RunID:        "run-456",
				Status:       "completed",
				Branch:       "main",
				CommitSHA:    "abc123def456",
				StartTime:    now,
				EndTime:      &endTime,
				Duration:     int64(5 * time.Minute / time.Millisecond),
				TotalTests:   100,
				PassedTests:  85,
				FailedTests:  10,
				SkippedTests: 5,
				Environment:  "staging",
				Metadata:     database.JSONMap(metadata),
				SuiteRuns:    []database.SuiteRun{},
			}
		})

		It("should convert domain TestRun to database TestRun correctly", func() {
			result := converter.ConvertTestRunToDatabase(domainTestRun)

			Expect(result.ProjectID).To(Equal(expectedDB.ProjectID))
			Expect(result.RunID).To(Equal(expectedDB.RunID))
			Expect(result.Status).To(Equal(expectedDB.Status))
			Expect(result.Branch).To(Equal(expectedDB.Branch))
			Expect(result.CommitSHA).To(Equal(expectedDB.CommitSHA))
			Expect(result.StartTime).To(Equal(expectedDB.StartTime))
			Expect(result.EndTime).To(Equal(expectedDB.EndTime))
			Expect(result.Duration).To(Equal(expectedDB.Duration))
			Expect(result.TotalTests).To(Equal(expectedDB.TotalTests))
			Expect(result.PassedTests).To(Equal(expectedDB.PassedTests))
			Expect(result.FailedTests).To(Equal(expectedDB.FailedTests))
			Expect(result.SkippedTests).To(Equal(expectedDB.SkippedTests))
			Expect(result.Environment).To(Equal(expectedDB.Environment))
			Expect(result.Metadata).To(Equal(expectedDB.Metadata))
			Expect(result.SuiteRuns).To(HaveLen(0))
		})

		It("should handle nil metadata", func() {
			domainTestRun.Metadata = nil
			result := converter.ConvertTestRunToDatabase(domainTestRun)
			Expect(result.Metadata).To(BeNil())
		})

		It("should convert duration to milliseconds correctly", func() {
			domainTestRun.Duration = 2*time.Second + 500*time.Millisecond
			result := converter.ConvertTestRunToDatabase(domainTestRun)
			Expect(result.Duration).To(Equal(int64(2500))) // 2500 milliseconds
		})

		Context("with suite runs", func() {
			BeforeEach(func() {
				suiteEndTime := now.Add(time.Minute)
				domainTestRun.SuiteRuns = []domain.SuiteRun{
					{
						TestRunID:    112456,
						Name:         "TestSuite1",
						Status:       "passed",
						StartTime:    now,
						EndTime:      &suiteEndTime,
						TotalTests:   10,
						PassedTests:  10,
						FailedTests:  0,
						SkippedTests: 0,
						Duration:     time.Minute,
						SpecRuns:     []*domain.SpecRun{},
					},
				}
			})

			It("should convert suite runs", func() {
				result := converter.ConvertTestRunToDatabase(domainTestRun)
				Expect(result.SuiteRuns).To(HaveLen(1))
				Expect(result.SuiteRuns[0].SuiteName).To(Equal("TestSuite1"))
				Expect(result.SuiteRuns[0].TotalSpecs).To(Equal(10))
			})
		})
	})

	Describe("ConvertDomainSuiteRunsToDatabase", func() {
		var domainSuiteRuns []domain.SuiteRun

		BeforeEach(func() {
			suite1EndTime := now.Add(time.Minute)
			suite2EndTime := now.Add(2 * time.Minute)

			domainSuiteRuns = []domain.SuiteRun{
				{
					TestRunID:    112123,
					Name:         "Suite1",
					Status:       "passed",
					StartTime:    now,
					EndTime:      &suite1EndTime,
					TotalTests:   5,
					PassedTests:  5,
					FailedTests:  0,
					SkippedTests: 0,
					Duration:     time.Minute,
					SpecRuns:     []*domain.SpecRun{},
				},
				{
					TestRunID:    112123,
					Name:         "Suite2",
					Status:       "failed",
					StartTime:    now,
					EndTime:      &suite2EndTime,
					TotalTests:   3,
					PassedTests:  2,
					FailedTests:  1,
					SkippedTests: 0,
					Duration:     2 * time.Minute,
					SpecRuns:     []*domain.SpecRun{},
				},
			}
		})

		It("should convert multiple suite runs correctly", func() {
			result := converter.ConvertDomainSuiteRunsToDatabase(domainSuiteRuns)

			Expect(result).To(HaveLen(2))

			// Check first suite
			Expect(result[0].TestRunID).To(Equal(uint(112123)))
			Expect(result[0].SuiteName).To(Equal("Suite1"))
			Expect(result[0].Status).To(Equal("passed"))
			Expect(result[0].TotalSpecs).To(Equal(5))
			Expect(result[0].PassedSpecs).To(Equal(5))
			Expect(result[0].FailedSpecs).To(Equal(0))
			Expect(result[0].SkippedSpecs).To(Equal(0))
			Expect(result[0].Duration).To(Equal(int64(60000))) // 1 minute in milliseconds

			// Check second suite
			Expect(result[1].SuiteName).To(Equal("Suite2"))
			Expect(result[1].Status).To(Equal("failed"))
			Expect(result[1].TotalSpecs).To(Equal(3))
			Expect(result[1].FailedSpecs).To(Equal(1))
			Expect(result[1].Duration).To(Equal(int64(120000))) // 2 minutes in milliseconds
		})

		It("should handle empty suite runs", func() {
			result := converter.ConvertDomainSuiteRunsToDatabase([]domain.SuiteRun{})
			Expect(result).To(HaveLen(0))
		})
	})

	Describe("ConvertDomainSpecRunsToDatabase", func() {
		var domainSpecRuns []*domain.SpecRun

		BeforeEach(func() {
			spec1EndTime := now.Add(time.Second)
			spec2EndTime := now.Add(2 * time.Second)

			domainSpecRuns = []*domain.SpecRun{
				{
					SuiteRunID:     112456,
					Name:           "TestSpec1",
					Status:         "passed",
					StartTime:      now,
					EndTime:        &spec1EndTime,
					Duration:       time.Second,
					ErrorMessage:   "",
					FailureMessage: "",
					StackTrace:     "",
					RetryCount:     0,
					IsFlaky:        false,
				},
				{
					SuiteRunID:     112456,
					Name:           "TestSpec2",
					Status:         "failed",
					StartTime:      now,
					EndTime:        &spec2EndTime,
					Duration:       2 * time.Second,
					ErrorMessage:   "assertion failed",
					FailureMessage: "expected true, got false",
					StackTrace:     "stack trace here",
					RetryCount:     1,
					IsFlaky:        true,
				},
			}
		})

		It("should convert multiple spec runs correctly", func() {
			result := converter.ConvertDomainSpecRunsToDatabase(domainSpecRuns)

			Expect(result).To(HaveLen(2))

			// Check first spec
			Expect(result[0].SuiteRunID).To(Equal(uint(112456)))
			Expect(result[0].SpecName).To(Equal("TestSpec1"))
			Expect(result[0].Status).To(Equal("passed"))
			Expect(result[0].Duration).To(Equal(int64(1000))) // 1 second in milliseconds
			Expect(result[0].ErrorMessage).To(Equal(""))
			Expect(result[0].RetryCount).To(Equal(0))
			Expect(result[0].IsFlaky).To(BeFalse())

			// Check second spec
			Expect(result[1].SpecName).To(Equal("TestSpec2"))
			Expect(result[1].Status).To(Equal("failed"))
			Expect(result[1].Duration).To(Equal(int64(2000))) // 2 seconds in milliseconds
			Expect(result[1].ErrorMessage).To(Equal("assertion failed"))
			Expect(result[1].StackTrace).To(Equal("stack trace here"))
			Expect(result[1].RetryCount).To(Equal(1))
			Expect(result[1].IsFlaky).To(BeTrue())
		})

		It("should use FailureMessage when ErrorMessage is empty", func() {
			domainSpecRuns[0].ErrorMessage = ""
			domainSpecRuns[0].FailureMessage = "failure message only"

			result := converter.ConvertDomainSpecRunsToDatabase(domainSpecRuns)
			Expect(result[0].ErrorMessage).To(Equal("failure message only"))
		})

		It("should prioritize ErrorMessage over FailureMessage", func() {
			domainSpecRuns[0].ErrorMessage = "error message"
			domainSpecRuns[0].FailureMessage = "failure message"

			result := converter.ConvertDomainSpecRunsToDatabase(domainSpecRuns)
			Expect(result[0].ErrorMessage).To(Equal("error message"))
		})

		It("should handle empty spec runs", func() {
			result := converter.ConvertDomainSpecRunsToDatabase([]*domain.SpecRun{})
			Expect(result).To(HaveLen(0))
		})
	})

	Describe("ConvertTestRunToDomain", func() {
		var dbTestRun *database.TestRun

		BeforeEach(func() {
			metadata := database.JSONMap{
				"version":     "1.0.0",
				"environment": "test",
			}

			endTime := now.Add(10 * time.Minute)

			dbTestRun = &database.TestRun{
				ProjectID:    "project-456",
				RunID:        "run-789",
				Status:       "completed",
				Branch:       "develop",
				CommitSHA:    "def456abc123",
				StartTime:    now,
				EndTime:      &endTime,
				Duration:     int64(10 * time.Minute / time.Millisecond),
				TotalTests:   50,
				PassedTests:  40,
				FailedTests:  5,
				SkippedTests: 5,
				Environment:  "production",
				Metadata:     metadata,
				SuiteRuns:    []database.SuiteRun{},
			}
		})

		It("should convert database TestRun to domain TestRun correctly", func() {
			result := converter.ConvertTestRunToDomain(dbTestRun)

			Expect(result.ID).To(Equal(uint(0)))
			Expect(result.ProjectID).To(Equal("project-456"))
			Expect(result.RunID).To(Equal("run-789"))
			Expect(result.Status).To(Equal("completed"))
			Expect(result.Branch).To(Equal("develop"))
			Expect(result.GitBranch).To(Equal("develop"))
			Expect(result.GitCommit).To(Equal("def456abc123"))
			Expect(result.StartTime).To(Equal(now))
			Expect(result.EndTime).To(Equal(dbTestRun.EndTime))
			Expect(result.Duration).To(Equal(10 * time.Minute))
			Expect(result.TotalTests).To(Equal(50))
			Expect(result.PassedTests).To(Equal(40))
			Expect(result.FailedTests).To(Equal(5))
			Expect(result.SkippedTests).To(Equal(5))
			Expect(result.Environment).To(Equal("production"))
			Expect(result.Metadata).To(HaveKeyWithValue("version", "1.0.0"))
			Expect(result.Metadata).To(HaveKeyWithValue("environment", "test"))
			Expect(result.SuiteRuns).To(HaveLen(0))

			// Check fields not stored in database are empty
			Expect(result.Name).To(Equal(""))
			Expect(result.Source).To(Equal(""))
			Expect(result.SessionID).To(Equal(""))
		})

		It("should handle nil metadata", func() {
			dbTestRun.Metadata = nil
			result := converter.ConvertTestRunToDomain(dbTestRun)
			Expect(result.Metadata).To(Equal(map[string]interface{}{}))
		})

		It("should convert duration from milliseconds correctly", func() {
			dbTestRun.Duration = 3500 // 3.5 seconds in milliseconds
			result := converter.ConvertTestRunToDomain(dbTestRun)
			Expect(result.Duration).To(Equal(3*time.Second + 500*time.Millisecond))
		})
	})

	Describe("ConvertSuiteRunToDomain", func() {
		var dbSuite *database.SuiteRun

		BeforeEach(func() {
			suiteEndTime := now.Add(3 * time.Minute)

			dbSuite = &database.SuiteRun{
				TestRunID:    112456,
				SuiteName:    "DatabaseSuite",
				Status:       "passed",
				StartTime:    now,
				EndTime:      &suiteEndTime,
				TotalSpecs:   8,
				PassedSpecs:  7,
				FailedSpecs:  1,
				SkippedSpecs: 0,
				Duration:     int64(3 * time.Minute / time.Millisecond),
				SpecRuns:     []database.SpecRun{},
			}
		})

		It("should convert database SuiteRun to domain SuiteRun correctly", func() {
			result := converter.ConvertSuiteRunToDomain(dbSuite)

			Expect(result.ID).To(Equal(uint(0)))
			Expect(result.TestRunID).To(Equal(uint(112456)))
			Expect(result.Name).To(Equal("DatabaseSuite"))
			Expect(result.Status).To(Equal("passed"))
			Expect(result.StartTime).To(Equal(now))
			Expect(result.EndTime).To(Equal(dbSuite.EndTime))
			Expect(result.TotalTests).To(Equal(8))
			Expect(result.PassedTests).To(Equal(7))
			Expect(result.FailedTests).To(Equal(1))
			Expect(result.SkippedTests).To(Equal(0))
			Expect(result.Duration).To(Equal(3 * time.Minute))
			Expect(result.SpecRuns).To(HaveLen(0))

			// Check fields not in database model are empty
			Expect(result.PackageName).To(Equal(""))
			Expect(result.ClassName).To(Equal(""))
		})
	})

	Describe("ConvertSpecRunToDomain", func() {
		var dbSpec *database.SpecRun

		BeforeEach(func() {
			specEndTime := now.Add(5 * time.Second)

			dbSpec = &database.SpecRun{
				SuiteRunID:   112456,
				SpecName:     "TestValidation",
				Status:       "failed",
				StartTime:    now,
				EndTime:      &specEndTime,
				Duration:     int64(5 * time.Second / time.Millisecond),
				ErrorMessage: "validation error occurred",
				StackTrace:   "detailed stack trace",
				RetryCount:   2,
				IsFlaky:      true,
			}
		})

		It("should convert database SpecRun to domain SpecRun correctly", func() {
			result := converter.ConvertSpecRunToDomain(dbSpec)

			Expect(result.ID).To(Equal(uint(0)))
			Expect(result.SuiteRunID).To(Equal(uint(112456)))
			Expect(result.Name).To(Equal("TestValidation"))
			Expect(result.Status).To(Equal("failed"))
			Expect(result.StartTime).To(Equal(now))
			Expect(result.EndTime).To(Equal(dbSpec.EndTime))
			Expect(result.Duration).To(Equal(5 * time.Second))
			Expect(result.ErrorMessage).To(Equal("validation error occurred"))
			Expect(result.StackTrace).To(Equal("detailed stack trace"))
			Expect(result.RetryCount).To(Equal(2))
			Expect(result.IsFlaky).To(BeTrue())

			// Check fields not in database model are empty
			Expect(result.ClassName).To(Equal(""))
			Expect(result.FailureMessage).To(Equal(""))
		})
	})

	Describe("Round-trip conversions", func() {
		It("should maintain data integrity for TestRun conversions", func() {
			// Create original domain model
			originalMetadata := map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			}

			specEndTime := now.Add(time.Second)
			suiteEndTime := now.Add(2 * time.Minute)
			testEndTime := now.Add(5 * time.Minute)

			originalSpec := &domain.SpecRun{
				SuiteRunID:     112456,
				Name:           "TestRoundTrip",
				Status:         "passed",
				StartTime:      now,
				EndTime:        &specEndTime,
				Duration:       time.Second,
				ErrorMessage:   "some error",
				FailureMessage: "some failure",
				StackTrace:     "stack",
				RetryCount:     1,
				IsFlaky:        true,
			}

			originalSuite := domain.SuiteRun{
				TestRunID:    112456,
				Name:         "TestSuite",
				Status:       "failed",
				StartTime:    now,
				EndTime:      &suiteEndTime,
				TotalTests:   5,
				PassedTests:  4,
				FailedTests:  1,
				SkippedTests: 0,
				Duration:     2 * time.Minute,
				SpecRuns:     []*domain.SpecRun{originalSpec},
			}

			originalTestRun := &domain.TestRun{
				ProjectID:    "project-123",
				RunID:        "run-456",
				Status:       "completed",
				Branch:       "main",
				GitCommit:    "abc123",
				StartTime:    now,
				EndTime:      &testEndTime,
				Duration:     5 * time.Minute,
				TotalTests:   10,
				PassedTests:  9,
				FailedTests:  1,
				SkippedTests: 0,
				Environment:  "test",
				Metadata:     originalMetadata,
				SuiteRuns:    []domain.SuiteRun{originalSuite},
			}

			// Convert to database and back
			dbTestRun := converter.ConvertTestRunToDatabase(originalTestRun)
			// Simulate database storage by setting ID
			dbTestRun.ID = uint(123)
			dbTestRun.SuiteRuns[0].ID = uint(456)
			dbTestRun.SuiteRuns[0].SpecRuns[0].ID = uint(789)

			resultTestRun := converter.ConvertTestRunToDomain(dbTestRun)

			// Verify core fields are preserved
			Expect(resultTestRun.ProjectID).To(Equal(originalTestRun.ProjectID))
			Expect(resultTestRun.RunID).To(Equal(originalTestRun.RunID))
			Expect(resultTestRun.Status).To(Equal(originalTestRun.Status))
			Expect(resultTestRun.Duration).To(Equal(originalTestRun.Duration))
			Expect(resultTestRun.Metadata).To(Equal(originalMetadata))

			// Verify suite data
			Expect(resultTestRun.SuiteRuns).To(HaveLen(1))
			resultSuite := resultTestRun.SuiteRuns[0]
			Expect(resultSuite.Name).To(Equal(originalSuite.Name))
			Expect(resultSuite.Status).To(Equal(originalSuite.Status))
			Expect(resultSuite.Duration).To(Equal(originalSuite.Duration))

			// Verify spec data (note: ErrorMessage takes precedence over FailureMessage)
			Expect(resultSuite.SpecRuns).To(HaveLen(1))
			resultSpec := resultSuite.SpecRuns[0]
			Expect(resultSpec.Name).To(Equal(originalSpec.Name))
			Expect(resultSpec.ErrorMessage).To(Equal(originalSpec.ErrorMessage))
			Expect(resultSpec.IsFlaky).To(Equal(originalSpec.IsFlaky))
		})
	})
})
