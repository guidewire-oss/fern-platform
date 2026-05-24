package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	mathrand "math/rand"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedExtras populates the models that the core seedRuns flow doesn't
// touch: users, groups, scopes, permissions, jira connections, flaky
// tests, saved views, user preferences. Kept in a separate file so the
// hot path in main.go stays focused on bulk-loading test_runs +
// suite_runs + spec_runs at scale.
//
// All inserts here are small (≤ a few thousand rows total even on the
// 500-project seed), so we use plain INSERT rather than CopyFrom — the
// SQL is more readable and the perf overhead is irrelevant at this
// scale.
//
// Idempotency: each step uses ON CONFLICT DO NOTHING (or a delete-then-
// insert pattern) so re-running the seeder without SEED_TRUNCATE=true
// keeps things stable.
func seedExtras(
	ctx context.Context,
	pool *pgxpool.Pool,
	cfg seedConfig,
	projectIDs []string,
	projectCats []Category,
	tagIDs map[string]int64,
) extrasStats {
	_ = tagIDs // reserved for future spec_run_tags coverage
	var s extrasStats

	rng := mathrand.New(mathrand.NewSource(0xfe72)) // deterministic across reruns

	// Sample fleet — small fixed set, covers all three roles plus a
	// couple of group memberships per user so the role-derivation logic
	// has data to chew on.
	users := sampleUsers()
	s.users = seedUsers(ctx, pool, users)
	s.userGroups = seedUserGroups(ctx, pool, users)
	s.userPrefs = seedUserPreferences(ctx, pool, users, projectIDs, rng)
	s.userScopes = seedUserScopes(ctx, pool, users, projectIDs, projectCats, rng)
	s.projectPerms = seedProjectPermissions(ctx, pool, users, projectIDs, projectCats, rng)
	s.savedViews = seedSavedViews(ctx, pool, users, rng)
	s.jiraConns = seedJiraConnections(ctx, pool, projectIDs, projectCats, rng)
	s.flakyTests = seedFlakyTests(ctx, pool, cfg, projectIDs, projectCats, rng)

	return s
}

type extrasStats struct {
	users        int64
	userGroups   int64
	userPrefs    int64
	userScopes   int64
	projectPerms int64
	savedViews   int64
	jiraConns    int64
	flakyTests   int64
}

// ---- Users -----------------------------------------------------------------

type sampleUser struct {
	UserID, Email, Name, FirstName, LastName, Role, ProfileURL string
	Groups                                                     []string
}

// sampleUsers returns a hardcoded fleet covering the three roles plus
// representative group memberships. Keeping these stable means demo
// screens look identical across re-seeds.
func sampleUsers() []sampleUser {
	return []sampleUser{
		{
			UserID:    "dev-admin",
			Email:     "dev-admin@fern.local",
			Name:      "Dev Admin",
			FirstName: "Dev",
			LastName:  "Admin",
			Role:      "admin",
			Groups:    []string{"pod-capitola", "fern-platform-admins"},
		},
		{
			UserID:    "ada-lovelace",
			Email:     "ada@example.com",
			Name:      "Ada Lovelace",
			FirstName: "Ada",
			LastName:  "Lovelace",
			Role:      "admin",
			Groups:    []string{"pod-capitola", "platform-team"},
		},
		{
			UserID:    "grace-hopper",
			Email:     "grace@example.com",
			Name:      "Grace Hopper",
			FirstName: "Grace",
			LastName:  "Hopper",
			Role:      "manager",
			Groups:    []string{"core-services-team", "core-services-managers"},
		},
		{
			UserID:    "linus-torvalds",
			Email:     "linus@example.com",
			Name:      "Linus Torvalds",
			FirstName: "Linus",
			LastName:  "Torvalds",
			Role:      "manager",
			Groups:    []string{"infra-team", "infra-managers"},
		},
		{
			UserID:    "barbara-liskov",
			Email:     "barbara@example.com",
			Name:      "Barbara Liskov",
			FirstName: "Barbara",
			LastName:  "Liskov",
			Role:      "user",
			Groups:    []string{"core-services-team"},
		},
		{
			UserID:    "donald-knuth",
			Email:     "donald@example.com",
			Name:      "Donald Knuth",
			FirstName: "Donald",
			LastName:  "Knuth",
			Role:      "user",
			Groups:    []string{"infra-team"},
		},
		{
			UserID:    "alan-turing",
			Email:     "alan@example.com",
			Name:      "Alan Turing",
			FirstName: "Alan",
			LastName:  "Turing",
			Role:      "user",
			Groups:    []string{"core-services-team"},
		},
		{
			UserID:    "margaret-hamilton",
			Email:     "margaret@example.com",
			Name:      "Margaret Hamilton",
			FirstName: "Margaret",
			LastName:  "Hamilton",
			Role:      "user",
			Groups:    []string{"flight-software-team"},
		},
	}
}

