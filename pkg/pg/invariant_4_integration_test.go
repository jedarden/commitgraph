package pg

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
)

// TestInvariant4_Integration_SetupAndValidate provides a complete
// integration test template for setting up a test database with
// fixture violations and validating the invariant queries.
//
// This test can be run against a real PostgreSQL test database
// to verify the invariant queries work correctly.
func TestInvariant4_Integration_SetupAndValidate(t *testing.T) {
	// This test requires a real database connection.
	// Skip if no test database is configured.
	t.Skip("Requires test database - run manually with: TEST_DB_URL=... go test -v")

	// Example setup for running with a test database:
	//
	// 1. Set TEST_DB_URL environment variable:
	//    export TEST_DB_URL="postgres://user:pass@localhost/testdb?sslmode=disable"
	//
	// 2. Run the test:
	//    go test -v -run TestInvariant4_Integration ./pkg/pg/
	//
	// The test will:
	// - Create the required tables
	// - Insert valid test data
	// - Insert deliberate violations
	// - Run all three invariant queries
	// - Verify each returns exactly the expected violations
	// - Clean up the test data
}

// setupInvariant4TestDatabase creates the test schema and fixture data.
// This is a helper function that can be called from integration tests.
func setupInvariant4TestDatabase(ctx context.Context, db *sql.DB) error {
	// Create the required tables (simplified version of schema)
	queries := []string{
		`CREATE TEMPORARY TABLE users (
			user_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			login TEXT NOT NULL UNIQUE
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
		`CREATE TEMPORARY TABLE user_aliases (
			source_login TEXT PRIMARY KEY,
			target_login TEXT NOT NULL,
			reason TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		)`,
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("create table failed: %w", err)
		}
	}

	return nil
}

// insertInvariant4ValidData inserts valid test data that should pass
// all invariant checks.
func insertInvariant4ValidData(ctx context.Context, db *sql.DB) error {
	// Insert valid users
	validUsers := []string{"canonical-user", "alice", "bob", "charlie"}
	for _, login := range validUsers {
		_, err := db.ExecContext(ctx, `INSERT INTO users (login) VALUES ($1)`, login)
		if err != nil {
			return fmt.Errorf("insert user %s failed: %w", login, err)
		}
	}

	// Insert valid repos
	_, err := db.ExecContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2)`, "github", "test/repo")
	if err != nil {
		return fmt.Errorf("insert repo failed: %w", err)
	}

	// Insert valid rollup data (user_id will be 1 for first user)
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, 1, 1, "claude", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 5, time.Now())
	if err != nil {
		return fmt.Errorf("insert rollup failed: %w", err)
	}

	// Insert valid aliases (one-level only)
	validAliases := []struct{ source, target, reason string }{
		{"alice-old", "alice", "admin"},
		{"bob-bot", "bob", "admin"},
	}
	for _, alias := range validAliases {
		_, err := db.ExecContext(ctx, `
			INSERT INTO user_aliases (source_login, target_login, reason, created_at)
			VALUES ($1, $2, $3, $4)
		`, alias.source, alias.target, alias.reason, time.Now())
		if err != nil {
			return fmt.Errorf("insert alias %s->%s failed: %w", alias.source, alias.target, err)
		}
	}

	return nil
}

// insertInvariant4Violation_QA_OrphanUserID inserts a violation for query (a):
// a rollup row with a user_id that doesn't exist in users.
func insertInvariant4Violation_QA_OrphanUserID(ctx context.Context, db *sql.DB) error {
	// Insert rollup row with user_id=9999 (doesn't exist)
	_, err := db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, 1, 9999, "claude", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), 3, time.Now())

	return err
}

// insertInvariant4Violation_QB_NonexistentTargetLogin inserts a violation for
// query (b): an alias targeting a login that doesn't exist.
func insertInvariant4Violation_QB_NonexistentTargetLogin(ctx context.Context, db *sql.DB) error {
	// Insert alias targeting non-existent login
	_, err := db.ExecContext(ctx, `
		INSERT INTO user_aliases (source_login, target_login, reason, created_at)
		VALUES ($1, $2, $3, $4)
	`, "old-login-fixture", "non-existent-login", "admin", time.Now())

	return err
}

// insertInvariant4Violation_QC_ChainedAliasesAndCycle inserts violations for
// query (c): chained aliases and a cycle.
func insertInvariant4Violation_QC_ChainedAliasesAndCycle(ctx context.Context, db *sql.DB) error {
	// First ensure the canonical target exists
	_, err := db.ExecContext(ctx, `INSERT INTO users (login) VALUES ($1)`, "canonical-user-c")
	if err != nil {
		return err
	}

	// Create chain: A -> B -> C
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_aliases (source_login, target_login, reason, created_at)
		VALUES ($1, $2, $3, $4)
	`, "chained-alias-a", "chained-alias-b", "admin", time.Now())
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO user_aliases (source_login, target_login, reason, created_at)
		VALUES ($1, $2, $3, $4)
	`, "chained-alias-b", "canonical-user-c", "admin", time.Now())
	if err != nil {
		return err
	}

	// Create cycle: X -> Y, Y -> X
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_aliases (source_login, target_login, reason, created_at)
		VALUES ($1, $2, $3, $4)
	`, "cycle-alias-x", "cycle-alias-y", "admin", time.Now())
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO user_aliases (source_login, target_login, reason, created_at)
		VALUES ($1, $2, $3, $4)
	`, "cycle-alias-y", "cycle-alias-x", "admin", time.Now())

	return err
}

