package interfaces_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/internal/domains/summary/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/summary/infrastructure"
	"github.com/guidewire-oss/fern-platform/internal/domains/summary/interfaces"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

func TestSummaryHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Summary Handler Suite")
}

type testEnv struct {
	db      *gorm.DB
	router  *gin.Engine
	handler *interfaces.SummaryHandler
}

func setupTestEnv() testEnv {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	Expect(err).ToNot(HaveOccurred())

	// Auto-migrate tables
	err = db.AutoMigrate(
		&database.ProjectDetails{},
		&database.TestRun{},
		&database.SuiteRun{},
		&database.SpecRun{},
		&database.Tag{},
	)
	Expect(err).ToNot(HaveOccurred())

	// Setup handler and router
	repo := infrastructure.NewGormSummaryRepository(db)
	service := application.NewSummaryService(repo)
	handler := interfaces.NewSummaryHandler(service)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/api/v1/summary/:projectId/:seed", handler.GetSummary)

	return testEnv{
		db:      db,
		router:  router,
		handler: handler,
	}
}

func createTestData(db *gorm.DB, projectID, seed string, specs []struct {
	Status string
	Tags   map[string]string
}) {
	// Create test run
	testRun := &database.TestRun{
		ProjectID: projectID,
		RunID:     seed,
		Branch:    "main",
		CommitSHA: "abc123",
		Status:    "passed",
		StartTime: time.Now().Add(-10 * time.Minute),
	}
	endTime := time.Now().Add(-5 * time.Minute)
	testRun.EndTime = &endTime
	Expect(db.Create(testRun).Error).ToNot(HaveOccurred())

	// Create suite run
	suiteRun := &database.SuiteRun{
		TestRunID: testRun.ID,
		SuiteName: "acceptance",
		StartTime: testRun.StartTime,
	}
	suiteRun.EndTime = testRun.EndTime
	Expect(db.Create(suiteRun).Error).ToNot(HaveOccurred())

	// Create spec runs with tags
	for _, spec := range specs {
		specRun := &database.SpecRun{
			SuiteRunID: suiteRun.ID,
			SpecName:   "test-spec",
			Status:     spec.Status,
			StartTime:  testRun.StartTime,
			EndTime:    testRun.EndTime,
		}
		Expect(db.Create(specRun).Error).ToNot(HaveOccurred())

		// Create and associate tags
		for category, value := range spec.Tags {
			tag := &database.Tag{
				Name:     fmt.Sprintf("%s:%s", category, value),
				Category: category,
				Value:    value,
			}
			// Use FirstOrCreate to avoid duplicate tag errors
			Expect(db.Where("name = ?", tag.Name).FirstOrCreate(tag).Error).ToNot(HaveOccurred())
			Expect(db.Model(specRun).Association("Tags").Append(tag)).To(Succeed())
		}
	}
}

