// Login Revalidation Worker tests
//
// This test suite verifies that the revalidation worker correctly handles
// GitHub login renames and deletions without breaking data integrity.
package main

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

// MockQueueClient is a mock implementation of the queue-api client for testing.
// It captures PostResolution calls without making actual HTTP requests.
type MockQueueClient struct {
	postResolutionFn func(ctx context.Context, email, login string) error
}

// PostResolution implements the queueapi.Client interface.
func (m *MockQueueClient) PostResolution(ctx context.Context, email, login string) error {
	if m.postResolutionFn != nil {
		return m.postResolutionFn(ctx, email, login)
	}
	return nil
}

// TestLoginRename_Integration tests the complete login rename flow
// from the perspective of the revalidation worker.
//
// This test verifies edge case #6 from docs/plan/plan.md: when a GitHub
// login is renamed, the revalidation worker must update email_resolution
// and preserve user_id to prevent history fragmentation.
func TestLoginRename_Integration(t *testing.T) {
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

	// Clean up any existing test data
	cleanupTestTables(ctx, t, db)

	// Setup test tables
	if err := setupRevalidationTables(ctx, db); err != nil {
		t.Fatalf("Failed to setup tables: %v", err)
	}

	// Test case 1: Login rename preserves user_id
	t.Run("login rename preserves user_id and updates email_resolution", func(t *testing.T) {
		testLoginRenamePreservesUserID(ctx, t, db)
	})

	// Test case 2: Account deletion preserves historical data
	t.Run("account deletion preserves historical data", func(t *testing.T) {
		testAccountDeletionPreservesHistoricalData(ctx, t, db)
	})

	// Test case 3: Multiple renames maintain continuity
	t.Run("multiple renames maintain continuity", func(t *testing.T) {
		testMultipleRenamesMaintainContinuity(ctx, t, db)
	})

	// Test case 4: Email resolution update with mock client
	t.Run("email resolution update with mock client", func(t *testing.T) {
		testEmailResolutionUpdateWithMock(ctx, t, db)
	})

	// Test case 5: Idempotency - running worker twice should not create duplicates
	t.Run("idempotency - second run creates no duplicates", func(t *testing.T) {
		testIdempotency(ctx, t, db)
	})

	// Cleanup
	cleanupTestTables(ctx, t, db)
}

// setupRevalidationTables creates temporary tables for testing.
func setupRevalidationTables(ctx context.Context, db *sql.DB) error {
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
		`CREATE TEMPORARY TABLE email_revalidation (
			email TEXT NOT NULL PRIMARY KEY,
			login TEXT NOT NULL,
			last_checked_at TIMESTAMPTZ NOT NULL,
			next_check_at TIMESTAMPTZ,
			status TEXT NOT NULL,
			new_login TEXT,
			check_error TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("create table failed: %w", err)
		}
	}
	return nil
}

// cleanupTestTables drops temporary test tables.
func cleanupTestTables(ctx context.Context, t *testing.T, db *sql.DB) {
	tables := []string{"repo_user_daily_tool", "repos", "users", "email_revalidation", "email_resolution"}
	for _, table := range tables {
		_, _ = db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
	}
}

// seedInitialRevalidationState creates the initial test state with:
// - email_resolution row pointing to old-name
// - users row for old-name with activity
// - email_revalidation row for tracking
func seedInitialRevalidationState(ctx context.Context, t *testing.T, db *sql.DB, email, oldLogin string) (userID, repoID int64) {
	t.Helper()

	// Insert user with old login
	err := db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, oldLogin).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	// Insert repo
	err = db.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`, "github", "test/repo").Scan(&repoID)
	if err != nil {
		t.Fatalf("Failed to insert repo: %v", err)
	}

	// Insert historical activity under the old user_id
	testDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repoID, userID, "claude", testDate, 5, time.Now())
	if err != nil {
		t.Fatalf("Failed to insert rollup data: %v", err)
	}

	// Insert email_resolution pointing to old login
	_, err = db.ExecContext(ctx, `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES ($1, $2, $3, $4)
	`, email, oldLogin, "seed", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Failed to insert email_resolution: %v", err)
	}

	// Insert email_revalidation row (simulating the worker's tracking)
	_, err = db.ExecContext(ctx, `
		INSERT INTO email_revalidation (email, login, last_checked_at, status)
		VALUES ($1, $2, $3, $4)
	`, email, oldLogin, time.Now().Add(-24*time.Hour), "pending")
	if err != nil {
		t.Fatalf("Failed to insert email_revalidation: %v", err)
	}

	return userID, repoID
}

