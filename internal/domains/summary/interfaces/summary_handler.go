package interfaces

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/internal/domains/summary/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/summary/domain"
)

// SummaryHandler handles HTTP requests for test summary
type SummaryHandler struct {
	service *application.SummaryService
}

// NewSummaryHandler creates a new summary handler
func NewSummaryHandler(service *application.SummaryService) *SummaryHandler {
	return &SummaryHandler{service: service}
}

// GetSummary handles GET requests for test summary
// Path: /api/v1/summary/:projectId/:seed
// Query params: group_by (can be repeated)
func (h *SummaryHandler) GetSummary(c *gin.Context) {
	projectUUID := c.Param("projectId")
	seed := c.Param("seed")

	// Get group_by query parameters (can be multiple)
	groupBy := c.QueryArray("group_by")

	// Build request
	req := domain.SummaryRequest{
		ProjectUUID: projectUUID,
		Seed:        seed,
		GroupBy:     groupBy,
	}

	// Get summary from service
	summary, err := h.service.GetSummary(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}
