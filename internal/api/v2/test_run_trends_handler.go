package v2

import (
	"context"
	"fmt"
	"math"
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
	scope ProjectScoper
}

// WithScope attaches the authorization boundary that restricts which
// projects a caller may read. Returns the handler for chaining.
func (h *TestRunTrendsHandler) WithScope(s ProjectScoper) *TestRunTrendsHandler {
	h.scope = s
	return h
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
	// Authorization: restrict the requested projects to those the caller
	// may read (admins unrestricted). A caller left with none gets an
	// empty (but well-formed) trends response rather than another team's
	// data.
	if h.scope != nil {
		allowed, unrestricted, err := h.scope.AccessibleProjects(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "authorization failed"})
			return
		}
		if !unrestricted {
			ids, ok := scopeProjectIDs(projectIDs, allowed)
			if !ok {
				c.JSON(http.StatusOK, trendsResponse{Buckets: map[string][]trendsRow{}})
				return
			}
			projectIDs = ids
		}
	}
	// Window selection: an explicit from/to range (anchored, historical
	// ranges — mirrors the test-runs date filter) takes precedence over
	// the rolling `days` window. Both bounds are required together.
	var from, to time.Time
	var days int
	fromStr, toStr := q.Get("from"), q.Get("to")
	if fromStr != "" || toStr != "" {
		if fromStr == "" || toStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from and to must be provided together"})
			return
		}
		var err error
		if from, err = time.Parse(time.RFC3339, fromStr); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from must be RFC3339"})
			return
		}
		if to, err = time.Parse(time.RFC3339, toStr); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to must be RFC3339"})
			return
		}
		from, to = from.UTC(), to.UTC()
		if !to.After(from) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to must be after from"})
			return
		}
		if to.Sub(from) > 366*24*time.Hour {
			c.JSON(http.StatusBadRequest, gin.H{"error": "range must be 366 days or less"})
			return
		}
		// Inclusive day span, for the response's Days hint and the
		// front-end's zero-fill.
		days = int(math.Ceil(to.Sub(from).Hours() / 24))
	} else {
		days = 7
		if v := q.Get("days"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 || n > 365 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "days must be 1..365"})
				return
			}
			days = n
		}
		to = time.Now().UTC()
		from = to.AddDate(0, 0, -days)
	}

	userID := h.user(c)
	key := buildTrendsKey(userID, projectIDs, windowToken(fromStr, toStr, days))
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
// sort projectIDs so call-order doesn't matter. `window` distinguishes
// rolling day-windows from anchored from/to ranges.
func buildTrendsKey(userID string, projectIDs []string, window string) string {
	ids := make([]string, len(projectIDs))
	copy(ids, projectIDs)
	sort.Strings(ids)
	return fmt.Sprintf("%s|%s|%s", userID, window, strings.Join(ids, ","))
}

// windowToken names the window for the cache key. An anchored range keys
// on its exact from/to strings; a rolling window keys on the day count
// (not the recomputed `to=now`, so back-to-back requests still hit cache).
func windowToken(fromStr, toStr string, days int) string {
	if fromStr != "" || toStr != "" {
		return "r" + fromStr + "_" + toStr
	}
	return "d" + strconv.Itoa(days)
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