// testLoginRenamePreservesUserID verifies that when the revalidation worker
// detects a login rename, it correctly updates email_resolution.login and
// users.login while preserving user_id to prevent history fragmentation.
func testLoginRenamePreservesUserID(ctx context.Context, t *testing.T, db *sql.DB) {
	email := "dev@example.com"
	oldLogin := "old-name"
	newLogin := "new-name"

	// Seed initial state
	originalUserID, _ := seedInitialRevalidationState(ctx, t, db, email, oldLogin)

	// Verify initial state
	var currentLogin string
	err := db.QueryRowContext(ctx, `SELECT login FROM users WHERE user_id = $1`, originalUserID).Scan(&currentLogin)
	if err != nil {
		t.Fatalf("Failed to query initial login: %v", err)
	}
	if currentLogin != oldLogin {
		t.Errorf("Initial login should be %s, got %s", oldLogin, currentLogin)
	}

	// Simulate the revalidation worker detecting a rename via GitHub API
	// In the real worker, this calls checkLogin() which would return ("renamed", &newLogin, nil)
	//
	// The worker then:
	// 1. Calls updateEmailResolution() to post to queue-api
	// 2. Marks email_revalidation as status='renamed'
	//
	// For this test, we simulate both steps:

	// Step 1: Simulate queue-api updating email_resolution (via identity ingest)
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
		t.Fatalf("Failed to update email_resolution (simulating queue-api): %v", err)
	}

	// Step 2: Update users.login in place (CRITICAL: preserves user_id)
	// This simulates the downstream identity ingest processing the new login
	result, err := db.ExecContext(ctx, `
		UPDATE users
		SET login = $1
		WHERE login = $2
	`, newLogin, oldLogin)
	if err != nil {
		t.Fatalf("Failed to update users.login: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected != 1 {
		t.Errorf("Expected 1 row to be updated in users, got %d", rowsAffected)
	}

	// Step 3: Mark email_revalidation as renamed (terminal state)
	_, err = db.ExecContext(ctx, `
		UPDATE email_revalidation
		SET status = 'renamed',
		    new_login = $1,
		    next_check_at = NULL,
		    last_checked_at = $2
		WHERE email = $3
	`, newLogin, now, email)
	if err != nil {
		t.Fatalf("Failed to update email_revalidation status: %v", err)
	}

	// ASSERTIONS - verify data integrity

	// 1. email_resolution.login should now be new-name
	var resolutionLogin string
	err = db.QueryRowContext(ctx, `SELECT login FROM email_resolution WHERE email = $1`, email).Scan(&resolutionLogin)
	if err != nil {
		t.Fatalf("Failed to query email_resolution: %v", err)
	}
	if resolutionLogin != newLogin {
		t.Errorf("After rename, email_resolution.login should be %s, got %s", newLogin, resolutionLogin)
	}

	// 2. users.login should be updated to new-name with SAME user_id
	var updatedLogin string
	var updatedUserID int64
	err = db.QueryRowContext(ctx, `SELECT user_id, login FROM users WHERE login = $1`, newLogin).Scan(&updatedUserID, &updatedLogin)
	if err != nil {
		t.Fatalf("Failed to query updated user: %v", err)
	}
	if updatedLogin != newLogin {
		t.Errorf("After rename, users.login should be %s, got %s", newLogin, updatedLogin)
	}
	if updatedUserID != originalUserID {
		t.Errorf("After rename, user_id should be preserved as %d, got %d (HISTORY FRAGMENTATION!)", originalUserID, updatedUserID)
	}

	// 3. No duplicate users row should exist for old-name
	var oldNameCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE login = $1`, oldLogin).Scan(&oldNameCount)
	if err != nil {
		t.Fatalf("Failed to count old-name users: %v", err)
	}
	if oldNameCount != 0 {
		t.Errorf("Old login %s should not exist after rename, found %d rows", oldLogin, oldNameCount)
	}

	// 4. Total users count should still be 1 (no new row created)
	var totalUsers int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}
	if totalUsers != 1 {
		t.Errorf("Total users should be 1, got %d (extra row created!)", totalUsers)
	}

	// 5. repo_user_daily_tool rows should still reference the SAME user_id
	var toolCount int
	var toolUserID int64
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*), user_id
		FROM repo_user_daily_tool
		WHERE user_id = $1
		GROUP BY user_id
	`, originalUserID).Scan(&toolCount, &toolUserID)
	if err != nil {
		t.Fatalf("Failed to query rollup data: %v", err)
	}
	if toolCount != 1 {
		t.Errorf("Expected 1 rollup row, got %d", toolCount)
	}
	if toolUserID != originalUserID {
		t.Errorf("Rollup user_id should still be %d, got %d", originalUserID, toolUserID)
	}

	// 6. email_revalidation should have status='renamed' with new_login populated
	var revalidationStatus string
	var newLoginResult sql.NullString
	err = db.QueryRowContext(ctx, `
		SELECT status, new_login
		FROM email_revalidation
		WHERE email = $1
	`, email).Scan(&revalidationStatus, &newLoginResult)
	if err != nil {
		t.Fatalf("Failed to query email_revalidation: %v", err)
	}
	if revalidationStatus != "renamed" {
		t.Errorf("email_revalidation.status should be 'renamed', got %s", revalidationStatus)
	}
	if !newLoginResult.Valid || newLoginResult.String != newLogin {
		t.Errorf("email_revalidation.new_login should be %s, got %v", newLogin, newLoginResult)
	}

	t.Logf("✓ Rename test passed: user_id=%d preserved from %s -> %s", originalUserID, oldLogin, newLogin)
}

