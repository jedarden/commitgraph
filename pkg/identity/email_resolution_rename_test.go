// Package identity tests email_resolution updates during GitHub login rename detection.
//
// This test suite verifies that when the revalidation worker detects a GitHub login rename,
// the email_resolution table is correctly updated to reflect the new login while preserving
// user_id continuity to prevent silent fragmentation of a developer's history.
package identity

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/jedarden/commitgraph/pkg/client/github"
)

// TestEmailResolutionLoginUpdateOnRename verifies that email_resolution.login is updated
// when the revalidation worker detects a GitHub login rename via the mock GitHub client.
//
// This test covers edge case #6 from docs/plan/plan.md:
// "Login renamed after resolution — email_resolution now points at a dead login."
//
// Test scenario:
// 1. Seed email_resolution with dev@example.com -> old-name (from cg-4vkqn pattern)
// 2. Configure mock GitHub client to return rename: old-name -> new-name (from cg-4rhpp)
// 3. Simulate revalidation worker detecting rename via mock client
// 4. Update email_resolution.login to new-name
// 5. Verify email_resolution.login is updated correctly
// 6. Verify no duplicate rows are created (idempotency)
func TestEmailResolutionLoginUpdateOnRename(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("Skipping integration test: TEST_DB_URL environment variable not set")
	}

	ctx := context.Background()
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	// Setup cleanup handler to ensure test database is cleaned up
	t.Cleanup(func() {
		cleanupEmailResolutionTestTables(ctx, db)
	})

	// Setup test tables
	if err := setupEmailResolutionTestTables(ctx, db); err != nil {
		t.Fatalf("Failed to setup test tables: %v", err)
	}

	// Run the actual test
	t.Run("rename updates email_resolution.login", func(t *testing.T) {
		testEmailResolutionRenameUpdate(ctx, t, db)
	})

	t.Run("rename preserves user_id continuity", func(t *testing.T) {
		testEmailResolutionRenamePreservesUserID(ctx, t, db)
	})

	t.Run("idempotent rename handling", func(t *testing.T) {
		testEmailResolutionRenameIdempotency(ctx, t, db)
	})
}

// setupEmailResolutionTestTables creates temporary tables for email_resolution rename testing.
func setupEmailResolutionTestTables(ctx context.Context, db *sql.DB) error {
	queries := []string{
		`CREATE TEMPORARY TABLE users (
			user_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			login TEXT NOT NULL UNIQUE,
			profile_url TEXT,
			avatar_url TEXT
		)`,
		`CREATE TEMPORARY TABLE repos (
			repo_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			provider TEXT NOT NULL,
			repo_full_name TEXT NOT NULL,
			UNIQUE (provider, repo_full_name)
		)`,
		`CREATE TEMPORARY TABLE repo_user_daily_tool (
			repo_id BIGINT NOT NULL,
			user_id BIGINT NOT NULL,
			tool TEXT NOT NULL,
			day DATE NOT NULL,
			commits INT NOT NULL,
			insert_time TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (repo_id, user_id, tool, day)
		)`,
		`CREATE TEMPORARY TABLE email_resolution (
			email TEXT PRIMARY KEY,
			login TEXT NOT NULL,
			source TEXT NOT NULL,
			resolved_at TIMESTAMPTZ NOT NULL
		)`,
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("create table failed: %w", err)
		}
	}
	return nil
}

// cleanupEmailResolutionTestTables drops temporary test tables.
func cleanupEmailResolutionTestTables(ctx context.Context, db *sql.DB) {
	tables := []string{"repo_user_daily_tool", "repos", "users", "email_resolution"}
	for _, table := range tables {
		_, _ = db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
	}
}

// testEmailResolutionRenameUpdate verifies the basic email_resolution update flow.
func testEmailResolutionRenameUpdate(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	email := "test@example.com"
	oldLogin := "old-name"
	newLogin := "new-name"

	// Seed email_resolution row (from cg-4vkqn pattern)
	resolvedAt := time.Now().UTC().Add(-24 * time.Hour)
	_, err := db.ExecContext(ctx,
		`INSERT INTO email_resolution (email, login, source, resolved_at) VALUES ($1, $2, $3, $4)`,
		email, oldLogin, "seed", resolvedAt)
	if err != nil {
		t.Fatalf("Failed to seed email_resolution: %v", err)
	}

	// Create mock GitHub client (from cg-4rhpp)
	mockClient := github.NewMockClient()
	mockClient.SetResponse(oldLogin, &github.LoginResult{
		Status:   github.StatusRenamed,
		NewLogin: &newLogin,
	})

	// Simulate revalidation worker detecting rename
	result, err := mockClient.CheckLogin(ctx, oldLogin)
	if err != nil {
		t.Fatalf("Mock client CheckLogin failed: %v", err)
	}
	if result.Status != github.StatusRenamed {
		t.Fatalf("Expected StatusRenamed, got %s", result.Status)
	}
	if result.NewLogin == nil || *result.NewLogin != newLogin {
		t.Fatalf("Expected new_login=%s, got %v", newLogin, result.NewLogin)
	}

	// Update email_resolution (simulating queue-api call)
	now := time.Now()
	_, err = db.ExecContext(ctx, `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO UPDATE
		  SET login = excluded.login,
		      source = excluded.source,
		      resolved_at = excluded.resolved_at
		  WHERE excluded.source = 'manual'
		     OR (email_resolution.source <> 'manual'
		         AND excluded.resolved_at > email_resolution.resolved_at)
	`, email, newLogin, "live", now)
	if err != nil {
		t.Fatalf("Failed to update email_resolution: %v", err)
	}

	// Verify email_resolution.login is updated
	var updatedLogin string
	err = db.QueryRowContext(ctx, `SELECT login FROM email_resolution WHERE email = $1`, email).Scan(&updatedLogin)
	if err != nil {
		t.Fatalf("Failed to query updated email_resolution: %v", err)
	}
	if updatedLogin != newLogin {
		t.Errorf("Expected login=%s, got %s", newLogin, updatedLogin)
	}

	t.Logf("✓ email_resolution.login updated: %s -> %s", oldLogin, newLogin)
}

