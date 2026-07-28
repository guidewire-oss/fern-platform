// Package v2 implements the /api/v2 REST surface introduced by
// RFC-004 (Frontend Modernization). v2 endpoints accept rich filters,
// return cursor-paginated connections, and embed facet counts.
//
// v1 handlers remain in internal/api/* and are untouched by this
// package — see docs/specs/frontend-modernization/migration-guide.md.
package v2

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// TestRunQueryService is the application-layer dependency the v2
// test-run list handler talks to. Defined here (consumer-owned
// interface) so the handler can be tested with a fake.
type TestRunQueryService interface {
	Query(ctx context.Context, filter domain.TestRunFilter, page domain.PageArgs) (domain.TestRunPage, error)
}

// TestRunHandler serves /api/v2/test-runs.
type TestRunHandler struct {
	svc   TestRunQueryService
	scope ProjectScoper
}

// NewTestRunHandler constructs a handler bound to the given query
// service. The service is typically wired to the GORM repository
// behind a small application service, but the handler does not care
// how the query is fulfilled.
func NewTestRunHandler(svc TestRunQueryService) *TestRunHandler {
	return &TestRunHandler{svc: svc}
}

// WithScope attaches the authorization boundary that restricts which
// projects a caller may read. Returns the handler for chaining. A nil
// scope leaves enforcement off (local no-auth runs only).
func (h *TestRunHandler) WithScope(s ProjectScoper) *TestRunHandler {
	h.scope = s
	return h
}

// Register mounts the handler's routes on the given v2 group.
func (h *TestRunHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/test-runs", h.list)
}

func (h *TestRunHandler) list(c *gin.Context) {
	filter, page, err := decodeListQuery(c.Request.URL.Query())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := filter.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Authorization: constrain the project filter to what the caller may
	// read. Without this a signed-in non-admin could read any team's runs
	// by passing its project id. Admins are unrestricted. A caller left
	// with no accessible projects gets an empty connection — never an
	// unfiltered query.
	if h.scope != nil {
		allowed, unrestricted, err := h.scope.AccessibleProjects(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "authorization failed"})
			return
		}
		if !unrestricted {
			ids, ok := scopeProjectIDs(filter.ProjectIDs, allowed)
			if !ok {
				c.JSON(http.StatusOK, toConnectionDTO(domain.TestRunPage{}))
				return
			}
			filter.ProjectIDs = ids
			// Record the boundary separately from the selection above.
			// Facet computation clears ProjectIDs so the project facet can
			// offer projects the caller has not selected; without this
			// second field it would offer every project in the system.
			filter.AllowedProjectIDs = allowedIDs(allowed)
		}
	}
	// Default time window: when the client passes neither `from` nor
	// `to`, clamp to the last 7 days. At 1000 projects × 100 runs/day
	// a full-history sweep is ~18M rows; 7 days is ~700k. Matches the
	// typical triage span (users scan the last week of activity from
	// the dashboard). Wider windows are explicit — UI presets jump
	// to 30d/90d, or clients can pass `allTime=1` to opt out.
	if filter.StartedAt == nil && c.Request.URL.Query().Get("allTime") != "1" {
		gte := time.Now().UTC().Add(-7 * 24 * time.Hour)
		filter.StartedAt = &domain.DateTimeRange{Gte: &gte}
	}
	page.Normalize()

	res, err := h.svc.Query(c.Request.Context(), filter, page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	c.JSON(http.StatusOK, toConnectionDTO(res))
}