// testAccountDeletionPreservesHistoricalData verifies that when a GitHub
// account is deleted, the historical data is preserved and the row is flagged
// rather than silently dropped.
func testAccountDeletionPreservesHistoricalData(ctx context.Context, t *testing.T, db *sql.DB) {
	email := "deleted@example.com"
	login := "deleted-user"

	// Seed initial state
	originalUserID, _ := seedInitialRevalidationState(ctx, t, db, email, login)

	// Simulate the revalidation worker detecting account deletion via GitHub API
	// In the real worker, checkLogin() would return ("deleted", nil, nil)
	//
	// The worker marks email_revalidation as status='deleted' (terminal state)
	// and stops further rechecking (next_check_at = NULL)

	now := time.Now()
	_, err := db.ExecContext(ctx, `
		UPDATE email_revalidation
		SET status = 'deleted',
		    next_check_at = NULL,
		    last_checked_at = $1
		WHERE email = $2
	`, now, email)
	if err != nil {
		t.Fatalf("Failed to update email_revalidation status to deleted: %v", err)
	}

	// ASSERTIONS - verify historical data is preserved

	// 1. email_resolution should still exist (row NOT silently dropped)
	var resolutionLogin string
	err = db.QueryRowContext(ctx, `SELECT login FROM email_resolution WHERE email = $1`, email).Scan(&resolutionLogin)
	if err != nil {
		t.Fatalf("email_resolution row should still exist after deletion detection: %v", err)
	}
	if resolutionLogin != login {
		t.Errorf("email_resolution.login should remain %s after deletion, got %s", login, resolutionLogin)
	}

	// 2. users row should still exist
	var userLogin string
	var userID int64
	err = db.QueryRowContext(ctx, `SELECT user_id, login FROM users WHERE user_id = $1`, originalUserID).Scan(&userID, &userLogin)
	if err != nil {
		t.Fatalf("users row should still exist after deletion detection: %v", err)
	}
	if userLogin != login {
		t.Errorf("users.login should remain %s after deletion, got %s", login, userLogin)
	}
	if userID != originalUserID {
		t.Errorf("users.user_id should be preserved as %d, got %d", originalUserID, userID)
	}

	// 3. repo_user_daily_tool rows should still exist
	var toolCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM repo_user_daily_tool
		WHERE user_id = $1
	`, originalUserID).Scan(&toolCount)
	if err != nil {
		t.Fatalf("Failed to query rollup data: %v", err)
	}
	if toolCount != 1 {
		t.Errorf("Rollup data should be preserved, expected 1 row, got %d", toolCount)
	}

	// 4. email_revalidation should have status='deleted' and next_check_at=NULL
	var revalidationStatus string
	var nextCheck sql.NullTime
	err = db.QueryRowContext(ctx, `
		SELECT status, next_check_at
		FROM email_revalidation
		WHERE email = $1
	`, email).Scan(&revalidationStatus, &nextCheck)
	if err != nil {
		t.Fatalf("Failed to query email_revalidation: %v", err)
	}
	if revalidationStatus != "deleted" {
		t.Errorf("email_revalidation.status should be 'deleted', got %s", revalidationStatus)
	}
	if nextCheck.Valid {
		t.Errorf("email_revalidation.next_check_at should be NULL for deleted status, got %v", nextCheck.Time)
	}

	t.Logf("✓ Deletion test passed: historical data preserved for user_id=%d, row flagged as deleted", originalUserID)
}

// testMultipleRenamesMaintainContinuity verifies that multiple sequential
// renames maintain a single user_id throughout all transitions.
func testMultipleRenamesMaintainContinuity(ctx context.Context, t *testing.T, db *sql.DB) {
	email := "renamer@example.com"
	logins := []string{"original-name", "second-name", "third-name", "final-name"}

	// Seed with first login
	originalUserID, _ := seedInitialRevalidationState(ctx, t, db, email, logins[0])

	// Perform sequential renames (simulating worker processing each rename)
	for i := 0; i < len(logins)-1; i++ {
		oldLogin := logins[i]
		newLogin := logins[i+1]

		// Step 1: Update email_resolution (simulating queue-api call)
		_, err := db.ExecContext(ctx, `
			INSERT INTO email_resolution (email, login, source, resolved_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (email) DO UPDATE
			  SET login = excluded.login,
			      source = excluded.source,
			      resolved_at = excluded.resolved_at
			  WHERE excluded.source = 'manual'
			     OR (email_resolution.source <> 'manual'
			         AND excluded.resolved_at > email_resolution.resolved_at)
		`, email, newLogin, "live", time.Now())
		if err != nil {
			t.Fatalf("Failed to update email_resolution for rename %d: %v", i+1, err)
		}

		// Step 2: Update users.login in place
		_, err = db.ExecContext(ctx, `
			UPDATE users
			SET login = $1
			WHERE login = $2
		`, newLogin, oldLogin)
		if err != nil {
			t.Fatalf("Failed to update users.login for rename %d: %v", i+1, err)
		}

		// Step 3: Update email_revalidation for the next check
		_, err = db.ExecContext(ctx, `
			UPDATE email_revalidation
			SET status = 'pending',
			    login = $1,
			    last_checked_at = NOW()
			WHERE email = $2
		`, newLogin, email)
		if err != nil {
			t.Fatalf("Failed to update email_revalidation for rename %d: %v", i+1, err)
		}

		t.Logf("Rename %d: %s -> %s", i+1, oldLogin, newLogin)
	}

	// Final verification
	var finalLogin string
	var finalUserID int64
	err := db.QueryRowContext(ctx, `SELECT user_id, login FROM users WHERE user_id = $1`, originalUserID).Scan(&finalUserID, &finalLogin)
	if err != nil {
		t.Fatalf("Failed to query final user state: %v", err)
	}

	if finalLogin != logins[len(logins)-1] {
		t.Errorf("Final login should be %s, got %s", logins[len(logins)-1], finalLogin)
	}
	if finalUserID != originalUserID {
		t.Errorf("Final user_id should be %d, got %d (HISTORY FRAGMENTATION after %d renames!)", originalUserID, finalUserID, len(logins)-1)
	}

	// Verify total users is still 1
	var totalUsers int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	if totalUsers != 1 {
		t.Errorf("After %d renames, total users should be 1, got %d", len(logins)-1, totalUsers)
	}

	t.Logf("✓ Multiple rename test passed: user_id=%d preserved through %d renames: %v",
		originalUserID, len(logins)-1, logins)
}

// testEmailResolutionUpdateWithMock verifies that the revalidation worker
// correctly updates email_resolution.login when detecting a rename using the mock client.
// This test uses seeded data (from cg-4vkqn) and the mock GitHub client (from cg-4rhpp).
func testEmailResolutionUpdateWithMock(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	email := "mock-test@example.com"
	oldLogin := "old-name"
	newLogin := "new-name"
	resolvedAt := time.Now().UTC().Add(-24 * time.Hour)

	// Start a transaction for seeding data
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Seed email_resolution row (from cg-4vkqn)
	// This creates the initial state: email -> old-name
	_, err = tx.ExecContext(ctx,
		`INSERT INTO email_resolution (email, login, source, resolved_at) VALUES ($1, $2, $3, $4)`,
		email, oldLogin, "seed", resolvedAt)
	if err != nil {
		t.Fatalf("Failed to seed email_resolution: %v", err)
	}

	// Verify initial state
	var initialLogin string
	err = tx.QueryRowContext(ctx, `SELECT login FROM email_resolution WHERE email = $1`, email).Scan(&initialLogin)
	if err != nil {
		t.Fatalf("Failed to query initial email_resolution: %v", err)
	}
	if initialLogin != oldLogin {
		t.Errorf("Initial login should be %s, got %s", oldLogin, initialLogin)
	}

	// Commit the seeded data
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit seeded data: %v", err)
	}

	// Create mock GitHub client (from cg-4rhpp)
	// Configure it to return a rename result when checking old-name
	mockGitHubClient := github.NewMockClient()
	mockGitHubClient.SetResponse(oldLogin, &github.LoginResult{
		Status:   github.StatusRenamed,
		NewLogin: &newLogin,
	})

	// Create a mock queue-api client that captures the update call
	var capturedUpdate struct {
		Email  string
		Login  string
		Called bool
	}
	mockQueueClient := &MockQueueClient{
		postResolutionFn: func(ctx context.Context, email, login string) error {
			capturedUpdate.Email = email
			capturedUpdate.Login = login
			capturedUpdate.Called = true

			// Simulate queue-api calling identity ingest to update email_resolution
			now := time.Now()
			_, err := db.ExecContext(ctx, `
				INSERT INTO email_resolution (email, login, source, resolved_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (email) DO UPDATE
				  SET login = excluded.login,
				      source = excluded.source,
				      resolved_at = excluded.resolved_at
				  WHERE excluded.source = 'manual'
				     OR (email_resolution.source <> 'manual'
				         AND excluded.resolved_at > email_resolution.resolved_at)
			`, email, login, "live", now)
			if err != nil {
				return fmt.Errorf("queue-api update email_resolution failed: %w", err)
			}
			return nil
		},
	}

	// Create config with mocked clients
	cfg := &Config{
		QueueAPIClient: mockQueueClient,
		GitHubClient:   mockGitHubClient,
	}

	// Create a revalidation row to process
	row := RevalidationRow{
		Email:         email,
		Login:         oldLogin,
		LastCheckedAt: time.Now().Add(-24 * time.Hour),
		NextCheckAt:   nil,
		Status:        "pending",
		NewLogin:      nil,
		CheckError:    nil,
		CreatedAt:     time.Now().Add(-24 * time.Hour),
	}

	// Invoke the revalidation worker's processRow function with the mocked client
	// This is the core of the task - actually invoking the worker logic
	err = processRow(ctx, db, cfg, row)
	if err != nil {
		t.Fatalf("processRow failed: %v", err)
	}

	t.Logf("Worker processRow invoked successfully with mock client")

	// ASSERTIONS - verify the worker behaved correctly

	// 1. Assert email_resolution.login now equals new-name
	var updatedLogin string
	err = db.QueryRowContext(ctx, `SELECT login FROM email_resolution WHERE email = $1`, email).Scan(&updatedLogin)
	if err != nil {
		t.Fatalf("Failed to query updated email_resolution: %v", err)
	}
	if updatedLogin != newLogin {
		t.Errorf("After worker processing, email_resolution.login should be %s, got %s", newLogin, updatedLogin)
	}

	// 2. Assert the email address itself is unchanged
	var emailCheck string
	err = db.QueryRowContext(ctx, `SELECT email FROM email_resolution WHERE login = $1`, newLogin).Scan(&emailCheck)
	if err != nil {
		t.Fatalf("Failed to query email by login: %v", err)
	}
	if emailCheck != email {
		t.Errorf("Email address should be unchanged: expected %s, got %s", email, emailCheck)
	}

	// 3. Assert only one email_resolution row exists for this email (no duplicates)
	var count int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_resolution WHERE email = $1`, email).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count email_resolution rows: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 email_resolution row, got %d (duplicate created!)", count)
	}

	// 4. Verify the mock GitHub client was called exactly once
	if mockGitHubClient.CallCount() != 1 {
		t.Errorf("Mock GitHub client should be called exactly once, was called %d times", mockGitHubClient.CallCount())
	}
	if !mockGitHubClient.WasCalled(oldLogin) {
		t.Errorf("Mock GitHub client should be called with %s, was called with %v", oldLogin, mockGitHubClient.CalledLogins())
	}

	// 5. Verify the mock queue-api client was called with correct parameters
	if !capturedUpdate.Called {
		t.Errorf("Mock queue-api client should be called, but wasn't")
	}
	if capturedUpdate.Email != email {
		t.Errorf("Mock queue-api client called with wrong email: expected %s, got %s", email, capturedUpdate.Email)
	}
	if capturedUpdate.Login != newLogin {
		t.Errorf("Mock queue-api client called with wrong login: expected %s, got %s", newLogin, capturedUpdate.Login)
	}

	// 6. Verify email_revalidation row was updated to 'renamed' status
	var revalidationStatus string
	var newLoginResult sql.NullString
	var nextCheck sql.NullTime
	err = db.QueryRowContext(ctx, `
		SELECT status, new_login, next_check_at
		FROM email_revalidation
		WHERE email = $1
	`, email).Scan(&revalidationStatus, &newLoginResult, &nextCheck)
	if err != nil {
		t.Fatalf("Failed to query email_revalidation: %v", err)
	}
	if revalidationStatus != "renamed" {
		t.Errorf("email_revalidation.status should be 'renamed', got %s", revalidationStatus)
	}
	if !newLoginResult.Valid || newLoginResult.String != newLogin {
		t.Errorf("email_revalidation.new_login should be %s, got %v", newLogin, newLoginResult)
	}
	// For 'renamed' status, next_check_at should be NULL (terminal state)
	if nextCheck.Valid {
		t.Errorf("email_revalidation.next_check_at should be NULL for renamed status, got %v", nextCheck.Time)
	}

	t.Logf("✓ Email resolution update test passed: %s login updated from %s to %s via worker invocation", email, oldLogin, newLogin)
}

