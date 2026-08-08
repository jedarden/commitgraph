package pg

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupLoginRenameTestDB starts a real Postgres container and applies the full
// initial schema needed for login rename fixture tests.
func setupLoginRenameTestDB(t *testing.T) (*sql.DB, func()) {
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

	// Apply the full initial schema
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

		CREATE TABLE IF NOT EXISTS email_resolution (
			email       TEXT PRIMARY KEY,
			login       TEXT NOT NULL,
			source      TEXT NOT NULL,
			resolved_at TIMESTAMPTZ NOT NULL
		);
		CREATE INDEX IF NOT EXISTS email_resolution_login_idx ON email_resolution (login);

		CREATE TABLE IF NOT EXISTS repo_user_daily_tool (
			repo_id     BIGINT NOT NULL REFERENCES repos(repo_id),
			user_id     BIGINT NOT NULL REFERENCES users(user_id),
			tool        TEXT   NOT NULL,
			day         DATE   NOT NULL,
			commits     INT    NOT NULL,
			insert_time TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (repo_id, user_id, tool, day)
		);
		CREATE INDEX IF NOT EXISTS repo_user_daily_tool_user_tool_day_idx ON repo_user_daily_tool (user_id, tool, day);
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

// TestSeedEmailResolution verifies that SeedEmailResolution creates a valid row.
func TestSeedEmailResolution(t *testing.T) {
	db, cleanup := setupLoginRenameTestDB(t)
	defer cleanup()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Seed an email resolution
	email := "old@example.com"
	login := "old-name"
	source := "seed"
	resolvedAt := time.Now().UTC()

	err = SeedEmailResolution(tx, email, login, source, resolvedAt)
	if err != nil {
		t.Fatalf("SeedEmailResolution failed: %v", err)
	}

	// Verify the row was created
	var retrievedLogin string
	var retrievedSource string
	var retrievedResolvedAt time.Time
	err = tx.QueryRowContext(context.Background(),
		`SELECT login, source, resolved_at FROM email_resolution WHERE email = $1`, email).
		Scan(&retrievedLogin, &retrievedSource, &retrievedResolvedAt)
	if err != nil {
		t.Fatalf("query email_resolution failed: %v", err)
	}

	if retrievedLogin != login {
		t.Errorf("login mismatch: got %q, want %q", retrievedLogin, login)
	}
	if retrievedSource != source {
		t.Errorf("source mismatch: got %q, want %q", retrievedSource, source)
	}
	// Allow small time difference (microseconds)
	if retrievedResolvedAt.Round(time.Second) != resolvedAt.Round(time.Second) {
		t.Errorf("resolved_at mismatch: got %v, want %v", retrievedResolvedAt, resolvedAt)
	}
}

// TestSeedUser verifies that SeedUser creates a valid row and returns the user_id.
func TestSeedUser(t *testing.T) {
	db, cleanup := setupLoginRenameTestDB(t)
	defer cleanup()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	login := "old-name"
	userID, err := SeedUser(tx, login, "https://github.com/old-name", "https://avatar.url/old")
	if err != nil {
		t.Fatalf("SeedUser failed: %v", err)
	}

	// Verify the user_id is non-zero
	if userID == 0 {
		t.Error("expected non-zero user_id, got 0")
	}

	// Verify the row was created
	var retrievedLogin string
	err = tx.QueryRowContext(context.Background(),
		`SELECT login FROM users WHERE user_id = $1`, userID).
		Scan(&retrievedLogin)
	if err != nil {
		t.Fatalf("query users failed: %v", err)
	}

	if retrievedLogin != login {
		t.Errorf("login mismatch: got %q, want %q", retrievedLogin, login)
	}
}

// TestSeedRepoUserDailyTool verifies that SeedRepoUserDailyTool creates valid rows.
func TestSeedRepoUserDailyTool(t *testing.T) {
	db, cleanup := setupLoginRenameTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// First create prerequisite repos and users
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	var repoID, userID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`,
		"github", "test/repo").Scan(&repoID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("create repo: %v", err)
	}

	err = tx.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`,
		"testuser").Scan(&userID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("create user: %v", err)
	}

	// Seed historical activity
	now := time.Now().UTC()
	days := []time.Time{
		now.Add(-48 * time.Hour),
		now.Add(-24 * time.Hour),
		now,
	}

	err = SeedRepoUserDailyTool(tx, repoID, userID, "claude", days, 5)
	if err != nil {
		tx.Rollback()
		t.Fatalf("SeedRepoUserDailyTool failed: %v", err)
	}

	// Verify the rows were created
	var count int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM repo_user_daily_tool WHERE repo_id = $1 AND user_id = $2 AND tool = 'claude'`,
		repoID, userID).Scan(&count)
	if err != nil {
		tx.Rollback()
		t.Fatalf("count repo_user_daily_tool failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 rows, got %d", count)
	}

	tx.Commit()
}

// TestSeedLoginRenameScenario verifies the complete fixture creation scenario.
func TestSeedLoginRenameScenario(t *testing.T) {
	db, cleanup := setupLoginRenameTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Create the complete scenario
	email := "old@example.com"
	oldLogin := "old-name"
	tool := "claude"

	fixtures, err := SeedLoginRenameScenario(tx, email, oldLogin, tool)
	if err != nil {
		t.Fatalf("SeedLoginRenameScenario failed: %v", err)
	}

	// Verify fixtures structure
	if fixtures.Email != email {
		t.Errorf("email mismatch: got %q, want %q", fixtures.Email, email)
	}
	if fixtures.OldLogin != oldLogin {
		t.Errorf("old login mismatch: got %q, want %q", fixtures.OldLogin, oldLogin)
	}
	if fixtures.UserID == 0 {
		t.Error("expected non-zero user_id")
	}
	if fixtures.RepoID == 0 {
		t.Error("expected non-zero repo_id")
	}
	if fixtures.Tool != tool {
		t.Errorf("tool mismatch: got %q, want %q", fixtures.Tool, tool)
	}
	if len(fixtures.DayData) != 3 {
		t.Errorf("expected 3 days of data, got %d", len(fixtures.DayData))
	}

	// Verify email_resolution exists
	var erLogin string
	err = tx.QueryRowContext(ctx, `SELECT login FROM email_resolution WHERE email = $1`, email).
		Scan(&erLogin)
	if err != nil {
		t.Errorf("email_resolution not found: %v", err)
	}
	if erLogin != oldLogin {
		t.Errorf("email_resolution login mismatch: got %q, want %q", erLogin, oldLogin)
	}

	// Verify user exists
	var userLogin string
	err = tx.QueryRowContext(ctx, `SELECT login FROM users WHERE user_id = $1`, fixtures.UserID).
		Scan(&userLogin)
	if err != nil {
		t.Errorf("user not found: %v", err)
	}
	if userLogin != oldLogin {
		t.Errorf("user login mismatch: got %q, want %q", userLogin, oldLogin)
	}

	// Verify historical activity exists
	var activityCount int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM repo_user_daily_tool WHERE user_id = $1`,
		fixtures.UserID).Scan(&activityCount)
	if err != nil {
		t.Errorf("activity count failed: %v", err)
	}
	if activityCount != 3 {
		t.Errorf("expected 3 activity rows, got %d", activityCount)
	}
}

