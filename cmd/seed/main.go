// Fern Platform — bulk perf-test seeder.
//
// Designed to populate 1000 projects × 180 days × 100 runs/day = 18M
// test_runs in minutes (not hours) using pgx's CopyFrom — Postgres's
// native bulk-load path. Suite_runs, spec_runs, tags, and the
// many-to-many junctions are seeded too.
//
// Projects pick a realistic technology template (Java / Infra /
// FluxCD / Helm / Node.js) so the dataset looks like a real CI
// fleet for perf measurement.
//
// All knobs are env vars — see seedConfigFromEnv.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	mathrand "math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guidewire-oss/fern-platform/pkg/config"
)

type seedConfig struct {
	Projects             int
	Days                 int
	RunsPerDay           int
	BatchSize            int
	SuitesPerRun         int     // mean number of suite_runs per test_run
	SpecsPerFailingSuite int     // mean number of spec_runs per failing suite
	SuiteTagRate         float64 // fraction of suites that get tags
	TruncateBeforeSeed   bool
	DryRun               bool
	// HealthBands distributes projects across 5 pass-rate bands instead
	// of using the default fixed 75/20/5 distribution. Useful for
	// exercising the treemap's red→green color gradient: with bands
	// off, every project clusters around 75% and the gradient just
	// shows the green/amber slice. See healthBandFor.
	HealthBands bool

	// SkipRuns skips the heavy seedRuns step (test_runs / suite_runs /
	// spec_runs / suite_run_tags). Use it when those tables already
	// have data you want to keep and you only need to re-run the
	// extras (users / scopes / flaky / jira / saved_views). Implies
	// truncate-runs=false even if SEED_TRUNCATE=true.
	SkipRuns bool
}

func seedConfigFromEnv() seedConfig {
	return seedConfig{
		Projects:             envInt("SEED_PROJECTS", 5),
		Days:                 envInt("SEED_DAYS", 60),
		RunsPerDay:           envInt("SEED_RUNS_PER_DAY", 5),
		BatchSize:            envInt("SEED_BATCH_SIZE", 50_000),
		SuitesPerRun:         envInt("SEED_SUITES_PER_RUN", 3),
		SpecsPerFailingSuite: envInt("SEED_SPECS_PER_FAILED_SUITE", 5),
		SuiteTagRate:         envFloat("SEED_SUITE_TAG_RATE", 0.4),
		TruncateBeforeSeed:   envBool("SEED_TRUNCATE", false),
		DryRun:               envBool("SEED_DRY_RUN", false),
		HealthBands:          envBool("SEED_HEALTH_BANDS", false),
		SkipRuns:             envBool("SEED_SKIP_RUNS", false),
	}
}

