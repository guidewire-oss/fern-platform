// Package api provides domain-based REST API handlers
package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// ProjectHandler handles project related endpoints
type ProjectHandler struct {
	*BaseHandler
	projectService *projectsApp.ProjectService
}

// NewProjectHandler creates a new project handler
func NewProjectHandler(projectService *projectsApp.ProjectService, logger *logging.Logger) *ProjectHandler {
	return &ProjectHandler{
		BaseHandler:    NewBaseHandler(logger),
		projectService: projectService,
	}
}

// createProject godoc
// @Summary      Create a project
// @Description  Creates a new project (manager or admin only)
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        body  body  object{projectId=string,name=string,team=string,description=string,repository=string,defaultBranch=string,settings=object}  true  "Project details"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/projects [post]
// @Security     BearerAuth
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

		updateErr := h.projectService.UpdateProject(c.Request.Context(), project.ProjectID(), updates)
		if updateErr != nil {
			h.logger.WithError(updateErr).Warn("Failed to update project details after creation")
		}
	}

	// Convert to API response format
	c.JSON(http.StatusCreated, h.convertProjectToAPI(project))
}

// getProject godoc
// @Summary      Get a project
// @Description  Returns a project by its project ID string
// @Tags         projects
// @Produce      json
// @Param        projectId  path  string  true  "Project ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Failure      501  {object}  map[string]string
// @Router       /api/v1/projects/{projectId} [get]
// @Security     BearerAuth
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

// getProjectByProjectID godoc
// @Summary      Get a project by project ID
// @Description  Explicit lookup by project ID string (avoids numeric ID ambiguity)
// @Tags         projects
// @Produce      json
// @Param        projectId  path  string  true  "Project ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/projects/by-project-id/{projectId} [get]
// @Security     BearerAuth
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

// updateProject godoc
// @Summary      Update a project
// @Description  Updates project metadata (manager or admin only)
// @Tags         projects
// @Accept       json
// @Produce      json
// @Param        projectId  path  string  true  "Project ID"
// @Param        body       body  object{name=string,description=string,team=string,repository=string,defaultBranch=string,settings=object}  false  "Fields to update"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/projects/{projectId} [put]
// @Security     BearerAuth
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

// deleteProject godoc
// @Summary      Delete a project
// @Description  Permanently deletes a project and its data (manager or admin only)
// @Tags         projects
// @Produce      json
// @Param        projectId  path  string  true  "Project ID"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/projects/{projectId} [delete]
// @Security     BearerAuth
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

// listProjects godoc
// @Summary      List projects
// @Description  Returns a paginated list of all accessible projects
// @Tags         projects
// @Produce      json
// @Param        limit   query  int  false  "Page size (default 20)"
// @Param        offset  query  int  false  "Page offset (default 0)"
// @Success      200  {array}   map[string]interface{}
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/projects [get]
// @Security     BearerAuth
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

// activateProject godoc
// @Summary      Activate a project
// @Description  Marks a project as active so it accepts new test runs
// @Tags         projects
// @Produce      json
// @Param        projectId  path  string  true  "Project ID"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /api/v1/projects/{projectId}/activate [post]
// @Security     BearerAuth
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

// grantProjectAccess handles POST /api/v1/admin/projects/:projectId/users/:userId/access
func (h *ProjectHandler) grantProjectAccess(c *gin.Context) {
	// TODO: Implement project access management in domain service
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Project access management not yet implemented"})
}

// revokeProjectAccess handles DELETE /api/v1/admin/projects/:projectId/users/:userId/access
func (h *ProjectHandler) revokeProjectAccess(c *gin.Context) {
	// TODO: Implement project access management in domain service
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Project access management not yet implemented"})
}

// getProjectUsers handles GET /api/v1/admin/projects/:projectId/users
func (h *ProjectHandler) getProjectUsers(c *gin.Context) {
	// TODO: Implement get project users
	c.JSON(http.StatusOK, gin.H{"users": []gin.H{}})
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
