package domain

import (
	"fmt"
	"time"
)

// LogicMode selects how a multi-value filter is combined.
type LogicMode string

const (
	LogicAnd LogicMode = "AND"
	LogicOr  LogicMode = "OR"
)

// Page size bounds for list queries. Clients that ask for more get
// clamped silently; this is the standard Relay-style behavior.
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// IntRange is an inclusive range filter on an integer field.
// Either bound may be nil to leave that side unbounded.
type IntRange struct {
	Gte *int
	Lte *int
}

// DateTimeRange is an inclusive range filter on a timestamp field.
type DateTimeRange struct {
	Gte *time.Time
	Lte *time.Time
}

// TestRunFilter narrows a test-run list query.
//
// All fields are optional. Empty slices and nil pointers mean "no
// constraint on this field". Validate() must be called before use.
type TestRunFilter struct {
	ProjectIDs []string
	Status     []string
	Branches   []string
	Tags       []string
	TagMode    LogicMode
	GitCommit  *string
	Authors    []string
	DurationMs *IntRange
	StartedAt  *DateTimeRange
	Search     *string

	// IncludeTagFacet controls whether ComputeFacets runs the
	// suite_runs↔suite_run_tags↔tags join for ByTag counts. That join
	// scans ~6× more rows than the other facets, so by default we skip
	// it on initial list loads and only compute when the UI explicitly
	// expands the Tag section (?facets=tag). The filter's own Tags
	// constraint is unaffected — filtering by tag always works.
	IncludeTagFacet bool
}

// PageArgs is cursor-based pagination input.
type PageArgs struct {
	First int
	After string // opaque cursor; empty = first page
}

// Normalize clamps First into [1, MaxPageSize] with DefaultPageSize for
// zero or negative inputs. Callers should call this before issuing a
// query.
func (p *PageArgs) Normalize() {
	if p.First <= 0 {
		p.First = DefaultPageSize
		return
	}
	if p.First > MaxPageSize {
		p.First = MaxPageSize
	}
}

// Validate checks the filter for internal consistency and fills in
// defaults. It mutates the receiver where it can repair the input
// (e.g., picking a default tag mode); it returns an error for inputs
// that cannot be auto-corrected.
func (f *TestRunFilter) Validate() error {
	if r := f.StartedAt; r != nil && r.Gte != nil && r.Lte != nil {
		if r.Lte.Before(*r.Gte) {
			return fmt.Errorf("startedAt: lte (%s) is before gte (%s)", r.Lte, r.Gte)
		}
	}
	if r := f.DurationMs; r != nil && r.Gte != nil && r.Lte != nil {
		if *r.Lte < *r.Gte {
			return fmt.Errorf("durationMs: lte (%d) is less than gte (%d)", *r.Lte, *r.Gte)
		}
	}
	if len(f.Tags) > 0 {
		switch f.TagMode {
		case "":
			f.TagMode = LogicOr
		case LogicAnd, LogicOr:
			// ok
		default:
			return fmt.Errorf("tagMode: %q is not AND or OR", f.TagMode)
		}
	}
	return nil
}

// IsNarrow reports whether the filter constrains the result set to a
// small subset of the table. The repository uses this to decide
// between an exact COUNT(*) and a fast reltuples estimate.
//
// Heuristic: any of project, branch, status, tag, commit, author, or
// search is "narrow" enough; date-only or duration-only is "broad".
func (f *TestRunFilter) IsNarrow() bool {
	if len(f.ProjectIDs) > 0 || len(f.Branches) > 0 || len(f.Status) > 0 ||
		len(f.Tags) > 0 || len(f.Authors) > 0 {
		return true
	}
	if f.GitCommit != nil && *f.GitCommit != "" {
		return true
	}
	if f.Search != nil && *f.Search != "" {
		return true
	}
	return false
}

// TestRunEdge is one entry in a paged list response.
type TestRunEdge struct {
	Cursor string
	Node   *TestRun
}

// PageInfo describes whether more pages exist.
type PageInfo struct {
	HasNextPage bool
	EndCursor   string
}

// FacetCount is one (value, count) pair in a faceted response.
type FacetCount struct {
	Value string
	Count int64
}

// TestRunFacets groups facet counts by field, scoped to the current
// filter (excluding the field itself, so a user can broaden it).
type TestRunFacets struct {
	ByStatus  []FacetCount
	ByBranch  []FacetCount
	ByTag     []FacetCount
	ByProject []FacetCount
}

// TestRunPage is the paged, faceted result of a filtered query.
type TestRunPage struct {
	Edges                []TestRunEdge
	PageInfo             PageInfo
	TotalCount           int64
	TotalCountIsEstimate bool
	Facets               TestRunFacets
}
