package domain

import "time"

// DefaultRunHistoryLimit is how many runs the run-detail history strip
// covers when the caller does not ask for a specific number.
const DefaultRunHistoryLimit = 20

// MaxRunHistoryLimit caps the window so one request cannot pull a
// project's whole history.
const MaxRunHistoryLimit = 100

// HistoryRun is one run in a history window.
type HistoryRun struct {
	ID         uint
	RunID      string
	Branch     string
	Status     string
	StartTime  time.Time
	DurationMs int64
}

// HistoryRow is one suite or spec result inside a window run, as read
// from the database. SpecName is empty for a suite-level row.
type HistoryRow struct {
	TestRunID  uint
	SuiteName  string
	SpecName   string
	Status     string
	DurationMs int64
}

// HistoryCell is one suite or spec's result in one run. A nil
// *HistoryCell in a Results slice means it was absent from that run.
type HistoryCell struct {
	Status     string
	DurationMs int64
}

// SuiteHistory is a suite's results across the window, aligned with
// RunHistory.Runs.
type SuiteHistory struct {
	SuiteName string
	Results   []*HistoryCell
}

// SpecHistory is a spec's results across the window, aligned with
// RunHistory.Runs. A spec is identified by (suite name, spec name)
// within a project, so a renamed spec starts a new history.
type SpecHistory struct {
	SuiteName string
	SpecName  string
	Results   []*HistoryCell
}

// RunHistory is the horizontal view for one run: how each of its
// suites and specs fared in the project's recent runs on the same
// branch. Runs is oldest first and ends with the viewed run.
type RunHistory struct {
	// ProjectID is the viewed run's project, for the caller's access check.
	ProjectID string
	Runs      []HistoryRun
	Suites    []SuiteHistory
	Specs     []SpecHistory
}

// BuildRunHistory lays rows out per suite and spec, aligned with runs.
//
// runs must be oldest first and end with the viewed run. Only suites
// and specs present in the viewed run are returned, in the order they
// first appear in rows for that run. When a run holds the same suite
// or spec more than once (a suite reported twice), the most significant
// status wins (failed, then passed, pending, skipped), so a failure is
// never hidden by a later pass.
func BuildRunHistory(runs []HistoryRun, rows []HistoryRow) RunHistory {
	out := RunHistory{Runs: runs, Suites: []SuiteHistory{}, Specs: []SpecHistory{}}
	if len(runs) == 0 {
		return out
	}
	col := make(map[uint]int, len(runs))
	for i, r := range runs {
		col[r.ID] = i
	}
	viewed := runs[len(runs)-1].ID

	type key struct{ suite, spec string }
	index := map[key]int{}
	var keys []key
	for _, row := range rows {
		if row.TestRunID != viewed {
			continue
		}
		k := key{row.SuiteName, row.SpecName}
		if _, seen := index[k]; !seen {
			index[k] = len(keys)
			keys = append(keys, k)
		}
	}

	cells := make([][]*HistoryCell, len(keys))
	for i := range cells {
		cells[i] = make([]*HistoryCell, len(runs))
	}
	for _, row := range rows {
		c, inWindow := col[row.TestRunID]
		i, wanted := index[key{row.SuiteName, row.SpecName}]
		if !inWindow || !wanted {
			continue
		}
		prev := cells[i][c]
		if prev == nil || statusRank(row.Status) > statusRank(prev.Status) {
			cells[i][c] = &HistoryCell{Status: row.Status, DurationMs: row.DurationMs}
		}
	}

	for i, k := range keys {
		if k.spec == "" {
			out.Suites = append(out.Suites, SuiteHistory{SuiteName: k.suite, Results: cells[i]})
		} else {
			out.Specs = append(out.Specs, SpecHistory{SuiteName: k.suite, SpecName: k.spec, Results: cells[i]})
		}
	}
	return out
}

// SuiteOutcome is the status a suite shows in the history strip. A
// suite's stored status is a lifecycle state ("completed" once
// reporting finishes), not an outcome, so the strip derives one from
// its spec counts: any failure means failed, otherwise any pass means
// passed, otherwise skipped. A suite with no specs keeps its stored
// status.
func SuiteOutcome(status string, passed, failed, skipped int) string {
	switch {
	case failed > 0:
		return "failed"
	case passed > 0:
		return "passed"
	case skipped > 0:
		return "skipped"
	default:
		return status
	}
}

// statusRank orders statuses for picking among duplicates; higher wins.
func statusRank(status string) int {
	switch status {
	case "failed":
		return 4
	case "passed":
		return 3
	case "pending":
		return 2
	case "skipped":
		return 1
	default:
		return 0
	}
}

// SpecHistoryRow is one result of a single spec inside a window run,
// with its failure text, as read from the database.
type SpecHistoryRow struct {
	TestRunID    uint
	Status       string
	DurationMs   int64
	ErrorMessage string
	StackTrace   string
}

// SpecHistoryCell is one spec's result in one run, with its failure
// text. A nil *SpecHistoryCell means the spec was absent from that run.
type SpecHistoryCell struct {
	Status       string
	DurationMs   int64
	ErrorMessage string
	StackTrace   string
}

// SpecRunHistory is one spec's results across the same window as the
// run's history strip, for the run-detail spec history panel.
type SpecRunHistory struct {
	// ProjectID is the viewed run's project, for the caller's access check.
	ProjectID string
	Runs      []HistoryRun
	SuiteName string
	SpecName  string
	Results   []*SpecHistoryCell
}

// BuildSpecRunHistory lays one spec's rows out aligned with runs (oldest
// first, ending with the viewed run). As in BuildRunHistory, when a run
// holds the spec more than once the most significant status wins, and
// the winning row's failure text comes with it.
func BuildSpecRunHistory(runs []HistoryRun, suiteName, specName string, rows []SpecHistoryRow) SpecRunHistory {
	out := SpecRunHistory{
		Runs:      runs,
		SuiteName: suiteName,
		SpecName:  specName,
		Results:   make([]*SpecHistoryCell, len(runs)),
	}
	col := make(map[uint]int, len(runs))
	for i, r := range runs {
		col[r.ID] = i
	}
	for _, row := range rows {
		c, ok := col[row.TestRunID]
		if !ok {
			continue
		}
		prev := out.Results[c]
		if prev == nil || statusRank(row.Status) > statusRank(prev.Status) {
			out.Results[c] = &SpecHistoryCell{
				Status:       row.Status,
				DurationMs:   row.DurationMs,
				ErrorMessage: row.ErrorMessage,
				StackTrace:   row.StackTrace,
			}
		}
	}
	return out
}
