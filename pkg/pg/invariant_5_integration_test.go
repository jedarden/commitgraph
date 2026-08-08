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

// setupInvariant5TestDB starts a real Postgres container, applies the
// initial schema needed for invariant 5 tests, and returns an open *sql.DB
// plus a cleanup function.
func setupInvariant5TestDB(t *testing.T) (*sql.DB, func()) {
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

	// Create the required schema
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

		CREATE TABLE IF NOT EXISTS repo_user_daily_tool (
			repo_id     BIGINT NOT NULL REFERENCES repos(repo_id),
			user_id     BIGINT NOT NULL REFERENCES users(user_id),
			tool        TEXT   NOT NULL,
			day         DATE   NOT NULL,
			commits     INT    NOT NULL,
			insert_time TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (repo_id, user_id, tool, day)
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

// runInvariant5Query executes the invariant 5 query and returns the count
// of violations (repos with mixed insert_time values).
func runInvariant5Query(ctx context.Context, db *sql.DB) (int, error) {
	query := `
		SELECT
			rut.repo_id,
			COUNT(DISTINCT rut.insert_time) AS distinct_insert_time_count
		FROM repo_user_daily_tool rut
		JOIN repos r ON rut.repo_id = r.repo_id
		GROUP BY rut.repo_id
		HAVING COUNT(DISTINCT rut.insert_time) > 1
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		var repoID int64
		var distinctCount int
		if err := rows.Scan(&repoID, &distinctCount); err != nil {
			return 0, err
		}
	}

	return count, nil
}

// TestInvariant5_Integration_ValidWritePath tests that a correct write path
// (DELETE + bulk INSERT in a single transaction) produces uniform insert_time.
func TestInvariant5_Integration_ValidWritePath(t *testing.T) {
	db, cleanup := setupInvariant5TestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Insert user and repo
	var userID int64
	err := db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, "test-user-inv5").Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var repoID int64
	err = db.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`, "github", "test/repo-inv5").Scan(&repoID)
	if err != nil {
		t.Fatalf("insert repo: %v", err)
	}

	// Simulate the real write path: whole-slice DELETE + bulk INSERT in one transaction
	scanTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	// DELETE existing rows for this repo (whole-slice replace)
	_, err = tx.ExecContext(ctx, `DELETE FROM repo_user_daily_tool WHERE repo_id = $1`, repoID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("delete existing rows: %v", err)
	}

	// Bulk INSERT new rollup rows with uniform insert_time
	_, err = tx.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES
			($1, $2, $3, $4, $5, $6),
			($1, $2, $7, $8, $9, $6),
			($1, $2, $10, $11, $12, $6)
	`, repoID, userID, "claude", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 5, scanTime,
		"copilot", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), 3,
		"claude", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), 7)

	if err != nil {
		tx.Rollback()
		t.Fatalf("bulk insert: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Run invariant query - should return 0 violations
	violationCount, err := runInvariant5Query(ctx, db)
	if err != nil {
		t.Fatalf("run invariant query: %v", err)
	}

	if violationCount != 0 {
		t.Errorf("Expected 0 violations after valid write path, got %d", violationCount)
	}

	// Verify all rows have the same insert_time
	var distinctInsertTimeCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT insert_time)
		FROM repo_user_daily_tool
		WHERE repo_id = $1
	`, repoID).Scan(&distinctInsertTimeCount)
	if err != nil {
		t.Fatalf("count distinct insert_time: %v", err)
	}

	if distinctInsertTimeCount != 1 {
		t.Errorf("Expected 1 distinct insert_time, got %d", distinctInsertTimeCount)
	}

	t.Logf("✓ Valid write path produces uniform insert_time (1 distinct value)")
}

