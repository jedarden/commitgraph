package identity

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestLoginRenameFixtures validates that all fixture helpers work correctly
// and can be called with different parameters for reusability.
//
// To run this test:
//   export TEST_DB_URL="postgres://user:pass@localhost:5432/testdb?sslmode=disable"
//   go test -v -run TestLoginRenameFixtures ./pkg/identity/
func TestLoginRenameFixtures(t *testing.T) {
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

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	// Clean up any existing test data
	cleanupTestTables(ctx, t, db)

	// Setup test tables
	if err := setupLoginRenameTables(ctx, db); err != nil {
		t.Fatalf("Failed to setup tables: %v", err)
	}

	// Test all fixture helpers
	t.Run("SeedUser creates user with login and returns user_id", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		// Test basic user creation
		userID := fixtures.SeedUser(ctx, t, "test-user")
		if userID == 0 {
			t.Error("Expected non-zero user_id")
		}

		// Verify user exists in database
		var login string
		err = tx.QueryRowContext(ctx, `SELECT login FROM users WHERE user_id = $1`, userID).Scan(&login)
		if err != nil {
			t.Fatalf("Failed to query user: %v", err)
		}
		if login != "test-user" {
			t.Errorf("Expected login 'test-user', got '%s'", login)
		}
	})

	t.Run("SeedUser with optional parameters", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		userID := fixtures.SeedUser(ctx, t, "detailed-user",
			WithProfileURL("https://github.com/detailed-user"),
			WithAvatarURL("https://github.com/detailed-user.png"),
		)

		var profileURL, avatarURL string
		err = tx.QueryRowContext(ctx, `
			SELECT profile_url, avatar_url FROM users WHERE user_id = $1
		`, userID).Scan(&profileURL, &avatarURL)
		if err != nil {
			t.Fatalf("Failed to query user details: %v", err)
		}
		if profileURL != "https://github.com/detailed-user" {
			t.Errorf("Expected profile URL, got '%s'", profileURL)
		}
		if avatarURL != "https://github.com/detailed-user.png" {
			t.Errorf("Expected avatar URL, got '%s'", avatarURL)
		}
	})

	t.Run("SeedEmailResolution creates email_resolution row", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		fixtures.SeedEmailResolution(ctx, t, "user@example.com", "target-login")

		var login string
		err = tx.QueryRowContext(ctx, `
			SELECT login FROM email_resolution WHERE email = $1
		`, "user@example.com").Scan(&login)
		if err != nil {
			t.Fatalf("Failed to query email_resolution: %v", err)
		}
		if login != "target-login" {
			t.Errorf("Expected login 'target-login', got '%s'", login)
		}
	})

	t.Run("SeedEmailResolution with optional parameters", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		testTime := time.Now().Add(-48 * time.Hour)
		fixtures.SeedEmailResolution(ctx, t, "custom@example.com", "custom-login",
			WithSource("manual"),
			WithResolvedAt(testTime),
		)

		var source string
		var resolvedAt time.Time
		err = tx.QueryRowContext(ctx, `
			SELECT source, resolved_at FROM email_resolution WHERE email = $1
		`, "custom@example.com").Scan(&source, &resolvedAt)
		if err != nil {
			t.Fatalf("Failed to query email_resolution: %v", err)
		}
		if source != "manual" {
			t.Errorf("Expected source 'manual', got '%s'", source)
		}
		if resolvedAt.Unix() != testTime.Unix() {
			t.Errorf("Expected resolved_at %v, got %v", testTime, resolvedAt)
		}
	})

	t.Run("SeedRepoUserDailyTool creates historical activity rows", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		// Create dependencies
		userID := fixtures.SeedUser(ctx, t, "activity-user")
		repoID := fixtures.SeedRepo(ctx, t, "github", "test/activity-repo")

		// Seed 5 days of historical activity
		dayCount := 5
		fixtures.SeedRepoUserDailyTool(ctx, t, repoID, userID, "claude", dayCount)

		// Verify count
		var count int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM repo_user_daily_tool WHERE user_id = $1
		`, userID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count activity rows: %v", err)
		}
		if count != dayCount {
			t.Errorf("Expected %d activity rows, got %d", dayCount, count)
		}
	})

	t.Run("SeedRepoUserDailyTool with optional parameters", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		userID := fixtures.SeedUser(ctx, t, "custom-activity-user")
		repoID := fixtures.SeedRepo(ctx, t, "github", "test/custom-activity-repo")

		// Seed with custom commits per day
		fixtures.SeedRepoUserDailyTool(ctx, t, repoID, userID, "copilot", 3,
			WithCommits(10),
			WithStartDate(time.Now().AddDate(0, 0, -30)),
		)

		// Verify commits count
		var commits int
		err = tx.QueryRowContext(ctx, `
			SELECT commits FROM repo_user_daily_tool WHERE user_id = $1 LIMIT 1
		`, userID).Scan(&commits)
		if err != nil {
			t.Fatalf("Failed to query activity: %v", err)
		}
		if commits != 10 {
			t.Errorf("Expected 10 commits, got %d", commits)
		}
	})

	t.Run("SeedRepo creates repo and returns repo_id", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		repoID := fixtures.SeedRepo(ctx, t, "github", "test/fixture-repo")
		if repoID == 0 {
			t.Error("Expected non-zero repo_id")
		}

		var repoFullName string
		err = tx.QueryRowContext(ctx, `
			SELECT repo_full_name FROM repos WHERE repo_id = $1
		`, repoID).Scan(&repoFullName)
		if err != nil {
			t.Fatalf("Failed to query repo: %v", err)
		}
		if repoFullName != "test/fixture-repo" {
			t.Errorf("Expected repo_full_name 'test/fixture-repo', got '%s'", repoFullName)
		}
	})

	t.Run("SeedLoginRenameScenario seeds complete scenario", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		email := "old-user@example.com"
		oldLogin := "old-name"

		userID, repoID := fixtures.SeedLoginRenameScenario(ctx, t, email, oldLogin)

		// Verify user
		var login string
		err = tx.QueryRowContext(ctx, `SELECT login FROM users WHERE user_id = $1`, userID).Scan(&login)
		if err != nil {
			t.Fatalf("Failed to query user: %v", err)
		}
		if login != oldLogin {
			t.Errorf("Expected login '%s', got '%s'", oldLogin, login)
		}

		// Verify email_resolution
		var resolvedLogin string
		err = tx.QueryRowContext(ctx, `
			SELECT login FROM email_resolution WHERE email = $1
		`, email).Scan(&resolvedLogin)
		if err != nil {
			t.Fatalf("Failed to query email_resolution: %v", err)
		}
		if resolvedLogin != oldLogin {
			t.Errorf("Expected email resolved to '%s', got '%s'", oldLogin, resolvedLogin)
		}

		// Verify historical activity exists
		var activityCount int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM repo_user_daily_tool WHERE user_id = $1
		`, userID).Scan(&activityCount)
		if err != nil {
			t.Fatalf("Failed to count activity: %v", err)
		}
		if activityCount != 3 { // default is 3 days
			t.Errorf("Expected 3 historical activity rows, got %d", activityCount)
		}
	})

	t.Run("SeedLoginRenameScenario with custom options", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		userID, repoID := fixtures.SeedLoginRenameScenario(ctx, t, "custom@example.com", "custom-old",
			WithProvider("gitlab"),
			WithRepoFullName("org/custom-repo"),
			WithTool("cursor"),
			WithHistoricalDays(7),
			WithCommitsPerDay(15),
			WithSource("live"),
		)

		// Verify custom tool name
		var tool string
		err = tx.QueryRowContext(ctx, `
			SELECT tool FROM repo_user_daily_tool WHERE user_id = $1 LIMIT 1
		`, userID).Scan(&tool)
		if err != nil {
			t.Fatalf("Failed to query activity: %v", err)
		}
		if tool != "cursor" {
			t.Errorf("Expected tool 'cursor', got '%s'", tool)
		}

		// Verify custom historical days
		var count int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM repo_user_daily_tool WHERE user_id = $1
		`, userID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to count activity: %v", err)
		}
		if count != 7 {
			t.Errorf("Expected 7 historical activity rows, got %d", count)
		}

		// Verify custom commits per day
		var commits int
		err = tx.QueryRowContext(ctx, `
			SELECT commits FROM repo_user_daily_tool WHERE user_id = $1 LIMIT 1
		`, userID).Scan(&commits)
		if err != nil {
			t.Fatalf("Failed to query activity: %v", err)
		}
		if commits != 15 {
			t.Errorf("Expected 15 commits per day, got %d", commits)
		}
	})

	t.Run("Fixtures are reusable with different parameters", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		// Seed multiple different users
		user1 := fixtures.SeedUser(ctx, t, "user-1")
		user2 := fixtures.SeedUser(ctx, t, "user-2")
		user3 := fixtures.SeedUser(ctx, t, "user-3")

		// Verify all users exist
		var userCount int
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&userCount)
		if err != nil {
			t.Fatalf("Failed to count users: %v", err)
		}
		if userCount != 3 {
			t.Errorf("Expected 3 users, got %d", userCount)
		}

		// Verify each user has distinct user_id
		if user1 == user2 || user2 == user3 || user1 == user3 {
			t.Error("Expected distinct user_ids for different users")
		}
	})

	t.Run("Transaction rollback cleans up fixture data", func(t *testing.T) {
		// Begin a transaction
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}

		fixtures := &LoginRenameFixtures{Tx: tx}

		// Seed data within transaction
		userID, _ := fixtures.SeedLoginRenameScenario(ctx, t, "rollback@example.com", "rollback-user")

		// Verify data exists within transaction
		var count int
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE user_id = $1`, userID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query within transaction: %v", err)
		}
		if count != 1 {
			t.Error("Expected 1 user within transaction")
		}

		// Rollback transaction
		tx.Rollback()

		// Verify data does not exist after rollback
		var finalCount int
		err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE user_id = $1`, userID).Scan(&finalCount)
		if err != nil {
			t.Fatalf("Failed to query after rollback: %v", err)
		}
		if finalCount != 0 {
			t.Error("Expected 0 users after transaction rollback")
		}
	})

	// Cleanup
	cleanupTestTables(ctx, t, db)
}

// TestLoginRenameFixtures_RealWorldScenario demonstrates a realistic login rename
// test using the fixtures, simulating what happens when a GitHub user renames their account.
func TestLoginRenameFixtures_RealWorldScenario(t *testing.T) {
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

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	cleanupTestTables(ctx, t, db)
	if err := setupLoginRenameTables(ctx, db); err != nil {
		t.Fatalf("Failed to setup tables: %v", err)
	}
	defer cleanupTestTables(ctx, t, db)

	t.Run("Complete login rename flow using fixtures", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback()

		fixtures := &LoginRenameFixtures{Tx: tx}

		// Setup initial state: user has activity under old-name
		email := "developer@example.com"
		oldLogin := "old-developer-name"
		newLogin := "new-developer-name"

		// Seed complete scenario
		originalUserID, repoID := fixtures.SeedLoginRenameScenario(ctx, t, email, oldLogin)

		// Verify initial state
		var initialLogin string
		err = tx.QueryRowContext(ctx, `SELECT login FROM users WHERE user_id = $1`, originalUserID).Scan(&initialLogin)
		if err != nil {
			t.Fatalf("Failed to query initial login: %v", err)
		}

		// Simulate rename detection and update
		// Update email_resolution
		_, err = tx.ExecContext(ctx, `
			INSERT INTO email_resolution (email, login, source, resolved_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (email) DO UPDATE
			  SET login = excluded.login,
			      source = excluded.source,
			      resolved_at = excluded.resolved_at
		`, email, newLogin, "live", time.Now())
		if err != nil {
			t.Fatalf("Failed to update email_resolution: %v", err)
		}

		// Update users.login in place
		_, err = tx.ExecContext(ctx, `
			UPDATE users SET login = $1 WHERE login = $2
		`, newLogin, oldLogin)
		if err != nil {
			t.Fatalf("Failed to update users.login: %v", err)
		}

		// Verify user_id is preserved
		var updatedUserID int64
		var updatedLogin string
		err = tx.QueryRowContext(ctx, `
			SELECT user_id, login FROM users WHERE user_id = $1
		`, originalUserID).Scan(&updatedUserID, &updatedLogin)
		if err != nil {
			t.Fatalf("Failed to query updated user: %v", err)
		}

		if updatedUserID != originalUserID {
			t.Errorf("user_id changed: %d -> %d (HISTORY FRAGMENTATION!)", originalUserID, updatedUserID)
		}
		if updatedLogin != newLogin {
			t.Errorf("login not updated to %s, got %s", newLogin, updatedLogin)
		}

		// Verify historical activity still linked to same user_id
		var activityCount int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM repo_user_daily_tool WHERE user_id = $1
		`, originalUserID).Scan(&activityCount)
		if err != nil {
			t.Fatalf("Failed to count activity: %v", err)
		}
		if activityCount != 3 {
			t.Errorf("Historical activity count changed: expected 3, got %d", activityCount)
		}

		t.Logf("✓ Login rename test passed: user_id=%d preserved from %s -> %s", originalUserID, oldLogin, newLogin)
	})
}
