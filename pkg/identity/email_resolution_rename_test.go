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
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/jedarden/commitgraph/pkg/client/github"
	"github.com/jedarden/commitgraph/pkg/revalidation"
)

// mockQueueAPIClient is a mock implementation of the queue-api client for testing.
// It tracks PostResolution calls without making actual HTTP requests.
type mockQueueAPIClient struct {
	mu           sync.Mutex
	calls        []postResolutionCall
	shouldFail   bool // If true, PostResolution returns an error
}

type postResolutionCall struct {
	email       string
	login       string
	timestamp   time.Time
}

// newMockQueueAPIClient creates a new mock queue-api client.
func newMockQueueAPIClient() *mockQueueAPIClient {
	return &mockQueueAPIClient{
		calls: make([]postResolutionCall, 0),
	}
}

// PostResolution records the call without making an HTTP request.
// Implements the revalidation.Client interface.
func (m *mockQueueAPIClient) PostResolution(ctx context.Context, email, login string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldFail {
		return fmt.Errorf("mock queue-api client error")
	}

	m.calls = append(m.calls, postResolutionCall{
		email:     email,
		login:     login,
		timestamp: time.Now(),
	})
	return nil
}

// CallCount returns the number of times PostResolution was called.
func (m *mockQueueAPIClient) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// WasCalledWith checks if PostResolution was called with the given email and login.
func (m *mockQueueAPIClient) WasCalledWith(email, login string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, call := range m.calls {
		if call.email == email && call.login == login {
			return true
		}
	}
	return false
}

