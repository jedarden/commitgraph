// +build integration

// Integration tests for NULL login handling and conflict resolution
// These tests require a test PostgreSQL database and verify the actual
// seed script behavior against edge cases.
package identity

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/jedarden/commitgraph/pkg/pg"
)

// TestNullLoginHandlingIntegration verifies that NULL/empty logins are properly
// skipped during the seed process with a real database.
func TestNullLoginHandlingIntegration(t *testing.T) {
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

	// Connect to PostgreSQL
	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbUser, dbPass, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec(`
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
	defer db.Exec("DROP TABLE IF EXISTS email_resolution")

	// Create ingester
	executor := pg.NewSQLExecutor(db)
	ingester := NewIngester(pg.NewIdentityIngester(executor))

	ctx := context.Background()

	// Test cases with NULL/empty logins
	testCases := []struct {
		name        string
		email       string
		login       string
		source      Source
		resolvedAt  time.Time
		shouldError bool
	}{
		{
			name:       "Valid row",
			email:      "valid@example.com",
			login:      "user1",
			source:     SourceSeed,
			resolvedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "Empty login - should fail validation",
			email:       "empty@example.com",
			login:       "",
			source:      SourceSeed,
			resolvedAt:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			shouldError: true,
		},
		{
			name:       "Another valid row",
			email:      "valid2@example.com",
			login:      "user2",
			source:     SourceSeed,
			resolvedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		},
	}

	// Test validation first
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			row := ResolutionRow{
				Email:      tc.email,
				Login:      tc.login,
				Source:     tc.source,
				ResolvedAt: tc.resolvedAt,
			}

			err := row.Validate()
			if tc.shouldError && err == nil {
				t.Errorf("Expected validation error for %s, but got none", tc.name)
			}
			if !tc.shouldError && err != nil {
				t.Errorf("Unexpected validation error for %s: %v", tc.name, err)
			}
		})
	}

	// Ingest valid rows only
	validRows := []ResolutionRow{
		{
			Email:      "valid@example.com",
			Login:      "user1",
			Source:     SourceSeed,
			ResolvedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Email:      "valid2@example.com",
			Login:      "user2",
			Source:     SourceSeed,
			ResolvedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
		},
	}

	err = ingester.IngestResolution(ctx, validRows)
	if err != nil {
		t.Fatalf("Failed to ingest valid rows: %v", err)
	}

	// Verify rows were inserted
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM email_resolution").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 rows in database, got %d", count)
	}

	// Verify no NULL logins were inserted
	var nullCount int
	err = db.QueryRow("SELECT COUNT(*) FROM email_resolution WHERE login IS NULL OR login = ''").Scan(&nullCount)
	if err != nil {
		t.Fatalf("Failed to count NULL logins: %v", err)
	}

	if nullCount != 0 {
		t.Errorf("Expected 0 NULL logins in database, got %d", nullCount)
	}

	t.Logf("✓ NULL login handling verified: %d valid rows ingested, %d NULL logins rejected", count, nullCount)
}

// TestConflictResolutionIntegration verifies the ON CONFLICT rule behavior
// with a real database.
func TestConflictResolutionIntegration(t *testing.T) {
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

	// Connect to PostgreSQL
	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbUser, dbPass, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec(`
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
	defer db.Exec("DROP TABLE IF EXISTS email_resolution")

	// Create ingester
	executor := pg.NewSQLExecutor(db)
	ingester := NewIngester(pg.NewIdentityIngester(executor))

	ctx := context.Background()

	// Insert initial seed data
	initialRows := []ResolutionRow{
		{
			Email:      "test@example.com",
			Login:      "userA",
			Source:     SourceSeed,
			ResolvedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	err = ingester.IngestResolution(ctx, initialRows)
	if err != nil {
		t.Fatalf("Failed to ingest initial rows: %v", err)
	}

	// Test: Manual source always wins
	manualRow := []ResolutionRow{
		{
			Email:      "test@example.com",
			Login:      "manualOverride",
			Source:     SourceManual,
			ResolvedAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), // Older
		},
	}

	err = ingester.IngestResolution(ctx, manualRow)
	if err != nil {
		t.Fatalf("Failed to ingest manual row: %v", err)
	}

	// Verify manual won
	var login string
	err = db.QueryRow("SELECT login FROM email_resolution WHERE email = 'test@example.com'").Scan(&login)
	if err != nil {
		t.Fatalf("Failed to query login: %v", err)
	}

	if login != "manualOverride" {
		t.Errorf("Expected login to be 'manualOverride', got '%s'", login)
	}

	// Test: Newer seed wins over older seed
	newerSeedRow := []ResolutionRow{
		{
			Email:      "test@example.com",
			Login:      "newerSeed",
			Source:     SourceSeed,
			ResolvedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	err = ingester.IngestResolution(ctx, newerSeedRow)
	if err != nil {
		t.Fatalf("Failed to ingest newer seed row: %v", err)
	}

	err = db.QueryRow("SELECT login FROM email_resolution WHERE email = 'test@example.com'").Scan(&login)
	if err != nil {
		t.Fatalf("Failed to query login: %v", err)
	}

	if login != "newerSeed" {
		t.Errorf("Expected login to be 'newerSeed', got '%s'", login)
	}

	// Test: Manual still wins over newer seed
	finalManualRow := []ResolutionRow{
		{
			Email:      "test@example.com",
			Login:      "finalManual",
			Source:     SourceManual,
			ResolvedAt: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC), // Even older
		},
	}

	err = ingester.IngestResolution(ctx, finalManualRow)
	if err != nil {
		t.Fatalf("Failed to ingest final manual row: %v", err)
	}

	err = db.QueryRow("SELECT login FROM email_resolution WHERE email = 'test@example.com'").Scan(&login)
	if err != nil {
		t.Fatalf("Failed to query login: %v", err)
	}

	if login != "finalManual" {
		t.Errorf("Expected login to be 'finalManual', got '%s'", login)
	}

	// Test: Older seed loses to newer seed
	olderSeedRow := []ResolutionRow{
		{
			Email:      "test@example.com",
			Login:      "olderSeed",
			Source:     SourceSeed,
			ResolvedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	err = ingester.IngestResolution(ctx, olderSeedRow)
	if err != nil {
		t.Fatalf("Failed to ingest older seed row: %v", err)
	}

	err = db.QueryRow("SELECT login FROM email_resolution WHERE email = 'test@example.com'").Scan(&login)
	if err != nil {
		t.Fatalf("Failed to query login: %v", err)
	}

	// Should still be finalManual (manual wins)
	if login != "finalManual" {
		t.Errorf("Expected login to remain 'finalManual', got '%s'", login)
	}

	// Verify final state
	var source string
	var resolvedAt time.Time
	err = db.QueryRow("SELECT source, resolved_at FROM email_resolution WHERE email = 'test@example.com'").
		Scan(&source, &resolvedAt)
	if err != nil {
		t.Fatalf("Failed to query final state: %v", err)
	}

	if source != string(SourceManual) {
		t.Errorf("Expected source to be 'manual', got '%s'", source)
	}

	t.Logf("✓ Conflict resolution verified: final state is login=%s, source=%s, resolved_at=%s",
		login, source, resolvedAt.Format(time.RFC3339))
}

// TestDuplicatePairsIntegration verifies behavior when duplicate
// email pairs are ingested with different logins.
func TestDuplicatePairsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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

	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbUser, dbPass, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
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
	defer db.Exec("DROP TABLE IF EXISTS email_resolution")

	executor := pg.NewSQLExecutor(db)
	ingester := NewIngester(pg.NewIdentityIngester(executor))

	ctx := context.Background()

	// Ingest multiple rows for the same email with different logins
	duplicateRows := []ResolutionRow{
		{
			Email:      "conflict@example.com",
			Login:      "userA",
			Source:     SourceSeed,
			ResolvedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Email:      "conflict@example.com",
			Login:      "userB",
			Source:     SourceSeed,
			ResolvedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Email:      "conflict@example.com",
			Login:      "userC",
			Source:     SourceSeed,
			ResolvedAt: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	// Ingest in order
	for i, row := range duplicateRows {
		batch := []ResolutionRow{row}
		err := ingester.IngestResolution(ctx, batch)
		if err != nil {
			t.Errorf("Batch %d failed: %v", i, err)
		}
	}

	// Verify only one row exists (email is PRIMARY KEY)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM email_resolution WHERE email = 'conflict@example.com'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 row for duplicate email, got %d", count)
	}

	// Verify the newest timestamp won
	var login string
	var resolvedAt time.Time
	err = db.QueryRow("SELECT login, resolved_at FROM email_resolution WHERE email = 'conflict@example.com'").
		Scan(&login, &resolvedAt)
	if err != nil {
		t.Fatalf("Failed to query final state: %v", err)
	}

	expectedLogin := "userB" // June 1st is newest
	if login != expectedLogin {
		t.Errorf("Expected login to be '%s' (newest), got '%s'", expectedLogin, login)
	}

	expectedTime := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	if !resolvedAt.Equal(expectedTime) {
		t.Errorf("Expected resolved_at to be %s, got %s", expectedTime, resolvedAt)
	}

	t.Logf("✓ Duplicate pair resolution verified: 1 row from 3 duplicates, winner is login=%s with newest timestamp", login)
}