// TestAssertLoginRenameFixturesIntact verifies the integrity check works.
func TestAssertLoginRenameFixturesIntact(t *testing.T) {
	db, cleanup := setupLoginRenameTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	// Create fixtures
	fixtures, err := SeedLoginRenameScenario(tx, "old@example.com", "old-name", "claude")
	if err != nil {
		tx.Rollback()
		t.Fatalf("seed fixtures: %v", err)
	}

	// Commit so the data persists for the integrity check
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit fixtures: %v", err)
	}

	// Verify fixtures are intact (should pass)
	err = AssertLoginRenameFixturesIntact(db, fixtures)
	if err != nil {
		t.Errorf("AssertLoginRenameFixturesIntact failed on intact fixtures: %v", err)
	}

	// Now simulate a rename by updating the login
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	_, err = tx2.ExecContext(ctx, `UPDATE users SET login = $1 WHERE user_id = $2`, "new-name", fixtures.UserID)
	if err != nil {
		tx2.Rollback()
		t.Fatalf("update login: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit rename: %v", err)
	}

	// Verify fixtures are still intact (user_id and data should be unchanged)
	err = AssertLoginRenameFixturesIntact(db, fixtures)
	if err != nil {
		t.Errorf("AssertLoginRenameFixturesIntact failed after rename (user_id should be stable): %v", err)
	}
}

// TestFixtureRollback verifies that fixtures can be rolled back.
func TestFixtureRollback(t *testing.T) {
	db, cleanup := setupLoginRenameTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create fixtures in a transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	fixtures, err := SeedLoginRenameScenario(tx, "rollback@example.com", "rollback-test", "cursor")
	if err != nil {
		tx.Rollback()
		t.Fatalf("seed fixtures: %v", err)
	}

	// Verify data exists before rollback
	var count int
	err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE user_id = $1`, fixtures.UserID).
		Scan(&count)
	if err != nil {
		tx.Rollback()
		t.Fatalf("count before rollback: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 user before rollback, got %d", count)
	}

	// Rollback the transaction
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// Verify data no longer exists after rollback
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE user_id = $1`, fixtures.UserID).
		Scan(&count)
	if err != nil {
		t.Fatalf("count after rollback: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 users after rollback, got %d", count)
	}
}

// TestReusableFixtures verifies that fixtures can be created multiple times
// with different parameters.
func TestReusableFixtures(t *testing.T) {
	db, cleanup := setupLoginRenameTestDB(t)
	defer cleanup()

	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	// Create first scenario
	fixtures1, err := SeedLoginRenameScenario(tx, "user1@example.com", "user-one", "claude")
	if err != nil {
		t.Fatalf("first scenario: %v", err)
	}

	// Create second scenario with different parameters
	fixtures2, err := SeedLoginRenameScenario(tx, "user2@example.com", "user-two", "copilot")
	if err != nil {
		t.Fatalf("second scenario: %v", err)
	}

	// Verify they have different IDs
	if fixtures1.UserID == fixtures2.UserID {
		t.Error("different users got the same user_id")
	}
	if fixtures1.RepoID == fixtures2.RepoID {
		t.Error("different repos got the same repo_id")
	}

	// Verify each has 3 days of activity
	var count1, count2 int
	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM repo_user_daily_tool WHERE user_id = $1`, fixtures1.UserID).
		Scan(&count1)
	if err != nil {
		t.Fatalf("count activity for user1: %v", err)
	}

	err = tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM repo_user_daily_tool WHERE user_id = $1`, fixtures2.UserID).
		Scan(&count2)
	if err != nil {
		t.Fatalf("count activity for user2: %v", err)
	}

	if count1 != 3 {
		t.Errorf("expected 3 activity rows for user1, got %d", count1)
	}
	if count2 != 3 {
		t.Errorf("expected 3 activity rows for user2, got %d", count2)
	}
}