// decodeListQuery parses query-string parameters into the typed
// domain inputs. Returns user-facing errors for malformed input.
func decodeListQuery(q url.Values) (domain.TestRunFilter, domain.PageArgs, error) {
	var f domain.TestRunFilter

	f.ProjectIDs = q["project"]
	f.Status = q["status"]
	f.Branches = q["branch"]
	f.Tags = q["tag"]
	if c := strings.TrimSpace(q.Get("commit")); c != "" {
		f.GitCommit = &c
	}
	if s := strings.TrimSpace(q.Get("q")); s != "" {
		f.Search = &s
	}
	if m := strings.TrimSpace(q.Get("tagMode")); m != "" {
		f.TagMode = domain.LogicMode(strings.ToUpper(m))
	}
	// ?facets=tag (or facets=tag,…) opts into the expensive tag-facet
	// join. Default list responses skip it.
	for _, v := range q["facets"] {
		for _, name := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(name), "tag") {
				f.IncludeTagFacet = true
			}
		}
	}

	if from, to := q.Get("from"), q.Get("to"); from != "" || to != "" {
		rng := &domain.DateTimeRange{}
		if from != "" {
			t, err := time.Parse(time.RFC3339, from)
			if err != nil {
				return f, domain.PageArgs{}, fmt.Errorf("from: invalid RFC3339 timestamp")
			}
			rng.Gte = &t
		}
		if to != "" {
			t, err := time.Parse(time.RFC3339, to)
			if err != nil {
				return f, domain.PageArgs{}, fmt.Errorf("to: invalid RFC3339 timestamp")
			}
			rng.Lte = &t
		}
		f.StartedAt = rng
	}

	if gte, lte := q.Get("durationGte"), q.Get("durationLte"); gte != "" || lte != "" {
		rng := &domain.IntRange{}
		if gte != "" {
			v, err := strconv.Atoi(gte)
			if err != nil {
				return f, domain.PageArgs{}, fmt.Errorf("durationGte: must be an integer")
			}
			rng.Gte = &v
		}
		if lte != "" {
			v, err := strconv.Atoi(lte)
			if err != nil {
				return f, domain.PageArgs{}, fmt.Errorf("durationLte: must be an integer")
			}
			rng.Lte = &v
		}
		f.DurationMs = rng
	}

	var p domain.PageArgs
	if v := q.Get("first"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, p, fmt.Errorf("first: must be an integer")
		}
		p.First = n
	}
	p.After = q.Get("after")

	return f, p, nil
}

// connectionDTO mirrors design.md §4.2 verbatim. Field names are
// camelCase to match the rest of the v2 surface and the GraphQL
// connection shape.
type connectionDTO struct {
	Edges                []edgeDTO   `json:"edges"`
	PageInfo             pageInfoDTO `json:"pageInfo"`
	TotalCount           int64       `json:"totalCount"`
	TotalCountIsEstimate bool        `json:"totalCountIsEstimate"`
	Facets               facetsDTO   `json:"facets"`
}

type edgeDTO struct {
	Cursor string   `json:"cursor"`
	Node   *nodeDTO `json:"node"`
}

// nodeDTO is the domain entity plus the wire-friendly fields the SPA
// needs. It embeds the entity so every existing field keeps its current
// JSON name and no field has to be restated here.
//
// DurationMs exists because domain.TestRun.Duration is a time.Duration,
// which marshals as nanoseconds — the client works in milliseconds, and
// deriving the value from end_time - start_time fails for runs that
// have no end_time.
type nodeDTO struct {
	*domain.TestRun
	DurationMs int64 `json:"duration_ms"`
}

func toNodeDTO(n *domain.TestRun) *nodeDTO {
	if n == nil {
		return nil
	}
	return &nodeDTO{TestRun: n, DurationMs: n.Duration.Milliseconds()}
}

type pageInfoDTO struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type facetsDTO struct {
	ByStatus  []facetCountDTO `json:"byStatus"`
	ByBranch  []facetCountDTO `json:"byBranch"`
	ByTag     []facetCountDTO `json:"byTag"`
	ByProject []facetCountDTO `json:"byProject"`
}

// facetCountDTO carries an optional Label — the human-readable name for
// Value, populated only for the project facet. `omitempty` keeps the key
// off the status/branch/tag entries, whose values are already readable.
type facetCountDTO struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
	Label string `json:"label,omitempty"`
}

func toConnectionDTO(p domain.TestRunPage) connectionDTO {
	edges := make([]edgeDTO, len(p.Edges))
	for i, e := range p.Edges {
		edges[i] = edgeDTO{Cursor: e.Cursor, Node: toNodeDTO(e.Node)}
	}
	return connectionDTO{
		Edges: edges,
		PageInfo: pageInfoDTO{
			HasNextPage: p.PageInfo.HasNextPage,
			EndCursor:   p.PageInfo.EndCursor,
		},
		TotalCount:           p.TotalCount,
		TotalCountIsEstimate: p.TotalCountIsEstimate,
		Facets: facetsDTO{
			ByStatus:  toFacetDTOs(p.Facets.ByStatus),
			ByBranch:  toFacetDTOs(p.Facets.ByBranch),
			ByTag:     toFacetDTOs(p.Facets.ByTag),
			ByProject: toFacetDTOs(p.Facets.ByProject),
		},
	}
}

func toFacetDTOs(in []domain.FacetCount) []facetCountDTO {
	out := make([]facetCountDTO, len(in))
	for i, fc := range in {
		out[i] = facetCountDTO{Value: fc.Value, Count: fc.Count, Label: fc.Label}
	}
	return out
}