// testEmailResolutionRenamePreservesUserID verifies that user_id is preserved during rename.
func testEmailResolutionRenamePreservesUserID(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	email := "user@example.com"
	oldLogin := "old-user"
	newLogin := "new-user"

	// Seed user with old login
	var userID int64
	err := db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, oldLogin).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}

	// Seed email_resolution
	_, err = db.ExecContext(ctx,
		`INSERT INTO email_resolution (email, login, source, resolved_at) VALUES ($1, $2, $3, $4)`,
		email, oldLogin, "seed", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Failed to seed email_resolution: %v", err)
	}

	// Update email_resolution and users.login
	now := time.Now()
	_, err = db.ExecContext(ctx, `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO UPDATE
		  SET login = excluded.login,
		      source = excluded.source,
		      resolved_at = excluded.resolved_at
		  WHERE excluded.source = 'manual'
		     OR (email_resolution.source <> 'manual'
		         AND excluded.resolved_at > email_resolution.resolved_at)
	`, email, newLogin, "live", now)
	if err != nil {
		t.Fatalf("Failed to update email_resolution: %v", err)
	}

	_, err = db.ExecContext(ctx, `UPDATE users SET login = $1 WHERE login = $2`, newLogin, oldLogin)
	if err != nil {
		t.Fatalf("Failed to update users.login: %v", err)
	}

	// Verify user_id is preserved
	var updatedUserID int64
	var updatedLogin string
	err = db.QueryRowContext(ctx, `SELECT user_id, login FROM users WHERE user_id = $1`, userID).Scan(&updatedUserID, &updatedLogin)
	if err != nil {
		t.Fatalf("Failed to query user: %v", err)
	}
	if updatedUserID != userID {
		t.Errorf("user_id not preserved: expected %d, got %d", userID, updatedUserID)
	}
	if updatedLogin != newLogin {
		t.Errorf("login not updated: expected %s, got %s", newLogin, updatedLogin)
	}

	t.Logf("✓ user_id preserved during rename: %d", userID)
}

// testEmailResolutionRenameIdempotency verifies that repeated rename operations don't create duplicates.
func testEmailResolutionRenameIdempotency(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	email := "idempotent@example.com"
	oldLogin := "old-login"
	newLogin := "new-login"

	// Seed email_resolution
	_, err := db.ExecContext(ctx,
		`INSERT INTO email_resolution (email, login, source, resolved_at) VALUES ($1, $2, $3, $4)`,
		email, oldLogin, "seed", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Failed to seed email_resolution: %v", err)
	}

	// Perform the same update twice (simulating duplicate processing)
	updateQuery := `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO UPDATE
		  SET login = excluded.login,
		      source = excluded.source,
		      resolved_at = excluded.resolved_at
		  WHERE excluded.source = 'manual'
		     OR (email_resolution.source <> 'manual'
		         AND excluded.resolved_at > email_resolution.resolved_at)
	`

	// First update
	_, err = db.ExecContext(ctx, updateQuery, email, newLogin, "live", time.Now())
	if err != nil {
		t.Fatalf("First update failed: %v", err)
	}

	// Second update (should be idempotent)
	_, err = db.ExecContext(ctx, updateQuery, email, newLogin, "live", time.Now())
	if err != nil {
		t.Fatalf("Second update failed: %v", err)
	}

	// Verify only one row exists
	var count int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_resolution WHERE email = $1`, email).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 row, got %d (duplicate created!)", count)
	}

	// Verify login is still new-login
	var login string
	err = db.QueryRowContext(ctx, `SELECT login FROM email_resolution WHERE email = $1`, email).Scan(&login)
	if err != nil {
		t.Fatalf("Failed to query login: %v", err)
	}
	if login != newLogin {
		t.Errorf("Expected login=%s, got %s", newLogin, login)
	}

	t.Logf("✓ Idempotent rename handling verified")
}
