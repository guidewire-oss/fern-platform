package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

var _ = Describe("SystemHandler", func() {
	var (
		handler *SystemHandler
		router  *gin.Engine
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		logger, err := logging.NewLogger(&config.LoggingConfig{Level: "info", Format: "json"})
		Expect(err).NotTo(HaveOccurred())
		handler = NewSystemHandler(logger)
		router = gin.New()
		adminGroup := router.Group("/api/v1/admin")
		handler.RegisterRoutes(adminGroup)
	})

	Describe("GET /api/v1/admin/system/stats", func() {
		It("returns 200 with system statistics", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/stats", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body).To(HaveKey("memory"))
			Expect(body).To(HaveKey("goroutines"))
			Expect(body).To(HaveKey("cpu_count"))
			Expect(body).To(HaveKey("go_version"))
			Expect(body).To(HaveKey("timestamp"))

			memory, ok := body["memory"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(memory).To(HaveKey("alloc_mb"))
			Expect(memory).To(HaveKey("total_alloc_mb"))
			Expect(memory).To(HaveKey("sys_mb"))
			Expect(memory).To(HaveKey("num_gc"))
		})

		It("returns positive goroutine count", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/stats", nil)
			router.ServeHTTP(w, req)

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["goroutines"]).To(BeNumerically(">", 0))
			Expect(body["cpu_count"]).To(BeNumerically(">", 0))
		})
	})

	Describe("GET /api/v1/admin/system/health", func() {
		It("returns 200 with health check results", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/health", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["status"]).To(Equal("healthy"))
			Expect(body).To(HaveKey("checks"))
			Expect(body).To(HaveKey("timestamp"))

			checks, ok := body["checks"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(checks).To(HaveKey("database"))
			Expect(checks).To(HaveKey("redis"))
		})
	})

	Describe("POST /api/v1/admin/system/cleanup", func() {
		It("returns 200 with cleanup confirmation", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/system/cleanup", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["message"]).To(Equal("System cleanup completed"))
			Expect(body).To(HaveKey("timestamp"))
		})
	})

	Describe("GET /api/v1/admin/audit-logs", func() {
		It("returns 200 with empty audit logs", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			items, ok := body["items"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(items).To(BeEmpty())
			Expect(body["total"]).To(BeNumerically("==", 0))
		})
	})
})
