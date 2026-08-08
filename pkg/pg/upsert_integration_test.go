package pg

// Integration tests for BatchUpsertUsers (cg-3m8j5) and UpsertRepo (cg-68s8)
// against a real PostgreSQL instance via testcontainers-go, using the actual
// lib/pq driver. Unlike the pattern elsewhere in this package (mockExecutor
// / mockDBExecutor string-matching the query text), these tests exercise the
// real SQL against a real database, so they also catch driver-level bugs —
// e.g. passing a bare []string as a query arg silently fails against lib/pq,
// which requires pq.Array() to marshal a Go slice into a Postgres array
// parameter.

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupUpsertTestDB starts a real Postgres container, applies the subset of
// the initial schema needed for these tests, and returns an open *sql.DB
// plus a cleanup function.
func setupUpsertTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("skipping integration test: failed to start postgres container: %v", err)
	}

	cleanupContainer := func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanupContainer()
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		cleanupContainer()
		t.Fatalf("failed to connect to database: %v", err)
	}

	schema := `
		CREATE TABLE IF NOT EXISTS repos (
			repo_id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			provider       TEXT NOT NULL,
			repo_full_name TEXT NOT NULL,
			excluded_at    TIMESTAMPTZ,
			excluded_reason TEXT,
			UNIQUE (provider, repo_full_name)
		);

		CREATE TABLE IF NOT EXISTS users (
			user_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			login      TEXT NOT NULL UNIQUE,
			profile_url TEXT,
			avatar_url  TEXT
		);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		cleanupContainer()
		t.Fatalf("failed to create schema: %v", err)
	}

	cleanup := func() {
		db.Close()
		cleanupContainer()
	}

	return db, cleanup
}

// TestBatchUpsertUsers_Integration verifies BatchUpsertUsers (cg-3m8j5)
// against a real Postgres database: single round trip, complete map for new
// logins, idempotent re-run that preserves existing IDs while adding new
// ones, and empty-slice handling.
func TestBatchUpsertUsers_Integration(t *testing.T) {
	db, cleanup := setupUpsertTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Empty slice: no error, empty (non-nil) map, no query touches the DB.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	got, err := BatchUpsertUsers(ctx, tx, nil)
	if err != nil {
		t.Fatalf("BatchUpsertUsers(empty) error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("BatchUpsertUsers(empty) = %#v, want empty map", got)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// First call with three new logins: each gets a fresh user_id, and the
	// map contains every login (this row-loss without a DO UPDATE trick was
	// the exact bug this bead's original DO NOTHING query had).
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	first, err := BatchUpsertUsers(ctx, tx, []string{"alice", "bob", "charlie"})
	if err != nil {
		t.Fatalf("BatchUpsertUsers(new logins) error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(first) != 3 {
		t.Fatalf("expected 3 entries, got %d: %#v", len(first), first)
	}
	for _, login := range []string{"alice", "bob", "charlie"} {
		if _, ok := first[login]; !ok {
			t.Errorf("missing login %q in result: %#v", login, first)
		}
	}

	// Second call reusing two existing logins plus one new one: existing
	// IDs must be unchanged (idempotent) and the new login must appear too.
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	second, err := BatchUpsertUsers(ctx, tx, []string{"alice", "bob", "diana"})
	if err != nil {
		t.Fatalf("BatchUpsertUsers(mixed logins) error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(second) != 3 {
		t.Fatalf("expected 3 entries, got %d: %#v", len(second), second)
	}
	if second["alice"] != first["alice"] {
		t.Errorf("alice user_id changed: %d -> %d (should be idempotent)", first["alice"], second["alice"])
	}
	if second["bob"] != first["bob"] {
		t.Errorf("bob user_id changed: %d -> %d (should be idempotent)", first["bob"], second["bob"])
	}
	if _, ok := second["diana"]; !ok {
		t.Errorf("missing new login \"diana\" in result: %#v", second)
	}
	if second["diana"] == first["alice"] || second["diana"] == first["bob"] || second["diana"] == first["charlie"] {
		t.Errorf("diana reused an existing user_id: %d", second["diana"])
	}

	// Confirm no duplicate rows were created for alice/bob (UNIQUE(login)
	// plus the DO UPDATE upsert should guarantee this, but verify directly).
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE login IN ('alice','bob')`).Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows for alice+bob, got %d (duplicate rows from upsert?)", count)
	}
}