// healthBandFor maps a project index to one of five pass-rate bands.
// Projects are stable across reruns because the index is the only
// input — same project always lands in the same band, so the treemap
// looks the same after a re-seed.
//
// Target pass rates (status distribution → pass/fail/flaky picker):
//
//	band 0 (broken)     ≈10%   — 5% passed, 90% failed, 5% flaky
//	band 1 (struggling) ≈30%   — 25% passed, 65% failed, 10% flaky
//	band 2 (flaky)      ≈60%   — 55% passed, 30% failed, 15% flaky
//	band 3 (healthy)    ≈85%   — 85% passed, 10% failed, 5% flaky
//	band 4 (rock solid) ≈98%   — 98% passed, 1% failed, 1% flaky
//
// The actual run-level passRate is further driven by the per-run
// passed/failed/skipped counts within statuses, so observed values
// drift from these targets by ±5pp — perfect for a gradient demo.
func healthBandFor(projectIdx, totalProjects int) int {
	if totalProjects <= 1 {
		return 4
	}
	// Even split across 5 bands. With 20 projects → 4 per band.
	return projectIdx * 5 / totalProjects
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
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
func envBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		return v == "1" || strings.EqualFold(v, "true")
	}
	return def
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg := seedConfigFromEnv()

	totalRuns := cfg.Projects * cfg.Days * cfg.RunsPerDay
	totalSuites := totalRuns * cfg.SuitesPerRun
	failedSuitesEst := totalSuites * 25 / 100
	totalSpecs := failedSuitesEst * cfg.SpecsPerFailingSuite

	log.Printf("seed plan:")
	log.Printf("  projects             %d", cfg.Projects)
	log.Printf("  days                 %d", cfg.Days)
	log.Printf("  runs/day/project     %d", cfg.RunsPerDay)
	log.Printf("  suites/run           %d", cfg.SuitesPerRun)
	log.Printf("  specs/failed suite   %d (est.)", cfg.SpecsPerFailingSuite)
	log.Printf("  batch size           %d", cfg.BatchSize)
	log.Printf("  truncate first       %v", cfg.TruncateBeforeSeed)
	log.Printf("estimated row counts:")
	log.Printf("  test_runs            %s", humanCount(totalRuns))
	log.Printf("  suite_runs           %s", humanCount(totalSuites))
	log.Printf("  spec_runs            %s", humanCount(totalSpecs))

	if cfg.DryRun {
		log.Printf("SEED_DRY_RUN=true — exiting before any work")
		return
	}

	cm := config.NewManager()
	if err := cm.Load(""); err != nil {
		log.Fatalf("load config: %v", err)
	}
	dbCfg := &config.GetConfig().Database

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbCfg.ConnectionString())
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if cfg.TruncateBeforeSeed {
		log.Printf("truncating tables ...")
		// Tables ordered child → parent so cascades resolve cleanly.
		// `users` is intentionally NOT truncated — `ensureDevAdminUser`
		// in fern-platform main.go creates the synthetic principal at
		// app startup. seedExtras re-upserts our sample fleet over the
		// top via INSERT…ON CONFLICT.
		//
		// SkipRuns preserves the heavy tables (test_runs/suite_runs/
		// spec_runs/junctions/project_details/tags) so a re-run only
		// has to repopulate the extras.
		var truncateSQL string
		if cfg.SkipRuns {
			truncateSQL = `
				TRUNCATE
					flaky_tests, jira_connections, saved_views,
					user_scopes, project_permissions
				RESTART IDENTITY CASCADE
			`
		} else {
			truncateSQL = `
				TRUNCATE
					spec_run_tags, suite_run_tags, test_run_tags,
					spec_runs, suite_runs, test_runs,
					flaky_tests, jira_connections, saved_views,
					user_scopes, project_permissions,
					project_details, tags
				RESTART IDENTITY CASCADE
			`
		}
		if _, err := pool.Exec(ctx, truncateSQL); err != nil {
			log.Fatalf("truncate: %v", err)
		}
	}

	start := time.Now()

	tagIDs := upsertTags(ctx, pool)
	log.Printf("tags ready: %d distinct", len(tagIDs))

	projectIDs, projectCats := upsertProjects(ctx, pool, cfg.Projects)
	log.Printf("projects ready: %d", len(projectIDs))

	var stats seedStats
	if cfg.SkipRuns {
		log.Printf("SEED_SKIP_RUNS=true — skipping test_runs/suite_runs/spec_runs seeding")
	} else {
		stats = seedRuns(ctx, pool, cfg, projectIDs, projectCats, tagIDs)
	}

	extras := seedExtras(ctx, pool, cfg, projectIDs, projectCats, tagIDs)

	elapsed := time.Since(start).Round(time.Second)
	log.Printf("done in %s:", elapsed)
	log.Printf("  test_runs     %s", humanCount(int(stats.testRuns)))
	log.Printf("  suite_runs    %s", humanCount(int(stats.suiteRuns)))
	log.Printf("  spec_runs     %s", humanCount(int(stats.specRuns)))
	log.Printf("  suite_tags    %s (junction rows)", humanCount(int(stats.suiteTags)))
	log.Printf("  users         %s", humanCount(int(extras.users)))
	log.Printf("  user_groups   %s", humanCount(int(extras.userGroups)))
	log.Printf("  user_prefs    %s", humanCount(int(extras.userPrefs)))
	log.Printf("  user_scopes   %s", humanCount(int(extras.userScopes)))
	log.Printf("  proj_perms    %s", humanCount(int(extras.projectPerms)))
	log.Printf("  saved_views   %s", humanCount(int(extras.savedViews)))
	log.Printf("  jira_conns    %s", humanCount(int(extras.jiraConns)))
	log.Printf("  flaky_tests   %s", humanCount(int(extras.flakyTests)))
	if stats.testRuns > 0 && elapsed.Seconds() >= 0.1 {
		rate := float64(stats.testRuns) / elapsed.Seconds()
		log.Printf("  rate          ~%s test_runs/sec", humanCount(int(rate)))
	}
}

