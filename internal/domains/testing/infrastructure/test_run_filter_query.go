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
				clauses = append(clauses, tagExistsClause())
				args = append(args, tag)
			}
		default: // LogicOr (default)
			clauses = append(clauses, `EXISTS (
					SELECT 1 FROM suite_runs sr
					JOIN suite_run_tags srt ON srt.suite_run_id = sr.id
					JOIN tags t ON t.id = srt.tag_id
					WHERE sr.test_run_id = test_runs.id
					  AND t.name IN (?)
				)`)
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

func tagExistsClause() string {
	return `EXISTS (
		SELECT 1 FROM suite_runs sr
		JOIN suite_run_tags srt ON srt.suite_run_id = sr.id
		JOIN tags t ON t.id = srt.tag_id
		WHERE sr.test_run_id = test_runs.id
		  AND t.name = ?
	)`
}
