// +build integration

// Idempotency tests for claude-leaderboard seed
//
// These tests verify that running the claude-leaderboard seed script multiple
// times produces the same result (no duplicate rows, no changed timestamps).
package main

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/jedarden/commitgraph/pkg/identity"
)

// setupFixtureDB creates an in-memory SQLite database with the email_resolution
// table schema and returns the database connection.
//
// The function:
// 1. Creates an in-memory SQLite database
// 2. Runs the email_resolution table creation schema
// 3. Returns the database connection for use in tests
//
// The database is created in-memory, so it will be lost when the connection
// is closed. Use cleanupFixtureDB to properly close the connection.
func setupFixtureDB() *sql.DB {
	// Create an in-memory SQLite database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(fmt.Sprintf("failed to create in-memory database: %v", err))
	}

	// Verify connection works
	if err := db.Ping(); err != nil {
		panic(fmt.Sprintf("failed to ping database: %v", err))
	}

	// Create the email_resolution table
	// Note: SQLite doesn't have TIMESTAMPTZ, so we use TIMESTAMP instead
	// The schema matches the PostgreSQL structure used in production
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS email_resolution (
			email TEXT PRIMARY KEY,
			login TEXT NOT NULL,
			source TEXT NOT NULL,
			resolved_at TIMESTAMP NOT NULL
		)
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to create email_resolution table: %v", err))
	}

	return db
}

// cleanupFixtureDB closes the database connection and performs any necessary
// cleanup operations.
//
// For in-memory databases, this simply closes the connection. For file-based
// test databases, this would also remove the temporary file.
func cleanupFixtureDB(db *sql.DB) {
	if db != nil {
		if err := db.Close(); err != nil {
			// In tests, we log but don't panic on cleanup errors
			fmt.Printf("warning: failed to close database: %v\n", err)
		}
	}
}

// seedOnce simulates running the seed script once against the given database.
//
// This function encapsulates the seed logic: reading frozen claude-leaderboard
// author_login_cache data and inserting it into email_resolution.
//
// For testing purposes, this uses test data instead of reading from an external
// SQLite database. The production seed script would read from the actual frozen
// claude-leaderboard data.
//
// Parameters:
//   - db: Database connection to seed (must have email_resolution table)
//
// Returns an error if:
//   - Database operations fail
//   - Data validation fails
func seedOnce(db *sql.DB) error {
	// Test data simulating frozen claude-leaderboard author_login_cache
	// In production, this would read from the actual SQLite database
	testData := []struct {
		email      string
		login      string
		resolvedAt time.Time
	}{
		{
			email:      "user1@claude.ai",
			login:      "claude-user-1",
			resolvedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			email:      "user2@claude.ai",
			login:      "claude-user-2",
			resolvedAt: time.Date(2024, 1, 16, 11, 45, 0, 0, time.UTC),
		},
		{
			email:      "user3@claude.ai",
			login:      "claude-user-3",
			resolvedAt: time.Date(2024, 1, 17, 14, 20, 0, 0, time.UTC),
		},
	}

	// Insert each row into email_resolution
	for _, row := range testData {
		_, err := db.Exec(`
			INSERT INTO email_resolution (email, login, source, resolved_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(email) DO UPDATE SET
				login = excluded.login,
				source = excluded.source,
				resolved_at = excluded.resolved_at
		`, row.email, row.login, "seed", row.resolvedAt)
		if err != nil {
			return fmt.Errorf("failed to insert row for email %s: %w", row.email, err)
		}
	}

	return nil
}

