package infrastructure_test

import (
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
)

func TestCountStrategy_NarrowFilterReturnsExact(t *testing.T) {
	f := domain.TestRunFilter{ProjectIDs: []string{"p1"}}
	if got := infrastructure.ChooseCountStrategy(f); got != infrastructure.CountExact {
		t.Errorf("narrow filter should use exact count, got %v", got)
	}
}

func TestCountStrategy_EmptyFilterReturnsEstimate(t *testing.T) {
	if got := infrastructure.ChooseCountStrategy(domain.TestRunFilter{}); got != infrastructure.CountEstimate {
		t.Errorf("empty filter should use estimate, got %v", got)
	}
}

func TestCountStrategy_DateOnlyIsBroad(t *testing.T) {
	from := time.Now().Add(-30 * 24 * time.Hour)
	f := domain.TestRunFilter{StartedAt: &domain.DateTimeRange{Gte: &from}}
	if got := infrastructure.ChooseCountStrategy(f); got != infrastructure.CountEstimate {
		t.Errorf("date-only filter should use estimate, got %v", got)
	}
}

func TestCountStrategy_SearchIsNarrow(t *testing.T) {
	q := "oauth"
	f := domain.TestRunFilter{Search: &q}
	if got := infrastructure.ChooseCountStrategy(f); got != infrastructure.CountExact {
		t.Errorf("search filter should use exact, got %v", got)
	}
}