func seedUsers(ctx context.Context, pool *pgxpool.Pool, users []sampleUser) int64 {
	now := time.Now().UTC()
	for _, u := range users {
		if _, err := pool.Exec(ctx, `
			INSERT INTO users
				(user_id, email, name, first_name, last_name, role, status,
				 email_verified, profile_url, last_login_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'active', true, $7, $8, $8, $8)
			ON CONFLICT (user_id) DO UPDATE SET
				email = EXCLUDED.email,
				name = EXCLUDED.name,
				first_name = EXCLUDED.first_name,
				last_name = EXCLUDED.last_name,
				role = EXCLUDED.role,
				updated_at = EXCLUDED.updated_at
		`, u.UserID, u.Email, u.Name, u.FirstName, u.LastName, u.Role,
			u.ProfileURL, now); err != nil {
			log.Printf("seed-extras: upsert user %s: %v (continuing)", u.UserID, err)
			continue
		}
	}
	return int64(len(users))
}

func seedUserGroups(ctx context.Context, pool *pgxpool.Pool, users []sampleUser) int64 {
	// Replace rather than dedupe — simpler than merging.
	if _, err := pool.Exec(ctx, `DELETE FROM user_groups`); err != nil {
		log.Fatalf("clear user_groups: %v", err)
	}
	now := time.Now().UTC()
	var rows int64
	for _, u := range users {
		for _, g := range u.Groups {
			if _, err := pool.Exec(ctx, `
				INSERT INTO user_groups (user_id, group_name, created_at, updated_at)
				VALUES ($1, $2, $3, $3)
			`, u.UserID, g, now); err != nil {
				log.Printf("seed-extras: insert user_group %s/%s: %v (continuing)", u.UserID, g, err)
				continue
			}
			rows++
		}
	}
	return rows
}

// ---- User preferences (per-seeded-user) ------------------------------------

func seedUserPreferences(
	ctx context.Context,
	pool *pgxpool.Pool,
	users []sampleUser,
	projectIDs []string,
	rng *mathrand.Rand,
) int64 {
	themes := []string{"light", "dark", "system"}
	tzs := []string{"UTC", "America/Los_Angeles", "America/New_York", "Asia/Kolkata", "Europe/London"}
	var rows int64
	now := time.Now().UTC()
	for _, u := range users {
		// Each user favorites ~3 random projects (or all of them when
		// there are fewer than 3). Stable per user by seeding rng with
		// the user_id length so re-seeds give the same favorites.
		favs := pickRandomProjectIDs(projectIDs, 3, rng)
		favsJSON, _ := json.Marshal(favs)
		if _, err := pool.Exec(ctx, `
			INSERT INTO user_preferences
				(user_id, theme, timezone, language, favorites, preferences,
				 created_at, updated_at)
			VALUES ($1, $2, $3, 'en', $4::jsonb, '{}'::jsonb, $5, $5)
			ON CONFLICT (user_id) DO UPDATE SET
				theme = EXCLUDED.theme,
				timezone = EXCLUDED.timezone,
				favorites = EXCLUDED.favorites,
				updated_at = EXCLUDED.updated_at
		`, u.UserID, themes[rng.Intn(len(themes))], tzs[rng.Intn(len(tzs))],
			string(favsJSON), now); err != nil {
			log.Printf("seed-extras: upsert user_preferences %s: %v (continuing)", u.UserID, err)
			continue
		}
		rows++
	}
	return rows
}

// ---- User scopes -----------------------------------------------------------

