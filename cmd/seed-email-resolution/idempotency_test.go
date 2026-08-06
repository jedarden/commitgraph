// +build integration

// Idempotency tests for seed-email-resolution
//
// These tests verify that running the claude-leaderboard seed script multiple
// times produces the same result (no duplicate rows, no changed timestamps).
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/jedarden/commitgraph/pkg/identity"
	"github.com/jedarden/commitgraph/pkg/pg"
)

// snapshot captures the state of email_resolution for comparison
type snapshot struct {
	rowCount int
	hash     string // SHA-256 hash of all concatenated row data
}

// captureSnapshot reads all rows from email_resolution and returns a snapshot
func captureSnapshot(db *sql.DB) (snapshot, error) {
	rows, err := db.Query(`
		SELECT email, login, source, resolved_at
		FROM email_resolution
		ORDER BY email, login, source, resolved_at
	`)
	if err != nil {
		return snapshot{}, fmt.Errorf("failed to query snapshot: %w", err)
	}
	defer rows.Close()

	var allData []byte
	count := 0

	for rows.Next() {
		var email, login, source string
		var resolvedAt time.Time

		if err := rows.Scan(&email, &login, &source, &resolvedAt); err != nil {
			return snapshot{}, fmt.Errorf("failed to scan row: %w", err)
		}

		// Concatenate row data for hashing
		// Format: email|login|source|resolved_at\n
		rowStr := fmt.Sprintf("%s|%s|%s|%s\n", email, login, source, resolvedAt.Format(time.RFC3339Nano))
		allData = append(allData, []byte(rowStr)...)
		count++
	}

	if err := rows.Err(); err != nil {
		return snapshot{}, fmt.Errorf("row iteration error: %w", err)
	}

	hash := sha256.Sum256(allData)
	return snapshot{
		rowCount: count,
		hash:     hex.EncodeToString(hash[:]),
	}, nil
}

// seedOnce runs the seed script logic once against the given databases
func seedOnce(ctx context.Context, pgDB *sql.DB, sqliteDBPath string) error {
	// Open SQLite database
	seedDB, err := sql.Open("sqlite3", sqliteDBPath)
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}
	defer seedDB.Close()

	// Read all rows from author_login_cache
	sqliteRows, err := seedDB.Query("SELECT author_email, github_login, resolved_at FROM author_login_cache")
	if err != nil {
		return fmt.Errorf("failed to query author_login_cache: %w", err)
	}
	defer sqliteRows.Close()

	var allRows []identity.ResolutionRow

	for sqliteRows.Next() {
		var email, login, resolvedAtStr string

		if err := sqliteRows.Scan(&email, &login, &resolvedAtStr); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Skip rows with empty login (no negative-cache seeding)
		if login == "" {
			continue
		}

		// Parse timestamp
		resolvedAt, err := time.Parse(time.RFC3339Nano, resolvedAtStr)
		if err != nil {
			return fmt.Errorf("failed to parse resolved_at %q for email %s: %w",
				resolvedAtStr, email, err)
		}

		allRows = append(allRows, identity.ResolutionRow{
			Email:      email,
			Login:      login,
			Source:     identity.SourceSeed,
			ResolvedAt: resolvedAt,
		})
	}

	if err := sqliteRows.Err(); err != nil {
		return fmt.Errorf("rows iteration failed: %w", err)
	}

	// Create identity ingester
	ingester := identity.NewIngester(pg.NewIdentityIngester(pg.NewSQLExecutor(pgDB)))

	// Ingest all rows
	if err := ingester.IngestResolution(ctx, allRows); err != nil {
		return fmt.Errorf("failed to ingest rows: %w", err)
	}

	return nil
}

