// Package api provides domain-based REST API handlers
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	*BaseHandler
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(logger *logging.Logger) *HealthHandler {
	return &HealthHandler{
		BaseHandler: NewBaseHandler(logger),
	}
}

// healthCheck handles GET /api/v1/health
func (h *HealthHandler) healthCheck(c *gin.Context) {
	h.respondWithJSON(c, http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "fern-platform",
		"version":   "1.0.0", // TODO: Get from build info
	})
}

// RegisterRoutes registers health routes
func (h *HealthHandler) RegisterRoutes(publicGroup *gin.RouterGroup) {
	publicGroup.GET("/health", h.healthCheck)
}