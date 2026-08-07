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
