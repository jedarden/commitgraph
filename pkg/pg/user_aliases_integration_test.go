package pg

// Integration tests for AliasIngester.GetAdminAliases against a real PostgreSQL
// instance via testcontainers-go. Unlike the unit tests in user_aliases_test.go
// which use mockDBExecutor and check SQL text patterns, these tests exercise
// the real SQL against a real database and verify the complete query execution.

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupUserAliasesTestDB starts a real Postgres container, applies the
// user_aliases schema, and returns an open *sql.DB plus a cleanup function.
func setupUserAliasesTestDB(t *testing.T) (*sql.DB, func()) {
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
		CREATE TABLE IF NOT EXISTS user_aliases (
			source_login TEXT NOT NULL PRIMARY KEY,
			target_login TEXT NOT NULL,
			reason TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
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

// TestGetAdminAliases_Integration verifies GetAdminAliases returns the correct
// mapping of source_login -> target_login for rows with reason='admin', excluding
// rows with other reasons.
func TestGetAdminAliases_Integration(t *testing.T) {
	db, cleanup := setupUserAliasesTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Insert test data: some admin aliases, some non-admin aliases
	testData := []struct {
		sourceLogin string
		targetLogin string
		reason      string
	}{
		{"old-johndoe", "johndoe", "admin"},
		{"jane-bot", "jane", "admin"},
		{"deprecated-bob", "robert", "admin"},
		{"name-match-alice", "alice", "name-match"}, // Should be excluded
		{"old-charlie", "charles", "name-match"},   // Should be excluded
	}

	for _, row := range testData {
		_, err := db.ExecContext(ctx, `
			INSERT INTO user_aliases (source_login, target_login, reason, created_at)
			VALUES ($1, $2, $3, $4)
		`, row.sourceLogin, row.targetLogin, row.reason, time.Now().UTC())
		if err != nil {
			t.Fatalf("failed to insert test data: %v", err)
		}
	}

	// Call GetAdminAliases
	ingester := NewAliasIngester(db)
	aliases, err := ingester.GetAdminAliases(ctx)
	if err != nil {
		t.Fatalf("GetAdminAliases failed: %v", err)
	}

	// Verify we got exactly the 3 admin aliases
	if len(aliases) != 3 {
		t.Errorf("expected 3 admin aliases, got %d: %#v", len(aliases), aliases)
	}

	// Verify each admin alias is present with correct target
	expectedAdminAliases := map[string]string{
		"old-johndoe":    "johndoe",
		"jane-bot":       "jane",
		"deprecated-bob": "robert",
	}

	for source, expectedTarget := range expectedAdminAliases {
		actualTarget, ok := aliases[source]
		if !ok {
			t.Errorf("missing admin alias for source %q", source)
		} else if actualTarget != expectedTarget {
			t.Errorf("wrong target for source %q: got %q, want %q", source, actualTarget, expectedTarget)
		}
	}

	// Verify non-admin aliases are NOT present
	if _, ok := aliases["name-match-alice"]; ok {
		t.Error("name-match-alice should not be in admin aliases")
	}
	if _, ok := aliases["old-charlie"]; ok {
		t.Error("old-charlie should not be in admin aliases")
	}
}

// TestGetAdminAliases_Empty_Integration verifies GetAdminAliases returns an
// empty map when no admin aliases exist in the database.
func TestGetAdminAliases_Empty_Integration(t *testing.T) {
	db, cleanup := setupUserAliasesTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Don't insert any data - database is empty

	ingester := NewAliasIngester(db)
	aliases, err := ingester.GetAdminAliases(ctx)
	if err != nil {
		t.Fatalf("GetAdminAliases failed: %v", err)
	}

	// Should return empty (non-nil) map
	if aliases == nil {
		t.Fatal("expected non-nil map for empty result")
	}
	if len(aliases) != 0 {
		t.Errorf("expected 0 aliases, got %d: %#v", len(aliases), aliases)
	}
}

// TestGetAdminAliases_OnlyNonAdmin_Integration verifies GetAdminAliases returns
// an empty map when only non-admin aliases exist.
func TestGetAdminAliases_OnlyNonAdmin_Integration(t *testing.T) {
	db, cleanup := setupUserAliasesTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Insert only non-admin aliases
	_, err := db.ExecContext(ctx, `
		INSERT INTO user_aliases (source_login, target_login, reason, created_at)
		VALUES ($1, $2, $3, $4)
	`, "name-match-alice", "alice", "name-match", time.Now().UTC())
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	ingester := NewAliasIngester(db)
	aliases, err := ingester.GetAdminAliases(ctx)
	if err != nil {
		t.Fatalf("GetAdminAliases failed: %v", err)
	}

	// Should return empty (non-nil) map
	if aliases == nil {
		t.Fatal("expected non-nil map for empty result")
	}
	if len(aliases) != 0 {
		t.Errorf("expected 0 aliases, got %d: %#v", len(aliases), aliases)
	}
}

// TestUpsertAliasesAndDeleteAdminAliases_Integration verifies the full workflow:
// upsert admin aliases, retrieve them via GetAdminAliases, delete some, and verify
// the updated list.
func TestUpsertAliasesAndDeleteAdminAliases_Integration(t *testing.T) {
	db, cleanup := setupUserAliasesTestDB(t)
	defer cleanup()
	ctx := context.Background()

	ingester := NewAliasIngester(db)

	// Step 1: Upsert admin aliases
	now := time.Now().UTC()
	rows := []AliasRow{
		{SourceLogin: "old-johndoe", TargetLogin: "johndoe", Reason: "admin", CreatedAt: now},
		{SourceLogin: "jane-bot", TargetLogin: "jane", Reason: "admin", CreatedAt: now},
		{SourceLogin: "deprecated-bob", TargetLogin: "robert", Reason: "admin", CreatedAt: now},
	}

	err := ingester.UpsertAliases(ctx, rows)
	if err != nil {
		t.Fatalf("UpsertAliases failed: %v", err)
	}

	// Step 2: Verify all are present via GetAdminAliases
	aliases, err := ingester.GetAdminAliases(ctx)
	if err != nil {
		t.Fatalf("GetAdminAliases failed: %v", err)
	}

	if len(aliases) != 3 {
		t.Errorf("expected 3 admin aliases after upsert, got %d", len(aliases))
	}

	// Step 3: Delete two of them
	deleted, err := ingester.DeleteAdminAliases(ctx, []string{"old-johndoe", "jane-bot"})
	if err != nil {
		t.Fatalf("DeleteAdminAliases failed: %v", err)
	}

	if deleted != 2 {
		t.Errorf("expected to delete 2 rows, got %d", deleted)
	}

	// Step 4: Verify only one remains
	aliases, err = ingester.GetAdminAliases(ctx)
	if err != nil {
		t.Fatalf("GetAdminAliases failed after delete: %v", err)
	}

	if len(aliases) != 1 {
		t.Errorf("expected 1 admin alias after delete, got %d: %#v", len(aliases), aliases)
	}

	// Verify it's the correct one
	if _, ok := aliases["deprecated-bob"]; !ok {
		t.Error("expected deprecated-bob to remain, but it was deleted")
	}

	// Verify deleted ones are gone
	if _, ok := aliases["old-johndoe"]; ok {
		t.Error("old-johndoe should have been deleted")
	}
	if _, ok := aliases["jane-bot"]; ok {
		t.Error("jane-bot should have been deleted")
	}
}
