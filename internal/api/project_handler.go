// Package api provides domain-based REST API handlers
package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"gorm.io/gorm"
)

// ProjectHandler handles project related endpoints
type ProjectHandler struct {
	*BaseHandler
	projectService *projectsApp.ProjectService
	// db is used for the admin access-management endpoints
	// (grant/revoke/list users), which read from and write to the
	// shared `project_permissions` table. Direct SQL here keeps the
	// scope small; a future refactor can lift these into the projects
	// domain repo once the read model stabilizes.
	db *gorm.DB
}

// NewProjectHandler creates a new project handler
func NewProjectHandler(projectService *projectsApp.ProjectService, db *gorm.DB, logger *logging.Logger) *ProjectHandler {
	return &ProjectHandler{
		BaseHandler:    NewBaseHandler(logger),
		projectService: projectService,
		db:             db,
	}
}

// createProject handles POST /api/v1/projects
func (h *ProjectHandler) createProject(c *gin.Context) {
	var input struct {
		ProjectID     string                 `json:"projectId"`
		Name          string                 `json:"name" binding:"required"`
		Description   string                 `json:"description"`
		Repository    string                 `json:"repository"`
		DefaultBranch string                 `json:"defaultBranch"`
		Team          string                 `json:"team" binding:"required"`
		Settings      map[string]interface{} `json:"settings"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get current user for creator ID
	userID := h.getUserID(c)

	// Generate project ID if not provided
	projectID := input.ProjectID
	if projectID == "" {
		projectID = uuid.New().String()
	}

	// Create project using domain service
	project, err := h.projectService.CreateProject(
		c.Request.Context(),
		projectsDomain.ProjectID(projectID),
		input.Name,
		projectsDomain.Team(input.Team),
		userID,
	)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create project")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update additional fields if provided
	if input.Description != "" || input.Repository != "" || input.DefaultBranch != "" || input.Settings != nil {
		updates := projectsApp.UpdateProjectRequest{}
		if input.Description != "" {
			updates.Description = &input.Description
		}
		if input.Repository != "" {
			updates.Repository = &input.Repository
		}
		if input.DefaultBranch != "" {
			updates.DefaultBranch = &input.DefaultBranch
		}
		if input.Settings != nil {
			updates.Settings = input.Settings
		}

		if err := h.projectService.UpdateProject(c.Request.Context(), project.ProjectID(), updates); err != nil {
			h.logger.WithError(err).Warn("Failed to update project details after creation")
		}
	}

	// Convert to API response format
	c.JSON(http.StatusCreated, h.convertProjectToAPI(project))
}

// getProject handles GET /api/v1/projects/:projectId
func (h *ProjectHandler) getProject(c *gin.Context) {
	projectIDStr := c.Param("projectId")

	// Try to parse as numeric ID first (for backward compatibility)
	if _, err := strconv.ParseUint(projectIDStr, 10, 32); err == nil {
		// This is a numeric ID - need to implement GetProjectByID in domain service
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Get project by numeric ID not yet implemented"})
		return
	}

	// Otherwise treat as project ID string
	project, err := h.projectService.GetProject(c.Request.Context(), projectsDomain.ProjectID(projectIDStr))
	if err != nil {
		if errors.Is(err, projectsDomain.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project"})
		return
	}

	c.JSON(http.StatusOK, h.convertProjectToAPI(project))
}

// getProjectByProjectID handles GET /api/v1/projects/by-project-id/:projectId
func (h *ProjectHandler) getProjectByProjectID(c *gin.Context) {
	projectID := c.Param("projectId")

	project, err := h.projectService.GetProject(c.Request.Context(), projectsDomain.ProjectID(projectID))
	if err != nil {
		if errors.Is(err, projectsDomain.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project"})
		return
	}

	c.JSON(http.StatusOK, h.convertProjectToAPI(project))
}

// updateProject handles PUT /api/v1/projects/:projectId
func (h *ProjectHandler) updateProject(c *gin.Context) {
	projectID := c.Param("projectId")

	var input struct {
		Name          string                 `json:"name"`
		Description   string                 `json:"description"`
		Repository    string                 `json:"repository"`
		DefaultBranch string                 `json:"defaultBranch"`
		Team          string                 `json:"team"`
		Settings      map[string]interface{} `json:"settings"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build update request
	updates := projectsApp.UpdateProjectRequest{}
	if input.Name != "" {
		updates.Name = &input.Name
	}
	if input.Description != "" {
		updates.Description = &input.Description
	}
	if input.Repository != "" {
		updates.Repository = &input.Repository
	}
	if input.DefaultBranch != "" {
		updates.DefaultBranch = &input.DefaultBranch
	}
	if input.Team != "" {
		team := projectsDomain.Team(input.Team)
		updates.Team = &team
	}
	if input.Settings != nil {
		updates.Settings = input.Settings
	}

	// Update project
	if err := h.projectService.UpdateProject(c.Request.Context(), projectsDomain.ProjectID(projectID), updates); err != nil {
		if errors.Is(err, projectsDomain.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get updated project
	project, err := h.projectService.GetProject(c.Request.Context(), projectsDomain.ProjectID(projectID))
	if err != nil {
		if errors.Is(err, projectsDomain.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project"})
		return
	}

	c.JSON(http.StatusOK, h.convertProjectToAPI(project))
}

// deleteProject handles DELETE /api/v1/projects/:projectId
func (h *ProjectHandler) deleteProject(c *gin.Context) {
	projectID := c.Param("projectId")

	if err := h.projectService.DeleteProject(c.Request.Context(), projectsDomain.ProjectID(projectID)); err != nil {
		if errors.Is(err, projectsDomain.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project deleted successfully"})
}

// listProjects handles GET /api/v1/projects
func (h *ProjectHandler) listProjects(c *gin.Context) {
	limit := 20
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	projects, total, err := h.projectService.ListProjects(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to API response format
	apiProjects := make([]interface{}, len(projects))
	for i, p := range projects {
		apiProjects[i] = h.convertProjectToAPI(p)
	}

	c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	c.JSON(http.StatusOK, apiProjects)
}

// activateProject handles POST /api/v1/projects/:projectId/activate
func (h *ProjectHandler) activateProject(c *gin.Context) {
	projectID := c.Param("projectId")

	if err := h.projectService.ActivateProject(c.Request.Context(), projectsDomain.ProjectID(projectID)); err != nil {
		if errors.Is(err, projectsDomain.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project activated"})
}

// deactivateProject handles POST /api/v1/projects/:projectId/deactivate
func (h *ProjectHandler) deactivateProject(c *gin.Context) {
	projectID := c.Param("projectId")

	if err := h.projectService.DeactivateProject(c.Request.Context(), projectsDomain.ProjectID(projectID)); err != nil {
		if errors.Is(err, projectsDomain.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project deactivated"})
}

// getProjectStats handles GET /api/v1/projects/stats/:projectId
func (h *ProjectHandler) getProjectStats(c *gin.Context) {
	projectID := c.Param("projectId")

	// TODO: Implement actual stats calculation in domain service
	// For now, return placeholder stats
	stats := gin.H{
		"projectId":     projectID,
		"totalTestRuns": 0,
		"passedRuns":    0,
		"failedRuns":    0,
		"successRate":   0.0,
		"avgDuration":   0,
		"lastRun":       nil,
	}

	c.JSON(http.StatusOK, stats)
}

// projectPermissionRow is the wire shape for grant/list project_permissions
// rows. Defined inline because the handler shouldn't depend on the
// projects-domain repo for a single endpoint trio.
type projectPermissionRow struct {
	ID         uint       `gorm:"column:id"           json:"-"`
	ProjectID  string     `gorm:"column:project_id"   json:"projectId"`
	UserID     string     `gorm:"column:user_id"      json:"userId"`
	Permission string     `gorm:"column:permission"   json:"permission"`
	GrantedBy  string     `gorm:"column:granted_by"   json:"grantedBy,omitempty"`
	GrantedAt  *time.Time `gorm:"column:granted_at"   json:"grantedAt,omitempty"`
	ExpiresAt  *time.Time `gorm:"column:expires_at"   json:"expiresAt,omitempty"`
}

func (projectPermissionRow) TableName() string { return "project_permissions" }

// validPermissions are the values the grant endpoint accepts. Matches
// the GraphQL Project.canManage resolver's expectations
// (`project:write:<id>`, etc. — those go through user_scopes; this
// route writes to project_permissions which uses simple verbs).
var validPermissions = map[string]bool{
	"read":   true,
	"write":  true,
	"admin":  true,
	"delete": true,
}

// grantProjectAccess handles POST /api/v1/admin/projects/:projectId/users/:userId/access
// Body: { "permission": "read|write|admin|delete" }
func (h *ProjectHandler) grantProjectAccess(c *gin.Context) {
	if h.db == nil {
		h.respondWithError(c, http.StatusInternalServerError, "database not configured")
		return
	}
	projectID := c.Param("projectId")
	userID := c.Param("userId")
	var body struct {
		Permission string `json:"permission"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		h.respondWithError(c, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !validPermissions[body.Permission] {
		h.respondWithError(c, http.StatusBadRequest, "permission must be one of: read, write, admin, delete")
		return
	}
	grantedBy, _ := c.Get("user_id")
	grantedByStr, _ := grantedBy.(string)
	now := time.Now()
	row := projectPermissionRow{
		ProjectID:  projectID,
		UserID:     userID,
		Permission: body.Permission,
		GrantedBy:  grantedByStr,
		GrantedAt:  &now,
	}
	// Upsert so re-granting the same permission is idempotent.
	if err := h.db.WithContext(c.Request.Context()).
		Where("project_id = ? AND user_id = ? AND permission = ?",
			projectID, userID, body.Permission).
		Assign(row).
		FirstOrCreate(&row).Error; err != nil {
		h.respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, row)
}

// revokeProjectAccess handles DELETE /api/v1/admin/projects/:projectId/users/:userId/access
// Optional query param: ?permission=write (delete only that one). Without
// it, all permissions for the (project, user) pair are removed.
func (h *ProjectHandler) revokeProjectAccess(c *gin.Context) {
	if h.db == nil {
		h.respondWithError(c, http.StatusInternalServerError, "database not configured")
		return
	}
	projectID := c.Param("projectId")
	userID := c.Param("userId")
	q := h.db.WithContext(c.Request.Context()).
		Where("project_id = ? AND user_id = ?", projectID, userID)
	if perm := c.Query("permission"); perm != "" {
		q = q.Where("permission = ?", perm)
	}
	res := q.Delete(&projectPermissionRow{})
	if res.Error != nil {
		h.respondWithError(c, http.StatusInternalServerError, res.Error.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"projectId": projectID,
		"userId":    userID,
		"removed":   res.RowsAffected,
	})
}

// getProjectUsers handles GET /api/v1/admin/projects/:projectId/users
// Returns the list of users with any permission on the project. Each
// row carries the permission verb so callers can render the access
// level alongside the user. Expired rows are filtered out server-side.
func (h *ProjectHandler) getProjectUsers(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, gin.H{"users": []gin.H{}})
		return
	}
	projectID := c.Param("projectId")
	var rows []projectPermissionRow
	now := time.Now()
	if err := h.db.WithContext(c.Request.Context()).
		Where("project_id = ?", projectID).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("granted_at DESC").
		Find(&rows).Error; err != nil {
		h.respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": rows})
}

// convertProjectToAPI converts a domain project to API response format
func (h *ProjectHandler) convertProjectToAPI(p *projectsDomain.Project) gin.H {
	snapshot := p.ToSnapshot()
	
	return gin.H{
		"id":            snapshot.ID,
		"projectId":     string(snapshot.ProjectID),
		"name":          snapshot.Name,
		"description":   snapshot.Description,
		"repository":    snapshot.Repository,
		"defaultBranch": snapshot.DefaultBranch,
		"team":          string(snapshot.Team),
		"isActive":      snapshot.IsActive,
		"settings":      snapshot.Settings, // Return settings as map, not string
		"createdAt":     snapshot.CreatedAt,
		"updatedAt":     snapshot.UpdatedAt,
	}
}

// RegisterRoutes registers project routes
func (h *ProjectHandler) RegisterRoutes(userGroup, managerGroup, adminGroup *gin.RouterGroup) {
	// User routes (read operations)
	userGroup.GET("/projects", h.listProjects)
	userGroup.GET("/projects/:projectId", h.getProject)
	userGroup.GET("/projects/by-project-id/:projectId", h.getProjectByProjectID)
	userGroup.GET("/projects/stats/:projectId", h.getProjectStats)

	// Manager routes (create/update/delete)
	managerGroup.POST("/projects", h.createProject)
	managerGroup.PUT("/projects/:projectId", h.updateProject)
	managerGroup.DELETE("/projects/:projectId", h.deleteProject)
	managerGroup.POST("/projects/:projectId/activate", h.activateProject)
	managerGroup.POST("/projects/:projectId/deactivate", h.deactivateProject)

	// Admin routes (access management)
	adminGroup.POST("/projects/:projectId/users/:userId/access", h.grantProjectAccess)
	adminGroup.DELETE("/projects/:projectId/users/:userId/access", h.revokeProjectAccess)
	adminGroup.GET("/projects/:projectId/users", h.getProjectUsers)
}