package infrastructure_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

var historyBase = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

// seedHistoryRun creates a run at historyBase+hour with one suite "S"
// holding the given spec statuses (spec name -> status).
func seedHistoryRun(t *testing.T, db *gorm.DB, project, branch string, hour int, specs map[string]string) uint {
	t.Helper()
	seedCounter++
	run := database.TestRun{
		ProjectID: project,
		RunID:     fmt.Sprintf("hist-%d", seedCounter),
		Branch:    branch,
		Status:    "passed",
		StartTime: historyBase.Add(time.Duration(hour) * time.Hour),
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}
	// Ingest stores a lifecycle status on suites and the outcome in the
	// counts, so seed them the same way.
	suite := database.SuiteRun{TestRunID: run.ID, SuiteName: "S", Status: "completed", StartTime: run.StartTime}
	for _, status := range specs {
		switch status {
		case "passed":
			suite.PassedSpecs++
		case "failed":
			suite.FailedSpecs++
		case "skipped":
			suite.SkippedSpecs++
		}
	}
	if err := db.Create(&suite).Error; err != nil {
		t.Fatalf("seed suite: %v", err)
	}
	for name, status := range specs {
		spec := database.SpecRun{SuiteRunID: suite.ID, SpecName: name, Status: status, StartTime: run.StartTime, Duration: 42}
		if err := db.Create(&spec).Error; err != nil {
			t.Fatalf("seed spec: %v", err)
		}
	}
	return run.ID
}

func specCells(t *testing.T, h *domain.RunHistory, name string) []string {
	t.Helper()
	for _, s := range h.Specs {
		if s.SpecName == name {
			out := make([]string, len(s.Results))
			for i, c := range s.Results {
				if c == nil {
					out[i] = "-"
				} else {
					out[i] = c.Status
				}
			}
			return out
		}
	}
	t.Fatalf("spec %q not in history", name)
	return nil
}

func TestRunHistory_WindowEndsAtTheViewedRun(t *testing.T) {
	db := openSQLite(t)
	seedHistoryRun(t, db, "p", "main", 1, map[string]string{"a": "failed"})
	viewed := seedHistoryRun(t, db, "p", "main", 2, map[string]string{"a": "passed"})
	seedHistoryRun(t, db, "p", "main", 3, map[string]string{"a": "failed"}) // newer: excluded

	h, err := infrastructure.NewRunHistoryRepo(db).RunHistory(context.Background(), viewed, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Runs) != 2 || h.Runs[1].ID != viewed {
		t.Fatalf("runs = %+v, want 2 ending at %d", h.Runs, viewed)
	}
	if got := specCells(t, h, "a"); fmt.Sprint(got) != "[failed passed]" {
		t.Errorf("a = %v, want [failed passed]", got)
	}
	if h.ProjectID != "p" {
		t.Errorf("ProjectID = %q, want p", h.ProjectID)
	}
	if h.Specs[0].Results[1].DurationMs != 42 {
		t.Errorf("duration = %d, want 42", h.Specs[0].Results[1].DurationMs)
	}
}

func TestRunHistory_SameProjectAndBranchOnly(t *testing.T) {
	db := openSQLite(t)
	seedHistoryRun(t, db, "p", "feature", 1, map[string]string{"a": "failed"})
	seedHistoryRun(t, db, "other", "main", 2, map[string]string{"a": "failed"})
	viewed := seedHistoryRun(t, db, "p", "main", 3, map[string]string{"a": "passed"})

	h, err := infrastructure.NewRunHistoryRepo(db).RunHistory(context.Background(), viewed, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Runs) != 1 {
		t.Fatalf("runs = %+v, want only the viewed run", h.Runs)
	}
}

func TestRunHistory_LimitKeepsTheMostRecent(t *testing.T) {
	db := openSQLite(t)
	var viewed uint
	for hour := 1; hour <= 5; hour++ {
		viewed = seedHistoryRun(t, db, "p", "main", hour, map[string]string{"a": "passed"})
	}
	h, err := infrastructure.NewRunHistoryRepo(db).RunHistory(context.Background(), viewed, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Runs) != 3 {
		t.Fatalf("runs = %d, want 3", len(h.Runs))
	}
	for i := 1; i < len(h.Runs); i++ {
		if !h.Runs[i-1].StartTime.Before(h.Runs[i].StartTime) {
			t.Errorf("runs not oldest first: %v", h.Runs)
		}
	}
	if h.Runs[2].ID != viewed {
		t.Errorf("last run = %d, want viewed %d", h.Runs[2].ID, viewed)
	}
}