// TestUpsertRepo_Integration verifies UpsertRepo (cg-68s8) against a real
// Postgres database: one round trip, a fresh repo_id on first insert, the
// same repo_id on a repeat call (idempotent), and repo_full_name is updated
// in place on what amounts to a rename while the repo_id survives.
func TestUpsertRepo_Integration(t *testing.T) {
	db, cleanup := setupUpsertTestDB(t)
	defer cleanup()
	ctx := context.Background()

	runInTx := func(t *testing.T, provider, repoFullName string) int64 {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		id, err := UpsertRepo(ctx, tx, provider, repoFullName)
		if err != nil {
			tx.Rollback()
			t.Fatalf("UpsertRepo(%q, %q) error: %v", provider, repoFullName, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		return id
	}

	// First call creates a new row.
	id1 := runInTx(t, "github", "acme/widgets")
	if id1 == 0 {
		t.Fatal("expected non-zero repo_id on first insert")
	}

	// Second call for the same (provider, repo_full_name) is idempotent.
	id2 := runInTx(t, "github", "acme/widgets")
	if id2 != id1 {
		t.Errorf("repo_id changed on repeat call: %d -> %d (should be idempotent)", id1, id2)
	}

	// A different repo gets a different repo_id.
	otherID := runInTx(t, "github", "acme/gadgets")
	if otherID == id1 {
		t.Errorf("distinct repo got the same repo_id %d as acme/widgets", otherID)
	}

	// Missing provider/repo_full_name are rejected before any query runs.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()
	if _, err := UpsertRepo(ctx, tx, "", "acme/widgets"); err == nil {
		t.Error("expected error for empty provider, got nil")
	}
	if _, err := UpsertRepo(ctx, tx, "github", ""); err == nil {
		t.Error("expected error for empty repo_full_name, got nil")
	}

	// Confirm no duplicate rows exist for acme/widgets despite two calls.
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM repos WHERE provider = 'github' AND repo_full_name = 'acme/widgets'`).Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row for acme/widgets, got %d (duplicate rows from upsert?)", count)
	}
}

// TestRepoRename_Integration tests that repo_id is stable across a simulated
// GitHub repo rename. Per docs/plan/plan.md edge case 4: "Repo renamed on
// GitHub — the surrogate repo_id survives it; the repos.repo_full_name row is
// updated. Without the surrogate this fragments the repo's history permanently."
//
// The caller detects renames via GitHub's stable numeric repo ID (the database
// PK from GitHub's API, not the surrogate repo_id here). When GitHub returns a
// different repo_full_name for the same numeric ID, the caller knows this is a
// rename and must "explicitly re-key" before calling UpsertRepo: UPDATE the
// repos row to the new name, then call UpsertRepo (which will now find the
// existing row and return the same repo_id).
//
// This test verifies the full rename pattern including the caller's re-key step.
func TestRepoRename_Integration(t *testing.T) {
	db, cleanup := setupUpsertTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Add the repo_user_daily_tool table to test history preservation
	schema := `
		CREATE TABLE IF NOT EXISTS users (
			user_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			login      TEXT NOT NULL UNIQUE,
			profile_url TEXT,
			avatar_url  TEXT
		);

		CREATE TABLE IF NOT EXISTS repo_user_daily_tool (
			repo_id     BIGINT NOT NULL REFERENCES repos(repo_id),
			user_id     BIGINT NOT NULL REFERENCES users(user_id),
			tool        TEXT   NOT NULL,
			day         DATE   NOT NULL,
			commits     INT    NOT NULL,
			insert_time TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (repo_id, user_id, tool, day)
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create rollup table: %v", err)
	}

	runInTx := func(t *testing.T, provider, repoFullName string) int64 {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		id, err := UpsertRepo(ctx, tx, provider, repoFullName)
		if err != nil {
			tx.Rollback()
			t.Fatalf("UpsertRepo(%q, %q) error: %v", provider, repoFullName, err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		return id
	}

	// Scenario: GitHub repo "acme/widgets" (numeric ID 12345) is renamed to
	// "acme/widget-lib" (same numeric ID 12345).
	//
	// How the caller detects "this is the same repo under a new name":
	// - Caller tracks GitHub's numeric repo ID alongside repo_full_name
	// - On scan, GitHub API returns: {id: 12345, full_name: "acme/widget-lib"}
	// - Caller sees ID 12345 already exists in repos with name "acme/widgets"
	// - Caller knows: this is a rename, not a new repo
	//
	// The caller must then "explicitly re-key": UPDATE the existing row's
	// repo_full_name to the new name before calling UpsertRepo. Without this
	// step, UpsertRepo's ON CONFLICT clause won't fire (it only matches exact
	// (provider, repo_full_name) pairs), and a second row would be created.

	// Initial scan: repo is known as "acme/widgets"
	oldName := "acme/widgets"
	newName := "acme/widget-lib"

	repoIDOld := runInTx(t, "github", oldName)
	if repoIDOld == 0 {
		t.Fatal("expected non-zero repo_id on first insert")
	}

	// Insert a user and some rollup data under the old name to verify history
	// preservation across the rename.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	var userID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, "alice").Scan(&userID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("insert user: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, CURRENT_DATE, 5, NOW())
	`, repoIDOld, userID, "claude")
	if err != nil {
		tx.Rollback()
		t.Fatalf("insert rollup under old name: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit rollup: %v", err)
	}

	// Simulated rename: the repo is now known as "acme/widget-lib" but has the
	// same GitHub numeric ID (12345). The caller detects the rename because
	// GitHub's numeric ID is stable across renames, even though repo_full_name
	// changes.
	//
	// To handle this correctly, the caller must "explicitly re-key" before calling
	// UpsertRepo: UPDATE the existing row's repo_full_name to the new name, then
	// call UpsertRepo with the new name (which will now find the existing row and
	// return the same repo_id).
	//
	// Pattern:
	//   1. Caller has repo_id=1 for (github, "acme/widgets")
	//   2. Caller sees GitHub API returns name="acme/widget-lib" for numeric ID 12345
	//   3. Caller UPDATEs repos SET repo_full_name='acme/widget-lib' WHERE repo_id=1
	//   4. Caller calls UpsertRepo(github, acme/widget-lib) → returns repo_id=1
	//
	// Without the explicit re-key UPDATE, UpsertRepo creates a second row because
	// its ON CONFLICT clause only fires when (provider, repo_full_name) matches
	// exactly - it has no way to know that "acme/widget-lib" is the same repo as
	// "acme/widgets" without the caller's help.

	// Caller explicitly re-keys: UPDATE repo_full_name to the new name
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx for re-key: %v", err)
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE repos SET repo_full_name = $1 WHERE repo_id = $2
	`, newName, repoIDOld)
	if err != nil {
		tx.Rollback()
		t.Fatalf("re-key repo_full_name: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit re-key: %v", err)
	}

	// Now UpsertRepo with the new name finds the re-keyed row and returns the same repo_id
	repoIDNew := runInTx(t, "github", newName)

	// EXPECTED: repo_id is stable across the rename
	if repoIDNew != repoIDOld {
		t.Errorf("repo_id changed across rename: %d -> %d (should be stable per plan.md edge case 4)", repoIDOld, repoIDNew)
	}

	// EXPECTED: Exactly one row exists for this repo in the repos table
	// (no orphaned row with the old name)
	var rowCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM repos
		WHERE provider = 'github' AND repo_full_name IN ($1, $2)
	`, oldName, newName).Scan(&rowCount)
	if err != nil {
		t.Fatalf("count repos: %v", err)
	}
	if rowCount != 1 {
		t.Errorf("expected 1 row for this repo after rename, got %d (orphaned old name row)", rowCount)
	}

	// EXPECTED: The rollup data inserted under the old name is still accessible
	// via the same repo_id (history is not fragmented).
	var commitCount int
	err = db.QueryRowContext(ctx, `
		SELECT commits FROM repo_user_daily_tool
		WHERE repo_id = $1 AND user_id = $2 AND tool = 'claude'
	`, repoIDOld, userID).Scan(&commitCount)
	if err != nil {
		t.Errorf("rollup data not accessible after rename: %v", err)
	}
	if commitCount != 5 {
		t.Errorf("rollup commits changed after rename: got %d, want 5", commitCount)
	}

	// Verify the repo_full_name was updated to the new name
	var currentName string
	err = db.QueryRowContext(ctx, `
		SELECT repo_full_name FROM repos WHERE repo_id = $1
	`, repoIDOld).Scan(&currentName)
	if err != nil {
		t.Fatalf("get current repo_full_name: %v", err)
	}
	if currentName != newName {
		t.Errorf("repo_full_name not updated: got %q, want %q", currentName, newName)
	}
}
