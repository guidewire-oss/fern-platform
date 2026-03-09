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

var _ = Describe("BaseHandler", func() {
	var (
		handler *BaseHandler
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		logger, err := logging.NewLogger(&config.LoggingConfig{Level: "info", Format: "json"})
		Expect(err).NotTo(HaveOccurred())
		handler = NewBaseHandler(logger)
	})

	Describe("respondWithError", func() {
		It("returns the given status code and error message", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handler.respondWithError(c, http.StatusBadRequest, "invalid request")

			Expect(w.Code).To(Equal(http.StatusBadRequest))
			var body map[string]string
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["error"]).To(Equal("invalid request"))
		})

		It("returns 500 with server error message", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handler.respondWithError(c, http.StatusInternalServerError, "something went wrong")

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			var body map[string]string
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["error"]).To(Equal("something went wrong"))
		})
	})

	Describe("respondWithJSON", func() {
		It("returns the given status code and payload", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			payload := gin.H{"name": "test", "value": 42}
			handler.respondWithJSON(c, http.StatusOK, payload)

			Expect(w.Code).To(Equal(http.StatusOK))
			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["name"]).To(Equal("test"))
			Expect(body["value"]).To(BeNumerically("==", 42))
		})

		It("returns 201 with created resource", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			payload := gin.H{"id": "abc-123"}
			handler.respondWithJSON(c, http.StatusCreated, payload)

			Expect(w.Code).To(Equal(http.StatusCreated))
			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["id"]).To(Equal("abc-123"))
		})
	})

	Describe("getUserID", func() {
		It("returns the user ID from context", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_id", "user-123")

			result := handler.getUserID(c)
			Expect(result).To(Equal("user-123"))
		})
	})

	Describe("getTeamID", func() {
		It("returns the team ID from context", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("team_id", "team-456")

			result := handler.getTeamID(c)
			Expect(result).To(Equal("team-456"))
		})
	})

	Describe("getUserRole", func() {
		It("returns the user role from context", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("role", "admin")

			result := handler.getUserRole(c)
			Expect(result).To(Equal("admin"))
		})
	})

	Describe("isAdmin", func() {
		It("returns true when user role is admin", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("role", "admin")

			Expect(handler.isAdmin(c)).To(BeTrue())
		})

		It("returns false when user role is not admin", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("role", "viewer")

			Expect(handler.isAdmin(c)).To(BeFalse())
		})

		It("returns false when user role is manager", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("role", "manager")

			Expect(handler.isAdmin(c)).To(BeFalse())
		})
	})

	Describe("isManager", func() {
		It("returns true when user role is admin", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("role", "admin")

			Expect(handler.isManager(c)).To(BeTrue())
		})

		It("returns true when user role is manager", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("role", "manager")

			Expect(handler.isManager(c)).To(BeTrue())
		})

		It("returns false when user role is viewer", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("role", "viewer")

			Expect(handler.isManager(c)).To(BeFalse())
		})
	})

	Describe("getUserEmail", func() {
		It("returns the user email from context", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("user_email", "test@example.com")

			result := handler.getUserEmail(c)
			Expect(result).To(Equal("test@example.com"))
		})
	})

	Describe("ErrorResponse", func() {
		It("delegates to respondWithError", func() {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			handler.ErrorResponse(c, http.StatusForbidden, "access denied")

			Expect(w.Code).To(Equal(http.StatusForbidden))
			var body map[string]string
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["error"]).To(Equal("access denied"))
		})
	})
})
