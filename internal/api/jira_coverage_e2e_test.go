package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	tagsInfra "github.com/guidewire-oss/fern-platform/internal/domains/tags/infrastructure"
	testingApp "github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	testingInfra "github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
	testMocks "github.com/guidewire-oss/fern-platform/internal/testhelpers"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// This is the end-to-end "supported scenario" test for JIRA release coverage:
// a test run ingested through the PUBLIC POST /api/v1/test-runs, carrying
// jira:<KEY> spec tags, must show up in GetJiraTagCoverageByProject.
//
// The per-layer tests exercise the ingest path and the coverage query only in
// ISOLATION (the repo test seeds junction rows directly). This test ties the
// two halves together — proving a tag that arrives through the real ingest API
// actually lands in the coverage result — which is exactly the seam that
// isolated unit tests can't catch.
var _ = Describe("E2E: JIRA-tagged ingest -> release coverage", Label("integration", "api", "jira", "coverage", "e2e"), func() {
	var (
		db      *gorm.DB
		router  *gin.Engine
		covRepo *tagsInfra.GormTagRepository
		ctx     context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		Expect(err).NotTo(HaveOccurred())
		Expect(db.AutoMigrate(
			&database.Tag{},
			&database.TestRun{},
			&database.TestRunTag{},
			&database.SuiteRun{},
			&database.SpecRun{},
		)).To(Succeed())

		logger, err := logging.NewLogger(&config.LoggingConfig{Level: "error", Format: "json"})
		Expect(err).NotTo(HaveOccurred())

		// Real, gorm-backed services so ingest actually persists tag junctions.
		tagService := tagsApp.NewTagService(tagsInfra.NewGormTagRepository(db))
		testingService := testingApp.NewTestRunService(
			testingInfra.NewGormTestRunRepository(db),
			testingInfra.NewGormSuiteRunRepository(db),
			testingInfra.NewGormSpecRunRepository(db),
		)
		// projectService is unused by the public record path; a mock keeps NewTestRunHandler happy.
		projectService := projectsApp.NewProjectService(
			new(testMocks.MockProjectRepository),
			new(testMocks.MockProjectPermissionRepository),
		)

		handler := NewTestRunHandler(testingService, projectService, logger)
		handler.SetTagService(tagService)

		gin.SetMode(gin.TestMode)
		router = gin.New()
		userGroup := router.Group("/api/v1")
		handler.RegisterRoutes(userGroup, router.Group("/api/v1/admin"))
		// The public ingest endpoint (POST /api/v1/test-runs) lives on the
		// submission routes, which reporters actually call.
		handler.RegisterSubmissionRoutes(userGroup)

		covRepo = tagsInfra.NewGormTagRepository(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	ingest := func(payload map[string]interface{}) int {
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}

	It("surfaces jira:<KEY> spec tags (passed/failed, multi-tag) in coverage", func() {
		iso := time.Now().UTC().Format(time.RFC3339)
		code := ingest(map[string]interface{}{
			"test_project_id": "e2e-proj",
			"git_branch":      "main",
			"environment":     "ci",
			"suite_runs": []map[string]interface{}{{
				"suite_name": "checkout",
				"start_time": iso,
				"end_time":   iso,
				"spec_runs": []map[string]interface{}{
					{"spec_description": "guest checkout", "status": "passed", "start_time": iso, "end_time": iso,
						"tags": []map[string]string{{"name": "jira:E2E-1"}}},
					{"spec_description": "discount code", "status": "failed", "start_time": iso, "end_time": iso,
						"tags": []map[string]string{{"name": "jira:E2E-2"}}},
					{"spec_description": "tax calc", "status": "passed", "start_time": iso, "end_time": iso,
						"tags": []map[string]string{{"name": "jira:E2E-1"}, {"name": "jira:E2E-3"}}},
				},
			}},
		})
		Expect(code).To(BeNumerically("<", 300), "ingest via POST /api/v1/test-runs should succeed")

		result, err := covRepo.GetJiraTagCoverageByProject(ctx, "e2e-proj")
		Expect(err).NotTo(HaveOccurred())

		// Keys come back upper-cased (matches JIRA's canonical casing).
		// E2E-1: two passing specs. E2E-2: one failing. E2E-3: one passing (multi-tagged with E2E-1).
		Expect(result).To(HaveKey("E2E-1"))
		Expect(result["E2E-1"].Total).To(Equal(2))
		Expect(result["E2E-1"].Passed).To(Equal(2))
		Expect(result["E2E-1"].Failed).To(Equal(0))

		Expect(result).To(HaveKey("E2E-2"))
		Expect(result["E2E-2"].Total).To(Equal(1))
		Expect(result["E2E-2"].Failed).To(Equal(1))

		Expect(result).To(HaveKey("E2E-3"))
		Expect(result["E2E-3"].Total).To(Equal(1))
		Expect(result["E2E-3"].Passed).To(Equal(1))
	})
})