// TestSeedIdempotency verifies that running the seed twice produces no changes
func TestSeedIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Get PostgreSQL connection details from environment
	dbHost := os.Getenv("PGHOST")
	dbUser := os.Getenv("PGUSER")
	dbPass := os.Getenv("PGPASSWORD")
	dbName := os.Getenv("PGDATABASE")

	if dbHost == "" || dbUser == "" || dbPass == "" {
		t.Skip("Set PGHOST, PGUSER, PGPASSWORD to run integration test")
	}
	if dbName == "" {
		dbName = "commitgraph_test"
	}

	// Get path to test data SQLite database
	testDataDir := filepath.Join(".", "testdata")
	sqliteDBPath := filepath.Join(testDataDir, "sample.db")

	if _, err := os.Stat(sqliteDBPath); os.IsNotExist(err) {
		t.Fatalf("Test SQLite database not found: %s", sqliteDBPath)
	}

	// Connect to PostgreSQL
	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbUser, dbPass, dbName)
	pgDB, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgDB.Close()

	// Create a fresh test table
	_, err = pgDB.Exec(`
		CREATE TABLE IF NOT EXISTS email_resolution (
			email TEXT PRIMARY KEY,
			login TEXT NOT NULL,
			source TEXT NOT NULL,
			resolved_at TIMESTAMPTZ NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	// Clean up after test
	defer pgDB.Exec("DROP TABLE IF EXISTS email_resolution")

	ctx := context.Background()

	// First seed run
	t.Log("Running first seed...")
	if err := seedOnce(ctx, pgDB, sqliteDBPath); err != nil {
		t.Fatalf("First seed failed: %v", err)
	}

	// Capture snapshot after first run
	snapshot1, err := captureSnapshot(pgDB)
	if err != nil {
		t.Fatalf("Failed to capture snapshot after first seed: %v", err)
	}

	t.Logf("After first seed: %d rows, hash=%s", snapshot1.rowCount, snapshot1.hash)

	// Verify we got at least some rows from the sample data
	if snapshot1.rowCount == 0 {
		t.Error("Expected at least some rows to be seeded, but got 0")
	}

	// Second seed run (should be idempotent)
	t.Log("Running second seed...")
	if err := seedOnce(ctx, pgDB, sqliteDBPath); err != nil {
		t.Fatalf("Second seed failed: %v", err)
	}

	// Capture snapshot after second run
	snapshot2, err := captureSnapshot(pgDB)
	if err != nil {
		t.Fatalf("Failed to capture snapshot after second seed: %v", err)
	}

	t.Logf("After second seed: %d rows, hash=%s", snapshot2.rowCount, snapshot2.hash)

	// Assert the two snapshots are identical
	if snapshot1.rowCount != snapshot2.rowCount {
		t.Errorf("Row count changed: first=%d, second=%d", snapshot1.rowCount, snapshot2.rowCount)
	}

	if snapshot1.hash != snapshot2.hash {
		t.Errorf("Data hash changed - seed is NOT idempotent!\n  First:  %s\n  Second: %s",
			snapshot1.hash, snapshot2.hash)
	}

	t.Log("✓ Seed idempotency verified: no changes after second run")
}

// TestSeedWithLiveResolution verifies that a live resolution with newer
// resolved_at is preserved when seed runs again
func TestSeedWithLiveResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Get PostgreSQL connection details from environment
	dbHost := os.Getenv("PGHOST")
	dbUser := os.Getenv("PGUSER")
	dbPass := os.Getenv("PGPASSWORD")
	dbName := os.Getenv("PGDATABASE")

	if dbHost == "" || dbUser == "" || dbPass == "" {
		t.Skip("Set PGHOST, PGUSER, PGPASSWORD to run integration test")
	}
	if dbName == "" {
		dbName = "commitgraph_test"
	}

	// Get path to test data SQLite database
	testDataDir := filepath.Join(".", "testdata")
	sqliteDBPath := filepath.Join(testDataDir, "sample.db")

	if _, err := os.Stat(sqliteDBPath); os.IsNotExist(err) {
		t.Fatalf("Test SQLite database not found: %s", sqliteDBPath)
	}

	// Connect to PostgreSQL
	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbUser, dbPass, dbName)
	pgDB, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgDB.Close()

	// Create a fresh test table
	_, err = pgDB.Exec(`
		CREATE TABLE IF NOT EXISTS email_resolution (
			email TEXT PRIMARY KEY,
			login TEXT NOT NULL,
			source TEXT NOT NULL,
			resolved_at TIMESTAMPTZ NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	defer pgDB.Exec("DROP TABLE IF EXISTS email_resolution")

	ctx := context.Background()

	// First seed run
	t.Log("Running first seed...")
	if err := seedOnce(ctx, pgDB, sqliteDBPath); err != nil {
		t.Fatalf("First seed failed: %v", err)
	}

	// Pick one email from the seeded data to test with
	var testEmail, testLogin, testSource string
	var testResolvedAt time.Time
	err = pgDB.QueryRow(`
		SELECT email, login, source, resolved_at
		FROM email_resolution
		WHERE source = 'seed'
		LIMIT 1
	`).Scan(&testEmail, &testLogin, &testSource, &testResolvedAt)
	if err != nil {
		t.Fatalf("Failed to query a seeded row: %v", err)
	}

	t.Logf("Using test email: %s (login=%s, source=%s, resolved_at=%s)",
		testEmail, testLogin, testSource, testResolvedAt.Format(time.RFC3339Nano))

	// Add a live resolution for the same email with a NEWER resolved_at
	newerResolvedAt := testResolvedAt.Add(24 * time.Hour) // 1 day newer
	liveLogin := "live-" + testLogin

	ingester := identity.NewIngester(pg.NewIdentityIngester(pg.NewSQLExecutor(pgDB)))
	liveRow := []identity.ResolutionRow{
		{
			Email:      testEmail,
			Login:      liveLogin,
			Source:     identity.SourceLive,
			ResolvedAt: newerResolvedAt,
		},
	}

	t.Logf("Adding live resolution: login=%s, resolved_at=%s (newer than seed)",
		liveLogin, newerResolvedAt.Format(time.RFC3339Nano))

	if err := ingester.IngestResolution(ctx, liveRow); err != nil {
		t.Fatalf("Failed to ingest live resolution: %v", err)
	}

	// Verify the live resolution won
	var currentLogin, currentSource string
	var currentResolvedAt time.Time
	err = pgDB.QueryRow(`
		SELECT login, source, resolved_at
		FROM email_resolution
		WHERE email = $1
	`, testEmail).Scan(&currentLogin, &currentSource, &currentResolvedAt)
	if err != nil {
		t.Fatalf("Failed to query after live ingest: %v", err)
	}

	if currentLogin != liveLogin {
		t.Errorf("Expected login to be '%s' after live ingest, got '%s'", liveLogin, currentLogin)
	}
	if currentSource != string(identity.SourceLive) {
		t.Errorf("Expected source to be 'live' after live ingest, got '%s'", currentSource)
	}

	// Second seed run (should NOT overwrite the newer live resolution)
	t.Log("Running second seed...")
	if err := seedOnce(ctx, pgDB, sqliteDBPath); err != nil {
		t.Fatalf("Second seed failed: %v", err)
	}

	// Verify the live resolution is still there (preserved by conflict rule)
	err = pgDB.QueryRow(`
		SELECT login, source, resolved_at
		FROM email_resolution
		WHERE email = $1
	`, testEmail).Scan(&currentLogin, &currentSource, &currentResolvedAt)
	if err != nil {
		t.Fatalf("Failed to query after second seed: %v", err)
	}

	if currentLogin != liveLogin {
		t.Errorf("Expected login to remain '%s' (live), got '%s' - seed overwrote live!",
			liveLogin, currentLogin)
	}
	if currentSource != string(identity.SourceLive) {
		t.Errorf("Expected source to remain 'live', got '%s'", currentSource)
	}
	if !currentResolvedAt.Equal(newerResolvedAt) {
		t.Errorf("Expected resolved_at to remain %s, got %s",
			newerResolvedAt.Format(time.RFC3339Nano),
			currentResolvedAt.Format(time.RFC3339Nano))
	}

	t.Log("✓ Live resolution preserved: seed did not overwrite newer live data")
}