func seedUserScopes(
	ctx context.Context,
	pool *pgxpool.Pool,
	users []sampleUser,
	projectIDs []string,
	_ []Category,
	rng *mathrand.Rand,
) int64 {
	if _, err := pool.Exec(ctx, `DELETE FROM user_scopes`); err != nil {
		log.Fatalf("clear user_scopes: %v", err)
	}
	now := time.Now().UTC()
	var rows int64
	// Plain `user`-role users get write access to ~2 individual projects each.
	// Skips users who are already admin/manager (their roles grant them
	// broad access via the role check, separate from scopes).
	for _, u := range users {
		if u.Role != "user" {
			continue
		}
		picks := pickRandomProjectIDs(projectIDs, 2, rng)
		for _, pid := range picks {
			scope := fmt.Sprintf("project:write:%s", pid)
			// user_scopes has created_at/updated_at but NOT granted_at —
			// migration 16 only added granted_at to project_permissions.
			if _, err := pool.Exec(ctx, `
				INSERT INTO user_scopes
					(user_id, scope, granted_by, created_at, updated_at)
				VALUES ($1, $2, 'dev-admin', $3, $3)
				ON CONFLICT (user_id, scope) DO NOTHING
			`, u.UserID, scope, now); err != nil {
				log.Printf("seed-extras: insert user_scope %s/%s: %v (continuing)", u.UserID, scope, err)
				continue
			}
			rows++
		}
	}
	return rows
}

// ---- Project permissions (team-style team-grant rows) ---------------------

func seedProjectPermissions(
	ctx context.Context,
	pool *pgxpool.Pool,
	users []sampleUser,
	projectIDs []string,
	_ []Category,
	rng *mathrand.Rand,
) int64 {
	if _, err := pool.Exec(ctx, `DELETE FROM project_permissions`); err != nil {
		log.Fatalf("clear project_permissions: %v", err)
	}
	now := time.Now().UTC()
	var rows int64
	// Pick ~10% of projects and grant explicit read permission to two
	// non-admin users on each. Demonstrates per-project ACLs in the UI.
	sample := pickRandomProjectIDs(projectIDs, len(projectIDs)/10+1, rng)
	nonAdmin := []sampleUser{}
	for _, u := range users {
		if u.Role != "admin" {
			nonAdmin = append(nonAdmin, u)
		}
	}
	if len(nonAdmin) == 0 {
		return 0
	}
	for _, pid := range sample {
		picks := 2
		if picks > len(nonAdmin) {
			picks = len(nonAdmin)
		}
		for j := 0; j < picks; j++ {
			u := nonAdmin[(j+rng.Intn(len(nonAdmin)))%len(nonAdmin)]
			perm := "read"
			if u.Role == "manager" {
				perm = "write"
			}
			if _, err := pool.Exec(ctx, `
				INSERT INTO project_permissions
					(project_id, user_id, permission, granted_by, granted_at,
					 created_at, updated_at)
				VALUES ($1, $2, $3, 'dev-admin', $4, $4, $4)
			`, pid, u.UserID, perm, now); err != nil {
				// project_permissions has no UNIQUE constraint matching this
				// in the migration; collisions are unlikely but tolerate them.
				if !isUniqueViolation(err) {
					log.Printf("seed-extras: insert project_permissions: %v (continuing)", err)
					continue
				}
				continue
			}
			rows++
		}
	}
	return rows
}

// ---- Saved views (filter-bar bookmarks) -----------------------------------

func seedSavedViews(
	ctx context.Context,
	pool *pgxpool.Pool,
	users []sampleUser,
	_ *mathrand.Rand,
) int64 {
	if _, err := pool.Exec(ctx, `DELETE FROM saved_views`); err != nil {
		log.Fatalf("clear saved_views: %v", err)
	}
	now := time.Now().UTC()
	var rows int64
	// One generic "failures-only" view per user, and a separate
	// "my recent runs" for managers + admins. JSON shape matches the
	// /api/v2/test-runs filter contract.
	for _, u := range users {
		views := []struct {
			page, name string
			filter     map[string]any
		}{
			{"test-runs", "Failed runs", map[string]any{"status": []string{"failed"}, "days": 7}},
		}
		if u.Role == "manager" || u.Role == "admin" {
			views = append(views, struct {
				page, name string
				filter     map[string]any
			}{"test-runs", "Last 24h", map[string]any{"days": 1}})
		}
		for _, v := range views {
			body, _ := json.Marshal(v.filter)
			if _, err := pool.Exec(ctx, `
				INSERT INTO saved_views (user_id, page, name, filter_json, created_at, updated_at)
				VALUES ($1, $2, $3, $4::jsonb, $5, $5)
				ON CONFLICT (user_id, page, name) DO NOTHING
			`, u.UserID, v.page, v.name, string(body), now); err != nil {
				log.Printf("seed-extras: insert saved_view %s/%s: %v (continuing)", u.UserID, v.name, err)
				continue
			}
			rows++
		}
	}
	return rows
}

// ---- Jira connections -----------------------------------------------------

