package v2

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

// JiraTestLookup finds runs in a project tagged with a JIRA issue key.
// integrations.CoverageService satisfies it.
type JiraTestLookup interface {
	GetSpecRunsByJiraTag(ctx context.Context, projectID, issueKey string) ([]tagsdomain.CoveredSpecRun, error)
}

// JiraTestsHandler serves GET /projects/:projectId/jira/:issueKey/tests.
type JiraTestsHandler struct {
	repo  JiraTestLookup
	scope ProjectScoper
}

func NewJiraTestsHandler(repo JiraTestLookup) *JiraTestsHandler {
	return &JiraTestsHandler{repo: repo}
}

// WithScope attaches the project read-access boundary. Nil disables it.
func (h *JiraTestsHandler) WithScope(s ProjectScoper) *JiraTestsHandler {
	h.scope = s
	return h
}

func (h *JiraTestsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/projects/:projectId/jira/:issueKey/tests", h.get)
}

type jiraTestsResponse struct {
	ProjectID string `json:"projectId"`
	IssueKey  string `json:"issueKey"`
	HasTests  bool   `json:"hasTests"`
}

func (h *JiraTestsHandler) get(c *gin.Context) {
	projectID, key := c.Param("projectId"), strings.ToUpper(c.Param("issueKey"))
	if !integrations.JiraKeyPattern.MatchString(key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid jira issue key"})
		return
	}
	if h.scope != nil {
		allowed, unrestricted, err := h.scope.AccessibleProjects(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "authorization failed"})
			return
		}
		if _, ok := allowed[projectID]; !unrestricted && !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "no access to project"})
			return
		}
	}
	// ponytail: fetches up to 200 rows to answer a boolean; swap for an EXISTS query if latency matters.
	runs, err := h.repo.GetSpecRunsByJiraTag(c.Request.Context(), projectID, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	c.JSON(http.StatusOK, jiraTestsResponse{
		ProjectID: projectID,
		IssueKey:  key,
		HasTests:  len(runs) > 0,
	})
}
