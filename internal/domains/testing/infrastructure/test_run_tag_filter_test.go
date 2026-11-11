package infrastructure_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

// tagFixture seeds one run per tagging shape so each test can assert
// exactly which runs a tag filter returns.
type tagFixture struct {
	db                                *gorm.DB
	tRun, tSuite, tSpec, tDeleted, tX database.Tag
}

func newTagFixture(t *testing.T) *tagFixture {
	t.Helper()
	db := openSQLite(t)
	f := &tagFixture{
		db:       db,
		tRun:     database.Tag{Name: "jira:RUN-1"},
		tSuite:   database.Tag{Name: "smoke"},
		tSpec:    database.Tag{Name: "jira:SPEC-1"},
		tDeleted: database.Tag{Name: "jira:GONE-1"},
		tX:       database.Tag{Name: "unused"},
	}
	for _, tg := range []*database.Tag{&f.tRun, &f.tSuite, &f.tSpec, &f.tDeleted, &f.tX} {
		if err := db.Create(tg).Error; err != nil {
			t.Fatal(err)
		}
	}
	return f
}

type runTags struct{ run, suite, spec []database.Tag }

// mkRun creates a run with one suite and one spec, attaching tags at
// the requested levels. Returns the run's RunID.
func (f *tagFixture) mkRun(t *testing.T, project string, tags runTags) string {
	t.Helper()
	seedCounter++
	run := database.TestRun{
		ProjectID: project,
		RunID:     uniqueRunID(),
		Status:    "passed",
		Branch:    "main",
		StartTime: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(seedCounter) * time.Hour),
		Tags:      tags.run,
	}
	if err := f.db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	suite := database.SuiteRun{TestRunID: run.ID, SuiteName: "s", Tags: tags.suite}
	if err := f.db.Create(&suite).Error; err != nil {
		t.Fatal(err)
	}
	spec := database.SpecRun{SuiteRunID: suite.ID, SpecName: "sp", Status: "passed", Tags: tags.spec}
	if err := f.db.Create(&spec).Error; err != nil {
		t.Fatal(err)
	}
	return run.RunID
}

func (f *tagFixture) query(t *testing.T, filter domain.TestRunFilter) []string {
	t.Helper()
	page, err := infrastructure.NewTestRunQueryRepo(f.db).Query(context.Background(), filter, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(page.Edges))
	for _, e := range page.Edges {
		ids = append(ids, e.Node.RunID)
	}
	sort.Strings(ids)
	if int64(len(ids)) != page.TotalCount {
		t.Errorf("TotalCount = %d, want %d (edges %v)", page.TotalCount, len(ids), ids)
	}
	return ids
}

func sorted(s ...string) []string { sort.Strings(s); return s }

func assertRuns(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("runs = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("runs = %v, want %v", got, want)
		}
	}
}

func TestQuery_TagFilterMatchesEveryLevel(t *testing.T) {
	f := newTagFixture(t)
	runLvl := f.mkRun(t, "p1", runTags{run: []database.Tag{f.tRun}})
	suiteLvl := f.mkRun(t, "p1", runTags{suite: []database.Tag{f.tSuite}})
	specLvl := f.mkRun(t, "p1", runTags{spec: []database.Tag{f.tSpec}})
	f.mkRun(t, "p1", runTags{}) // untagged

	cases := []struct {
		tag  string
		want []string
	}{
		{"jira:RUN-1", []string{runLvl}},
		{"smoke", []string{suiteLvl}},
		{"jira:SPEC-1", []string{specLvl}},
		{"unused", nil},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			got := f.query(t, domain.TestRunFilter{ProjectIDs: []string{"p1"}, Tags: []string{tc.tag}})
			assertRuns(t, got, sorted(tc.want...))
		})
	}
}

