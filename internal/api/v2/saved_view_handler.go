package v2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// SavedViewRepo is the dependency the handler needs. It is exactly the
// production SavedViewRepository interface; declared here for
// handler-test substitution.
type SavedViewRepo interface {
	Create(ctx context.Context, v *domain.SavedView) error
	List(ctx context.Context, userID, page string) ([]*domain.SavedView, error)
	Delete(ctx context.Context, userID string, id uint) error
}

// UserIDProvider extracts the authenticated user from a gin context.
// The default implementation reads the "user_id" key set by the auth
// middleware. Tests inject a fixed-value provider.
type UserIDProvider func(*gin.Context) string

// DefaultUserIDProvider matches the existing BaseHandler.getUserID
// behavior: read "user_id" from the context, set by auth middleware.
func DefaultUserIDProvider(c *gin.Context) string {
	v, _ := c.Get("user_id")
	s, _ := v.(string)
	return s
}

// SavedViewHandler serves /api/v2/me/saved-views.
type SavedViewHandler struct {
	repo SavedViewRepo
	uid  UserIDProvider
}

// NewSavedViewHandler builds a handler. Pass nil for uid to use
// DefaultUserIDProvider.
func NewSavedViewHandler(repo SavedViewRepo, uid UserIDProvider) *SavedViewHandler {
	if uid == nil {
		uid = DefaultUserIDProvider
	}
	return &SavedViewHandler{repo: repo, uid: uid}
}

// Register mounts routes under /api/v2/me/saved-views.
func (h *SavedViewHandler) Register(rg *gin.RouterGroup) {
	g := rg.Group("/me/saved-views")
	g.GET("", h.list)
	g.POST("", h.create)
	g.DELETE("/:id", h.delete)
}

type savedViewDTO struct {
	ID         uint            `json:"id"`
	Page       string          `json:"page"`
	Name       string          `json:"name"`
	FilterJSON json.RawMessage `json:"filter"`
}

// savedViewsListLimitDefault and Max bound the slice the handler
// returns. Users rarely have more than a few dozen saved views, so
// the default is generous and the max is forgiving.
const (
	savedViewsListLimitDefault = 50
	savedViewsListLimitMax     = 200
)

func (h *SavedViewHandler) list(c *gin.Context) {
	uid := h.uid(c)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}

	limit := savedViewsListLimitDefault
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a positive integer"})
			return
		}
		if n > savedViewsListLimitMax {
			n = savedViewsListLimitMax
		}
		limit = n
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "offset must be a non-negative integer"})
			return
		}
		offset = n
	}

	views, err := h.repo.List(c.Request.Context(), uid, c.Query("page"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}

	total := len(views)
	views = paginate(views, offset, limit)

	out := make([]savedViewDTO, len(views))
	for i, v := range views {
		out[i] = savedViewDTO{ID: v.ID, Page: v.Page, Name: v.Name, FilterJSON: v.FilterJSON}
	}
	c.JSON(http.StatusOK, gin.H{
		"views":      out,
		"totalCount": total,
		"limit":      limit,
		"offset":     offset,
	})
}

// paginate slices xs to a [offset, offset+limit) window. Returns an
// empty slice when offset overruns the input, which the JSON encoder
// renders as `[]` — distinguishable from a query error.
func paginate[T any](xs []T, offset, limit int) []T {
	if offset >= len(xs) {
		return xs[:0]
	}
	end := offset + limit
	if end > len(xs) {
		end = len(xs)
	}
	return xs[offset:end]
}

func (h *SavedViewHandler) create(c *gin.Context) {
	uid := h.uid(c)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	var body struct {
		Page   string          `json:"page"`
		Name   string          `json:"name"`
		Filter json.RawMessage `json:"filter"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	if body.Page == "" || body.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page and name are required"})
		return
	}
	v := &domain.SavedView{
		UserID:     uid,
		Page:       body.Page,
		Name:       body.Name,
		FilterJSON: []byte(body.Filter),
	}
	if err := h.repo.Create(c.Request.Context(), v); err != nil {
		if errors.Is(err, domain.ErrSavedViewConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "view name already taken on this page"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}
	c.JSON(http.StatusCreated, savedViewDTO{ID: v.ID, Page: v.Page, Name: v.Name, FilterJSON: v.FilterJSON})
}

func (h *SavedViewHandler) delete(c *gin.Context) {
	uid := h.uid(c)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be a positive integer"})
		return
	}
	if err := h.repo.Delete(c.Request.Context(), uid, uint(id64)); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "view not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}
	c.Status(http.StatusNoContent)
}