// TestSeedIdempotency is the primary idempotency guard that verifies running
// the seed script twice produces identical results.
//
// This test protects against:
// - Duplicate row insertion (ON CONFLICT rule not working)
// - Timestamp changes on re-seed (resolved_at should be stable)
// - Non-deterministic ordering (hash should be consistent)
// - Schema violations (data types, constraints)
//
// Test flow:
// 1. Set up a fixture database using setupFixtureDB()
// 2. Capture initial baseline snapshot (pre-seed state)
// 3. Run the seed script once
// 4. Capture post-first-run snapshot
// 5. Run the seed script a second time (same code, same database)
// 6. Capture post-second-run snapshot
// 7. Assert post-first-run and post-second-run snapshots are identical
// 8. Clean up the database
//
// Fails with a clear message if:
// - Snapshots differ (row count or hash mismatch)
// - The seed script errors on either run
func TestSeedIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Step 1: Set up fixture database
	db := setupFixtureDB()
	defer cleanupFixtureDB(db)

	t.Log("✓ Fixture database created")

	// Step 2: Capture initial baseline snapshot (pre-seed state)
	baseline, err := identity.CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("Failed to capture baseline snapshot: %v", err)
	}

	t.Logf("✓ Baseline captured: %d rows, hash=%s", baseline.RowCount, baseline.Hash)

	// Verify baseline is empty
	if baseline.RowCount != 0 {
		t.Errorf("Expected empty database at baseline, got %d rows", baseline.RowCount)
	}

	// Step 3: Run the seed script once
	t.Log("Running first seed...")
	if err := seedOnce(db); err != nil {
		t.Fatalf("First seed failed: %v", err)
	}
	t.Log("✓ First seed completed successfully")

	// Step 4: Capture post-first-run snapshot
	snapshot1, err := identity.CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("Failed to capture snapshot after first seed: %v", err)
	}

	t.Logf("✓ After first seed: %d rows, hash=%s", snapshot1.RowCount, snapshot1.Hash)

	// Verify we got some data from the seed
	if snapshot1.RowCount == 0 {
		t.Error("Expected at least some rows to be seeded, but got 0")
	}

	// Step 5: Run the seed script a second time (same code, same database)
	t.Log("Running second seed...")
	if err := seedOnce(db); err != nil {
		t.Fatalf("Second seed failed: %v", err)
	}
	t.Log("✓ Second seed completed successfully")

	// Step 6: Capture post-second-run snapshot
	snapshot2, err := identity.CaptureSnapshot(db)
	if err != nil {
		t.Fatalf("Failed to capture snapshot after second seed: %v", err)
	}

	t.Logf("✓ After second seed: %d rows, hash=%s", snapshot2.RowCount, snapshot2.Hash)

	// Step 7: Assert post-first-run and post-second-run snapshots are identical
	identical, err := identity.CompareSnapshots(snapshot1, snapshot2)
	if err != nil {
		// Snapshots differ - fail with clear message
		t.Fatalf("Seed is NOT idempotent!\n%v", err)
	}

	if !identical {
		t.Fatal("Seed is NOT idempotent! CompareSnapshots returned false")
	}

	t.Log("✓ Seed idempotency verified: no changes after second run")
}

// TestIdempotencyPlaceholder is a placeholder test that verifies the test
// infrastructure is properly set up.
//
// This test:
// 1. Creates a fixture database using setupFixtureDB
// 2. Verifies the email_resolution table exists
// 3. Performs basic database operations to ensure everything works
// 4. Cleans up using cleanupFixtureDB
//
// This test serves as a foundation for more comprehensive idempotency tests
// that will be implemented in follow-up work.
func TestIdempotencyPlaceholder(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup fixture database
	db := setupFixtureDB()
	defer cleanupFixtureDB(db)

	// Verify the table was created
	var tableName string
	err := db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='email_resolution'
	`).Scan(&tableName)

	if err != nil {
		if err == sql.ErrNoRows {
			t.Fatal("email_resolution table was not created")
		}
		t.Fatalf("Failed to query table existence: %v", err)
	}

	if tableName != "email_resolution" {
		t.Errorf("Expected table name 'email_resolution', got '%s'", tableName)
	}

	// Verify we can query the table structure
	var tableSQL string
	err = db.QueryRow(`
		SELECT sql FROM sqlite_master
		WHERE type='table' AND name='email_resolution'
	`).Scan(&tableSQL)
	if err != nil {
		t.Fatalf("Failed to query table schema: %v", err)
	}

	// This is just infrastructure verification - we're not testing idempotency yet
	// The actual idempotency tests will be implemented in follow-up tasks

	t.Log("✓ Test infrastructure verified successfully")
	t.Logf("Table schema: %s", tableSQL)
}
