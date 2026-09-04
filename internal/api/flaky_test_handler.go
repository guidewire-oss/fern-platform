// Package api provides domain-based REST API handlers
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	analyticsApp "github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	analyticsDomain "github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// parseFlakyTestID reads :id as a row ID and writes its own error response.
func parseFlakyTestID(c *gin.Context) (uint, bool) {
	raw := c.Param("id")
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "test ID is required"})
		return 0, false
	}

	id, err := analyticsDomain.ParseFlakyTestRowID(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "test ID must be a positive integer"})
		return 0, false
	}

	return id, true
}

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

// getFlakyTests handles GET /api/v1/flaky-tests
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

// resolveFlakyTest handles POST /api/v1/flaky-tests/:id/resolve
func (h *FlakyTestHandler) resolveFlakyTest(c *gin.Context) {
	id, ok := parseFlakyTestID(c)
	if !ok {
		return
	}

	if err := h.flakyDetectionService.MarkTestResolved(c.Request.Context(), id); err != nil {
		if errors.Is(err, analyticsDomain.ErrFlakyTestNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Flaky test not found"})
			return
		}
		h.logger.WithError(err).Error("Failed to mark test as resolved")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark test as resolved"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test marked as resolved",
	})
}

// ignoreFlakyTest handles POST /api/v1/flaky-tests/:id/ignore
func (h *FlakyTestHandler) ignoreFlakyTest(c *gin.Context) {
	id, ok := parseFlakyTestID(c)
	if !ok {
		return
	}

	if err := h.flakyDetectionService.IgnoreTest(c.Request.Context(), id); err != nil {
		if errors.Is(err, analyticsDomain.ErrFlakyTestNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Flaky test not found"})
			return
		}
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
