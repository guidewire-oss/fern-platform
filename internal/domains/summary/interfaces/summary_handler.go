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

// GetSummary godoc
// @Summary      Get test summary
// @Description  Returns an aggregated test summary for a project, optionally grouped by one or more dimensions
// @Tags         test-runs
// @Produce      json
// @Param        projectId  path      string    true   "Project ID"
// @Param        seed       path      string    true   "Test seed or run identifier"
// @Param        group_by   query     []string  false  "Dimensions to group results by (repeatable)"  collectionFormat(multi)
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/summary/{projectId}/{seed} [get]
// @Security     BearerAuth
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
