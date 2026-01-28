package infrastructure_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
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

type AnyTime struct{}

// Match satisfies sqlmock.Argument interface
func (a AnyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

var _ = Describe("GormSpecRunRepository", func() {
	var (
		repository *infrastructure.GormSpecRunRepository
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

		repository = infrastructure.NewGormSpecRunRepository(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		err := mock.ExpectationsWereMet()
		Expect(err).NotTo(HaveOccurred())
		sqlDB.Close()
	})

	Describe("NewGormSpecRunRepository", func() {
		It("should create a new repository instance", func() {
			repo := infrastructure.NewGormSpecRunRepository(db)
			Expect(repo).NotTo(BeNil())
		})
	})

	Describe("Create", func() {
		var specRun *domain.SpecRun

		BeforeEach(func() {
			endTime := time.Now().Add(time.Second)
			specRun = &domain.SpecRun{
				SuiteRunID:   1,
				Name:         "test-spec",
				Status:       "passed",
				StartTime:    time.Now(),
				EndTime:      &endTime,
				Duration:     time.Second,
				ErrorMessage: "",
				StackTrace:   "",
				RetryCount:   0,
				IsFlaky:      false,
			}
		})

		Context("when creation is successful", func() {
			It("should create a spec run and set the ID", func() {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "spec_runs"`)).
					WithArgs(
						AnyTime{},            // created_at
						AnyTime{},            // updated_at
						nil,                  // deleted_at
						specRun.SuiteRunID,   // suite_run_id
						specRun.Name,         // spec_name
						specRun.Status,       // status
						AnyTime{},            // start_time
						AnyTime{},            // end_time
						int64(1000),          // duration_ms (Duration in milliseconds)
						specRun.ErrorMessage, // error_message
						specRun.StackTrace,   // stack_trace
						specRun.RetryCount,   // retry_count
						specRun.IsFlaky,      // is_flaky
					).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(123))
				mock.ExpectCommit()

				err := repository.Create(ctx, specRun)

				Expect(err).NotTo(HaveOccurred())
				Expect(specRun.ID).To(Equal(uint(123)))
			})
		})

		Context("when creation fails", func() {
			It("should return an error", func() {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "spec_runs"`)).
					WillReturnError(errors.New("database error"))
				mock.ExpectRollback()

				err := repository.Create(ctx, specRun)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create spec run"))
			})
		})
	})

	Describe("CreateBatch", func() {
		Context("when the batch is empty", func() {
			It("should return nil without any database operations", func() {
				err := repository.CreateBatch(ctx, []*domain.SpecRun{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when batch creation is successful", func() {
			It("should create multiple spec runs and set their IDs", func() {
				now1 := time.Now()
				end1 := now1.Add(time.Second)
				now2 := time.Now()
				end2 := now2.Add(2 * time.Second)

				specRuns := []*domain.SpecRun{
					{
						SuiteRunID: 1,
						Name:       "test-spec-1",
						Status:     "passed",
						StartTime:  now1,
						EndTime:    &end1,
						Duration:   time.Second,
					},
					{
						SuiteRunID: 1,
						Name:       "test-spec-2",
						Status:     "failed",
						StartTime:  now2,
						EndTime:    &end2,
						Duration:   2 * time.Second,
					},
				}

				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "spec_runs"`)).
					WithArgs(
						AnyTime{}, AnyTime{}, nil, // created_at, updated_at, deleted_at for first record
						uint(1), "test-spec-1", "passed", AnyTime{}, AnyTime{}, int64(1000), "", "", 0, false,
						AnyTime{}, AnyTime{}, nil, // created_at, updated_at, deleted_at for second record
						uint(1), "test-spec-2", "failed", AnyTime{}, AnyTime{}, int64(2000), "", "", 0, false,
					).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
				mock.ExpectCommit()

				err := repository.CreateBatch(ctx, specRuns)

				Expect(err).NotTo(HaveOccurred())
				Expect(specRuns[0].ID).To(Equal(uint(1)))
				Expect(specRuns[1].ID).To(Equal(uint(2)))
			})
		})

		Context("when batch creation fails", func() {
			It("should return an error", func() {
				now := time.Now()
				specRuns := []*domain.SpecRun{
					{
						SuiteRunID: 1,
						Name:       "test-spec-1",
						Status:     "passed",
						StartTime:  now,
					},
				}

				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "spec_runs"`)).
					WithArgs(
						AnyTime{}, AnyTime{}, nil, // created_at, updated_at, deleted_at
						uint(1), "test-spec-1", "passed", AnyTime{}, nil, int64(0), "", "", 0, false,
					).
					WillReturnError(errors.New("batch insert failed"))
				mock.ExpectRollback()

				err := repository.CreateBatch(ctx, specRuns)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create spec runs in batch"))
			})
		})
	})

	Describe("FindBySuiteRunID", func() {
		Context("when spec runs are found", func() {
			It("should return the spec runs", func() {
				suiteRunID := uint(1)
				now := time.Now()
				end1 := now.Add(time.Second)
				end2 := now.Add(2 * time.Second)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE suite_run_id = $1 AND "spec_runs"."deleted_at" IS NULL`)).
					WithArgs(suiteRunID).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "created_at", "updated_at", "deleted_at", "suite_run_id", "spec_name", "status", "start_time",
						"end_time", "duration_ms", "error_message", "stack_trace",
						"retry_count", "is_flaky",
					}).
						AddRow(1, now, now, nil, suiteRunID, "test-spec-1", "passed", now, end1, 1000, "", "", 0, false).
						AddRow(2, now, now, nil, suiteRunID, "test-spec-2", "failed", now, end2, 2000, "error", "trace", 1, true))

				specRuns, err := repository.FindBySuiteRunID(ctx, suiteRunID)

				Expect(err).NotTo(HaveOccurred())
				Expect(specRuns).To(HaveLen(2))

				Expect(specRuns[0].ID).To(Equal(uint(1)))
				Expect(specRuns[0].SuiteRunID).To(Equal(suiteRunID))
				Expect(specRuns[0].Name).To(Equal("test-spec-1"))
				Expect(specRuns[0].Status).To(Equal("passed"))
				Expect(specRuns[0].Duration).To(Equal(time.Second))
				Expect(specRuns[0].ClassName).To(Equal("")) // Not stored in database

				Expect(specRuns[1].ID).To(Equal(uint(2)))
				Expect(specRuns[1].Name).To(Equal("test-spec-2"))
				Expect(specRuns[1].Status).To(Equal("failed"))
				Expect(specRuns[1].ErrorMessage).To(Equal("error"))
				Expect(specRuns[1].FailureMessage).To(Equal("error")) // Uses error message
				Expect(specRuns[1].StackTrace).To(Equal("trace"))
				Expect(specRuns[1].RetryCount).To(Equal(1))
				Expect(specRuns[1].IsFlaky).To(BeTrue())
			})
		})

		Context("when no spec runs are found", func() {
			It("should return an empty slice", func() {
				suiteRunID := uint(999)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE suite_run_id = $1 AND "spec_runs"."deleted_at" IS NULL`)).
					WithArgs(suiteRunID).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "created_at", "updated_at", "deleted_at", "suite_run_id", "spec_name", "status", "start_time",
						"end_time", "duration_ms", "error_message", "stack_trace",
						"retry_count", "is_flaky",
					}))

				specRuns, err := repository.FindBySuiteRunID(ctx, suiteRunID)

				Expect(err).NotTo(HaveOccurred())
				Expect(specRuns).To(HaveLen(0))
			})
		})

		Context("when database query fails", func() {
			It("should return an error", func() {
				suiteRunID := uint(1)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE suite_run_id = $1 AND "spec_runs"."deleted_at" IS NULL`)).
					WithArgs(suiteRunID).
					WillReturnError(errors.New("database error"))

				specRuns, err := repository.FindBySuiteRunID(ctx, suiteRunID)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to find spec runs"))
				Expect(specRuns).To(BeNil())
			})
		})
	})

	Describe("Update", func() {
		It("should return not implemented error", func() {
			specRun := &domain.SpecRun{ID: 1}

			err := repository.Update(ctx, specRun)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Update not implemented"))
		})
	})

	Describe("GetByID", func() {
		Context("when spec run is found", func() {
			It("should return the spec run", func() {
				id := uint(1)
				now := time.Now()
				endTime := now.Add(time.Second)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE "spec_runs"."id" = $1 AND "spec_runs"."deleted_at" IS NULL ORDER BY "spec_runs"."id" LIMIT $2`)).
					WithArgs(id, 1).
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "created_at", "updated_at", "deleted_at", "suite_run_id", "spec_name", "status", "start_time",
						"end_time", "duration_ms", "error_message", "stack_trace",
						"retry_count", "is_flaky",
					}).
						AddRow(id, now, now, nil, 1, "test-spec", "passed", now, endTime, 1000, "", "", 0, false))

				specRun, err := repository.GetByID(ctx, id)

				Expect(err).NotTo(HaveOccurred())
				Expect(specRun).NotTo(BeNil())
				Expect(specRun.ID).To(Equal(id))
				Expect(specRun.Name).To(Equal("test-spec"))
				Expect(specRun.Status).To(Equal("passed"))
				Expect(specRun.Duration).To(Equal(time.Second))
			})
		})

		Context("when spec run is not found", func() {
			It("should return a not found error", func() {
				id := uint(999)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE "spec_runs"."id" = $1 AND "spec_runs"."deleted_at" IS NULL ORDER BY "spec_runs"."id" LIMIT $2`)).
					WithArgs(id, 1).
					WillReturnError(gorm.ErrRecordNotFound)

				specRun, err := repository.GetByID(ctx, id)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("spec run not found"))
				Expect(specRun).To(BeNil())
			})
		})

		Context("when database query fails", func() {
			It("should return a database error", func() {
				id := uint(1)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE "spec_runs"."id" = $1 AND "spec_runs"."deleted_at" IS NULL ORDER BY "spec_runs"."id" LIMIT $2`)).
					WithArgs(id, 1).
					WillReturnError(errors.New("database connection failed"))

				specRun, err := repository.GetByID(ctx, id)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get spec run"))
				Expect(specRun).To(BeNil())
			})
		})
	})

	Describe("CountByStatus", func() {
		Context("when spec runs exist", func() {
			It("should return counts grouped by status", func() {
				suiteRunID := uint(1)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT status, COUNT(*) as count FROM "spec_runs" WHERE suite_run_id = $1 AND "spec_runs"."deleted_at" IS NULL GROUP BY "status"`)).
					WithArgs(suiteRunID).
					WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
						AddRow("passed", 5).
						AddRow("failed", 2).
						AddRow("skipped", 1))

				counts, err := repository.CountByStatus(ctx, suiteRunID)

				Expect(err).NotTo(HaveOccurred())
				Expect(counts).To(HaveLen(3))
				Expect(counts["passed"]).To(Equal(5))
				Expect(counts["failed"]).To(Equal(2))
				Expect(counts["skipped"]).To(Equal(1))
			})
		})

		Context("when no spec runs exist", func() {
			It("should return an empty map", func() {
				suiteRunID := uint(999)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT status, COUNT(*) as count FROM "spec_runs" WHERE suite_run_id = $1 AND "spec_runs"."deleted_at" IS NULL GROUP BY "status"`)).
					WithArgs(suiteRunID).
					WillReturnRows(sqlmock.NewRows([]string{"status", "count"}))

				counts, err := repository.CountByStatus(ctx, suiteRunID)

				Expect(err).NotTo(HaveOccurred())
				Expect(counts).To(HaveLen(0))
			})
		})

		Context("when database query fails", func() {
			It("should return an error", func() {
				suiteRunID := uint(1)

				mock.ExpectQuery(regexp.QuoteMeta(`SELECT status, COUNT(*) as count FROM "spec_runs" WHERE suite_run_id = $1 AND "spec_runs"."deleted_at" IS NULL GROUP BY "status"`)).
					WithArgs(suiteRunID).
					WillReturnError(errors.New("database error"))

				counts, err := repository.CountByStatus(ctx, suiteRunID)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to count spec runs by status"))
				Expect(counts).To(BeNil())
			})
		})
	})

	Describe("toDomainSpecRun", func() {
		It("should correctly convert database model to domain model", func() {
			now := time.Now()
			endTime := now.Add(time.Second)

			// We need to access the private method through the public interface
			// by creating a spec run through GetByID
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "spec_runs" WHERE "spec_runs"."id" = $1 AND "spec_runs"."deleted_at" IS NULL ORDER BY "spec_runs"."id" LIMIT $2`)).
				WithArgs(uint(1), 1).
				WillReturnRows(sqlmock.NewRows([]string{
					"id", "created_at", "updated_at", "deleted_at", "suite_run_id", "spec_name", "status", "start_time",
					"end_time", "duration_ms", "error_message", "stack_trace",
					"retry_count", "is_flaky",
				}).
					AddRow(1, now, now, nil, 2, "test-spec", "passed", now, endTime, 1000, "error message", "stack trace", 3, true))

			domainSpecRun, err := repository.GetByID(ctx, 1)

			Expect(err).NotTo(HaveOccurred())
			Expect(domainSpecRun.ID).To(Equal(uint(1)))
			Expect(domainSpecRun.SuiteRunID).To(Equal(uint(2)))
			Expect(domainSpecRun.Name).To(Equal("test-spec"))
			Expect(domainSpecRun.ClassName).To(Equal("")) // Not stored in database
			Expect(domainSpecRun.Status).To(Equal("passed"))
			Expect(domainSpecRun.StartTime).To(Equal(now))
			Expect(domainSpecRun.EndTime).To(Equal(&endTime))
			Expect(domainSpecRun.Duration).To(Equal(time.Duration(1000) * time.Millisecond))
			Expect(domainSpecRun.ErrorMessage).To(Equal("error message"))
			Expect(domainSpecRun.FailureMessage).To(Equal("error message")) // Uses error message
			Expect(domainSpecRun.StackTrace).To(Equal("stack trace"))
			Expect(domainSpecRun.RetryCount).To(Equal(3))
			Expect(domainSpecRun.IsFlaky).To(BeTrue())
		})
	})
})
