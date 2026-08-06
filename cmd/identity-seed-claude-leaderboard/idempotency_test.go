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

	_ "github.com/mattn/go-sqlite3"
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