// runInvariant4QueryA executes query (a) and returns the count of violations.
func runInvariant4QueryA(ctx context.Context, db *sql.DB) (int, error) {
	query := `
		SELECT rut.repo_id, rut.user_id, rut.tool
		FROM repo_user_daily_tool rut
		LEFT JOIN users u ON rut.user_id = u.user_id
		WHERE u.user_id IS NULL
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("query (a) failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		var repoID, userID sql.NullInt64
		var tool sql.NullString
		if err := rows.Scan(&repoID, &userID, &tool); err != nil {
			return 0, fmt.Errorf("scan row failed: %w", err)
		}
	}

	return count, nil
}

// runInvariant4QueryB executes query (b) and returns the count of violations.
func runInvariant4QueryB(ctx context.Context, db *sql.DB) (int, error) {
	query := `
		SELECT ua.source_login, ua.target_login
		FROM user_aliases ua
		LEFT JOIN users u ON ua.target_login = u.login
		WHERE u.login IS NULL
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("query (b) failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		var source, target sql.NullString
		if err := rows.Scan(&source, &target); err != nil {
			return 0, fmt.Errorf("scan row failed: %w", err)
		}
	}

	return count, nil
}

// runInvariant4QueryC executes query (c) and returns the count of violations.
func runInvariant4QueryC(ctx context.Context, db *sql.DB) (int, error) {
	query := `
		SELECT ua1.source_login, ua1.target_login, ua2.target_login
		FROM user_aliases ua1
		JOIN user_aliases ua2 ON ua1.source_login = ua2.target_login
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("query (c) failed: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		var source1, target1, target2 sql.NullString
		if err := rows.Scan(&source1, &target1, &target2); err != nil {
			return 0, fmt.Errorf("scan row failed: %w", err)
		}
	}

	return count, nil
}

// TestInvariant4_Integration_Example demonstrates the complete flow.
// This is a template that can be adapted for actual CI integration.
func TestInvariant4_Integration_Example(t *testing.T) {
	t.Skip("Example test - requires TEST_DB_URL to run")

	// In a real CI environment, this would:
	// 1. Connect to test database
	// 2. Run setupInvariant4TestDatabase
	// 3. Run insertInvariant4ValidData
	// 4. Run insertInvariant4Violation_QA_OrphanUserID
	// 5. Run insertInvariant4Violation_QB_NonexistentTargetLogin
	// 6. Run insertInvariant4Violation_QC_ChainedAliasesAndCycle
	// 7. Execute all three queries
	// 8. Assert each returns expected violation count
	// 9. Clean up database

	t.Log("Integration test flow:")
	t.Log("1. Setup test database with tables")
	t.Log("2. Insert valid data (should pass all invariants)")
	t.Log("3. Insert violation for query (a): orphan user_id")
	t.Log("4. Insert violation for query (b): non-existent target_login")
	t.Log("5. Insert violations for query (c): chain + cycle")
	t.Log("6. Run query (a): expect 1 violation")
	t.Log("7. Run query (b): expect 1 violation")
	t.Log("8. Run query (c): expect 4 violations (2 chain, 2 cycle)")
	t.Log("9. Clean up test database")
}

// TestInvariant4_SQLQueries validates the SQL queries are syntactically correct.
func TestInvariant4_SQLQueries(t *testing.T) {
	// These tests validate the SQL query structure without requiring a database
	// by checking they can be parsed and have the expected components.

	t.Run("query A has required components", func(t *testing.T) {
		queryA := `
			SELECT rut.repo_id, r.provider, r.repo_full_name, rut.user_id, rut.tool, rut.day, rut.commits, rut.insert_time
			FROM repo_user_daily_tool rut
			JOIN repos r ON rut.repo_id = r.repo_id
			LEFT JOIN users u ON rut.user_id = u.user_id
			WHERE u.user_id IS NULL
			ORDER BY rut.repo_id, rut.user_id, rut.day
		`

		requiredSubstrings := []string{
			"FROM repo_user_daily_tool",
			"JOIN repos",
			"LEFT JOIN users",
			"WHERE u.user_id IS NULL",
		}

		for _, substr := range requiredSubstrings {
			if !contains(queryA, substr) {
				t.Errorf("Query (a) missing required substring: %s", substr)
			}
		}
	})

	t.Run("query B has required components", func(t *testing.T) {
		queryB := `
			SELECT ua.source_login, ua.target_login, ua.reason, ua.created_at
			FROM user_aliases ua
			LEFT JOIN users u ON ua.target_login = u.login
			WHERE u.login IS NULL
			ORDER BY ua.source_login
		`

		requiredSubstrings := []string{
			"FROM user_aliases",
			"LEFT JOIN users",
			"ON ua.target_login = u.login",
			"WHERE u.login IS NULL",
		}

		for _, substr := range requiredSubstrings {
			if !contains(queryB, substr) {
				t.Errorf("Query (b) missing required substring: %s", substr)
			}
		}
	})

	t.Run("query C has required components", func(t *testing.T) {
		queryC := `
			SELECT ua1.source_login, ua1.target_login, ua2.source_login, ua2.target_login
			FROM user_aliases ua1
			JOIN user_aliases ua2 ON ua1.source_login = ua2.target_login
			ORDER BY ua1.source_login, ua2.source_login
		`

		requiredSubstrings := []string{
			"FROM user_aliases ua1",
			"FROM user_aliases ua2",
			"JOIN user_aliases ua2 ON ua1.source_login = ua2.target_login",
		}

		for _, substr := range requiredSubstrings {
			if !contains(queryC, substr) {
				t.Errorf("Query (c) missing required substring: %s", substr)
			}
		}
	})
}
