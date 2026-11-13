package v2

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// RunHistoryService reads the horizontal views behind the run-detail
// history strip and spec history panel. Implemented by the testing
// infrastructure's RunHistoryRepo.
type RunHistoryService interface {
	RunHistory(ctx context.Context, runID uint, limit int) (*domain.RunHistory, error)
	SpecHistory(ctx context.Context, runID uint, limit int, suiteName, specName string) (*domain.SpecRunHistory, error)
}

// RunHistoryHandler serves /api/v2/test-runs/:id/history and
// /api/v2/test-runs/:id/history/spec.
type RunHistoryHandler struct {
	svc   RunHistoryService
	scope ProjectScoper
}

// NewRunHistoryHandler constructs the handler.
func NewRunHistoryHandler(svc RunHistoryService) *RunHistoryHandler {
	return &RunHistoryHandler{svc: svc}
}

// WithScope attaches the project authorization boundary. A nil scope
// leaves enforcement off (local no-auth runs only).
func (h *RunHistoryHandler) WithScope(s ProjectScoper) *RunHistoryHandler {
	h.scope = s
	return h
}

// Register mounts the handler's routes on the given v2 group.
func (h *RunHistoryHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/test-runs/:id/history", h.get)
	rg.GET("/test-runs/:id/history/spec", h.getSpec)
}

func (h *RunHistoryHandler) get(c *gin.Context) {
	id, limit, ok := parseHistoryParams(c)
	if !ok {
		return
	}
	hist, err := h.svc.RunHistory(c.Request.Context(), id, limit)
	if !h.checkResult(c, err, func() string { return hist.ProjectID }) {
		return
	}
	c.JSON(http.StatusOK, toRunHistoryDTO(hist))
}

func (h *RunHistoryHandler) getSpec(c *gin.Context) {
	id, limit, ok := parseHistoryParams(c)
	if !ok {
		return
	}
	suite, spec := c.Query("suite"), c.Query("spec")
	if suite == "" || spec == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "suite and spec are required"})
		return
	}
	hist, err := h.svc.SpecHistory(c.Request.Context(), id, limit, suite, spec)
	if !h.checkResult(c, err, func() string { return hist.ProjectID }) {
		return
	}
	c.JSON(http.StatusOK, toSpecHistoryDTO(hist))
}

// parseHistoryParams reads the run id path parameter and the optional
// limit, writing a 400 and returning ok=false when either is invalid.
func parseHistoryParams(c *gin.Context) (id uint, limit int, ok bool) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a positive integer"})
		return 0, 0, false
	}
	limit = domain.DefaultRunHistoryLimit
	if s := c.Query("limit"); s != "" {
		l, err := strconv.Atoi(s)
		if err != nil || l < 1 || l > domain.MaxRunHistoryLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and " + strconv.Itoa(domain.MaxRunHistoryLimit)})
			return 0, 0, false
		}
		limit = l
	}
	return uint(n), limit, true
}

// checkResult maps a service error to a response and applies the
// project authorization check, returning false when it has written the
// response. projectID is only called once err is nil.
//
// A caller who may not read the run's project gets the same 404 as for
// a missing run, so run ids don't reveal which other teams' runs exist.
func (h *RunHistoryHandler) checkResult(c *gin.Context, err error, projectID func() string) bool {
	if errors.Is(err, domain.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "test run not found"})
		return false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return false
	}
	if h.scope == nil {
		return true
	}
	allowed, unrestricted, err := h.scope.AccessibleProjects(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authorization failed"})
		return false
	}
	if _, ok := allowed[projectID()]; !unrestricted && !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "test run not found"})
		return false
	}
	return true
}

type historyRunDTO struct {
	ID         uint      `json:"id"`
	RunID      string    `json:"runId"`
	Branch     string    `json:"branch"`
	Status     string    `json:"status"`
	StartTime  time.Time `json:"startTime"`
	DurationMs int64     `json:"durationMs"`
}

// historyCellDTO is one result; a null entry in a results array means
// the suite or spec was absent from that run.
type historyCellDTO struct {
	Status     string `json:"status"`
	DurationMs int64  `json:"durationMs"`
}

type suiteHistoryDTO struct {
	SuiteName string            `json:"suiteName"`
	Results   []*historyCellDTO `json:"results"`
}

type specHistoryDTO struct {
	SuiteName string            `json:"suiteName"`
	SpecName  string            `json:"specName"`
	Results   []*historyCellDTO `json:"results"`
}

// runHistoryDTO: runs is oldest first and ends with the viewed run;
// every results array is aligned with it.
type runHistoryDTO struct {
	Runs   []historyRunDTO   `json:"runs"`
	Suites []suiteHistoryDTO `json:"suites"`
	Specs  []specHistoryDTO  `json:"specs"`
}

func toRunHistoryDTO(h *domain.RunHistory) runHistoryDTO {
	out := runHistoryDTO{
		Runs:   toHistoryRunDTOs(h.Runs),
		Suites: make([]suiteHistoryDTO, len(h.Suites)),
		Specs:  make([]specHistoryDTO, len(h.Specs)),
	}
	for i, s := range h.Suites {
		out.Suites[i] = suiteHistoryDTO{SuiteName: s.SuiteName, Results: toCellDTOs(s.Results)}
	}
	for i, s := range h.Specs {
		out.Specs[i] = specHistoryDTO{SuiteName: s.SuiteName, SpecName: s.SpecName, Results: toCellDTOs(s.Results)}
	}
	return out
}

func toCellDTOs(cells []*domain.HistoryCell) []*historyCellDTO {
	out := make([]*historyCellDTO, len(cells))
	for i, c := range cells {
		if c != nil {
			out[i] = &historyCellDTO{Status: c.Status, DurationMs: c.DurationMs}
		}
	}
	return out
}

// specHistoryCellDTO is one result of the spec; a null entry means the
// spec was absent from that run.
type specHistoryCellDTO struct {
	Status       string `json:"status"`
	DurationMs   int64  `json:"durationMs"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	StackTrace   string `json:"stackTrace,omitempty"`
}

// specRunHistoryDTO: runs is the same window as the run's history strip,
// oldest first; results is aligned with it.
type specRunHistoryDTO struct {
	Runs      []historyRunDTO       `json:"runs"`
	SuiteName string                `json:"suiteName"`
	SpecName  string                `json:"specName"`
	Results   []*specHistoryCellDTO `json:"results"`
}

func toSpecHistoryDTO(h *domain.SpecRunHistory) specRunHistoryDTO {
	out := specRunHistoryDTO{
		Runs:      toHistoryRunDTOs(h.Runs),
		SuiteName: h.SuiteName,
		SpecName:  h.SpecName,
		Results:   make([]*specHistoryCellDTO, len(h.Results)),
	}
	for i, c := range h.Results {
		if c != nil {
			out.Results[i] = &specHistoryCellDTO{
				Status: c.Status, DurationMs: c.DurationMs,
				ErrorMessage: c.ErrorMessage, StackTrace: c.StackTrace,
			}
		}
	}
	return out
}

func toHistoryRunDTOs(runs []domain.HistoryRun) []historyRunDTO {
	out := make([]historyRunDTO, len(runs))
	for i, r := range runs {
		out[i] = historyRunDTO{
			ID: r.ID, RunID: r.RunID, Branch: r.Branch, Status: r.Status,
			StartTime: r.StartTime, DurationMs: r.DurationMs,
		}
	}
	return out
}