func TestQuery_TagFilterIsScopedToProject(t *testing.T) {
	f := newTagFixture(t)
	inP1 := f.mkRun(t, "p1", runTags{spec: []database.Tag{f.tSpec}})
	inP2 := f.mkRun(t, "p2", runTags{spec: []database.Tag{f.tSpec}})

	assertRuns(t, f.query(t, domain.TestRunFilter{ProjectIDs: []string{"p1"}, Tags: []string{"jira:SPEC-1"}}), []string{inP1})
	assertRuns(t, f.query(t, domain.TestRunFilter{Tags: []string{"jira:SPEC-1"}}), sorted(inP1, inP2))
}

// A run tagged at more than one level, or on several specs, is one run:
// the UNION ALL must not duplicate rows or inflate TotalCount.
func TestQuery_TagFilterDoesNotDuplicateRuns(t *testing.T) {
	f := newTagFixture(t)
	both := f.mkRun(t, "p1", runTags{run: []database.Tag{f.tSpec}, suite: []database.Tag{f.tSpec}, spec: []database.Tag{f.tSpec}})
	got := f.query(t, domain.TestRunFilter{ProjectIDs: []string{"p1"}, Tags: []string{"jira:SPEC-1"}})
	assertRuns(t, got, []string{both})
}

func TestQuery_TagFilterOrAndModesAcrossLevels(t *testing.T) {
	f := newTagFixture(t)
	runOnly := f.mkRun(t, "p1", runTags{run: []database.Tag{f.tRun}})
	specOnly := f.mkRun(t, "p1", runTags{spec: []database.Tag{f.tSpec}})
	both := f.mkRun(t, "p1", runTags{run: []database.Tag{f.tRun}, spec: []database.Tag{f.tSpec}})

	tags := []string{"jira:RUN-1", "jira:SPEC-1"}
	assertRuns(t,
		f.query(t, domain.TestRunFilter{ProjectIDs: []string{"p1"}, Tags: tags, TagMode: domain.LogicOr}),
		sorted(runOnly, specOnly, both))
	assertRuns(t,
		f.query(t, domain.TestRunFilter{ProjectIDs: []string{"p1"}, Tags: tags, TagMode: domain.LogicAnd}),
		[]string{both})
}

func TestQuery_TagFilterIgnoresSoftDeletedSpecs(t *testing.T) {
	f := newTagFixture(t)
	f.mkRun(t, "p1", runTags{spec: []database.Tag{f.tDeleted}})
	if err := f.db.Where("1 = 1").Delete(&database.SpecRun{}).Error; err != nil { // soft delete
		t.Fatal(err)
	}
	assertRuns(t, f.query(t, domain.TestRunFilter{ProjectIDs: []string{"p1"}, Tags: []string{"jira:GONE-1"}}), nil)
}

// The tag facet counts run- and suite-level tags; spec-level tags are
// left out on purpose (cost), even though they still filter.
func TestComputeFacets_TagFacetCountsRunAndSuiteLevels(t *testing.T) {
	f := newTagFixture(t)
	f.mkRun(t, "p1", runTags{run: []database.Tag{f.tRun}})
	f.mkRun(t, "p1", runTags{run: []database.Tag{f.tRun}, suite: []database.Tag{f.tRun}}) // counted once
	f.mkRun(t, "p1", runTags{suite: []database.Tag{f.tSuite}})
	f.mkRun(t, "p1", runTags{spec: []database.Tag{f.tSpec}})

	facets, err := infrastructure.NewTestRunQueryRepo(f.db).ComputeFacets(context.Background(),
		domain.TestRunFilter{ProjectIDs: []string{"p1"}, IncludeTagFacet: true})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, fc := range facets.ByTag {
		counts[fc.Value] = fc.Count
	}
	if counts["jira:RUN-1"] != 2 || counts["smoke"] != 1 {
		t.Errorf("byTag = %+v, want jira:RUN-1=2 smoke=1", counts)
	}
	if _, ok := counts["jira:SPEC-1"]; ok {
		t.Errorf("spec-level tag should not appear in the facet: %+v", counts)
	}
}
