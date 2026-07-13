// Package api provides domain-based REST API handlers
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	analyticsApp "github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// FlakyTestHandler handles flaky test related endpoints
type FlakyTestHandler struct {
	*BaseHandler
	flakyDetectionService *analyticsApp.FlakyDetectionService
}

// NewFlakyTestHandler creates a new flaky test handler
func NewFlakyTestHandler(flakyDetectionService *analyticsApp.FlakyDetectionService, logger *logging.Logger) *FlakyTestHandler {
	return &FlakyTestHandler{
		BaseHandler:           NewBaseHandler(logger),
		flakyDetectionService: flakyDetectionService,
	}
}

// getFlakyTests godoc
// @Summary      List flaky tests
// @Description  Returns tests detected as intermittently failing for a project
// @Tags         flaky-tests
// @Produce      json
// @Param        projectId  query  string  true  "Project ID to query flaky tests for"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/flaky-tests [get]
// @Security     BearerAuth
func (h *FlakyTestHandler) getFlakyTests(c *gin.Context) {
	projectID := c.Query("projectId")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "projectId query parameter is required"})
		return
	}

	flakyTests, err := h.flakyDetectionService.GetFlakyTests(c.Request.Context(), projectID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get flaky tests")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get flaky tests"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"flaky_tests": flakyTests,
		"total":       len(flakyTests),
	})
}

// resolveFlakyTest godoc
// @Summary      Resolve a flaky test
// @Description  Marks a detected flaky test as resolved
// @Tags         flaky-tests
// @Produce      json
// @Param        id  path  string  true  "Flaky test ID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/flaky-tests/{id}/resolve [post]
// @Security     BearerAuth
func (h *FlakyTestHandler) resolveFlakyTest(c *gin.Context) {
	testID := c.Param("id")
	if testID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "test ID is required"})
		return
	}

	if err := h.flakyDetectionService.MarkTestResolved(c.Request.Context(), testID); err != nil {
		h.logger.WithError(err).Error("Failed to mark test as resolved")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark test as resolved"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test marked as resolved",
	})
}

// ignoreFlakyTest godoc
// @Summary      Ignore a flaky test
// @Description  Suppresses alerts for a detected flaky test
// @Tags         flaky-tests
// @Produce      json
// @Param        id  path  string  true  "Flaky test ID"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/flaky-tests/{id}/ignore [post]
// @Security     BearerAuth
func (h *FlakyTestHandler) ignoreFlakyTest(c *gin.Context) {
	testID := c.Param("id")
	if testID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "test ID is required"})
		return
	}

	if err := h.flakyDetectionService.IgnoreTest(c.Request.Context(), testID); err != nil {
		h.logger.WithError(err).Error("Failed to ignore test")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ignore test"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test marked as ignored",
	})
}

// RegisterRoutes registers flaky test routes
func (h *FlakyTestHandler) RegisterRoutes(userGroup *gin.RouterGroup) {
	userGroup.GET("/flaky-tests", h.getFlakyTests)
	userGroup.POST("/flaky-tests/:id/resolve", h.resolveFlakyTest)
	userGroup.POST("/flaky-tests/:id/ignore", h.ignoreFlakyTest)
}
