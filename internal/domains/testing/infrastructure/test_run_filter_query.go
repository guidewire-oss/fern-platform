package infrastructure

import (
	"strings"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// BuildTestRunWhere translates a domain.TestRunFilter into parameterized
// SQL WHERE clauses and matching args. Each returned clause is a
// self-contained predicate; callers join them with " AND ".
//
// The function is pure: no DB access, no global state. It can be
// unit-tested in isolation, and the resulting clauses can be passed
// directly to GORM's Where() or to a raw query.
func BuildTestRunWhere(f domain.TestRunFilter) ([]string, []any) {
	var (
		clauses []string
		args    []any
	)

	if len(f.ProjectIDs) > 0 {
		clauses = append(clauses, "test_runs.project_id IN ?")
		args = append(args, f.ProjectIDs)
	}
	// Authorization boundary, ANDed with the caller's own selection.
	// Survives facet exclusion — see TestRunFilter.AllowedProjectIDs.
	if len(f.AllowedProjectIDs) > 0 {
		clauses = append(clauses, "test_runs.project_id IN ?")
		args = append(args, f.AllowedProjectIDs)
	}
	if len(f.Status) > 0 {
		clauses = append(clauses, "test_runs.status IN ?")
		args = append(args, f.Status)
	}
	if len(f.Branches) > 0 {
		clauses = append(clauses, "test_runs.branch IN ?")
		args = append(args, f.Branches)
	}
	if f.GitCommit != nil && *f.GitCommit != "" {
		clauses = append(clauses, "test_runs.commit_sha = ?")
		args = append(args, *f.GitCommit)
	}
	if r := f.StartedAt; r != nil {
		if r.Gte != nil {
			clauses = append(clauses, "test_runs.start_time >= ?")
			args = append(args, *r.Gte)
		}
		if r.Lte != nil {
			clauses = append(clauses, "test_runs.start_time <= ?")
			args = append(args, *r.Lte)
		}
	}
	if r := f.DurationMs; r != nil {
		if r.Gte != nil {
			clauses = append(clauses, "test_runs.duration_ms >= ?")
			args = append(args, *r.Gte)
		}
		if r.Lte != nil {
			clauses = append(clauses, "test_runs.duration_ms <= ?")
			args = append(args, *r.Lte)
		}
	}
	if f.Search != nil && *f.Search != "" {
		// Substring search on spec_name + error_message. The previous
		// FTS path used to_tsvector('english') which tokenizes compound
		// identifiers as single stems — searching for "DataIntegrity"
		// never matched "DataIntegrityViolationException" because the
		// stored token was "dataintegrityviolationexcept". Test/error
		// messages aren't English prose, so a trigram-indexed ILIKE
		// reflects what users actually want: "find any row whose text
		// contains this string". Migration 000024 backs this with a
		// GIN gin_trgm_ops index.
		clauses = append(clauses, `EXISTS (
			SELECT 1 FROM suite_runs sr
			JOIN spec_runs sp ON sp.suite_run_id = sr.id
			WHERE sr.test_run_id = test_runs.id
			  AND (COALESCE(sp.spec_name,'') || ' ' || COALESCE(sp.error_message,''))
				  ILIKE ?
		)`)
		args = append(args, "%"+*f.Search+"%")
	}

	if len(f.Tags) > 0 {
		switch f.TagMode {
		case domain.LogicAnd:
			// Each tag must match: one EXISTS per tag, AND-joined naturally.
			for _, tag := range f.Tags {
				clauses = append(clauses, tagExistsClause("= ?"))
				args = append(args, tag)
			}
		default: // LogicOr (default)
			clauses = append(clauses, tagExistsClause("IN (?)"))
			args = append(args, f.Tags)
		}
	}

	return clauses, args
}

// BuildTestRunOrderBy returns the canonical ORDER BY for keyset
// pagination of test runs. It must align with the cursor codec's
// encoded fields and the (project_id, start_time DESC) index.
func BuildTestRunOrderBy() string {
	return "test_runs.start_time DESC, test_runs.id DESC"
}

// JoinWhere joins a set of clauses with AND, or returns the empty
// string when no clauses are present. Useful for assembling raw SQL.
func JoinWhere(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	return strings.Join(clauses, " AND ")
}

// Tag links per level, each yielding (run_id, tag_name). tags is joined
// inside every branch on purpose: callers filter on rt.tag_name, and
// Postgres pushes a plain restriction like that into each UNION ALL
// branch, so every branch starts from the tags.name and *_tags.tag_id
// indexes. Joining tags once outside the union instead leaves the
// suite/spec branches (which contain joins, so they stay subqueries)
// unable to receive the tag condition, and they scan their link tables
// whole: fine on a dev DB, a full spec_run_tags scan in production.
const (
	runLevelTagLinksSQL = `
	SELECT trt.test_run_id AS run_id, t.name AS tag_name
	FROM test_run_tags trt
	JOIN tags t ON t.id = trt.tag_id`

	suiteLevelTagLinksSQL = `
	SELECT sr.test_run_id, t.name
	FROM suite_run_tags srt
	JOIN tags t ON t.id = srt.tag_id
	JOIN suite_runs sr ON sr.id = srt.suite_run_id
	WHERE sr.deleted_at IS NULL`

	specLevelTagLinksSQL = `
	SELECT sr.test_run_id, t.name
	FROM spec_run_tags spt
	JOIN tags t ON t.id = spt.tag_id
	JOIN spec_runs sp ON sp.id = spt.spec_run_id
	JOIN suite_runs sr ON sr.id = sp.suite_run_id
	WHERE sp.deleted_at IS NULL AND sr.deleted_at IS NULL`
)

// RunAndSuiteTagLinksSQL backs the tag facet (spec level excluded; see
// TestRunQueryRepo.tagFacet). AllLevelsTagLinksSQL backs the tag filter:
// ingest writes tags at all three levels (e.g. `@jira:KEY` labels land
// on specs), so a filter that only looked at suites missed most tags.
const (
	RunAndSuiteTagLinksSQL = runLevelTagLinksSQL + "\n\tUNION ALL" + suiteLevelTagLinksSQL
	AllLevelsTagLinksSQL   = RunAndSuiteTagLinksSQL + "\n\tUNION ALL" + specLevelTagLinksSQL
)

// tagExistsClause matches runs carrying a tag whose name satisfies
// nameCond (`= ?` or `IN (?)`) at any level. It is a single EXISTS with
// a single placeholder, which keeps the one-arg-per-clause contract the
// callers rely on and lets Postgres plan it as a semi-join (three OR'd
// EXISTS would force a per-row subplan).
func tagExistsClause(nameCond string) string {
	return `EXISTS (
		SELECT 1 FROM (` + AllLevelsTagLinksSQL + `
		) rt
		WHERE rt.run_id = test_runs.id
		  AND rt.tag_name ` + nameCond + `
	)`
}