func TestRunHistory_AbsentSpecAndSuiteRows(t *testing.T) {
	db := openSQLite(t)
	seedHistoryRun(t, db, "p", "main", 1, map[string]string{"old": "passed"})
	viewed := seedHistoryRun(t, db, "p", "main", 2, map[string]string{"new": "passed"})

	h, err := infrastructure.NewRunHistoryRepo(db).RunHistory(context.Background(), viewed, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Specs) != 1 {
		t.Fatalf("specs = %+v, want only the viewed run's spec", h.Specs)
	}
	if got := specCells(t, h, "new"); fmt.Sprint(got) != "[- passed]" {
		t.Errorf("new = %v, want [- passed]", got)
	}
	if len(h.Suites) != 1 || h.Suites[0].Results[0] == nil {
		t.Errorf("suite S should have a result in both runs: %+v", h.Suites)
	}
}

func TestRunHistory_UnknownRun(t *testing.T) {
	db := openSQLite(t)
	_, err := infrastructure.NewRunHistoryRepo(db).RunHistory(context.Background(), 999, 20)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRunHistory_SuiteCellsShowTheOutcomeNotTheLifecycleStatus(t *testing.T) {
	db := openSQLite(t)
	seedHistoryRun(t, db, "p", "main", 1, map[string]string{"a": "failed", "b": "passed"})
	viewed := seedHistoryRun(t, db, "p", "main", 2, map[string]string{"a": "passed", "b": "passed"})

	h, err := infrastructure.NewRunHistoryRepo(db).RunHistory(context.Background(), viewed, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Suites) != 1 {
		t.Fatalf("suites = %+v, want one", h.Suites)
	}
	got := []string{h.Suites[0].Results[0].Status, h.Suites[0].Results[1].Status}
	if fmt.Sprint(got) != "[failed passed]" {
		t.Errorf("suite S = %v, want [failed passed]", got)
	}
}

func TestSpecHistory_SameWindowAsTheStripWithFailureText(t *testing.T) {
	db := openSQLite(t)
	seedHistoryRun(t, db, "p", "feature", 1, map[string]string{"a": "failed"}) // other branch
	seedHistoryRun(t, db, "p", "main", 2, map[string]string{"a": "failed", "b": "failed"})
	seedHistoryRun(t, db, "p", "main", 3, map[string]string{"b": "passed"}) // a absent
	viewed := seedHistoryRun(t, db, "p", "main", 4, map[string]string{"a": "passed", "b": "passed"})
	if err := db.Model(&database.SpecRun{}).Where("status = ?", "failed").
		Updates(map[string]any{"error_message": "boom", "stack_trace": "at line 1"}).Error; err != nil {
		t.Fatal(err)
	}

	repo := infrastructure.NewRunHistoryRepo(db)
	strip, err := repo.RunHistory(context.Background(), viewed, 20)
	if err != nil {
		t.Fatal(err)
	}
	h, err := repo.SpecHistory(context.Background(), viewed, 20, "S", "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Runs) != len(strip.Runs) {
		t.Fatalf("spec window has %d runs, strip has %d", len(h.Runs), len(strip.Runs))
	}
	for i := range h.Runs {
		if h.Runs[i].ID != strip.Runs[i].ID {
			t.Errorf("run %d: spec window id %d, strip id %d", i, h.Runs[i].ID, strip.Runs[i].ID)
		}
	}
	if h.ProjectID != "p" || h.SuiteName != "S" || h.SpecName != "a" {
		t.Errorf("identity = %q %q %q", h.ProjectID, h.SuiteName, h.SpecName)
	}
	if c := h.Results[0]; c == nil || c.Status != "failed" || c.ErrorMessage != "boom" || c.StackTrace != "at line 1" {
		t.Errorf("run 0 = %+v, want the failure with its text", c)
	}
	if h.Results[1] != nil {
		t.Errorf("run 1 = %+v, want absent", h.Results[1])
	}
	if c := h.Results[2]; c == nil || c.Status != "passed" || c.ErrorMessage != "" {
		t.Errorf("run 2 = %+v, want a pass without text", c)
	}
}

func TestSpecHistory_UnknownSpecIsAllAbsent(t *testing.T) {
	db := openSQLite(t)
	viewed := seedHistoryRun(t, db, "p", "main", 1, map[string]string{"a": "passed"})
	h, err := infrastructure.NewRunHistoryRepo(db).SpecHistory(context.Background(), viewed, 20, "S", "nope")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Results) != 1 || h.Results[0] != nil {
		t.Errorf("results = %+v, want one absent cell", h.Results)
	}
}

func TestSpecHistory_UnknownRun(t *testing.T) {
	db := openSQLite(t)
	_, err := infrastructure.NewRunHistoryRepo(db).SpecHistory(context.Background(), 999, 20, "S", "a")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