// ---- Reference data: tags + projects ---------------------------------------

func uniqueTagNames() []string {
	set := map[string]struct{}{}
	for _, c := range Categories {
		for _, t := range c.Tags {
			set[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

func upsertTags(ctx context.Context, pool *pgxpool.Pool) map[string]int64 {
	names := uniqueTagNames()
	for _, name := range names {
		if _, err := pool.Exec(ctx,
			`INSERT INTO tags (name, category, value, color, created_at, updated_at)
			 VALUES ($1, 'test', '', '', NOW(), NOW())
			 ON CONFLICT (name) DO NOTHING`, name); err != nil {
			log.Fatalf("upsert tag %q: %v", name, err)
		}
	}
	rows, err := pool.Query(ctx, `SELECT id, name FROM tags WHERE name = ANY($1)`, names)
	if err != nil {
		log.Fatalf("read tag ids: %v", err)
	}
	defer rows.Close()
	out := make(map[string]int64, len(names))
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			log.Fatalf("scan tag: %v", err)
		}
		out[name] = id
	}
	return out
}

func upsertProjects(ctx context.Context, pool *pgxpool.Pool, n int) ([]string, []Category) {
	ids := make([]string, n)
	cats := make([]Category, n)
	for i := 0; i < n; i++ {
		cat := PickCategory(i)
		seq := (i / len(Categories)) + 1
		pid := fmt.Sprintf("%s-%s-%03d", cat.Slug, shortBase36(seq), seq)
		ids[i] = pid
		cats[i] = cat

		if _, err := pool.Exec(ctx, `
			INSERT INTO project_details
				(project_id, name, description, repository, default_branch, settings, is_active, team, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'main', '{}', TRUE, $5, NOW(), NOW())
			ON CONFLICT (project_id) DO UPDATE SET name = EXCLUDED.name
		`,
			pid,
			fmt.Sprintf("%s %d", titleCase(cat.Name), seq),
			fmt.Sprintf("Seeded %s (%s)", cat.Name, cat.Framework),
			fmt.Sprintf("https://example.com/seed/%s/%s", cat.Slug, pid),
			cat.Team,
		); err != nil {
			log.Fatalf("upsert project %q: %v", pid, err)
		}
	}
	return ids, cats
}

// ---- Bulk insert via pgx.CopyFrom ------------------------------------------

type seedStats struct {
	testRuns  int64
	suiteRuns int64
	specRuns  int64
	suiteTags int64
}

type runRow struct {
	RunID, ProjectID                   string
	Branch, CommitSHA, Status, Environ string
	StartTime, EndTime                 time.Time
	Total, Passed, Failed, Skipped     int
	DurationMs                         int64
}

// sampleNearTarget picks a failure count clustered near `target` with a
// small downward jitter (up to ~10% of target). Used by both the
// run-level and suite-level seed paths when a band/status indicates a
// non-passing outcome.
//
// Why not `1 + rng.Intn(target)` (the previous logic)? Uniform sampling
// across `[1, target]` averages to target/2, which collapses the
// observed failure rate to half of what `failFractionForBand` intends.
// That made band 0 (intended ~10% pass) actually land near 57% pass
// across the whole project's aggregate — and every band ended up green
// on the treemap gradient. Biasing the sample near the target gives
// the band-driven gradient the variance it was designed for.
func sampleNearTarget(target int, rng *mathrand.Rand) int {
	if target < 1 {
		return 1
	}
	jitter := target/10 + 1
	v := target - rng.Intn(jitter)
	if v < 1 {
		v = 1
	}
	if v > target {
		v = target
	}
	return v
}

// computeSpecFailures returns how many of the `perSuite` seeded
// spec_runs for a suite should be marked failed.
//
// Full-coverage path (perSuite == totalSpecs): every spec gets a row,
// so the body count MUST match the suite header's failedSpecs exactly
// — no jitter, no proportional sampling. Without this, the UI shows
// "17 specs (3 failed)" in the header and 4 failed rows below, which
// looks like an application bug.
//
// Sampling path (perSuite < totalSpecs): we're only writing a slice
// of spec_runs to keep volume manageable. Use the proportional ratio
// so the sample's mix mirrors the suite-level mix on average, plus a
// small jitter so identically-shaped suites don't all sample to the
// exact same row mix.
func computeSpecFailures(perSuite, totalSpecs, failedSpecs int, rng *mathrand.Rand) int {
	var failPart int
	if perSuite == totalSpecs {
		failPart = failedSpecs
	} else {
		failPart = int(float64(failedSpecs) / float64(totalSpecs) * float64(perSuite))
		if rng.Intn(2) == 0 && failPart < perSuite && failedSpecs > 0 {
			failPart++ // jitter up
		}
	}
	if failPart < 0 {
		failPart = 0
	}
	if failPart > perSuite {
		failPart = perSuite
	}
	return failPart
}

type suiteSeed struct {
	runID       int64
	suiteName   string
	status      string
	startTime   time.Time
	endTime     time.Time
	durationMs  int64
	totalSpecs  int
	failedSpecs int
	tagNames    []string
	assignedID  int64
}

func seedRuns(
	ctx context.Context,
	pool *pgxpool.Pool,
	cfg seedConfig,
	projectIDs []string,
	projectCats []Category,
	tagIDs map[string]int64,
) seedStats {
	var stats seedStats
	now := time.Now().UTC()

	for pi, pid := range projectIDs {
		cat := projectCats[pi]
		rng := mathrand.New(mathrand.NewSource(int64(pi)*1_000_003 + 42))
		band := healthBandFor(pi, len(projectIDs))

		seeded := 0
		runIdx := 0
		for seeded < cfg.Days*cfg.RunsPerDay {
			remaining := cfg.Days*cfg.RunsPerDay - seeded
			n := cfg.BatchSize
			if n > remaining {
				n = remaining
			}

			runs := make([]runRow, n)
			for i := 0; i < n; i++ {
				runIdx++
				day := (seeded + i) / cfg.RunsPerDay
				start := now.AddDate(0, 0, -cfg.Days+day).
					Add(time.Duration(rng.Intn(24*60)) * time.Minute)
				duration := time.Duration(30+rng.Intn(20*60)) * time.Second
				end := start.Add(duration)
				var status string
				if cfg.HealthBands {
					status = pickStatusForBand(rng, band)
				} else {
					status = pickStatus(rng)
				}
				total := 20 + rng.Intn(400)
				failed := 0
				if status == "failed" || status == "flaky" {
					// Scale fail count so the run-level pass rate
					// matches the band's target — broken projects
					// fail bigger fractions, healthy projects fail
					// small numbers.
					maxFailFrac := failFractionForBand(band, cfg.HealthBands)
					maxFail := int(float64(total) * maxFailFrac)
					if maxFail < 1 {
						maxFail = 1
					}
					failed = sampleNearTarget(maxFail, rng)
				}
				passed := total - failed
				skipped := rng.Intn(5)

				runs[i] = runRow{
					RunID:      fmt.Sprintf("%s-%d-%d", pid, start.Unix(), runIdx),
					ProjectID:  pid,
					Branch:     cat.Branches[rng.Intn(len(cat.Branches))],
					CommitSHA:  fakeSHA(rng),
					Status:     status,
					StartTime:  start,
					EndTime:    end,
					Total:      total,
					Passed:     passed,
					Failed:     failed,
					Skipped:    skipped,
					DurationMs: duration.Milliseconds(),
					Environ:    pickEnv(rng),
				}
			}

			runIDs, err := copyTestRuns(ctx, pool, runs)
			if err != nil {
				log.Fatalf("copy test_runs (project=%s, batch=%d): %v", pid, n, err)
			}
			stats.testRuns += int64(len(runs))

			suiteStats := seedSuitesForBatch(ctx, pool, cfg, cat, tagIDs, runs, runIDs, rng, band)
			stats.suiteRuns += suiteStats.suiteRuns
			stats.specRuns += suiteStats.specRuns
			stats.suiteTags += suiteStats.suiteTags

			seeded += n

			if seeded%(cfg.BatchSize*4) == 0 || seeded == cfg.Days*cfg.RunsPerDay {
				log.Printf("  project %d/%d (%s) %s runs seeded · totals: runs=%s suites=%s specs=%s",
					pi+1, len(projectIDs), pid,
					humanCount(seeded),
					humanCount(int(stats.testRuns)),
					humanCount(int(stats.suiteRuns)),
					humanCount(int(stats.specRuns)),
				)
			}
			_ = now
		}
	}
	return stats
}

func copyTestRuns(ctx context.Context, pool *pgxpool.Pool, runs []runRow) ([]int64, error) {
	if len(runs) == 0 {
		return nil, nil
	}
	now := time.Now().UTC()
	rowsIter := pgx.CopyFromSlice(len(runs), func(i int) ([]any, error) {
		r := runs[i]
		return []any{
			r.ProjectID, r.RunID, r.Branch, r.CommitSHA, r.Status,
			r.StartTime, r.EndTime,
			r.Total, r.Passed, r.Failed, r.Skipped,
			r.DurationMs, r.Environ,
			now, now,
		}, nil
	})

	if _, err := pool.CopyFrom(ctx,
		pgx.Identifier{"test_runs"},
		[]string{
			"project_id", "run_id", "branch", "commit_sha", "status",
			"start_time", "end_time",
			"total_tests", "passed_tests", "failed_tests", "skipped_tests",
			"duration_ms", "environment",
			"created_at", "updated_at",
		}, rowsIter); err != nil {
		return nil, err
	}

	runIDs := make([]string, len(runs))
	for i, r := range runs {
		runIDs[i] = r.RunID
	}
	idByRunID, err := lookupTestRunIDs(ctx, pool, runIDs)
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(runs))
	for i, r := range runs {
		id, ok := idByRunID[r.RunID]
		if !ok {
			return nil, fmt.Errorf("missing id for run_id %q after COPY", r.RunID)
		}
		out[i] = id
	}
	return out, nil
}

func lookupTestRunIDs(ctx context.Context, pool *pgxpool.Pool, runIDs []string) (map[string]int64, error) {
	rows, err := pool.Query(ctx, `SELECT id, run_id FROM test_runs WHERE run_id = ANY($1)`, runIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64, len(runIDs))
	for rows.Next() {
		var id int64
		var rid string
		if err := rows.Scan(&id, &rid); err != nil {
			return nil, err
		}
		out[rid] = id
	}
	return out, nil
}

func seedSuitesForBatch(
	ctx context.Context,
	pool *pgxpool.Pool,
	cfg seedConfig,
	cat Category,
	tagIDs map[string]int64,
	runs []runRow,
	runIDs []int64,
	rng *mathrand.Rand,
	band int,
) seedStats {
	if cfg.SuitesPerRun <= 0 {
		return seedStats{}
	}
	now := time.Now().UTC()

	// When SEED_HEALTH_BANDS is on, suite-level pass rates must mirror
	// the project's band — otherwise the treemap shows a red project
	// tile that drills into green suite tiles (broken project, but
	// every suite reads as healthy). Use the same fail-fraction cap as
	// the run-level numbers; legacy 1/3 cap stays when bands are off.
	suiteFailFrac := failFractionForBand(band, cfg.HealthBands)

	suites := make([]suiteSeed, 0, len(runs)*cfg.SuitesPerRun)
	for i, r := range runs {
		runID := runIDs[i]
		for s := 0; s < cfg.SuitesPerRun; s++ {
			suiteStatus := r.Status
			if r.Status == "flaky" && rng.Intn(3) == 0 {
				suiteStatus = "passed"
			}
			specs := 5 + rng.Intn(40)
			failedSpecs := 0
			if suiteStatus == "failed" || suiteStatus == "flaky" {
				maxFail := int(float64(specs) * suiteFailFrac)
				if maxFail < 1 {
					maxFail = 1
				}
				failedSpecs = sampleNearTarget(maxFail, rng)
			}
			suiteName := cat.SuiteNames[(i*cfg.SuitesPerRun+s)%len(cat.SuiteNames)]
			runMs := r.EndTime.Sub(r.StartTime).Milliseconds()
			if runMs < 1 {
				runMs = 1
			}
			dur := time.Duration(rng.Int63n(runMs)+1) * time.Millisecond
			startT := r.StartTime.Add(time.Duration(rng.Int63n(runMs)) * time.Millisecond)
			endT := startT.Add(dur)

			var tagNames []string
			if rng.Float64() < cfg.SuiteTagRate {
				for k := 0; k < 1+rng.Intn(3); k++ {
					tagNames = append(tagNames, cat.Tags[rng.Intn(len(cat.Tags))])
				}
			}

			suites = append(suites, suiteSeed{
				runID:       runID,
				suiteName:   suiteName,
				status:      suiteStatus,
				startTime:   startT,
				endTime:     endT,
				durationMs:  dur.Milliseconds(),
				totalSpecs:  specs,
				failedSpecs: failedSpecs,
				tagNames:    tagNames,
			})
		}
	}

	suiteIter := pgx.CopyFromSlice(len(suites), func(i int) ([]any, error) {
		s := suites[i]
		return []any{
			s.runID, s.suiteName, s.status, s.startTime, s.endTime,
			s.totalSpecs, s.totalSpecs - s.failedSpecs, s.failedSpecs, 0,
			s.durationMs,
			now, now,
		}, nil
	})

	if _, err := pool.CopyFrom(ctx,
		pgx.Identifier{"suite_runs"},
		[]string{
			"test_run_id", "suite_name", "status", "start_time", "end_time",
			"total_specs", "passed_specs", "failed_specs", "skipped_specs",
			"duration_ms",
			"created_at", "updated_at",
		}, suiteIter); err != nil {
		log.Fatalf("copy suite_runs: %v", err)
	}

	suiteIDs, err := lookupSuiteIDs(ctx, pool, suites)
	if err != nil {
		log.Fatalf("lookup suite_runs ids: %v", err)
	}
	for i := range suites {
		suites[i].assignedID = suiteIDs[i]
	}

	stats := seedStats{suiteRuns: int64(len(suites))}

	type tagJunc struct{ suite, tag int64 }
	juncs := make([]tagJunc, 0)
	for _, s := range suites {
		seen := map[int64]bool{}
		for _, name := range s.tagNames {
			id, ok := tagIDs[name]
			if !ok || seen[id] {
				continue
			}
			seen[id] = true
			juncs = append(juncs, tagJunc{s.assignedID, id})
		}
	}
	if len(juncs) > 0 {
		junIter := pgx.CopyFromSlice(len(juncs), func(i int) ([]any, error) {
			return []any{juncs[i].suite, juncs[i].tag, now}, nil
		})
		if _, err := pool.CopyFrom(ctx,
			pgx.Identifier{"suite_run_tags"},
			[]string{"suite_run_id", "tag_id", "created_at"},
			junIter); err != nil {
			log.Fatalf("copy suite_run_tags: %v", err)
		}
		stats.suiteTags = int64(len(juncs))
	}

	if cfg.SpecsPerFailingSuite > 0 {
		type specRow struct {
			suiteID    int64
			name       string
			status     string
			err        string
			startTime  time.Time
			endTime    time.Time
			durationMs int64
			retry      int
			isFlaky    bool
		}
		specs := make([]specRow, 0)
		// Seed `SpecsPerFailingSuite` spec_runs *per suite_run*
		// (regardless of pass/fail outcome of the parent suite). The
		// row mix mirrors the suite's failed_specs / total_specs ratio
		// so the spec-drill treemap sees realistic per-spec pass rates,
		// not "every spec is a failure" (which was the legacy seeder's
		// behavior — failures-only — and caused project ≈ 57% / spec
		// tiles all red).
		//
		// SpecsPerFailingSuite is the cap (default 5) so volume stays
		// manageable on big seeds. True 1-row-per-spec coverage would
		// be ≈ suites * avg-25-specs which is 25× more rows.
		for _, s := range suites {
			perSuite := cfg.SpecsPerFailingSuite
			if perSuite > s.totalSpecs {
				perSuite = s.totalSpecs
			}
			if perSuite <= 0 {
				continue
			}
			failPart := computeSpecFailures(perSuite, s.totalSpecs, s.failedSpecs, rng)
			for k := 0; k < perSuite; k++ {
				isFailed := k < failPart
				status := "passed"
				errMsg := ""
				if isFailed {
					status = "failed"
					errMsg = cat.ErrorPool[(int(s.assignedID)+k)%len(cat.ErrorPool)]
				}
				specs = append(specs, specRow{
					suiteID:    s.assignedID,
					name:       cat.SpecNames[(int(s.assignedID)+k)%len(cat.SpecNames)],
					status:     status,
					err:        errMsg,
					startTime:  s.startTime,
					endTime:    s.endTime,
					durationMs: s.durationMs / int64(perSuite+1),
					retry:      rng.Intn(3),
					// Only failing specs in flaky suites get the
					// is_flaky flag — a passing spec_run in a flaky
					// suite isn't itself flaky.
					isFlaky: isFailed && s.status == "flaky",
				})
			}
		}

		if len(specs) > 0 {
			specIter := pgx.CopyFromSlice(len(specs), func(i int) ([]any, error) {
				sp := specs[i]
				return []any{
					sp.suiteID, sp.name, sp.status,
					sp.startTime, sp.endTime, sp.durationMs,
					sp.err, "", sp.retry, sp.isFlaky,
					now, now,
				}, nil
			})
			if _, err := pool.CopyFrom(ctx,
				pgx.Identifier{"spec_runs"},
				[]string{
					"suite_run_id", "spec_name", "status",
					"start_time", "end_time", "duration_ms",
					"error_message", "stack_trace", "retry_count", "is_flaky",
					"created_at", "updated_at",
				}, specIter); err != nil {
				log.Fatalf("copy spec_runs: %v", err)
			}
			stats.specRuns = int64(len(specs))
		}
	}

	return stats
}

func lookupSuiteIDs(ctx context.Context, pool *pgxpool.Pool, seeds []suiteSeed) ([]int64, error) {
	runIDset := map[int64]bool{}
	for _, s := range seeds {
		runIDset[s.runID] = true
	}
	runIDs := make([]int64, 0, len(runIDset))
	for id := range runIDset {
		runIDs = append(runIDs, id)
	}
	rows, err := pool.Query(ctx, `
		SELECT id, test_run_id
		FROM suite_runs
		WHERE test_run_id = ANY($1)
		ORDER BY id
	`, runIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byRun := map[int64][]int64{}
	for rows.Next() {
		var id, runID int64
		if err := rows.Scan(&id, &runID); err != nil {
			return nil, err
		}
		byRun[runID] = append(byRun[runID], id)
	}

	cursor := map[int64]int{}
	out := make([]int64, len(seeds))
	for i, s := range seeds {
		idx := cursor[s.runID]
		ids := byRun[s.runID]
		if idx >= len(ids) {
			return nil, fmt.Errorf("missing suite id for run %d index %d", s.runID, idx)
		}
		out[i] = ids[idx]
		cursor[s.runID] = idx + 1
	}
	return out, nil
}

// ---- Small helpers ---------------------------------------------------------

func pickStatus(rng *mathrand.Rand) string {
	r := rng.Intn(100)
	switch {
	case r < 75:
		return "passed"
	case r < 95:
		return "failed"
	default:
		return "flaky"
	}
}

// failFractionForBand caps the fraction of tests that fail when a run
// is marked failed/flaky. With bands off, the legacy 1/5 cap is used.
// With bands on, the cap scales with band: broken projects fail nearly
// everything in a failing run; rock-solid projects fail only a sliver.
func failFractionForBand(band int, bandsEnabled bool) float64 {
	if !bandsEnabled {
		return 0.20 // legacy: 1 + rng.Intn(total/5)
	}
	switch band {
	case 0:
		return 0.90
	case 1:
		return 0.65
	case 2:
		return 0.35
	case 3:
		return 0.12
	default:
		return 0.03
	}
}

// pickStatusForBand returns a run status weighted by the project's
// health band. See healthBandFor for band semantics.
func pickStatusForBand(rng *mathrand.Rand, band int) string {
	r := rng.Intn(100)
	// Each band's thresholds for (passed, failed). Anything ≥ failed
	// threshold becomes flaky.
	var passThresh, failThresh int
	switch band {
	case 0: // broken
		passThresh, failThresh = 5, 95
	case 1: // struggling
		passThresh, failThresh = 25, 90
	case 2: // flaky
		passThresh, failThresh = 55, 85
	case 3: // healthy
		passThresh, failThresh = 85, 95
	default: // 4: rock solid
		passThresh, failThresh = 98, 99
	}
	switch {
	case r < passThresh:
		return "passed"
	case r < failThresh:
		return "failed"
	default:
		return "flaky"
	}
}

func pickEnv(rng *mathrand.Rand) string {
	envs := []string{"ci", "ci", "ci", "ci", "staging", "dev"}
	return envs[rng.Intn(len(envs))]
}

func fakeSHA(rng *mathrand.Rand) string {
	var b [20]byte
	if _, err := rand.Read(b[:]); err != nil {
		for i := range b {
			b[i] = byte(rng.Intn(256))
		}
	}
	return hex.EncodeToString(b[:])[:12]
}

func shortBase36(n int) string {
	if n == 0 {
		return "0"
	}
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	out := ""
	for n > 0 {
		out = string(alphabet[n%36]) + out
		n /= 36
	}
	return out
}

func humanCount(n int) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return strconv.Itoa(n)
	}
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