var _ = Describe("SummaryHandler", func() {
	var env testEnv

	BeforeEach(func() {
		env = setupTestEnv()
	})

	Describe("GetSummary", func() {
		Context("with valid project and seed", func() {
			It("returns test summary grouped by multiple tags", func() {
				// Create test data
				createTestData(env.db, "project-123", "seed-456", []struct {
					Status string
					Tags   map[string]string
				}{
					{
						Status: "passed",
						Tags: map[string]string{
							"testtype":  "acceptance",
							"component": "jspolicy",
							"owner":     "capitola", //nolint:misspell
							"category":  "infrastructure",
						},
					},
					{
						Status: "passed",
						Tags: map[string]string{
							"testtype":  "acceptance",
							"component": "jspolicy",
							"owner":     "capitola", //nolint:misspell
							"category":  "infrastructure",
						},
					},
				})

				// Make request
				url := "/api/v1/summary/project-123/seed-456?group_by=testtype&group_by=component&group_by=owner&group_by=category"
				req, _ := http.NewRequest(http.MethodGet, url, nil)
				rec := httptest.NewRecorder()

				env.router.ServeHTTP(rec, req)

				// Verify response
				Expect(rec.Code).To(Equal(http.StatusOK))

				var response map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				Expect(err).ToNot(HaveOccurred())

				Expect(response["project_id"]).To(Equal("project-123"))
				Expect(response["seed"]).To(Equal("seed-456"))
				Expect(response["branch"]).To(Equal("main"))
				Expect(response["status"]).To(Equal("passed"))
				Expect(int(response["tests"].(float64))).To(Equal(2))

				summary := response["summary"].([]interface{})
				Expect(summary).To(HaveLen(1))

				entry := summary[0].(map[string]interface{})
				Expect(entry["testtype"]).To(Equal("acceptance"))
				Expect(entry["component"]).To(Equal("jspolicy"))
				Expect(entry["owner"]).To(Equal("capitola")) //nolint:misspell
				Expect(entry["category"]).To(Equal("infrastructure"))
				Expect(int(entry["passed"].(float64))).To(Equal(2))
			})

			It("returns test summary for multiple components", func() {
				createTestData(env.db, "project-123", "seed-789", []struct {
					Status string
					Tags   map[string]string
				}{
					{
						Status: "passed",
						Tags: map[string]string{
							"testtype":  "acceptance",
							"component": "jspolicy",
							"owner":     "capitola", //nolint:misspell
						},
					},
					{
						Status: "passed",
						Tags: map[string]string{
							"testtype":  "acceptance",
							"component": "keda",
							"owner":     "capitola", //nolint:misspell
						},
					},
				})

				url := "/api/v1/summary/project-123/seed-789?group_by=testtype&group_by=component"
				req, _ := http.NewRequest(http.MethodGet, url, nil)
				rec := httptest.NewRecorder()

				env.router.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))

				var response map[string]interface{}
				json.Unmarshal(rec.Body.Bytes(), &response)

				summary := response["summary"].([]interface{})
				Expect(summary).To(HaveLen(2))

				entry0 := summary[0].(map[string]interface{})
				Expect(entry0["component"]).To(Equal("jspolicy"))

				entry1 := summary[1].(map[string]interface{})
				Expect(entry1["component"]).To(Equal("keda"))
			})

			It("handles failed tests correctly", func() {
				createTestData(env.db, "project-123", "seed-fail", []struct {
					Status string
					Tags   map[string]string
				}{
					{
						Status: "passed",
						Tags: map[string]string{
							"testtype":  "acceptance",
							"component": "jspolicy",
						},
					},
					{
						Status: "failed",
						Tags: map[string]string{
							"testtype":  "acceptance",
							"component": "jspolicy",
						},
					},
				})

				url := "/api/v1/summary/project-123/seed-fail?group_by=testtype&group_by=component"
				req, _ := http.NewRequest(http.MethodGet, url, nil)
				rec := httptest.NewRecorder()

				env.router.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))

				var response map[string]interface{}
				json.Unmarshal(rec.Body.Bytes(), &response)

				Expect(response["status"]).To(Equal("failed"))

				summary := response["summary"].([]interface{})
				entry := summary[0].(map[string]interface{})
				Expect(int(entry["passed"].(float64))).To(Equal(1))
				Expect(int(entry["failed"].(float64))).To(Equal(1))
			})

			It("handles grouping by non-existent tag", func() {
				createTestData(env.db, "project-123", "seed-empty", []struct {
					Status string
					Tags   map[string]string
				}{
					{
						Status: "passed",
						Tags: map[string]string{
							"testtype": "acceptance",
						},
					},
				})

				url := "/api/v1/summary/project-123/seed-empty?group_by=banana"
				req, _ := http.NewRequest(http.MethodGet, url, nil)
				rec := httptest.NewRecorder()

				env.router.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))

				var response map[string]interface{}
				json.Unmarshal(rec.Body.Bytes(), &response)

				summary := response["summary"].([]interface{})
				Expect(summary).To(HaveLen(1))

				entry := summary[0].(map[string]interface{})
				Expect(entry["banana"]).To(Equal("unspecified"))
			})
		})

		Context("with no matching data", func() {
			It("returns empty summary", func() {
				url := "/api/v1/summary/nonexistent/seed-999?group_by=testtype"
				req, _ := http.NewRequest(http.MethodGet, url, nil)
				rec := httptest.NewRecorder()

				env.router.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))

				var response map[string]interface{}
				json.Unmarshal(rec.Body.Bytes(), &response)

				Expect(response["project_id"]).To(Equal("nonexistent"))
				Expect(response["seed"]).To(Equal("seed-999"))
				Expect(response["status"]).To(Equal("NA"))
				Expect(int(response["tests"].(float64))).To(Equal(0))

				summary := response["summary"].([]interface{})
				Expect(summary).To(HaveLen(0))
			})
		})

		Context("without group_by parameter", func() {
			It("returns overall summary", func() {
				createTestData(env.db, "project-123", "seed-overall", []struct {
					Status string
					Tags   map[string]string
				}{
					{
						Status: "passed",
						Tags: map[string]string{
							"testtype": "acceptance",
						},
					},
					{
						Status: "failed",
						Tags: map[string]string{
							"testtype": "integration",
						},
					},
				})

				url := "/api/v1/summary/project-123/seed-overall"
				req, _ := http.NewRequest(http.MethodGet, url, nil)
				rec := httptest.NewRecorder()

				env.router.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))

				var response map[string]interface{}
				json.Unmarshal(rec.Body.Bytes(), &response)

				Expect(response["status"]).To(Equal("failed"))
				Expect(int(response["tests"].(float64))).To(Equal(2))

				summary := response["summary"].([]interface{})
				Expect(summary).To(HaveLen(1))

				entry := summary[0].(map[string]interface{})
				Expect(int(entry["passed"].(float64))).To(Equal(1))
				Expect(int(entry["failed"].(float64))).To(Equal(1))
			})
		})
	})
})
