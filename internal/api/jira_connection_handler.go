package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
)

// JiraConnectionHandler handles JIRA connection HTTP requests
type JiraConnectionHandler struct {
	*BaseHandler
	jiraService    *integrations.JiraConnectionService
	projectService *projectsApp.ProjectService
}

// NewJiraConnectionHandler creates a new JIRA connection handler
func NewJiraConnectionHandler(
	baseHandler *BaseHandler,
	jiraService *integrations.JiraConnectionService,
	projectService *projectsApp.ProjectService,
) *JiraConnectionHandler {
	return &JiraConnectionHandler{
		BaseHandler:    baseHandler,
		jiraService:    jiraService,
		projectService: projectService,
	}
}

// CreateJiraConnectionRequest represents the request to create a JIRA connection
type CreateJiraConnectionRequest struct {
	Name               string `json:"name" binding:"required"`
	JiraURL            string `json:"jiraUrl" binding:"required"`
	AuthenticationType string `json:"authenticationType" binding:"required"`
	ProjectKey         string `json:"projectKey" binding:"required"`
	Username           string `json:"username"`
	Credential         string `json:"credential" binding:"required"`
}

// UpdateJiraConnectionRequest represents the request to update a JIRA connection
type UpdateJiraConnectionRequest struct {
	Name       string `json:"name"`
	JiraURL    string `json:"jiraUrl"`
	ProjectKey string `json:"projectKey"`
}

// UpdateJiraCredentialsRequest represents the request to update JIRA credentials
type UpdateJiraCredentialsRequest struct {
	AuthenticationType string `json:"authenticationType" binding:"required"`
	Username           string `json:"username"`
	Credential         string `json:"credential" binding:"required"`
}

