package infrastructure_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var _ = Describe("GormSuiteRunRepository", func() {
	var (
		repository *infrastructure.GormSuiteRunRepository
		db         *gorm.DB
		sqlDB      *sql.DB
		mock       sqlmock.Sqlmock
		ctx        context.Context
	)

	BeforeEach(func() {
		var err error
		sqlDB, mock, err = sqlmock.New()
		Expect(err).NotTo(HaveOccurred())

		db, err = gorm.Open(postgres.New(postgres.Config{
			Conn: sqlDB,
		}), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())

		repository = infrastructure.NewGormSuiteRunRepository(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		err := mock.ExpectationsWereMet()
		Expect(err).NotTo(HaveOccurred())
		sqlDB.Close()
	})

	Describe("NewGormSuiteRunRepository", func() {
		It("should create a new repository instance", func() {
			repo := infrastructure.NewGormSuiteRunRepository(db)
			Expect(repo).NotTo(BeNil())
		})
	})

	Describe("Create", func() {
		var suiteRun *domain.SuiteRun

		BeforeEach(func() {
			now := time.Now()
			endTime := now.Add(time.Minute)
			suiteRun = &domain.SuiteRun{
				TestRunID:    1,
				Name:         "test-suite",
				Status:       "passed",
				StartTime:    now,
				EndTime:      &endTime,
				TotalTests:   10,
				PassedTests:  8,
				FailedTests:  1,
				SkippedTests: 1,
				Duration:     time.Minute,
			}
		})

		Context("when creation is successful", func() {
			It("should create a suite run and set the ID", func() {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "suite_runs"`)).
					WithArgs(
						AnyTime{},             // created_at
						AnyTime{},             // updated_at
						nil,                   // deleted_at
						suiteRun.TestRunID,    // test_run_id
						suiteRun.Name,         // suite_name
						suiteRun.Status,       // status
						AnyTime{},             // start_time
						AnyTime{},             // end_time
						suiteRun.TotalTests,   // total_specs
						suiteRun.PassedTests,  // passed_specs
						suiteRun.FailedTests,  // failed_specs
						suiteRun.SkippedTests, // skipped_specs
						int64(60000),          // duration in milliseconds
					).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(123))
				mock.ExpectCommit()

				err := repository.Create(ctx, suiteRun)

				Expect(err).NotTo(HaveOccurred())
				Expect(suiteRun.ID).To(Equal(uint(123)))
			})
		})

		Context("when creation fails", func() {
			It("should return an error", func() {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "suite_runs"`)).
					WillReturnError(errors.New("database error"))
				mock.ExpectRollback()

				err := repository.Create(ctx, suiteRun)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create suite run"))
			})
		})
	})

	Describe("CreateBatch", func() {
		Context("when the batch is empty", func() {
			It("should return nil without any database operations", func() {
				err := repository.CreateBatch(ctx, []*domain.SuiteRun{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when batch creation is successful", func() {
			It("should create multiple suite runs and set their IDs", func() {
				now1 := time.Now()
				end1 := now1.Add(time.Minute)
				now2 := time.Now()
				end2 := now2.Add(2 * time.Minute)

				suiteRuns := []*domain.SuiteRun{
					{
						TestRunID:   1,
						Name:        "test-suite-1",
						Status:      "passed",
						StartTime:   now1,
						EndTime:     &end1,
						TotalTests:  10,
						PassedTests: 10,
						Duration:    time.Minute,
					},
					{
						TestRunID:   1,
						Name:        "test-suite-2",
						Status:      "failed",
						StartTime:   now2,
						EndTime:     &end2,
						TotalTests:  5,
						FailedTests: 1,
						PassedTests: 4,
						Duration:    2 * time.Minute,
					},
				}

				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "suite_runs"`)).
					WithArgs(
						AnyTime{}, AnyTime{}, nil, // created_at, updated_at, deleted_at for first record
						uint(1), "test-suite-1", "passed", AnyTime{}, AnyTime{}, 10, 10, 0, 0, int64(60000),
						AnyTime{}, AnyTime{}, nil, // created_at, updated_at, deleted_at for second record
						uint(1), "test-suite-2", "failed", AnyTime{}, AnyTime{}, 5, 4, 1, 0, int64(120000),
					).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
				mock.ExpectCommit()

				err := repository.CreateBatch(ctx, suiteRuns)

				Expect(err).NotTo(HaveOccurred())
				Expect(suiteRuns[0].ID).To(Equal(uint(1)))
				Expect(suiteRuns[1].ID).To(Equal(uint(2)))
			})
		})

		Context("when batch creation fails", func() {
			It("should return an error", func() {
				now := time.Now()
				suiteRuns := []*domain.SuiteRun{
					{
						TestRunID: 1,
						Name:      "test-suite-1",
						Status:    "passed",
						StartTime: now,
					},
				}

				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "suite_runs"`)).
					WithArgs(
						AnyTime{}, AnyTime{}, nil, // created_at, updated_at, deleted_at
						uint(1), "test-suite-1", "passed", AnyTime{}, nil, 0, 0, 0, 0, int64(0),
					).
					WillReturnError(errors.New("batch insert failed"))
				mock.ExpectRollback()

				err := repository.CreateBatch(ctx, suiteRuns)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create suite runs in batch"))
			})
		})
	})

	Describe("FindByTestRunID", func() {
		Context("when suite runs are found", func() {
			It("should return the suite runs", func() {
				testRunID := uint(1)
				now := time.Now()
				end1 := now.Add(time.Minute)
				end2 := now.Add(2 * time.Minute)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE test_run_id = $1 AND "suite_runs"."deleted_at" IS NULL`)).
					WithArgs(testRunID).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "created_at", "updated_at", "deleted_at", "test_run_id", "suite_name", "status", "start_time",
						"end_time", "total_specs", "passed_specs", "failed_specs", "skipped_specs", "duration",
					}).
						AddRow(1, now, now, nil, testRunID, "test-suite-1", "passed", now, end1, 10, 10, 0, 0, 60000).
						AddRow(2, now, now, nil, testRunID, "test-suite-2", "failed", now, end2, 5, 4, 1, 0, 120000))

				suiteRuns, err := repository.FindByTestRunID(ctx, testRunID)

				Expect(err).NotTo(HaveOccurred())
				Expect(suiteRuns).To(HaveLen(2))

				Expect(suiteRuns[0].ID).To(Equal(uint(1)))
				Expect(suiteRuns[0].TestRunID).To(Equal(testRunID))
				Expect(suiteRuns[0].Name).To(Equal("test-suite-1"))
				Expect(suiteRuns[0].Status).To(Equal("passed"))
				Expect(suiteRuns[0].TotalTests).To(Equal(10))
				Expect(suiteRuns[0].PassedTests).To(Equal(10))
				Expect(suiteRuns[0].Duration).To(Equal(time.Duration(0)))
				Expect(suiteRuns[0].PackageName).To(Equal("")) // Not stored in database
				Expect(suiteRuns[0].ClassName).To(Equal(""))   // Not stored in database

				Expect(suiteRuns[1].ID).To(Equal(uint(2)))
				Expect(suiteRuns[1].Name).To(Equal("test-suite-2"))
				Expect(suiteRuns[1].Status).To(Equal("failed"))
				Expect(suiteRuns[1].TotalTests).To(Equal(5))
				Expect(suiteRuns[1].FailedTests).To(Equal(1))
				Expect(suiteRuns[1].Duration).To(Equal(time.Duration(0)))
			})
		})

		Context("when no suite runs are found", func() {
			It("should return an empty slice", func() {
				testRunID := uint(999)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE test_run_id = $1 AND "suite_runs"."deleted_at" IS NULL`)).
					WithArgs(testRunID).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "created_at", "updated_at", "deleted_at", "test_run_id", "suite_name", "status", "start_time",
						"end_time", "total_specs", "passed_specs", "failed_specs", "skipped_specs", "duration",
					}))

				suiteRuns, err := repository.FindByTestRunID(ctx, testRunID)

				Expect(err).NotTo(HaveOccurred())
				Expect(suiteRuns).To(HaveLen(0))
			})
		})

		Context("when database query fails", func() {
			It("should return an error", func() {
				testRunID := uint(1)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE test_run_id = $1 AND "suite_runs"."deleted_at" IS NULL`)).
					WithArgs(testRunID).
					WillReturnError(errors.New("database error"))

				suiteRuns, err := repository.FindByTestRunID(ctx, testRunID)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to find suite runs"))
				Expect(suiteRuns).To(BeNil())
			})
		})
	})

	Describe("GetByID", func() {
		Context("when suite run is found", func() {
			It("should return the suite run", func() {
				id := uint(1)
				now := time.Now()
				endTime := now.Add(time.Minute)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE "suite_runs"."id" = $1 AND "suite_runs"."deleted_at" IS NULL ORDER BY "suite_runs"."id" LIMIT $2`)).
					WithArgs(id, 1).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "created_at", "updated_at", "deleted_at", "test_run_id", "suite_name", "status", "start_time",
						"end_time", "total_specs", "passed_specs", "failed_specs", "skipped_specs", "duration",
					}).
						AddRow(id, now, now, nil, 1, "test-suite", "passed", now, endTime, 10, 8, 1, 1, 60000))

				suiteRun, err := repository.GetByID(ctx, id)

				Expect(err).NotTo(HaveOccurred())
				Expect(suiteRun).NotTo(BeNil())
				Expect(suiteRun.ID).To(Equal(id))
				Expect(suiteRun.Name).To(Equal("test-suite"))
				Expect(suiteRun.Status).To(Equal("passed"))
				Expect(suiteRun.TotalTests).To(Equal(10))
				Expect(suiteRun.PassedTests).To(Equal(8))
				Expect(suiteRun.FailedTests).To(Equal(1))
				Expect(suiteRun.SkippedTests).To(Equal(1))
				Expect(suiteRun.Duration).To(Equal(time.Duration(0)))
			})
		})

		Context("when suite run is not found", func() {
			It("should return a not found error", func() {
				id := uint(999)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE "suite_runs"."id" = $1 AND "suite_runs"."deleted_at" IS NULL ORDER BY "suite_runs"."id" LIMIT $2`)).
					WithArgs(id, 1).
					WillReturnError(gorm.ErrRecordNotFound)

				suiteRun, err := repository.GetByID(ctx, id)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("suite run not found"))
				Expect(suiteRun).To(BeNil())
			})
		})

		Context("when database query fails", func() {
			It("should return a database error", func() {
				id := uint(1)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE "suite_runs"."id" = $1 AND "suite_runs"."deleted_at" IS NULL ORDER BY "suite_runs"."id" LIMIT $2`)).
					WithArgs(id, 1).
					WillReturnError(errors.New("database connection failed"))

				suiteRun, err := repository.GetByID(ctx, id)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get suite run"))
				Expect(suiteRun).To(BeNil())
			})
		})
	})

	Describe("Update", func() {
		var suiteRun *domain.SuiteRun

		BeforeEach(func() {
			now := time.Now()
			endTime := now.Add(time.Minute)
			suiteRun = &domain.SuiteRun{
				ID:           1,
				TestRunID:    1,
				Name:         "test-suite",
				Status:       "completed",
				StartTime:    now,
				EndTime:      &endTime,
				TotalTests:   10,
				PassedTests:  8,
				FailedTests:  1,
				SkippedTests: 1,
				Duration:     time.Minute,
			}
		})

		Context("when update is successful", func() {
			It("should update the suite run", func() {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta(`UPDATE "suite_runs" SET "duration"=$1,"end_time"=$2,"failed_specs"=$3,"passed_specs"=$4,"skipped_specs"=$5,"status"=$6,"total_specs"=$7,"updated_at"=$8 WHERE id = $9 AND "suite_runs"."deleted_at" IS NULL`)).
					WithArgs(
						int64(60000), // duration
						AnyTime{},    // end_time
						1,            // failed_specs
						8,            // passed_specs
						1,            // skipped_specs
						"completed",  // status
						10,           // total_specs
						AnyTime{},    // updated_at
						uint(1),      // id
					).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()

				err := repository.Update(ctx, suiteRun)

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when suite run is not found", func() {
			It("should return a not found error", func() {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta(`UPDATE "suite_runs" SET "duration"=$1,"end_time"=$2,"failed_specs"=$3,"passed_specs"=$4,"skipped_specs"=$5,"status"=$6,"total_specs"=$7,"updated_at"=$8 WHERE id = $9 AND "suite_runs"."deleted_at" IS NULL`)).
					WithArgs(
						int64(60000), AnyTime{}, 1, 8, 1, "completed", 10, AnyTime{}, uint(1),
					).
					WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected
				mock.ExpectCommit()

				err := repository.Update(ctx, suiteRun)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("suite run not found"))
			})
		})

		Context("when update fails", func() {
			It("should return a database error", func() {
				mock.ExpectBegin()
				mock.ExpectExec(regexp.QuoteMeta(`UPDATE "suite_runs" SET "duration"=$1,"end_time"=$2,"failed_specs"=$3,"passed_specs"=$4,"skipped_specs"=$5,"status"=$6,"total_specs"=$7,"updated_at"=$8 WHERE id = $9 AND "suite_runs"."deleted_at" IS NULL`)).
					WithArgs(
						int64(60000), AnyTime{}, 1, 8, 1, "completed", 10, AnyTime{}, uint(1),
					).
					WillReturnError(errors.New("database error"))
				mock.ExpectRollback()

				err := repository.Update(ctx, suiteRun)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to update suite run"))
			})
		})
	})

	Describe("GetWithSpecRuns", func() {
		Context("when suite run with spec runs is found", func() {
			It("should return the suite run with preloaded spec runs", func() {
				id := uint(1)
				now := time.Now()
				endTime := now.Add(time.Minute)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE "suite_runs"."id" = $1 AND "suite_runs"."deleted_at" IS NULL ORDER BY "suite_runs"."id" LIMIT $2`)).
					WithArgs(id, 1).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "created_at", "updated_at", "deleted_at", "test_run_id", "suite_name", "status", "start_time",
						"end_time", "total_specs", "passed_specs", "failed_specs", "skipped_specs", "duration",
					}).
						AddRow(id, now, now, nil, 1, "test-suite", "passed", now, endTime, 2, 2, 0, 0, 60000))

				// Mock the preload query for spec runs
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE "spec_runs"."suite_run_id" = $1 AND "spec_runs"."deleted_at" IS NULL`)).
					WithArgs(id).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "created_at", "updated_at", "deleted_at", "suite_run_id", "spec_name", "status", "start_time",
						"end_time", "duration_ms", "error_message", "stack_trace", "retry_count", "is_flaky",
					}).
						AddRow(1, now, now, nil, id, "spec-1", "passed", now, endTime, 5000, "", "", 0, false).
						AddRow(2, now, now, nil, id, "spec-2", "passed", now, endTime, 3000, "", "", 0, false))

				suiteRun, err := repository.GetWithSpecRuns(ctx, id)

				Expect(err).NotTo(HaveOccurred())
				Expect(suiteRun).NotTo(BeNil())
				Expect(suiteRun.ID).To(Equal(id))
				Expect(suiteRun.Name).To(Equal("test-suite"))
				Expect(suiteRun.SpecRuns).To(HaveLen(2))

				Expect(suiteRun.SpecRuns[0].ID).To(Equal(uint(1)))
				Expect(suiteRun.SpecRuns[0].Name).To(Equal("spec-1"))
				Expect(suiteRun.SpecRuns[0].Status).To(Equal("passed"))
				Expect(suiteRun.SpecRuns[0].Duration).To(Equal(5 * time.Second))
				Expect(suiteRun.SpecRuns[0].ClassName).To(Equal("")) // Not stored in database

				Expect(suiteRun.SpecRuns[1].ID).To(Equal(uint(2)))
				Expect(suiteRun.SpecRuns[1].Name).To(Equal("spec-2"))
				Expect(suiteRun.SpecRuns[1].Duration).To(Equal(3 * time.Second))
			})
		})

		Context("when suite run is found but has no spec runs", func() {
			It("should return the suite run without spec runs", func() {
				id := uint(1)
				now := time.Now()
				endTime := now.Add(time.Minute)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE "suite_runs"."id" = $1 AND "suite_runs"."deleted_at" IS NULL ORDER BY "suite_runs"."id" LIMIT $2`)).
					WithArgs(id, 1).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "created_at", "updated_at", "deleted_at", "test_run_id", "suite_name", "status", "start_time",
						"end_time", "total_specs", "passed_specs", "failed_specs", "skipped_specs", "duration",
					}).
						AddRow(id, now, now, nil, 1, "test-suite", "passed", now, endTime, 0, 0, 0, 0, 60000))

				// Mock empty preload query for spec runs
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE "spec_runs"."suite_run_id" = $1 AND "spec_runs"."deleted_at" IS NULL`)).
					WithArgs(id).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "created_at", "updated_at", "deleted_at", "suite_run_id", "spec_name", "status", "start_time",
						"end_time", "duration_ms", "error_message", "stack_trace", "retry_count", "is_flaky",
					}))

				suiteRun, err := repository.GetWithSpecRuns(ctx, id)

				Expect(err).NotTo(HaveOccurred())
				Expect(suiteRun).NotTo(BeNil())
				Expect(suiteRun.ID).To(Equal(id))
				Expect(suiteRun.SpecRuns).To(BeNil()) // Should be nil when no spec runs
			})
		})

		Context("when suite run is not found", func() {
			It("should return a not found error", func() {
				id := uint(999)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE "suite_runs"."id" = $1 AND "suite_runs"."deleted_at" IS NULL ORDER BY "suite_runs"."id" LIMIT $2`)).
					WithArgs(id, 1).
					WillReturnError(gorm.ErrRecordNotFound)

				suiteRun, err := repository.GetWithSpecRuns(ctx, id)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("suite run not found"))
				Expect(suiteRun).To(BeNil())
			})
		})

		Context("when database query fails", func() {
			It("should return a database error", func() {
				id := uint(1)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE "suite_runs"."id" = $1 AND "suite_runs"."deleted_at" IS NULL ORDER BY "suite_runs"."id" LIMIT $2`)).
					WithArgs(id, 1).
					WillReturnError(errors.New("database connection failed"))

				suiteRun, err := repository.GetWithSpecRuns(ctx, id)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get suite run with spec runs"))
				Expect(suiteRun).To(BeNil())
			})
		})
	})

	Describe("toDomainSuiteRun", func() {
		It("should correctly convert database model to domain model", func() {
			now := time.Now()
			endTime := now.Add(time.Minute)

			// We need to access the private method through the public interface
			// by creating a suite run through GetByID
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "suite_runs" WHERE "suite_runs"."id" = $1 AND "suite_runs"."deleted_at" IS NULL ORDER BY "suite_runs"."id" LIMIT $2`)).
				WithArgs(uint(1), 1).
				WillReturnRows(sqlmock.NewRows([]string{
					"id", "created_at", "updated_at", "deleted_at", "test_run_id", "suite_name", "status", "start_time",
					"end_time", "total_specs", "passed_specs", "failed_specs", "skipped_specs", "duration",
				}).
					AddRow(1, now, now, nil, 2, "test-suite", "completed", now, endTime, 15, 10, 3, 2, 90000))

			domainSuiteRun, err := repository.GetByID(ctx, 1)

			Expect(err).NotTo(HaveOccurred())
			Expect(domainSuiteRun.ID).To(Equal(uint(1)))
			Expect(domainSuiteRun.TestRunID).To(Equal(uint(2)))
			Expect(domainSuiteRun.Name).To(Equal("test-suite"))
			Expect(domainSuiteRun.PackageName).To(Equal("")) // Not stored in database
			Expect(domainSuiteRun.ClassName).To(Equal(""))   // Not stored in database
			Expect(domainSuiteRun.Status).To(Equal("completed"))
			Expect(domainSuiteRun.StartTime).To(Equal(now))
			Expect(domainSuiteRun.EndTime).To(Equal(&endTime))
			Expect(domainSuiteRun.TotalTests).To(Equal(15))
			Expect(domainSuiteRun.PassedTests).To(Equal(10))
			Expect(domainSuiteRun.FailedTests).To(Equal(3))
			Expect(domainSuiteRun.SkippedTests).To(Equal(2))
			Expect(domainSuiteRun.Duration).To(Equal(time.Duration(0)))
		})
	})
})
