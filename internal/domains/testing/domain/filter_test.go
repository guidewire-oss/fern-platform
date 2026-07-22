package domain_test

import (
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

func TestPageArgs_Normalize_ClampsFirst(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, domain.DefaultPageSize},
		{-5, domain.DefaultPageSize},
		{10, 10},
		{domain.MaxPageSize, domain.MaxPageSize},
		{domain.MaxPageSize + 1, domain.MaxPageSize},
		{99999, domain.MaxPageSize},
	}
	for _, tc := range cases {
		p := domain.PageArgs{First: tc.in}
		p.Normalize()
		if p.First != tc.want {
			t.Errorf("First=%d: got %d, want %d", tc.in, p.First, tc.want)
		}
	}
}

func TestTestRunFilter_Validate_RejectsInvertedRanges(t *testing.T) {
	from := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	to := from.Add(-time.Hour)
	f := domain.TestRunFilter{
		StartedAt: &domain.DateTimeRange{Gte: &from, Lte: &to},
	}
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for inverted date range")
	}
}

func TestTestRunFilter_Validate_RejectsInvertedDuration(t *testing.T) {
	gte, lte := 100, 10
	f := domain.TestRunFilter{
		DurationMs: &domain.IntRange{Gte: &gte, Lte: &lte},
	}
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for inverted duration range")
	}
}

func TestTestRunFilter_Validate_AcceptsEmpty(t *testing.T) {
	var f domain.TestRunFilter
	if err := f.Validate(); err != nil {
		t.Errorf("empty filter should be valid, got %v", err)
	}
}

func TestTestRunFilter_IsNarrow(t *testing.T) {
	q := "oauth"
	cases := []struct {
		name   string
		filter domain.TestRunFilter
		want   bool
	}{
		{"empty", domain.TestRunFilter{}, false},
		{"project", domain.TestRunFilter{ProjectIDs: []string{"p1"}}, true},
		{"branch", domain.TestRunFilter{Branches: []string{"main"}}, true},
		{"status", domain.TestRunFilter{Status: []string{"failed"}}, true},
		{"tags", domain.TestRunFilter{Tags: []string{"smoke"}}, true},
		{"search", domain.TestRunFilter{Search: &q}, true},
	}
	for _, tc := range cases {
		if got := tc.filter.IsNarrow(); got != tc.want {
			t.Errorf("%s: IsNarrow=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestTestRunFilter_Validate_NormalizesTagMode(t *testing.T) {
	f := domain.TestRunFilter{Tags: []string{"a"}}
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
	if f.TagMode != domain.LogicOr {
		t.Errorf("default tag mode should be OR, got %q", f.TagMode)
	}
}

func TestTestRunFilter_Validate_RejectsBadTagMode(t *testing.T) {
	bogus := domain.LogicMode("XOR")
	f := domain.TestRunFilter{Tags: []string{"a"}, TagMode: bogus}
	if err := f.Validate(); err == nil {
		t.Fatal("expected error for invalid tag mode")
	}
}