// JiraConnectionResponse represents a JIRA connection response
type JiraConnectionResponse struct {
	ID                 string  `json:"id"`
	ProjectID          string  `json:"projectId"`
	Name               string  `json:"name"`
	JiraURL            string  `json:"jiraUrl"`
	AuthenticationType string  `json:"authenticationType"`
	ProjectKey         string  `json:"projectKey"`
	Username           string  `json:"username"`
	Status             string  `json:"status"`
	IsActive           bool    `json:"isActive"`
	LastTestedAt       *string `json:"lastTestedAt,omitempty"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

// CreateConnection creates a new JIRA connection
func (h *JiraConnectionHandler) CreateConnection(c *gin.Context) {
	projectID := c.Param("projectId")
	
	// Check if user can manage the project
	userID := h.getUserID(c)
	if userID == "" {
		h.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Check project permissions
	permissions, err := h.projectService.GetUserPermissions(c.Request.Context(), projectsDomain.ProjectID(projectID), userID)
	if err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, "failed to get permissions")
		return
	}

	// Check if user has write permission (needed to manage connections)
	canManage := false
	for _, perm := range permissions {
		if perm.CanWrite() || perm.CanAdmin() {
			canManage = true
			break
		}
	}

	if !canManage {
		h.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	var req CreateJiraConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	connection, err := h.jiraService.CreateConnection(
		c.Request.Context(),
		projectID,
		req.Name,
		req.JiraURL,
		integrations.AuthenticationType(req.AuthenticationType),
		req.ProjectKey,
		req.Username,
		req.Credential,
	)
	if err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondWithJSON(c, http.StatusCreated, h.convertToResponse(connection))
}

// GetConnections retrieves all JIRA connections for a project
func (h *JiraConnectionHandler) GetConnections(c *gin.Context) {
	projectID := c.Param("projectId")
	
	// Check if user can view the project
	userID := h.getUserID(c)
	if userID == "" {
		h.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	connections, err := h.jiraService.GetProjectConnections(c.Request.Context(), projectID)
	if err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]JiraConnectionResponse, len(connections))
	for i, conn := range connections {
		responses[i] = *h.convertToResponse(conn)
	}

	h.respondWithJSON(c, http.StatusOK, responses)
}

// GetConnection retrieves a specific JIRA connection
func (h *JiraConnectionHandler) GetConnection(c *gin.Context) {
	connectionID := c.Param("connectionId")
	
	// Check if user can manage the connection
	userID := h.getUserID(c)
	if userID == "" {
		h.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	connection, err := h.jiraService.GetConnection(c.Request.Context(), connectionID)
	if err != nil {
		h.ErrorResponse(c, http.StatusNotFound, "connection not found")
		return
	}

	// Check project permissions
	permissions, err := h.projectService.GetUserPermissions(c.Request.Context(), projectsDomain.ProjectID(connection.ProjectID()), userID)
	if err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, "failed to get permissions")
		return
	}

	// Check if user has read permission
	canView := false
	for _, perm := range permissions {
		if perm.CanRead() {
			canView = true
			break
		}
	}

	if !canView {
		h.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	h.respondWithJSON(c, http.StatusOK, h.convertToResponse(connection))
}

// UpdateConnection updates a JIRA connection
func (h *JiraConnectionHandler) UpdateConnection(c *gin.Context) {
	connectionID := c.Param("connectionId")
	
	// Check if user can manage the connection
	userID := h.getUserID(c)
	if userID == "" {
		h.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	connection, err := h.jiraService.GetConnection(c.Request.Context(), connectionID)
	if err != nil {
		h.ErrorResponse(c, http.StatusNotFound, "connection not found")
		return
	}

	// Check project permissions
	permissions, err := h.projectService.GetUserPermissions(c.Request.Context(), projectsDomain.ProjectID(connection.ProjectID()), userID)
	if err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, "failed to get permissions")
		return
	}

	// Check if user has write permission
	canManage := false
	for _, perm := range permissions {
		if perm.CanWrite() || perm.CanAdmin() {
			canManage = true
			break
		}
	}

	if !canManage {
		h.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	var req UpdateJiraConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.jiraService.UpdateConnection(
		c.Request.Context(),
		connectionID,
		req.Name,
		req.JiraURL,
		req.ProjectKey,
	)
	if err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondWithJSON(c, http.StatusOK, h.convertToResponse(updated))
}

// UpdateCredentials updates JIRA connection credentials
func (h *JiraConnectionHandler) UpdateCredentials(c *gin.Context) {
	connectionID := c.Param("connectionId")
	
	// Check if user can manage the connection
	userID := h.getUserID(c)
	if userID == "" {
		h.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	connection, err := h.jiraService.GetConnection(c.Request.Context(), connectionID)
	if err != nil {
		h.ErrorResponse(c, http.StatusNotFound, "connection not found")
		return
	}

	// Check project permissions
	permissions, err := h.projectService.GetUserPermissions(c.Request.Context(), projectsDomain.ProjectID(connection.ProjectID()), userID)
	if err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, "failed to get permissions")
		return
	}

	// Check if user has write permission
	canManage := false
	for _, perm := range permissions {
		if perm.CanWrite() || perm.CanAdmin() {
			canManage = true
			break
		}
	}

	if !canManage {
		h.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	var req UpdateJiraCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.jiraService.UpdateCredentials(
		c.Request.Context(),
		connectionID,
		integrations.AuthenticationType(req.AuthenticationType),
		req.Username,
		req.Credential,
	)
	if err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondWithJSON(c, http.StatusOK, h.convertToResponse(updated))
}

// TestConnection tests a JIRA connection
func (h *JiraConnectionHandler) TestConnection(c *gin.Context) {
	connectionID := c.Param("connectionId")
	
	// Check if user can manage the connection
	userID := h.getUserID(c)
	if userID == "" {
		h.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	connection, err := h.jiraService.GetConnection(c.Request.Context(), connectionID)
	if err != nil {
		h.ErrorResponse(c, http.StatusNotFound, "connection not found")
		return
	}

	// Check project permissions
	permissions, err := h.projectService.GetUserPermissions(c.Request.Context(), projectsDomain.ProjectID(connection.ProjectID()), userID)
	if err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, "failed to get permissions")
		return
	}

	// Check if user has write permission
	canManage := false
	for _, perm := range permissions {
		if perm.CanWrite() || perm.CanAdmin() {
			canManage = true
			break
		}
	}

	if !canManage {
		h.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.jiraService.TestConnection(c.Request.Context(), connectionID); err != nil {
		h.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	h.respondWithJSON(c, http.StatusOK, gin.H{"message": "Connection test successful"})
}

// DeleteConnection deletes a JIRA connection
func (h *JiraConnectionHandler) DeleteConnection(c *gin.Context) {
	connectionID := c.Param("connectionId")
	
	// Check if user can manage the connection
	userID := h.getUserID(c)
	if userID == "" {
		h.ErrorResponse(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	connection, err := h.jiraService.GetConnection(c.Request.Context(), connectionID)
	if err != nil {
		h.ErrorResponse(c, http.StatusNotFound, "connection not found")
		return
	}

	// Check project permissions
	permissions, err := h.projectService.GetUserPermissions(c.Request.Context(), projectsDomain.ProjectID(connection.ProjectID()), userID)
	if err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, "failed to get permissions")
		return
	}

	// Check if user has write permission
	canManage := false
	for _, perm := range permissions {
		if perm.CanWrite() || perm.CanAdmin() {
			canManage = true
			break
		}
	}

	if !canManage {
		h.ErrorResponse(c, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.jiraService.DeleteConnection(c.Request.Context(), connectionID); err != nil {
		h.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondWithJSON(c, http.StatusNoContent, nil)
}

// convertToResponse converts a domain entity to response format
func (h *JiraConnectionHandler) convertToResponse(conn *integrations.JiraConnection) *JiraConnectionResponse {
	snapshot := conn.Snapshot()
	
	var lastTested *string
	if snapshot.LastTestedAt != nil {
		formatted := snapshot.LastTestedAt.Format(time.RFC3339)
		lastTested = &formatted
	}
	
	return &JiraConnectionResponse{
		ID:                 snapshot.ID,
		ProjectID:          snapshot.ProjectID,
		Name:               snapshot.Name,
		JiraURL:            snapshot.JiraURL,
		AuthenticationType: string(snapshot.AuthenticationType),
		ProjectKey:         snapshot.ProjectKey,
		Username:           snapshot.Username,
		Status:             string(snapshot.Status),
		IsActive:           snapshot.IsActive,
		LastTestedAt:       lastTested,
		CreatedAt:          snapshot.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          snapshot.UpdatedAt.Format(time.RFC3339),
	}
}