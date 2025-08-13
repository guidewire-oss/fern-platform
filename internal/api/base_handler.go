// Package api provides domain-based REST API handlers
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	logger *logging.Logger
}

// NewBaseHandler creates a new base handler
func NewBaseHandler(logger *logging.Logger) *BaseHandler {
	return &BaseHandler{
		logger: logger,
	}
}

// respondWithError sends an error response
func (h *BaseHandler) respondWithError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"error": message})
}

// respondWithJSON sends a JSON response
func (h *BaseHandler) respondWithJSON(c *gin.Context, code int, payload interface{}) {
	c.JSON(code, payload)
}

// getUserID extracts the user ID from the context
func (h *BaseHandler) getUserID(c *gin.Context) string {
	userID, _ := c.Get("user_id")
	return userID.(string)
}

// getTeamID extracts the team ID from the context
func (h *BaseHandler) getTeamID(c *gin.Context) string {
	teamID, _ := c.Get("team_id")
	return teamID.(string)
}

// getUserRole extracts the user role from the context
func (h *BaseHandler) getUserRole(c *gin.Context) string {
	role, _ := c.Get("role")
	return role.(string)
}

// isAdmin checks if the user has admin role
func (h *BaseHandler) isAdmin(c *gin.Context) bool {
	return h.getUserRole(c) == "admin"
}

// isManager checks if the user has manager role
func (h *BaseHandler) isManager(c *gin.Context) bool {
	role := h.getUserRole(c)
	return role == "admin" || role == "manager"
}

// getUserEmail extracts the user email from the context
func (h *BaseHandler) getUserEmail(c *gin.Context) string {
	email, _ := c.Get("user_email")
	return email.(string)
}

// ErrorResponse sends an error response with the given status code and message
func (h *BaseHandler) ErrorResponse(c *gin.Context, code int, message string) {
	h.respondWithError(c, code, message)
}