// Command seedjiracoverage generates "golden" JIRA requirement-coverage
// data for a project + release. Coverage is computed by joining Fern test
// runs (tagged category='jira', value=<issue key>) against the epic/story
// tree fetched live from JIRA — so this reuses CoverageService.Build to
// discover the release's real story keys, then inserts tagged spec runs
// with a realistic passing / failing / uncovered mix.
//
// It is READ-then-WRITE against the live DB and calls JIRA via the
// project's existing connection. Configure with SEED_JIRA_* env vars.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envFloat(k string, def float64) float64 {
	if v := os.Getenv(k); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func main() {
	projectID := env("SEED_JIRA_PROJECT", "bd44097f-1a42-47fe-b414-8ba6af4fbb63")
	release := env("SEED_JIRA_RELEASE", "REVELSTOKE (2026.06M)")
	coveredPct := envFloat("SEED_COVERED_PCT", 0.70)      // 70% of stories covered
	failFracOfCovered := envFloat("SEED_FAIL_FRAC", 0.21) // ~15% of all stories fail

	cm := config.NewManager()
	if err := cm.Load(""); err != nil {
		log.Fatalf("load config: %v", err)
	}
	cfg := config.GetConfig()
	if err := logging.Initialize(&cfg.Logging); err != nil {
		log.Fatalf("init logging: %v", err)
	}
	logger := logging.GetLogger()

	db, err := database.NewDatabase(&cfg.Database)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()
	gdb := db.DB

	factory := domains.NewDomainFactory(gdb, logger, &cfg.Auth)
	cov := factory.GetCoverageService()

	ctx := context.Background()
	log.Printf("fetching coverage tree for project=%s release=%q …", projectID, release)
	tree, err := cov.Build(ctx, projectID, release)
	if err != nil {
		log.Fatalf("build coverage tree (check connection + release mapping): %v", err)
	}

	// Collect distinct top-level story keys across all epics + unassigned.
	seen := map[string]bool{}
	var keys []string
	for _, e := range tree.Epics {
		for _, s := range e.Stories {
			if k := s.Issue.Key; k != "" && !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	for _, s := range tree.Unassigned {
		if k := s.Issue.Key; k != "" && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		log.Fatalf("no stories found for release %q — nothing to seed", release)
	}
	log.Printf("release has %d epics, %d distinct stories", len(tree.Epics), len(keys))

	// Deterministic shuffle so re-runs are stable.
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })

	nCovered := int(coveredPct * float64(len(keys)))
	covered := keys[:nCovered]
	nFail := int(failFracOfCovered * float64(len(keys)))
	if nFail > nCovered {
		nFail = nCovered
	}

	now := time.Now().UTC()
	start := now.Add(-12 * time.Minute)
	runID := fmt.Sprintf("jira-coverage-%s-%d", slug(release), now.Unix())

	tr := &database.TestRun{
		ProjectID: projectID, RunID: runID, Branch: "main", CommitSHA: fmt.Sprintf("%08x", rng.Uint32()),
		Status: "completed", StartTime: start, EndTime: &now, Environment: "ci",
		TotalTests: nCovered, PassedTests: nCovered - nFail, FailedTests: nFail,
		Duration: (12 * time.Minute).Milliseconds(),
	}
	if err := gdb.Create(tr).Error; err != nil {
		log.Fatalf("create test_run: %v", err)
	}
	su := &database.SuiteRun{
		TestRunID: tr.ID, SuiteName: "Requirement Coverage — " + release, Status: "completed",
		StartTime: start, EndTime: &now, TotalSpecs: nCovered, PassedSpecs: nCovered - nFail, FailedSpecs: nFail,
		Duration: (12 * time.Minute).Milliseconds(),
	}
	if err := gdb.Create(su).Error; err != nil {
		log.Fatalf("create suite_run: %v", err)
	}

	passed, failed := 0, 0
	for i, key := range covered {
		status := "passed"
		if i < nFail {
			status = "failed"
		}
		// Ensure the JIRA tag (category='jira', value=<key>) exists.
		tag := database.Tag{Name: "jira:" + key, Category: "jira", Value: key}
		if err := gdb.Where("name = ?", tag.Name).
			Attrs(database.Tag{Category: "jira", Value: key, Color: "#2684FF"}).
			FirstOrCreate(&tag).Error; err != nil {
			log.Fatalf("ensure tag %s: %v", key, err)
		}
		sEnd := now
		spec := &database.SpecRun{
			SuiteRunID: su.ID, SpecName: fmt.Sprintf("verifies %s", key), Status: status,
			StartTime: start, EndTime: &sEnd, Duration: int64(200 + rng.Intn(4000)),
		}
		if status == "failed" {
			spec.ErrorMessage = "seeded failure for coverage demo"
		}
		if err := gdb.Create(spec).Error; err != nil {
			log.Fatalf("create spec_run for %s: %v", key, err)
		}
		if err := gdb.Exec(
			`INSERT INTO spec_run_tags (spec_run_id, tag_id) VALUES (?, ?) ON CONFLICT DO NOTHING`,
			spec.ID, tag.ID,
		).Error; err != nil {
			log.Fatalf("link spec_run_tag for %s: %v", key, err)
		}
		if status == "passed" {
			passed++
		} else {
			failed++
		}
	}

	uncovered := len(keys) - nCovered
	log.Printf("done: run_id=%s", runID)
	log.Printf("  stories total   %d", len(keys))
	log.Printf("  covered passing %d", passed)
	log.Printf("  covered failing %d", failed)
	log.Printf("  uncovered       %d", uncovered)
	log.Printf("  => %d%% covered", int(float64(nCovered)/float64(len(keys))*100))
}

// slug makes a release name safe for a run id.
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