// TestInvariant5_Integration_RollbackNoPartialWrite tests that a rolled-back
// transaction leaves no partial rows visible, ensuring atomicity.
func TestInvariant5_Integration_RollbackNoPartialWrite(t *testing.T) {
	db, cleanup := setupInvariant5TestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Insert user and repo
	var userID int64
	err := db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, "test-user-rollback").Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var repoID int64
	err = db.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`, "github", "test/repo-rollback").Scan(&repoID)
	if err != nil {
		t.Fatalf("insert repo: %v", err)
	}

	// First, write a valid set of rows
	scanTime1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM repo_user_daily_tool WHERE repo_id = $1`, repoID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("delete: %v", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repoID, userID, "claude", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 5, scanTime1)

	if err != nil {
		tx.Rollback()
		t.Fatalf("insert: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Now simulate a failed mid-write transaction
	// This transaction starts DELETE+INSERT but rolls back before commit
	scanTime2 := time.Date(2024, 1, 16, 11, 0, 0, 0, time.UTC)

	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx 2: %v", err)
	}

	// DELETE existing rows
	_, err = tx.ExecContext(ctx, `DELETE FROM repo_user_daily_tool WHERE repo_id = $1`, repoID)
	if err != nil {
		tx.Rollback()
		t.Fatalf("delete 2: %v", err)
	}

	// INSERT some rows with different insert_time
	_, err = tx.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES
			($1, $2, $3, $4, $5, $6),
			($1, $2, $7, $8, $9, $6)
	`, repoID, userID, "claude", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), 3, scanTime2,
		"copilot", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), 2)

	if err != nil {
		tx.Rollback()
		t.Fatalf("insert 2: %v", err)
	}

	// Simulate an error - rollback the transaction
	if err := tx.Rollback(); err != nil {
		t.Logf("rollback (expected, simulating failure): %v", err)
	}

	// Verify the rollback worked - no partial rows with scanTime2 should exist
	var countWithScanTime2 int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM repo_user_daily_tool
		WHERE repo_id = $1 AND insert_time = $2
	`, repoID, scanTime2).Scan(&countWithScanTime2)
	if err != nil {
		t.Fatalf("count rows with scanTime2: %v", err)
	}

	if countWithScanTime2 != 0 {
		t.Errorf("Expected 0 rows with scanTime2 after rollback, got %d", countWithScanTime2)
	}

	// Verify invariant still passes (only the original scanTime1 rows exist)
	violationCount, err := runInvariant5Query(ctx, db)
	if err != nil {
		t.Fatalf("run invariant query: %v", err)
	}

	if violationCount != 0 {
		t.Errorf("Expected 0 violations after rollback, got %d", violationCount)
	}

	// Verify all rows still have the original insert_time
	var distinctInsertTimeCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT insert_time)
		FROM repo_user_daily_tool
		WHERE repo_id = $1
	`, repoID).Scan(&distinctInsertTimeCount)
	if err != nil {
		t.Fatalf("count distinct insert_time: %v", err)
	}

	if distinctInsertTimeCount != 1 {
		t.Errorf("Expected 1 distinct insert_time after rollback, got %d", distinctInsertTimeCount)
	}

	t.Logf("✓ Rolled-back transaction leaves no partial rows (atomicity preserved)")
}

// TestInvariant5_Integration_MixedInsertTimeViolation tests that the
// invariant correctly detects when a repo has rows with different insert_time values.
func TestInvariant5_Integration_MixedInsertTimeViolation(t *testing.T) {
	db, cleanup := setupInvariant5TestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Insert user and repo
	var userID int64
	err := db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, "test-user-violation").Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var repoID int64
	err = db.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`, "github", "test/repo-violation").Scan(&repoID)
	if err != nil {
		t.Fatalf("insert repo: %v", err)
	}

	// Insert rows with one insert_time
	scanTime1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES
			($1, $2, $3, $4, $5, $6),
			($1, $2, $7, $8, $9, $6),
			($1, $2, $10, $11, $12, $6)
	`, repoID, userID, "claude", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 5, scanTime1,
		"copilot", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), 3,
		"claude", time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC), 7)

	if err != nil {
		t.Fatalf("insert first batch: %v", err)
	}

	// Insert MORE rows for the SAME repo with a DIFFERENT insert_time (violation!)
	// This simulates a concurrent write or manual insertion outside the write path
	scanTime2 := time.Date(2024, 1, 16, 11, 30, 0, 0, time.UTC)

	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES
			($1, $2, $3, $4, $5, $6),
			($1, $2, $7, $8, $9, $6)
	`, repoID, userID, "claude", time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC), 4, scanTime2,
		"copilot", time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), 2)

	if err != nil {
		t.Fatalf("insert second batch: %v", err)
	}

	// Run invariant query - should detect the violation
	violationCount, err := runInvariant5Query(ctx, db)
	if err != nil {
		t.Fatalf("run invariant query: %v", err)
	}

	if violationCount != 1 {
		t.Errorf("Expected 1 violation (repo_id=%d), got %d", repoID, violationCount)
	}

	// Verify we have exactly 2 distinct insert_time values
	var distinctInsertTimeCount int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT insert_time)
		FROM repo_user_daily_tool
		WHERE repo_id = $1
	`, repoID).Scan(&distinctInsertTimeCount)
	if err != nil {
		t.Fatalf("count distinct insert_time: %v", err)
	}

	if distinctInsertTimeCount != 2 {
		t.Errorf("Expected 2 distinct insert_time values for violation, got %d", distinctInsertTimeCount)
	}

	// Get diagnostic details
	var diagnostic struct {
		RepoID                 int64
		DistinctInsertTimeCount int
		TotalRows              int
	}

	err = db.QueryRowContext(ctx, `
		SELECT
			repo_id,
			COUNT(DISTINCT insert_time) AS distinct_insert_time_count,
			COUNT(*) AS total_rows
		FROM repo_user_daily_tool
		WHERE repo_id = $1
		GROUP BY repo_id
	`, repoID).Scan(&diagnostic.RepoID, &diagnostic.DistinctInsertTimeCount, &diagnostic.TotalRows)

	if err != nil {
		t.Fatalf("query diagnostic details: %v", err)
	}

	t.Logf("✓ Violation detected correctly:")
	t.Logf("  repo_id=%d has %d distinct insert_time values across %d rows",
		diagnostic.RepoID, diagnostic.DistinctInsertTimeCount, diagnostic.TotalRows)
}

// TestInvariant5_Integration_MultipleRepos tests that the invariant
// correctly handles multiple repos with different states.
func TestInvariant5_Integration_MultipleRepos(t *testing.T) {
	db, cleanup := setupInvariant5TestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Insert user
	var userID int64
	err := db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, "test-user-multi").Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Create three repos:
	// - repo1: uniform insert_time (should pass)
	// - repo2: mixed insert_time (should violate)
	// - repo3: uniform insert_time (should pass)

	repos := []struct {
		id     int64
		name   string
		violate bool
	}{
		{1, "test/repo-uniform-1", false},
		{2, "test/repo-violation", true},
		{3, "test/repo-uniform-2", false},
	}

	for _, repo := range repos {
		_, err := db.ExecContext(ctx, `INSERT INTO repos (repo_id, provider, repo_full_name) VALUES ($1, $2, $3)`, repo.id, "github", repo.name)
		if err != nil {
			t.Fatalf("insert repo %d: %v", repo.id, err)
		}
	}

	// Write uniform data for repo1
	scanTime1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6), ($1, $2, $7, $8, $9, $6)
	`, repos[0].id, userID, "claude", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 5, scanTime1,
		"copilot", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), 3)

	if err != nil {
		t.Fatalf("insert repo1 data: %v", err)
	}

	// Write mixed data for repo2 (violation)
	scanTime2a := time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repos[1].id, userID, "claude", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 5, scanTime2a)

	if err != nil {
		t.Fatalf("insert repo2 batch 1: %v", err)
	}

	scanTime2b := time.Date(2024, 1, 17, 11, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repos[1].id, userID, "copilot", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), 3, scanTime2b)

	if err != nil {
		t.Fatalf("insert repo2 batch 2: %v", err)
	}

	// Write uniform data for repo3
	scanTime3 := time.Date(2024, 1, 18, 12, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6), ($1, $2, $7, $8, $9, $6)
	`, repos[2].id, userID, "claude", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 7, scanTime3,
		"copilot", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), 2)

	if err != nil {
		t.Fatalf("insert repo3 data: %v", err)
	}

	// Run invariant query - should detect exactly 1 violation (repo2)
	violationCount, err := runInvariant5Query(ctx, db)
	if err != nil {
		t.Fatalf("run invariant query: %v", err)
	}

	if violationCount != 1 {
		t.Errorf("Expected 1 violation (repo2), got %d", violationCount)
	}

	// Verify the violating repo is repo2
	var violatingRepoID int64
	err = db.QueryRowContext(ctx, `
		SELECT repo_id
		FROM repo_user_daily_tool
		GROUP BY repo_id
		HAVING COUNT(DISTINCT insert_time) > 1
	`).Scan(&violatingRepoID)

	if err != nil {
		t.Fatalf("query violating repo_id: %v", err)
	}

	if violatingRepoID != repos[1].id {
		t.Errorf("Expected repo_id=%d to violate, got repo_id=%d", repos[1].id, violatingRepoID)
	}

	t.Logf("✓ Multiple repos handled correctly: only repo_id=%d violated (1/3 repos)", violatingRepoID)
}

// TestInvariant5_Integration_RescanIdempotency tests that rescanning the same
// repo produces the same uniform insert_time (idempotent behavior).
func TestInvariant5_Integration_RescanIdempotency(t *testing.T) {
	db, cleanup := setupInvariant5TestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Insert user and repo
	var userID int64
	err := db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, "test-user-idempotent").Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var repoID int64
	err = db.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`, "github", "test/repo-idempotent").Scan(&repoID)
	if err != nil {
		t.Fatalf("insert repo: %v", err)
	}

	// First scan: write initial data
	scanTime1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	tx1, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx1: %v", err)
	}

	_, err = tx1.ExecContext(ctx, `DELETE FROM repo_user_daily_tool WHERE repo_id = $1`, repoID)
	if err != nil {
		tx1.Rollback()
		t.Fatalf("delete tx1: %v", err)
	}

	_, err = tx1.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6), ($1, $2, $7, $8, $9, $6)
	`, repoID, userID, "claude", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 5, scanTime1,
		"copilot", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), 3)

	if err != nil {
		tx1.Rollback()
		t.Fatalf("insert tx1: %v", err)
	}

	if err := tx1.Commit(); err != nil {
		t.Fatalf("commit tx1: %v", err)
	}

	// Verify invariant passes after first scan
	violationCount1, err := runInvariant5Query(ctx, db)
	if err != nil {
		t.Fatalf("run invariant query after first scan: %v", err)
	}

	if violationCount1 != 0 {
		t.Errorf("Expected 0 violations after first scan, got %d", violationCount1)
	}

	// Second scan: re-scan same repo (simulating rescan)
	// This should DELETE old rows and INSERT new ones with a NEW insert_time
	scanTime2 := time.Date(2024, 1, 16, 11, 0, 0, 0, time.UTC)

	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}

	_, err = tx2.ExecContext(ctx, `DELETE FROM repo_user_daily_tool WHERE repo_id = $1`, repoID)
	if err != nil {
		tx2.Rollback()
		t.Fatalf("delete tx2: %v", err)
	}

	_, err = tx2.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6), ($1, $2, $7, $8, $9, $6)
	`, repoID, userID, "claude", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 8, scanTime2,
		"copilot", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), 4)

	if err != nil {
		tx2.Rollback()
		t.Fatalf("insert tx2: %v", err)
	}

	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit tx2: %v", err)
	}

	// Verify invariant still passes after second scan
	violationCount2, err := runInvariant5Query(ctx, db)
	if err != nil {
		t.Fatalf("run invariant query after second scan: %v", err)
	}

	if violationCount2 != 0 {
		t.Errorf("Expected 0 violations after second scan, got %d", violationCount2)
	}

	// Verify all rows now have the new insert_time (scanTime2)
	var distinctInsertTimeCount int
	var currentInsertTime time.Time

	err = db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT insert_time), MAX(insert_time)
		FROM repo_user_daily_tool
		WHERE repo_id = $1
	`, repoID).Scan(&distinctInsertTimeCount, &currentInsertTime)

	if err != nil {
		t.Fatalf("query current state: %v", err)
	}

	if distinctInsertTimeCount != 1 {
		t.Errorf("Expected 1 distinct insert_time after rescan, got %d", distinctInsertTimeCount)
	}

	if !currentInsertTime.Equal(scanTime2) {
		t.Errorf("Expected insert_time to be %v after rescan, got %v", scanTime2, currentInsertTime)
	}

	t.Logf("✓ Rescan is idempotent: old rows replaced, new rows have uniform insert_time")
}
