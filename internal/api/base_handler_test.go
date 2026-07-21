package api

import (
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BaseHandler context getters", func() {
	var handler *BaseHandler

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		handler = NewBaseHandler(nil)
	})

	// Regression for #180: the service-account auth path left user_id (and other
	// keys) unset in the gin context. The getters used to do a direct type
	// assertion on the nil interface, which panicked on project creation.
	Context("when context values are missing", func() {
		It("should return empty strings without panicking", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			Expect(func() {
				Expect(handler.getUserID(c)).To(Equal(""))
				Expect(handler.getTeamID(c)).To(Equal(""))
				Expect(handler.getUserRole(c)).To(Equal(""))
				Expect(handler.getUserEmail(c)).To(Equal(""))
			}).NotTo(Panic())
		})

		It("should report not-admin and not-manager", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			Expect(handler.isAdmin(c)).To(BeFalse())
			Expect(handler.isManager(c)).To(BeFalse())
		})
	})

	Context("when context values are present", func() {
		It("should return the values set in context", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("user_id", "user-1")
			c.Set("team_id", "team-1")
			c.Set("user_role", "manager")
			c.Set("user_email", "alice@example.com")

			Expect(handler.getUserID(c)).To(Equal("user-1"))
			Expect(handler.getTeamID(c)).To(Equal("team-1"))
			Expect(handler.getUserRole(c)).To(Equal("manager"))
			Expect(handler.getUserEmail(c)).To(Equal("alice@example.com"))
			Expect(handler.isManager(c)).To(BeTrue())
			Expect(handler.isAdmin(c)).To(BeFalse())
		})
	})
})
