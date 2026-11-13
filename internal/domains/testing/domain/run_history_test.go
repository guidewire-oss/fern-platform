package domain_test

import (
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

func historyRuns(ids ...uint) []domain.HistoryRun {
	runs := make([]domain.HistoryRun, len(ids))
	for i, id := range ids {
		runs[i] = domain.HistoryRun{ID: id}
	}
	return runs
}

func cellStatuses(cells []*domain.HistoryCell) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		if c == nil {
			out[i] = "-"
		} else {
			out[i] = c.Status
		}
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBuildRunHistory_AlignsResultsWithRuns(t *testing.T) {
	runs := historyRuns(1, 2, 3) // 3 is the viewed run
	rows := []domain.HistoryRow{
		{TestRunID: 1, SuiteName: "S", SpecName: "a", Status: "failed", DurationMs: 5},
		{TestRunID: 2, SuiteName: "S", SpecName: "a", Status: "passed", DurationMs: 6},
		{TestRunID: 3, SuiteName: "S", SpecName: "a", Status: "passed", DurationMs: 7},
		{TestRunID: 1, SuiteName: "S", Status: "failed"},
		{TestRunID: 3, SuiteName: "S", Status: "passed"},
	}
	h := domain.BuildRunHistory(runs, rows)

	if len(h.Specs) != 1 || h.Specs[0].SpecName != "a" {
		t.Fatalf("specs = %+v, want one spec a", h.Specs)
	}
	if got, want := cellStatuses(h.Specs[0].Results), []string{"failed", "passed", "passed"}; !equalStrings(got, want) {
		t.Errorf("spec results = %v, want %v", got, want)
	}
	if h.Specs[0].Results[2].DurationMs != 7 {
		t.Errorf("viewed-run duration = %d, want 7", h.Specs[0].Results[2].DurationMs)
	}
	if len(h.Suites) != 1 {
		t.Fatalf("suites = %+v, want one", h.Suites)
	}
	if got, want := cellStatuses(h.Suites[0].Results), []string{"failed", "-", "passed"}; !equalStrings(got, want) {
		t.Errorf("suite results = %v, want %v", got, want)
	}
}

func TestBuildRunHistory_OnlySpecsInTheViewedRun(t *testing.T) {
	runs := historyRuns(1, 2)
	rows := []domain.HistoryRow{
		{TestRunID: 1, SuiteName: "S", SpecName: "removed", Status: "passed"},
		{TestRunID: 2, SuiteName: "S", SpecName: "kept", Status: "passed"},
	}
	h := domain.BuildRunHistory(runs, rows)
	if len(h.Specs) != 1 || h.Specs[0].SpecName != "kept" {
		t.Fatalf("specs = %+v, want only kept", h.Specs)
	}
	if got, want := cellStatuses(h.Specs[0].Results), []string{"-", "passed"}; !equalStrings(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
}

func TestBuildRunHistory_SameSpecNameInDifferentSuitesIsSeparate(t *testing.T) {
	runs := historyRuns(1)
	rows := []domain.HistoryRow{
		{TestRunID: 1, SuiteName: "A", SpecName: "x", Status: "passed"},
		{TestRunID: 1, SuiteName: "B", SpecName: "x", Status: "failed"},
	}
	h := domain.BuildRunHistory(runs, rows)
	if len(h.Specs) != 2 {
		t.Fatalf("specs = %+v, want two", h.Specs)
	}
	if h.Specs[0].SuiteName != "A" || h.Specs[1].SuiteName != "B" {
		t.Errorf("order = %s, %s; want A, B (first appearance)", h.Specs[0].SuiteName, h.Specs[1].SuiteName)
	}
}

func TestBuildRunHistory_DuplicateInOneRunKeepsFailure(t *testing.T) {
	runs := historyRuns(1)
	rows := []domain.HistoryRow{
		{TestRunID: 1, SuiteName: "S", SpecName: "a", Status: "failed"},
		{TestRunID: 1, SuiteName: "S", SpecName: "a", Status: "passed"},
		{TestRunID: 1, SuiteName: "S", SpecName: "b", Status: "skipped"},
		{TestRunID: 1, SuiteName: "S", SpecName: "b", Status: "passed"},
	}
	h := domain.BuildRunHistory(runs, rows)
	if got := h.Specs[0].Results[0].Status; got != "failed" {
		t.Errorf("a = %s, want failed", got)
	}
	if got := h.Specs[1].Results[0].Status; got != "passed" {
		t.Errorf("b = %s, want passed", got)
	}
}

func TestBuildRunHistory_IgnoresRowsOutsideTheWindow(t *testing.T) {
	runs := historyRuns(2)
	rows := []domain.HistoryRow{
		{TestRunID: 9, SuiteName: "S", SpecName: "a", Status: "failed"},
		{TestRunID: 2, SuiteName: "S", SpecName: "a", Status: "passed"},
	}
	h := domain.BuildRunHistory(runs, rows)
	if got, want := cellStatuses(h.Specs[0].Results), []string{"passed"}; !equalStrings(got, want) {
		t.Errorf("results = %v, want %v", got, want)
	}
}

func TestBuildRunHistory_NoRuns(t *testing.T) {
	h := domain.BuildRunHistory(nil, []domain.HistoryRow{{TestRunID: 1, SuiteName: "S"}})
	if h.Suites == nil || h.Specs == nil || len(h.Suites)+len(h.Specs) != 0 {
		t.Errorf("want empty, non-nil slices; got %+v", h)
	}
}

func TestSuiteOutcome(t *testing.T) {
	cases := []struct {
		status                  string
		passed, failed, skipped int
		want                    string
	}{
		{"completed", 7, 1, 0, "failed"},
		{"completed", 8, 0, 0, "passed"},
		{"completed", 2, 0, 6, "passed"},
		{"completed", 0, 0, 3, "skipped"},
		{"running", 0, 0, 0, "running"},
	}
	for _, tc := range cases {
		if got := domain.SuiteOutcome(tc.status, tc.passed, tc.failed, tc.skipped); got != tc.want {
			t.Errorf("SuiteOutcome(%q, %d, %d, %d) = %q, want %q",
				tc.status, tc.passed, tc.failed, tc.skipped, got, tc.want)
		}
	}
}

func TestBuildSpecRunHistory(t *testing.T) {
	runs := historyRuns(1, 2, 3)
	rows := []domain.SpecHistoryRow{
		{TestRunID: 1, Status: "failed", DurationMs: 9, ErrorMessage: "boom", StackTrace: "at x"},
		{TestRunID: 3, Status: "passed", DurationMs: 5},
		{TestRunID: 3, Status: "failed", DurationMs: 6, ErrorMessage: "again"},
		{TestRunID: 99, Status: "failed"},
	}
	h := domain.BuildSpecRunHistory(runs, "S", "a", rows)
	if h.SuiteName != "S" || h.SpecName != "a" || len(h.Results) != 3 {
		t.Fatalf("history = %+v", h)
	}
	if c := h.Results[0]; c == nil || c.ErrorMessage != "boom" || c.StackTrace != "at x" || c.DurationMs != 9 {
		t.Errorf("run 1 = %+v, want the failure with its text", c)
	}
	if h.Results[1] != nil {
		t.Errorf("run 2 = %+v, want absent", h.Results[1])
	}
	if c := h.Results[2]; c == nil || c.Status != "failed" || c.ErrorMessage != "again" {
		t.Errorf("run 3 = %+v, want the failure to win with its message", c)
	}
}