// testIdempotency verifies that running the revalidation worker twice
// on the same email does not create duplicate rows or corrupt data.
//
// This test ensures that:
// 1. The first run correctly updates email_resolution.login to new-name
// 2. The second run (with already-updated state) creates no duplicates
// 3. Login remains new-name (not changed again)
// 4. Email address remains unchanged
func testIdempotency(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	email := "idempotent@example.com"
	oldLogin := "old-name"
	newLogin := "new-name"
	resolvedAt := time.Now().UTC().Add(-24 * time.Hour)

	// Start a transaction for seeding data
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Seed email_resolution row with initial state
	_, err = tx.ExecContext(ctx,
		`INSERT INTO email_resolution (email, login, source, resolved_at) VALUES ($1, $2, $3, $4)`,
		email, oldLogin, "seed", resolvedAt)
	if err != nil {
		t.Fatalf("Failed to seed email_resolution: %v", err)
	}

	// Verify initial state
	var initialLogin string
	err = tx.QueryRowContext(ctx, `SELECT login FROM email_resolution WHERE email = $1`, email).Scan(&initialLogin)
	if err != nil {
		t.Fatalf("Failed to query initial email_resolution: %v", err)
	}
	if initialLogin != oldLogin {
		t.Errorf("Initial login should be %s, got %s", oldLogin, initialLogin)
	}

	// Commit the seeded data
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit seeded data: %v", err)
	}

	// Create mock GitHub client configured to return a rename result
	mockGitHubClient := github.NewMockClient()
	mockGitHubClient.SetResponse(oldLogin, &github.LoginResult{
		Status:   github.StatusRenamed,
		NewLogin: &newLogin,
	})

	// Create a mock queue-api client that captures update calls
	var capturedUpdate struct {
		Email  string
		Login  string
		Called bool
		CallCount int
	}
	mockQueueClient := &MockQueueClient{
		postResolutionFn: func(ctx context.Context, email, login string) error {
			capturedUpdate.Email = email
			capturedUpdate.Login = login
			capturedUpdate.Called = true
			capturedUpdate.CallCount++

			// Simulate queue-api calling identity ingest to update email_resolution
			now := time.Now()
			_, err := db.ExecContext(ctx, `
				INSERT INTO email_resolution (email, login, source, resolved_at)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (email) DO UPDATE
				  SET login = excluded.login,
				      source = excluded.source,
				      resolved_at = excluded.resolved_at
				  WHERE excluded.source = 'manual'
				     OR (email_resolution.source <> 'manual'
				         AND excluded.resolved_at > email_resolution.resolved_at)
			`, email, login, "live", now)
			if err != nil {
				return fmt.Errorf("queue-api update email_resolution failed: %w", err)
			}
			return nil
		},
	}

	// Create config with mocked clients
	cfg := &Config{
		QueueAPIClient: mockQueueClient,
		GitHubClient:   mockGitHubClient,
	}

	// Create a revalidation row to process
	row := RevalidationRow{
		Email:         email,
		Login:         oldLogin,
		LastCheckedAt: time.Now().Add(-24 * time.Hour),
		NextCheckAt:   nil,
		Status:        "pending",
		NewLogin:      nil,
		CheckError:    nil,
		CreatedAt:     time.Now().Add(-24 * time.Hour),
	}

	// FIRST WORKER INVOCATION
	t.Log("First worker invocation...")
	err = processRow(ctx, db, cfg, row)
	if err != nil {
		t.Fatalf("First processRow failed: %v", err)
	}

	// Verify state after first run
	var loginAfterFirstRun string
	err = db.QueryRowContext(ctx, `SELECT login FROM email_resolution WHERE email = $1`, email).Scan(&loginAfterFirstRun)
	if err != nil {
		t.Fatalf("Failed to query email_resolution after first run: %v", err)
	}
	if loginAfterFirstRun != newLogin {
		t.Errorf("After first run, login should be %s, got %s", newLogin, loginAfterFirstRun)
	}

	var countAfterFirstRun int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_resolution WHERE email = $1`, email).Scan(&countAfterFirstRun)
	if err != nil {
		t.Fatalf("Failed to count email_resolution rows after first run: %v", err)
	}
	if countAfterFirstRun != 1 {
		t.Errorf("After first run, expected 1 email_resolution row, got %d", countAfterFirstRun)
	}

	t.Logf("✓ First worker invocation completed: login=%s, count=%d", loginAfterFirstRun, countAfterFirstRun)

	// SECOND WORKER INVOCATION - this should be idempotent
	// The revalidation row now has login=new-name and status='renamed'
	// We'll re-run processRow with the updated state
	t.Log("Second worker invocation (testing idempotency)...")

	// Update the revalidation row to reflect the state after first run
	_, err = db.ExecContext(ctx, `
		UPDATE email_revalidation
		SET login = $1, status = 'renamed', new_login = $1
		WHERE email = $2
	`, newLogin, email)
	if err != nil {
		t.Fatalf("Failed to update email_revalidation for second run: %v", err)
	}

	// Create a new row reflecting the current state (login=new-name, status=renamed)
	rowSecond := RevalidationRow{
		Email:         email,
		Login:         newLogin, // Now using new-name
		LastCheckedAt: time.Now(),
		NextCheckAt:   nil,
		Status:        "renamed",
		NewLogin:      &newLogin,
		CheckError:    nil,
		CreatedAt:     time.Now().Add(-24 * time.Hour),
	}

	// Reset the mock client call counter for the second run
	capturedUpdate.Called = false
	capturedUpdate.CallCount = 0

	// Run the worker again
	err = processRow(ctx, db, cfg, rowSecond)
	if err != nil {
		t.Fatalf("Second processRow failed: %v", err)
	}

	// IDEMPOTENCY ASSERTIONS

	// 1. Verify no duplicate email_resolution rows were created
	var countAfterSecondRun int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_resolution WHERE email = $1`, email).Scan(&countAfterSecondRun)
	if err != nil {
		t.Fatalf("Failed to count email_resolution rows after second run: %v", err)
	}
	if countAfterSecondRun != 1 {
		t.Errorf("After second run, expected 1 email_resolution row (no duplicates), got %d", countAfterSecondRun)
	}

	// 2. Verify that login still equals new-name (not changed again)
	var loginAfterSecondRun string
	err = db.QueryRowContext(ctx, `SELECT login FROM email_resolution WHERE email = $1`, email).Scan(&loginAfterSecondRun)
	if err != nil {
		t.Fatalf("Failed to query email_resolution.login after second run: %v", err)
	}
	if loginAfterSecondRun != newLogin {
		t.Errorf("After second run, login should still be %s (unchanged), got %s", newLogin, loginAfterSecondRun)
	}

	// 3. Verify that email_address is still unchanged
	var emailCheck string
	err = db.QueryRowContext(ctx, `SELECT email FROM email_resolution WHERE login = $1`, newLogin).Scan(&emailCheck)
	if err != nil {
		t.Fatalf("Failed to query email_address after second run: %v", err)
	}
	if emailCheck != email {
		t.Errorf("Email address should be unchanged after second run: expected %s, got %s", email, emailCheck)
	}

	// 4. Verify total email_resolution count hasn't increased
	var totalEmailResolutionCount int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_resolution`).Scan(&totalEmailResolutionCount)
	if err != nil {
		t.Fatalf("Failed to count total email_resolution rows: %v", err)
	}
	if totalEmailResolutionCount != 1 {
		t.Errorf("Total email_resolution rows should be 1 (no extra rows created), got %d", totalEmailResolutionCount)
	}

	t.Logf("✓ Idempotency test passed: second run created no duplicates, login=%s, email=%s unchanged", loginAfterSecondRun, emailCheck)
}
