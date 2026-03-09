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

var _ = Describe("HealthHandler", func() {
	var (
		handler *HealthHandler
		router  *gin.Engine
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		logger, err := logging.NewLogger(&config.LoggingConfig{Level: "info", Format: "json"})
		Expect(err).NotTo(HaveOccurred())
		handler = NewHealthHandler(logger)
		router = gin.New()
		group := router.Group("/api/v1")
		handler.RegisterRoutes(group)
	})

	Describe("GET /api/v1/health", func() {
		It("returns 200 with health status", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["status"]).To(Equal("healthy"))
			Expect(body["service"]).To(Equal("fern-platform"))
			Expect(body["version"]).To(Equal("1.0.0"))
			Expect(body).To(HaveKey("timestamp"))
		})

		It("returns a numeric timestamp", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			router.ServeHTTP(w, req)

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["timestamp"]).To(BeNumerically(">", 0))
		})
	})
})
