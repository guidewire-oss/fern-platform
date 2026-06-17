package infrastructure_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/internal/domains/tags/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

var _ = Describe("GormTagRepository/GetJiraTagCoverageByProject", Label("integration", "infrastructure", "tags", "coverage"), func() {
	var (
		db   *gorm.DB
		repo *infrastructure.GormTagRepository
		ctx  context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		Expect(err).NotTo(HaveOccurred())

		err = db.AutoMigrate(
			&database.Tag{},
			&database.TestRun{},
			&database.TestRunTag{},
			&database.SuiteRun{},
			&database.SpecRun{},
		)
		Expect(err).NotTo(HaveOccurred())

		repo = infrastructure.NewGormTagRepository(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})

	Describe("GetJiraTagCoverageByProject", func() {
		It("returns total, passed, and failed counts from test-run-level tags", func() {
			jiraTag := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			Expect(db.Create(jiraTag).Error).NotTo(HaveOccurred())

			tr1 := &database.TestRun{ProjectID: "proj-a", RunID: "run-1", Status: "passed", StartTime: time.Now()}
			tr2 := &database.TestRun{ProjectID: "proj-a", RunID: "run-2", Status: "failed", StartTime: time.Now()}
			tr3 := &database.TestRun{ProjectID: "proj-a", RunID: "run-3", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(tr1).Error).NotTo(HaveOccurred())
			Expect(db.Create(tr2).Error).NotTo(HaveOccurred())
			Expect(db.Create(tr3).Error).NotTo(HaveOccurred())

			Expect(db.Create(&database.TestRunTag{TestRunID: tr1.ID, TagID: jiraTag.ID}).Error).NotTo(HaveOccurred())
			Expect(db.Create(&database.TestRunTag{TestRunID: tr2.ID, TagID: jiraTag.ID}).Error).NotTo(HaveOccurred())
			Expect(db.Create(&database.TestRunTag{TestRunID: tr3.ID, TagID: jiraTag.ID}).Error).NotTo(HaveOccurred())

			result, err := repo.GetJiraTagCoverageByProject(ctx, "proj-a")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveKey("PROJ-1"))
			Expect(result["PROJ-1"].Total).To(Equal(3))
			Expect(result["PROJ-1"].Passed).To(Equal(2))
			Expect(result["PROJ-1"].Failed).To(Equal(1))
		})

		It("returns total, passed, and failed counts from spec-run-level tags", func() {
			jiraTag := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			Expect(db.Create(jiraTag).Error).NotTo(HaveOccurred())

			tr := &database.TestRun{ProjectID: "proj-a", RunID: "run-1", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(tr).Error).NotTo(HaveOccurred())
			su := &database.SuiteRun{TestRunID: tr.ID, SuiteName: "suite-1", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(su).Error).NotTo(HaveOccurred())

			sr1 := &database.SpecRun{SuiteRunID: su.ID, SpecName: "spec-1", Status: "passed", StartTime: time.Now()}
			sr2 := &database.SpecRun{SuiteRunID: su.ID, SpecName: "spec-2", Status: "failed", StartTime: time.Now()}
			sr3 := &database.SpecRun{SuiteRunID: su.ID, SpecName: "spec-3", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(sr1).Error).NotTo(HaveOccurred())
			Expect(db.Create(sr2).Error).NotTo(HaveOccurred())
			Expect(db.Create(sr3).Error).NotTo(HaveOccurred())

			Expect(db.Exec("INSERT INTO spec_run_tags (spec_run_id, tag_id) VALUES (?, ?)", sr1.ID, jiraTag.ID).Error).NotTo(HaveOccurred())
			Expect(db.Exec("INSERT INTO spec_run_tags (spec_run_id, tag_id) VALUES (?, ?)", sr2.ID, jiraTag.ID).Error).NotTo(HaveOccurred())
			Expect(db.Exec("INSERT INTO spec_run_tags (spec_run_id, tag_id) VALUES (?, ?)", sr3.ID, jiraTag.ID).Error).NotTo(HaveOccurred())

			result, err := repo.GetJiraTagCoverageByProject(ctx, "proj-a")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveKey("PROJ-1"))
			Expect(result["PROJ-1"].Total).To(Equal(3))
			Expect(result["PROJ-1"].Passed).To(Equal(2))
			Expect(result["PROJ-1"].Failed).To(Equal(1))
		})

		It("merges counts from both test-run-level and spec-run-level tags for the same issue", func() {
			jiraTag := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			Expect(db.Create(jiraTag).Error).NotTo(HaveOccurred())

			tr := &database.TestRun{ProjectID: "proj-a", RunID: "run-1", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(tr).Error).NotTo(HaveOccurred())

			// one tag at the test-run level
			Expect(db.Create(&database.TestRunTag{TestRunID: tr.ID, TagID: jiraTag.ID}).Error).NotTo(HaveOccurred())

			// one tag at the spec-run level
			su := &database.SuiteRun{TestRunID: tr.ID, SuiteName: "suite-1", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(su).Error).NotTo(HaveOccurred())
			sr := &database.SpecRun{SuiteRunID: su.ID, SpecName: "spec-1", Status: "failed", StartTime: time.Now()}
			Expect(db.Create(sr).Error).NotTo(HaveOccurred())
			Expect(db.Exec("INSERT INTO spec_run_tags (spec_run_id, tag_id) VALUES (?, ?)", sr.ID, jiraTag.ID).Error).NotTo(HaveOccurred())

			result, err := repo.GetJiraTagCoverageByProject(ctx, "proj-a")

			Expect(err).NotTo(HaveOccurred())
			Expect(result["PROJ-1"].Total).To(Equal(2))
			Expect(result["PROJ-1"].Passed).To(Equal(1))
			Expect(result["PROJ-1"].Failed).To(Equal(1))
		})

		It("returns counts for multiple jira issue keys", func() {
			tag1 := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			tag2 := &database.Tag{Name: "jira:PROJ-2", Category: "jira", Value: "PROJ-2"}
			Expect(db.Create(tag1).Error).NotTo(HaveOccurred())
			Expect(db.Create(tag2).Error).NotTo(HaveOccurred())

			tr1 := &database.TestRun{ProjectID: "proj-a", RunID: "run-1", Status: "passed", StartTime: time.Now()}
			tr2 := &database.TestRun{ProjectID: "proj-a", RunID: "run-2", Status: "failed", StartTime: time.Now()}
			Expect(db.Create(tr1).Error).NotTo(HaveOccurred())
			Expect(db.Create(tr2).Error).NotTo(HaveOccurred())

			Expect(db.Create(&database.TestRunTag{TestRunID: tr1.ID, TagID: tag1.ID}).Error).NotTo(HaveOccurred())
			Expect(db.Create(&database.TestRunTag{TestRunID: tr2.ID, TagID: tag2.ID}).Error).NotTo(HaveOccurred())

			result, err := repo.GetJiraTagCoverageByProject(ctx, "proj-a")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result["PROJ-1"].Total).To(Equal(1))
			Expect(result["PROJ-1"].Passed).To(Equal(1))
			Expect(result["PROJ-2"].Total).To(Equal(1))
			Expect(result["PROJ-2"].Failed).To(Equal(1))
		})

		It("excludes tags whose category is not 'jira'", func() {
			jiraTag := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			otherTag := &database.Tag{Name: "env:staging", Category: "env", Value: "staging"}
			Expect(db.Create(jiraTag).Error).NotTo(HaveOccurred())
			Expect(db.Create(otherTag).Error).NotTo(HaveOccurred())

			tr := &database.TestRun{ProjectID: "proj-a", RunID: "run-1", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(tr).Error).NotTo(HaveOccurred())

			Expect(db.Create(&database.TestRunTag{TestRunID: tr.ID, TagID: jiraTag.ID}).Error).NotTo(HaveOccurred())
			Expect(db.Create(&database.TestRunTag{TestRunID: tr.ID, TagID: otherTag.ID}).Error).NotTo(HaveOccurred())

			result, err := repo.GetJiraTagCoverageByProject(ctx, "proj-a")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result).To(HaveKey("PROJ-1"))
			Expect(result).NotTo(HaveKey("staging"))
		})

		It("excludes test runs from a different project", func() {
			jiraTag := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			Expect(db.Create(jiraTag).Error).NotTo(HaveOccurred())

			trA := &database.TestRun{ProjectID: "proj-a", RunID: "run-1", Status: "passed", StartTime: time.Now()}
			trB := &database.TestRun{ProjectID: "proj-b", RunID: "run-2", Status: "failed", StartTime: time.Now()}
			Expect(db.Create(trA).Error).NotTo(HaveOccurred())
			Expect(db.Create(trB).Error).NotTo(HaveOccurred())

			Expect(db.Create(&database.TestRunTag{TestRunID: trA.ID, TagID: jiraTag.ID}).Error).NotTo(HaveOccurred())
			Expect(db.Create(&database.TestRunTag{TestRunID: trB.ID, TagID: jiraTag.ID}).Error).NotTo(HaveOccurred())

			result, err := repo.GetJiraTagCoverageByProject(ctx, "proj-a")

			Expect(err).NotTo(HaveOccurred())
			Expect(result["PROJ-1"].Total).To(Equal(1))
			Expect(result["PROJ-1"].Passed).To(Equal(1))
			Expect(result["PROJ-1"].Failed).To(Equal(0))
		})

		It("excludes spec-run tags whose parent test run is soft-deleted", func() {
			jiraTag := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			Expect(db.Create(jiraTag).Error).NotTo(HaveOccurred())

			// A live test run with a tagged spec — should be counted.
			trLive := &database.TestRun{ProjectID: "proj-a", RunID: "run-live", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(trLive).Error).NotTo(HaveOccurred())
			suLive := &database.SuiteRun{TestRunID: trLive.ID, SuiteName: "suite-live", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(suLive).Error).NotTo(HaveOccurred())
			srLive := &database.SpecRun{SuiteRunID: suLive.ID, SpecName: "spec-live", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(srLive).Error).NotTo(HaveOccurred())
			Expect(db.Exec("INSERT INTO spec_run_tags (spec_run_id, tag_id) VALUES (?, ?)", srLive.ID, jiraTag.ID).Error).NotTo(HaveOccurred())

			// A soft-deleted test run with a tagged spec — must NOT be counted.
			trDel := &database.TestRun{ProjectID: "proj-a", RunID: "run-del", Status: "failed", StartTime: time.Now()}
			Expect(db.Create(trDel).Error).NotTo(HaveOccurred())
			suDel := &database.SuiteRun{TestRunID: trDel.ID, SuiteName: "suite-del", Status: "failed", StartTime: time.Now()}
			Expect(db.Create(suDel).Error).NotTo(HaveOccurred())
			srDel := &database.SpecRun{SuiteRunID: suDel.ID, SpecName: "spec-del", Status: "failed", StartTime: time.Now()}
			Expect(db.Create(srDel).Error).NotTo(HaveOccurred())
			Expect(db.Exec("INSERT INTO spec_run_tags (spec_run_id, tag_id) VALUES (?, ?)", srDel.ID, jiraTag.ID).Error).NotTo(HaveOccurred())
			Expect(db.Delete(trDel).Error).NotTo(HaveOccurred()) // soft delete

			result, err := repo.GetJiraTagCoverageByProject(ctx, "proj-a")

			Expect(err).NotTo(HaveOccurred())
			Expect(result["PROJ-1"].Total).To(Equal(1))
			Expect(result["PROJ-1"].Passed).To(Equal(1))
			Expect(result["PROJ-1"].Failed).To(Equal(0))
		})

		It("returns an empty map when no jira tags exist for the project", func() {
			tr := &database.TestRun{ProjectID: "proj-a", RunID: "run-1", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(tr).Error).NotTo(HaveOccurred())

			result, err := repo.GetJiraTagCoverageByProject(ctx, "proj-a")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
		})

		It("returns an error when the database is unavailable", func() {
			sqlDB, _ := db.DB()
			sqlDB.Close()

			result, err := repo.GetJiraTagCoverageByProject(ctx, "proj-a")

			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("GetSpecRunsByJiraTag", func() {
		It("returns spec-run-level tagged runs with spec and suite detail", func() {
			jiraTag := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			Expect(db.Create(jiraTag).Error).NotTo(HaveOccurred())

			tr := &database.TestRun{ProjectID: "proj-a", RunID: "run-1", Branch: "main", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(tr).Error).NotTo(HaveOccurred())
			su := &database.SuiteRun{TestRunID: tr.ID, SuiteName: "suite-1", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(su).Error).NotTo(HaveOccurred())
			sr := &database.SpecRun{SuiteRunID: su.ID, SpecName: "spec-1", Status: "passed", StartTime: time.Now(), Duration: 42}
			Expect(db.Create(sr).Error).NotTo(HaveOccurred())
			Expect(db.Exec("INSERT INTO spec_run_tags (spec_run_id, tag_id) VALUES (?, ?)", sr.ID, jiraTag.ID).Error).NotTo(HaveOccurred())

			rows, err := repo.GetSpecRunsByJiraTag(ctx, "proj-a", "PROJ-1")

			Expect(err).NotTo(HaveOccurred())
			Expect(rows).To(HaveLen(1))
			Expect(rows[0].SpecName).To(Equal("spec-1"))
			Expect(rows[0].SuiteName).To(Equal("suite-1"))
			Expect(rows[0].TestRunID).To(Equal("run-1"))
			Expect(rows[0].Branch).To(Equal("main"))
			Expect(rows[0].Status).To(Equal("passed"))
		})

		// Regression for the count/drill-down mismatch: GetJiraTagCoverageByProject counts
		// test-run-level tags, so the drill-down must surface them too (previously empty).
		It("returns test-run-level tagged runs even though they have no spec/suite", func() {
			jiraTag := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			Expect(db.Create(jiraTag).Error).NotTo(HaveOccurred())

			tr := &database.TestRun{ProjectID: "proj-a", RunID: "run-1", Branch: "main", Status: "failed", StartTime: time.Now()}
			Expect(db.Create(tr).Error).NotTo(HaveOccurred())
			Expect(db.Create(&database.TestRunTag{TestRunID: tr.ID, TagID: jiraTag.ID}).Error).NotTo(HaveOccurred())

			rows, err := repo.GetSpecRunsByJiraTag(ctx, "proj-a", "PROJ-1")

			Expect(err).NotTo(HaveOccurred())
			Expect(rows).To(HaveLen(1))
			Expect(rows[0].TestRunID).To(Equal("run-1"))
			Expect(rows[0].Status).To(Equal("failed"))
			Expect(rows[0].SpecName).To(BeEmpty())
			Expect(rows[0].SuiteName).To(BeEmpty())
		})

		It("returns both spec-run-level and test-run-level tagged runs for the same issue", func() {
			jiraTag := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			Expect(db.Create(jiraTag).Error).NotTo(HaveOccurred())

			tr := &database.TestRun{ProjectID: "proj-a", RunID: "run-1", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(tr).Error).NotTo(HaveOccurred())
			Expect(db.Create(&database.TestRunTag{TestRunID: tr.ID, TagID: jiraTag.ID}).Error).NotTo(HaveOccurred())
			su := &database.SuiteRun{TestRunID: tr.ID, SuiteName: "suite-1", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(su).Error).NotTo(HaveOccurred())
			sr := &database.SpecRun{SuiteRunID: su.ID, SpecName: "spec-1", Status: "passed", StartTime: time.Now()}
			Expect(db.Create(sr).Error).NotTo(HaveOccurred())
			Expect(db.Exec("INSERT INTO spec_run_tags (spec_run_id, tag_id) VALUES (?, ?)", sr.ID, jiraTag.ID).Error).NotTo(HaveOccurred())

			rows, err := repo.GetSpecRunsByJiraTag(ctx, "proj-a", "PROJ-1")

			Expect(err).NotTo(HaveOccurred())
			Expect(rows).To(HaveLen(2))
		})

		It("excludes runs whose test run is soft-deleted", func() {
			jiraTag := &database.Tag{Name: "jira:PROJ-1", Category: "jira", Value: "PROJ-1"}
			Expect(db.Create(jiraTag).Error).NotTo(HaveOccurred())

			tr := &database.TestRun{ProjectID: "proj-a", RunID: "run-del", Status: "failed", StartTime: time.Now()}
			Expect(db.Create(tr).Error).NotTo(HaveOccurred())
			Expect(db.Create(&database.TestRunTag{TestRunID: tr.ID, TagID: jiraTag.ID}).Error).NotTo(HaveOccurred())
			su := &database.SuiteRun{TestRunID: tr.ID, SuiteName: "suite-1", Status: "failed", StartTime: time.Now()}
			Expect(db.Create(su).Error).NotTo(HaveOccurred())
			sr := &database.SpecRun{SuiteRunID: su.ID, SpecName: "spec-1", Status: "failed", StartTime: time.Now()}
			Expect(db.Create(sr).Error).NotTo(HaveOccurred())
			Expect(db.Exec("INSERT INTO spec_run_tags (spec_run_id, tag_id) VALUES (?, ?)", sr.ID, jiraTag.ID).Error).NotTo(HaveOccurred())

			Expect(db.Delete(tr).Error).NotTo(HaveOccurred()) // soft delete

			rows, err := repo.GetSpecRunsByJiraTag(ctx, "proj-a", "PROJ-1")

			Expect(err).NotTo(HaveOccurred())
			Expect(rows).To(BeEmpty())
		})
	})
})
