package v2

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// TestRunTrendsService is the application-layer dependency.
// Defined here (consumer-owned) so the handler can be tested with a
// stub.
type TestRunTrendsService interface {
	AggregateDailyByProjects(
		ctx context.Context, projectIDs []string, startDate, endDate time.Time,
	) ([]*domain.DailyProjectAggregate, error)
}

// TestRunTrendsHandler serves GET /api/v2/test-runs/trends.
//
// The handler returns per-(project, day) sums for the requested window
// in a single response. The Test Summaries page previously fanned out
// one HTTP request per visible project to compute the same data
// client-side; this collapses to one request and one SQL GROUP BY.
//
// A small in-process cache (60s TTL, user-scoped) handles the common
// case of a user clicking back and forth between summaries / projects.
type TestRunTrendsHandler struct {
	svc   TestRunTrendsService
	user  UserIDProvider
	cache *trendsCache
}

// NewTestRunTrendsHandler wires the service and a default user-ID
// provider. The cache TTL matches the treemap's at 60s — short enough
// that any meaningful new data shows up quickly, long enough to cover
// rapid navigation.
func NewTestRunTrendsHandler(svc TestRunTrendsService, user UserIDProvider) *TestRunTrendsHandler {
	if user == nil {
		user = DefaultUserIDProvider
	}
	return &TestRunTrendsHandler{
		svc:   svc,
		user:  user,
		cache: newTrendsCache(60 * time.Second),
	}
}

// Register mounts the handler's routes.
func (h *TestRunTrendsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/test-runs/trends", h.list)
}

type trendsRow struct {
	Day          string `json:"day"` // YYYY-MM-DD, UTC
	TotalRuns    int    `json:"totalRuns"`
	TotalTests   int    `json:"totalTests"`
	PassedTests  int    `json:"passedTests"`
	FailedTests  int    `json:"failedTests"`
	SkippedTests int    `json:"skippedTests"`
	DurationMs   int64  `json:"durationMs"`
}

type trendsResponse struct {
	From    string                 `json:"from"`
	To      string                 `json:"to"`
	Days    int                    `json:"days"`
	Buckets map[string][]trendsRow `json:"buckets"` // projectId -> daily rows (oldest first)
}

func (h *TestRunTrendsHandler) list(c *gin.Context) {
	q := c.Request.URL.Query()
	projectIDs := q["project"]
	if len(projectIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one project required"})
		return
	}
	days := 7
	if v := q.Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 365 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "days must be 1..365"})
			return
		}
		days = n
	}

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -days)

	userID := h.user(c)
	key := buildTrendsKey(userID, projectIDs, days)
	if cached, ok := h.cache.get(key); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	aggs, err := h.svc.AggregateDailyByProjects(c.Request.Context(), projectIDs, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "trends query failed"})
		return
	}

	buckets := make(map[string][]trendsRow, len(projectIDs))
	for _, a := range aggs {
		buckets[a.ProjectID] = append(buckets[a.ProjectID], trendsRow{
			Day:          a.Day.UTC().Format("2006-01-02"),
			TotalRuns:    a.TotalRuns,
			TotalTests:   a.TotalTests,
			PassedTests:  a.PassedTests,
			FailedTests:  a.FailedTests,
			SkippedTests: a.SkippedTests,
			DurationMs:   a.DurationMs,
		})
	}
	// Projects with no runs at all in the window still appear with an
	// empty array — the front-end zero-fills, but the key must exist
	// so the React Query consumer can map(projectId)→buckets safely.
	for _, pid := range projectIDs {
		if _, ok := buckets[pid]; !ok {
			buckets[pid] = []trendsRow{}
		}
	}

	resp := trendsResponse{
		From:    from.Format(time.RFC3339),
		To:      to.Format(time.RFC3339),
		Days:    days,
		Buckets: buckets,
	}
	h.cache.set(key, resp)
	c.JSON(http.StatusOK, resp)
}

// buildTrendsKey produces a stable cache key for the trends response.
// User-scoped so two users with different access cannot share entries;
// sort projectIDs so call-order doesn't matter.
func buildTrendsKey(userID string, projectIDs []string, days int) string {
	ids := make([]string, len(projectIDs))
	copy(ids, projectIDs)
	sort.Strings(ids)
	return fmt.Sprintf("%s|%d|%s", userID, days, strings.Join(ids, ","))
}

// trendsCache is a tiny TTL'd in-process cache for trend responses.
// Same shape as the treemap cache, with a different value type. Nil
// receiver is safe so tests don't have to construct one.
type trendsCache struct {
	mu  sync.Mutex
	m   map[string]trendsCacheEntry
	ttl time.Duration
}

type trendsCacheEntry struct {
	data trendsResponse
	exp  time.Time
}

func newTrendsCache(ttl time.Duration) *trendsCache {
	return &trendsCache{m: map[string]trendsCacheEntry{}, ttl: ttl}
}

func (c *trendsCache) get(key string) (trendsResponse, bool) {
	if c == nil {
		return trendsResponse{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || time.Now().After(e.exp) {
		if ok {
			delete(c.m, key)
		}
		return trendsResponse{}, false
	}
	return e.data, true
}

func (c *trendsCache) set(key string, v trendsResponse) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = trendsCacheEntry{data: v, exp: time.Now().Add(c.ttl)}
}