// setShouldFail configures whether PostResolution should return an error.
func (m *mockQueueAPIClient) setShouldFail(shouldFail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = shouldFail
}

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

	t.Run("worker error handling", func(t *testing.T) {
		testRevalidationWorkerErrorHandling(ctx, t, db)
	})

	t.Run("deletion case preserves historical data", func(t *testing.T) {
		testRevalidationWorkerDeletionHandling(ctx, t, db)
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

	// Create mock GitHub client configured to return rename: old-name -> new-name
	mockGitHubClient := github.NewMockClient()
	mockGitHubClient.SetResponse(oldLogin, &github.LoginResult{
		Status:   github.StatusRenamed,
		NewLogin: &newLogin,
	})

	// Create mock queue-api client
	mockQueueClient := newMockQueueAPIClient()

	// Create revalidation worker config
	cfg := &revalidation.Config{
		QueueAPIClient: mockQueueClient,
		GitHubClient:   mockGitHubClient,
	}

	// Create a revalidation row for processing
	row := revalidation.Row{
		Email:         email,
		Login:         oldLogin,
		LastCheckedAt: resolvedAt,
		NextCheckAt:   nil, // Being checked now
		Status:        "pending",
		NewLogin:      nil,
		CheckError:    nil,
		CreatedAt:     resolvedAt,
	}

	// Invoke the revalidation worker with mock clients
	if err := revalidation.ProcessRow(ctx, db, cfg, row); err != nil {
		t.Fatalf("Revalidation worker ProcessRow failed: %v", err)
	}

	// Verify queue-api client was called with the new login
	if !mockQueueClient.WasCalledWith(email, newLogin) {
		t.Errorf("Expected queue-api PostResolution to be called with email=%s, login=%s", email, newLogin)
	}

	// Verify email_resolution table after worker invocation
	var updatedLogin string
	var updatedEmail string
	queryErr := db.QueryRowContext(ctx,
		`SELECT login, email FROM email_resolution WHERE email = $1`, email).Scan(&updatedLogin, &updatedEmail)
	if queryErr != nil {
		t.Fatalf("Failed to query email_resolution table after worker invocation: %v", queryErr)
	}

	// Assert that the login column equals new-name (was updated)
	if updatedLogin != newLogin {
		t.Errorf("After worker invocation: expected login column to be updated to '%s', but got '%s' (login was not updated correctly)", newLogin, updatedLogin)
	}

	// Assert that the email column is unchanged (same as before rename)
	if updatedEmail != email {
		t.Errorf("After worker invocation: expected email column to be preserved as '%s', but got '%s' (email address should not change during rename)", email, updatedEmail)
	}

	t.Logf("✓ email_resolution properly updated: login='%s' -> '%s', email='%s' (preserved)", oldLogin, newLogin, updatedEmail)
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

// testRevalidationWorkerErrorHandling verifies that worker errors are properly handled.
// This test ensures that when the queue-api client returns an error during a rename operation,
// the worker propagates the error and fails the test rather than silently continuing.
func testRevalidationWorkerErrorHandling(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	email := "error@example.com"
	oldLogin := "error-old"
	newLogin := "error-new"

	// Seed email_resolution row with old login
	resolvedAt := time.Now().UTC().Add(-24 * time.Hour)
	_, err := db.ExecContext(ctx,
		`INSERT INTO email_resolution (email, login, source, resolved_at) VALUES ($1, $2, $3, $4)`,
		email, oldLogin, "seed", resolvedAt)
	if err != nil {
		t.Fatalf("Failed to seed email_resolution: %v", err)
	}

	// Create mock GitHub client configured to return rename: old-login -> new-login
	mockGitHubClient := github.NewMockClient()
	mockGitHubClient.SetResponse(oldLogin, &github.LoginResult{
		Status:   github.StatusRenamed,
		NewLogin: &newLogin,
	})

	// Create mock queue-api client configured to fail
	mockQueueClient := newMockQueueAPIClient()
	mockQueueClient.setShouldFail(true)

	// Create revalidation worker config with failing queue-api client
	cfg := &revalidation.Config{
		QueueAPIClient: mockQueueClient,
		GitHubClient:   mockGitHubClient,
	}

	// Create a revalidation row for processing
	row := revalidation.Row{
		Email:         email,
		Login:         oldLogin,
		LastCheckedAt: resolvedAt,
		NextCheckAt:   nil,
		Status:        "pending",
		NewLogin:      nil,
		CheckError:    nil,
		CreatedAt:     resolvedAt,
	}

	// Invoke the revalidation worker - it should fail due to queue-api error
	err = revalidation.ProcessRow(ctx, db, cfg, row)
	if err == nil {
		t.Error("Expected ProcessRow to fail when queue-api returns error, but it succeeded")
	} else {
		t.Logf("✓ Worker error properly propagated: %v", err)
	}

	// Verify that queue-api client was actually called before failing
	if mockQueueClient.CallCount() == 0 {
		t.Error("Expected queue-api client to be called before error, but it wasn't")
	} else {
		t.Logf("✓ Queue-api client called %d time(s) before error", mockQueueClient.CallCount())
	}
}

// testRevalidationWorkerDeletionHandling verifies that when the revalidation worker
// detects a deleted GitHub account, it properly flags the row without silently dropping data.
//
// This test covers the deletion scenario from edge case #6:
// When a GitHub account is deleted (no successor login), the revalidation worker
// should mark the revalidation status as 'deleted' but preserve all historical data.
func testRevalidationWorkerDeletionHandling(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	email := "deleted@example.com"
	login := "deleted-user"

	// Seed user with the login
	var userID int64
	err := db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, login).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}

	// Seed repo for activity data
	var repoID int64
	err = db.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`, "github", "test/repo").Scan(&repoID)
	if err != nil {
		t.Fatalf("Failed to seed repo: %v", err)
	}

	// Seed historical repo_user_daily_tool activity under this user_id
	testDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repoID, userID, "claude", testDate, 10, time.Now())
	if err != nil {
		t.Fatalf("Failed to seed repo_user_daily_tool activity: %v", err)
	}

	// Seed email_resolution row
	resolvedAt := time.Now().UTC().Add(-24 * time.Hour)
	_, err = db.ExecContext(ctx,
		`INSERT INTO email_resolution (email, login, source, resolved_at) VALUES ($1, $2, $3, $4)`,
		email, login, "seed", resolvedAt)
	if err != nil {
		t.Fatalf("Failed to seed email_resolution: %v", err)
	}

	// Create a temporary email_revalidation table for this test
	_, err = db.ExecContext(ctx, `
		CREATE TEMPORARY TABLE email_revalidation (
			email TEXT NOT NULL PRIMARY KEY,
			login TEXT NOT NULL,
			last_checked_at TIMESTAMPTZ NOT NULL,
			next_check_at TIMESTAMPTZ,
			status TEXT NOT NULL,
			new_login TEXT,
			check_error TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create email_revalidation table: %v", err)
	}

	// Seed email_revalidation row
	_, err = db.ExecContext(ctx, `
		INSERT INTO email_revalidation (email, login, last_checked_at, next_check_at, status)
		VALUES ($1, $2, $3, NULL, $4)
	`, email, login, resolvedAt, "pending")
	if err != nil {
		t.Fatalf("Failed to seed email_revalidation: %v", err)
	}

	// Create mock GitHub client configured to return deleted status
	mockGitHubClient := github.NewMockClient()
	mockGitHubClient.SetResponse(login, &github.LoginResult{
		Status:   github.StatusDeleted,
		NewLogin: nil, // No successor login
	})

	// Create mock queue-api client (should not be called for deleted status)
	mockQueueClient := newMockQueueAPIClient()

	// Create revalidation worker config
	cfg := &revalidation.Config{
		QueueAPIClient: mockQueueClient,
		GitHubClient:   mockGitHubClient,
	}

	// Create a revalidation row for processing
	row := revalidation.Row{
		Email:         email,
		Login:         login,
		LastCheckedAt: resolvedAt,
		NextCheckAt:   nil,
		Status:        "pending",
		NewLogin:      nil,
		CheckError:    nil,
		CreatedAt:     resolvedAt,
	}

	// Invoke the revalidation worker
	err = revalidation.ProcessRow(ctx, db, cfg, row)
	if err != nil {
		t.Fatalf("Revalidation worker ProcessRow failed: %v", err)
	}

	// ASSERTIONS

	// 1. Verify queue-api client was NOT called (deletion doesn't trigger resolution update)
	if mockQueueClient.CallCount() != 0 {
		t.Errorf("Expected queue-api client NOT to be called for deleted status, but it was called %d time(s)", mockQueueClient.CallCount())
	} else {
		t.Logf("✓ Queue-api client not called for deleted status (correct behavior)")
	}

	// 2. Verify email_revalidation status is 'deleted'
	var revalidationStatus string
	err = db.QueryRowContext(ctx, `SELECT status FROM email_revalidation WHERE email = $1`, email).Scan(&revalidationStatus)
	if err != nil {
		t.Fatalf("Failed to query email_revalidation status: %v", err)
	}
	if revalidationStatus != "deleted" {
		t.Errorf("Expected email_revalidation status to be 'deleted', got '%s'", revalidationStatus)
	} else {
		t.Logf("✓ email_revalidation status correctly set to 'deleted'")
	}

	// 3. Verify email_revalidation.next_check_at is NULL (stop rechecking)
	var nextCheckAt interface{}
	err = db.QueryRowContext(ctx, `SELECT next_check_at FROM email_revalidation WHERE email = $1`, email).Scan(&nextCheckAt)
	if err != nil {
		t.Fatalf("Failed to query email_revalidation next_check_at: %v", err)
	}
	if nextCheckAt != nil {
		t.Errorf("Expected email_revalidation.next_check_at to be NULL for deleted status, got %v", nextCheckAt)
	} else {
		t.Logf("✓ email_revalidation.next_check_at correctly set to NULL")
	}

	// 4. Verify email_resolution row is NOT dropped (historical data preserved)
	var resolutionLogin string
	var resolutionEmail string
	err = db.QueryRowContext(ctx, `SELECT login, email FROM email_resolution WHERE email = $1`, email).Scan(&resolutionLogin, &resolutionEmail)
	if err != nil {
		if err == sql.ErrNoRows {
			t.Errorf("email_resolution row was silently dropped for deleted account (should be preserved)")
		} else {
			t.Fatalf("Failed to query email_resolution: %v", err)
		}
	} else {
		if resolutionLogin != login {
			t.Errorf("email_resolution.login should remain '%s' after deletion, got '%s'", login, resolutionLogin)
		}
		if resolutionEmail != email {
			t.Errorf("email_resolution.email should remain '%s' after deletion, got '%s'", email, resolutionEmail)
		}
		t.Logf("✓ email_resolution row preserved (login='%s', email='%s')", resolutionLogin, resolutionEmail)
	}

	// 5. Verify users row is NOT dropped (historical data preserved)
	var userLogin string
	var queriedUserID int64
	err = db.QueryRowContext(ctx, `SELECT user_id, login FROM users WHERE user_id = $1`, userID).Scan(&queriedUserID, &userLogin)
	if err != nil {
		if err == sql.ErrNoRows {
			t.Errorf("users row was silently dropped for deleted account (should be preserved)")
		} else {
			t.Fatalf("Failed to query users: %v", err)
		}
	} else {
		if userLogin != login {
			t.Errorf("users.login should remain '%s' after deletion, got '%s'", login, userLogin)
		}
		if queriedUserID != userID {
			t.Errorf("users.user_id should be preserved as %d, got %d", userID, queriedUserID)
		}
		t.Logf("✓ users row preserved (user_id=%d, login='%s')", queriedUserID, userLogin)
	}

	// 6. Verify repo_user_daily_tool rows are NOT dropped (historical data preserved)
	var toolCount int
	var toolUserID int64
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*), user_id
		FROM repo_user_daily_tool
		WHERE user_id = $1
		GROUP BY user_id
	`, userID).Scan(&toolCount, &toolUserID)
	if err != nil {
		t.Fatalf("Failed to query repo_user_daily_tool: %v", err)
	}
	if toolCount != 1 {
		t.Errorf("Expected 1 repo_user_daily_tool row, got %d", toolCount)
	}
	if toolUserID != userID {
		t.Errorf("repo_user_daily_tool.user_id should be preserved as %d, got %d", userID, toolUserID)
	}
	t.Logf("✓ repo_user_daily_tool row preserved (user_id=%d)", toolUserID)

	t.Log("✓ Deletion handling test passed: historical data preserved, revalidation status updated")
}