func seedJiraConnections(
	ctx context.Context,
	pool *pgxpool.Pool,
	projectIDs []string,
	projectCats []Category,
	rng *mathrand.Rand,
) int64 {
	if _, err := pool.Exec(ctx, `DELETE FROM jira_connections`); err != nil {
		log.Fatalf("clear jira_connections: %v", err)
	}
	now := time.Now().UTC()
	var rows int64
	// Attach a Jira connection to ~30% of projects. The encrypted
	// credential is a placeholder string — the Jira API isn't actually
	// reached from seeded data; status='pending' reflects that.
	for i, pid := range projectIDs {
		if rng.Intn(10) >= 3 {
			continue
		}
		cat := projectCats[i]
		key := strings.ToUpper(cat.Slug)
		if len(key) > 8 {
			key = key[:8]
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO jira_connections
				(project_id, name, jira_url, authentication_type, project_key,
				 username, encrypted_credential, status, is_active,
				 created_at, updated_at)
			VALUES ($1, $2, 'https://jira.example.com', 'api_token', $3,
				'seed@example.com', 'placeholder-credential', 'pending', false,
				$4, $4)
		`, pid, fmt.Sprintf("%s Jira", pid), key, now); err != nil {
			log.Printf("seed-extras: insert jira_connection %s: %v (continuing)", pid, err)
			continue
		}
		rows++
	}
	return rows
}

// ---- Flaky tests ----------------------------------------------------------

func seedFlakyTests(
	ctx context.Context,
	pool *pgxpool.Pool,
	cfg seedConfig,
	projectIDs []string,
	_ []Category,
	rng *mathrand.Rand,
) int64 {
	if _, err := pool.Exec(ctx, `DELETE FROM flaky_tests`); err != nil {
		log.Fatalf("clear flaky_tests: %v", err)
	}
	now := time.Now().UTC()
	var rows int64
	// For each project, generate 0-3 flaky-test records. The window
	// (first_seen → last_seen) is anchored inside the seed-data range
	// so dashboard queries that filter by date catch them.
	for _, pid := range projectIDs {
		n := rng.Intn(4)
		for i := 0; i < n; i++ {
			flakeRate := 0.05 + rng.Float64()*0.4 // 5% – 45%
			totalExec := 100 + rng.Intn(1000)
			flakyExec := int(float64(totalExec) * flakeRate)
			lastSeen := now.Add(-time.Duration(rng.Intn(cfg.Days*24)) * time.Hour)
			firstSeen := lastSeen.Add(-time.Duration(rng.Intn(cfg.Days*24)+1) * time.Hour)
			severity := pickSeverity(flakeRate)
			testName := fmt.Sprintf("FlakyTest%d_%s", i, pid)
			suiteName := fmt.Sprintf("Suite%d", i+1)
			errMsg := "intermittent failure: timeout waiting for async state"

			if _, err := pool.Exec(ctx, `
				INSERT INTO flaky_tests
					(project_id, test_name, suite_name, flake_rate,
					 total_executions, flaky_executions, first_seen_at,
					 last_seen_at, status, severity, last_error_message,
					 created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active', $9, $10, $11, $11)
				ON CONFLICT (project_id, test_name, suite_name) DO NOTHING
			`, pid, testName, suiteName, flakeRate, totalExec, flakyExec,
				firstSeen, lastSeen, severity, errMsg, now); err != nil {
				log.Printf("seed-extras: insert flaky_test %s/%s: %v (continuing)", pid, testName, err)
				continue
			}
			rows++
		}
	}
	return rows
}

func pickSeverity(rate float64) string {
	switch {
	case rate >= 0.30:
		return "critical"
	case rate >= 0.20:
		return "high"
	case rate >= 0.10:
		return "medium"
	default:
		return "low"
	}
}

// ---- helpers --------------------------------------------------------------

func pickRandomProjectIDs(projects []string, n int, rng *mathrand.Rand) []string {
	if len(projects) == 0 {
		return nil
	}
	if n >= len(projects) {
		out := make([]string, len(projects))
		copy(out, projects)
		return out
	}
	// Fisher-Yates over a working copy — small n, fine to allocate.
	work := make([]string, len(projects))
	copy(work, projects)
	for i := 0; i < n; i++ {
		j := i + rng.Intn(len(work)-i)
		work[i], work[j] = work[j], work[i]
	}
	return work[:n]
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pgconn.PgError encodes Code=23505 on unique-violation; depend
	// only on the string to keep the cmd/seed module from dragging in
	// pgconn just for this branch.
	return strings.Contains(err.Error(), "23505")
}
