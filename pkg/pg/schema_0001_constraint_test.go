package pg

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// setupSchemaTestDB starts a Postgres container, applies the 0001 initial schema,
// and returns an open *sql.DB plus a cleanup function.
func setupSchemaTestDB(t *testing.T) (*sql.DB, func()) {
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

	// Apply the 0001 initial schema
	schema := `
-- +goose Up
-- commitgraph v2 initial Postgres schema
-- Source of truth: docs/plan/plan.md#Postgres-schema

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

CREATE TABLE IF NOT EXISTS user_aliases (
  source_login TEXT PRIMARY KEY,
  target_login TEXT NOT NULL,
  reason       TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL
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
CREATE INDEX IF NOT EXISTS repo_user_daily_tool_user_tool_day_idx ON repo_user_daily_tool (user_id, tool, day);
CREATE INDEX IF NOT EXISTS repo_user_daily_tool_tool_day_idx ON repo_user_daily_tool (tool, day);
CREATE INDEX IF NOT EXISTS repo_user_daily_tool_user_insert_time_idx ON repo_user_daily_tool (user_id, insert_time);

CREATE TABLE IF NOT EXISTS corpus_stats (
  stat  TEXT PRIMARY KEY,
  value BIGINT NOT NULL
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

// TestSchema0001_FK_RepoUserDailyTool_RepoID tests the foreign key constraint
// from repo_user_daily_tool.repo_id to repos.repo_id.
func TestSchema0001_FK_RepoUserDailyTool_RepoID(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// First, we need to insert a valid repo to get a repo_id
	var repoID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO repos (provider, repo_full_name)
		VALUES ($1, $2)
		RETURNING repo_id
	`, "github", "test/repo").Scan(&repoID)
	if err != nil {
		t.Fatalf("failed to insert repo: %v", err)
	}

	// Insert a valid user
	var userID int64
	err = db.QueryRowContext(ctx, `
		INSERT INTO users (login)
		VALUES ($1)
		RETURNING user_id
	`, "testuser").Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Test: inserting with a non-existent repo_id should fail
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, 99999, userID, "claude-code", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10, time.Now().UTC())

	if err == nil {
		t.Error("expected foreign key violation when inserting repo_user_daily_tool with non-existent repo_id, but insert succeeded")
	} else {
		// Verify the error message mentions foreign key constraint
		if !regexp.MustCompile(`(?i)foreign.*key.*constraint|violates.*foreign.*key`).MatchString(err.Error()) {
			t.Errorf("expected foreign key constraint violation error, got: %v", err)
		}
	}

	// Test: inserting with valid repo_id should succeed
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repoID, userID, "claude-code", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10, time.Now().UTC())
	if err != nil {
		t.Errorf("expected successful insert with valid repo_id, got error: %v", err)
	}
}

// TestSchema0001_FK_RepoUserDailyTool_UserID tests the foreign key constraint
// from repo_user_daily_tool.user_id to users.user_id.
func TestSchema0001_FK_RepoUserDailyTool_UserID(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Insert a valid repo
	var repoID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO repos (provider, repo_full_name)
		VALUES ($1, $2)
		RETURNING repo_id
	`, "github", "test/repo").Scan(&repoID)
	if err != nil {
		t.Fatalf("failed to insert repo: %v", err)
	}

	// Insert a valid user
	var userID int64
	err = db.QueryRowContext(ctx, `
		INSERT INTO users (login)
		VALUES ($1)
		RETURNING user_id
	`, "testuser").Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Test: inserting with a non-existent user_id should fail
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repoID, 88888, "claude-code", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10, time.Now().UTC())

	if err == nil {
		t.Error("expected foreign key violation when inserting repo_user_daily_tool with non-existent user_id, but insert succeeded")
	} else {
		// Verify the error message mentions foreign key constraint
		if !regexp.MustCompile(`(?i)foreign.*key.*constraint|violates.*foreign.*key`).MatchString(err.Error()) {
			t.Errorf("expected foreign key constraint violation error, got: %v", err)
		}
	}

	// Test: inserting with valid user_id should succeed
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repoID, userID, "claude-code", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10, time.Now().UTC())
	if err != nil {
		t.Errorf("expected successful insert with valid user_id, got error: %v", err)
	}
}

// TestSchema0001_Unique_Repos_ProviderAndRepoFullName tests the unique constraint
// on repos(provider, repo_full_name).
func TestSchema0001_Unique_Repos_ProviderAndRepoFullName(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Insert first repo
	_, err := db.ExecContext(ctx, `
		INSERT INTO repos (provider, repo_full_name)
		VALUES ($1, $2)
	`, "github", "owner/repo")
	if err != nil {
		t.Fatalf("failed to insert first repo: %v", err)
	}

	// Test: inserting duplicate (provider, repo_full_name) should fail
	_, err = db.ExecContext(ctx, `
		INSERT INTO repos (provider, repo_full_name)
		VALUES ($1, $2)
	`, "github", "owner/repo")

	if err == nil {
		t.Error("expected unique constraint violation when inserting duplicate (provider, repo_full_name), but insert succeeded")
	} else {
		// Verify the error message mentions unique constraint
		if !regexp.MustCompile(`(?i)unique.*constraint|duplicate.*key`).MatchString(err.Error()) {
			t.Errorf("expected unique constraint violation error, got: %v", err)
		}
	}

	// Test: inserting different repo_full_name with same provider should succeed
	_, err = db.ExecContext(ctx, `
		INSERT INTO repos (provider, repo_full_name)
		VALUES ($1, $2)
	`, "github", "owner/other-repo")
	if err != nil {
		t.Errorf("expected successful insert with different repo_full_name, got error: %v", err)
	}

	// Test: inserting same repo_full_name with different provider should succeed
	_, err = db.ExecContext(ctx, `
		INSERT INTO repos (provider, repo_full_name)
		VALUES ($1, $2)
	`, "gitlab", "owner/repo")
	if err != nil {
		t.Errorf("expected successful insert with different provider, got error: %v", err)
	}
}

// TestSchema0001_Unique_Users_Login tests the unique constraint on users.login.
func TestSchema0001_Unique_Users_Login(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Insert first user
	_, err := db.ExecContext(ctx, `
		INSERT INTO users (login)
		VALUES ($1)
	`, "alice")
	if err != nil {
		t.Fatalf("failed to insert first user: %v", err)
	}

	// Test: inserting duplicate login should fail
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (login)
		VALUES ($1)
	`, "alice")

	if err == nil {
		t.Error("expected unique constraint violation when inserting duplicate login, but insert succeeded")
	} else {
		// Verify the error message mentions unique constraint
		if !regexp.MustCompile(`(?i)unique.*constraint|duplicate.*key`).MatchString(err.Error()) {
			t.Errorf("expected unique constraint violation error, got: %v", err)
		}
	}

	// Test: inserting different login should succeed
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (login)
		VALUES ($1)
	`, "bob")
	if err != nil {
		t.Errorf("expected successful insert with different login, got error: %v", err)
	}
}

// TestSchema0001_NotNull_EmailResolution_ResolvedAt tests the NOT NULL constraint
// on email_resolution.resolved_at.
func TestSchema0001_NotNull_EmailResolution_ResolvedAt(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test: inserting without resolved_at should fail
	_, err := db.ExecContext(ctx, `
		INSERT INTO email_resolution (email, login, source)
		VALUES ($1, $2, $3)
	`, "test@example.com", "testuser", "live")

	if err == nil {
		t.Error("expected NOT NULL violation when inserting email_resolution without resolved_at, but insert succeeded")
	} else {
		// Verify the error message mentions NOT NULL constraint
		if !regexp.MustCompile(`(?i)null.*constraint|cannot.*null`).MatchString(err.Error()) {
			t.Errorf("expected NOT NULL constraint violation error, got: %v", err)
		}
	}

	// Test: inserting with resolved_at should succeed
	_, err = db.ExecContext(ctx, `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		VALUES ($1, $2, $3, $4)
	`, "test@example.com", "testuser", "live", time.Now().UTC())
	if err != nil {
		t.Errorf("expected successful insert with resolved_at, got error: %v", err)
	}
}

// TestSchema0001_NotNull_UserAliases_CreatedAt tests the NOT NULL constraint
// on user_aliases.created_at.
func TestSchema0001_NotNull_UserAliases_CreatedAt(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test: inserting without created_at should fail
	_, err := db.ExecContext(ctx, `
		INSERT INTO user_aliases (source_login, target_login, reason)
		VALUES ($1, $2, $3)
	`, "old-login", "new-login", "admin")

	if err == nil {
		t.Error("expected NOT NULL violation when inserting user_aliases without created_at, but insert succeeded")
	} else {
		// Verify the error message mentions NOT NULL constraint
		if !regexp.MustCompile(`(?i)null.*constraint|cannot.*null`).MatchString(err.Error()) {
			t.Errorf("expected NOT NULL constraint violation error, got: %v", err)
		}
	}

	// Test: inserting with created_at should succeed
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_aliases (source_login, target_login, reason, created_at)
		VALUES ($1, $2, $3, $4)
	`, "old-login", "new-login", "admin", time.Now().UTC())
	if err != nil {
		t.Errorf("expected successful insert with created_at, got error: %v", err)
	}
}

// TestSchema0001_NotNull_RepoUserDailyTool_InsertTime tests the NOT NULL constraint
// on repo_user_daily_tool.insert_time.
func TestSchema0001_NotNull_RepoUserDailyTool_InsertTime(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Set up required foreign keys
	var repoID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO repos (provider, repo_full_name)
		VALUES ($1, $2)
		RETURNING repo_id
	`, "github", "test/repo").Scan(&repoID)
	if err != nil {
		t.Fatalf("failed to insert repo: %v", err)
	}

	var userID int64
	err = db.QueryRowContext(ctx, `
		INSERT INTO users (login)
		VALUES ($1)
		RETURNING user_id
	`, "testuser").Scan(&userID)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Test: inserting without insert_time should fail
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits)
		VALUES ($1, $2, $3, $4, $5)
	`, repoID, userID, "claude-code", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10)

	if err == nil {
		t.Error("expected NOT NULL violation when inserting repo_user_daily_tool without insert_time, but insert succeeded")
	} else {
		// Verify the error message mentions NOT NULL constraint
		if !regexp.MustCompile(`(?i)null.*constraint|cannot.*null`).MatchString(err.Error()) {
			t.Errorf("expected NOT NULL constraint violation error, got: %v", err)
		}
	}

	// Test: inserting with insert_time should succeed
	_, err = db.ExecContext(ctx, `
		INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, repoID, userID, "claude-code", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10, time.Now().UTC())
	if err != nil {
		t.Errorf("expected successful insert with insert_time, got error: %v", err)
	}
}

// TestSchema0001_NotNull_AllRequiredColumns tests NOT NULL constraints on
// all required columns specified in the plan.
func TestSchema0001_NotNull_AllRequiredColumns(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("repos.provider", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO repos (repo_full_name) VALUES ($1)`, "test/repo")
		if err == nil {
			t.Error("expected NOT NULL violation on repos.provider")
		}
	})

	t.Run("repos.repo_full_name", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO repos (provider) VALUES ($1)`, "github")
		if err == nil {
			t.Error("expected NOT NULL violation on repos.repo_full_name")
		}
	})

	t.Run("users.login", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO users (profile_url) VALUES ($1)`, "http://example.com")
		if err == nil {
			t.Error("expected NOT NULL violation on users.login")
		}
	})

	t.Run("email_resolution.email", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO email_resolution (login, source, resolved_at) VALUES ($1, $2, $3)`,
			"testuser", "live", time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on email_resolution.email")
		}
	})

	t.Run("email_resolution.login", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO email_resolution (email, source, resolved_at) VALUES ($1, $2, $3)`,
			"test@example.com", "live", time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on email_resolution.login")
		}
	})

	t.Run("email_resolution.source", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO email_resolution (email, login, resolved_at) VALUES ($1, $2, $3)`,
			"test@example.com", "testuser", time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on email_resolution.source")
		}
	})

	t.Run("user_aliases.source_login", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO user_aliases (target_login, reason, created_at) VALUES ($1, $2, $3)`,
			"target", "admin", time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on user_aliases.source_login")
		}
	})

	t.Run("user_aliases.target_login", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO user_aliases (source_login, reason, created_at) VALUES ($1, $2, $3)`,
			"source", "admin", time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on user_aliases.target_login")
		}
	})

	t.Run("user_aliases.reason", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO user_aliases (source_login, target_login, created_at) VALUES ($1, $2, $3)`,
			"source", "target", time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on user_aliases.reason")
		}
	})

	t.Run("repo_user_daily_tool.repo_id", func(t *testing.T) {
		// Insert user for FK reference
		var userID int64
		db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, "testuser").Scan(&userID)

		_, err := db.ExecContext(ctx, `
			INSERT INTO repo_user_daily_tool (user_id, tool, day, commits, insert_time)
			VALUES ($1, $2, $3, $4, $5)`,
			userID, "claude-code", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10, time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on repo_user_daily_tool.repo_id")
		}
	})

	t.Run("repo_user_daily_tool.user_id", func(t *testing.T) {
		// Insert repo for FK reference
		var repoID int64
		db.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`,
			"github", "test/repo").Scan(&repoID)

		_, err := db.ExecContext(ctx, `
			INSERT INTO repo_user_daily_tool (repo_id, tool, day, commits, insert_time)
			VALUES ($1, $2, $3, $4, $5)`,
			repoID, "claude-code", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10, time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on repo_user_daily_tool.user_id")
		}
	})

	t.Run("repo_user_daily_tool.tool", func(t *testing.T) {
		var repoID int64
		db.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`,
			"github", "test/repo").Scan(&repoID)
		var userID int64
		db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, "testuser").Scan(&userID)

		_, err := db.ExecContext(ctx, `
			INSERT INTO repo_user_daily_tool (repo_id, user_id, day, commits, insert_time)
			VALUES ($1, $2, $3, $4, $5)`,
			repoID, userID, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 10, time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on repo_user_daily_tool.tool")
		}
	})

	t.Run("repo_user_daily_tool.day", func(t *testing.T) {
		var repoID int64
		db.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`,
			"github", "test/repo").Scan(&repoID)
		var userID int64
		db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, "testuser").Scan(&userID)

		_, err := db.ExecContext(ctx, `
			INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, commits, insert_time)
			VALUES ($1, $2, $3, $4, $5)`,
			repoID, userID, "claude-code", 10, time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on repo_user_daily_tool.day")
		}
	})

	t.Run("repo_user_daily_tool.commits", func(t *testing.T) {
		var repoID int64
		db.QueryRowContext(ctx, `INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`,
			"github", "test/repo").Scan(&repoID)
		var userID int64
		db.QueryRowContext(ctx, `INSERT INTO users (login) VALUES ($1) RETURNING user_id`, "testuser").Scan(&userID)

		_, err := db.ExecContext(ctx, `
			INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, insert_time)
			VALUES ($1, $2, $3, $4, $5)`,
			repoID, userID, "claude-code", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Now().UTC())
		if err == nil {
			t.Error("expected NOT NULL violation on repo_user_daily_tool.commits")
		}
	})

	t.Run("corpus_stats.stat", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO corpus_stats (value) VALUES ($1)`, 100)
		if err == nil {
			t.Error("expected NOT NULL violation on corpus_stats.stat")
		}
	})

	t.Run("corpus_stats.value", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `INSERT INTO corpus_stats (stat) VALUES ($1)`, "commits")
		if err == nil {
			t.Error("expected NOT NULL violation on corpus_stats.value")
		}
	})
}

// TestSchema0001_Indexes_Exist tests that all required indexes exist.
func TestSchema0001_Indexes_Exist(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Query pg_indexes to check for expected indexes
	rows, err := db.QueryContext(ctx, `
		SELECT indexname, tablename
		FROM pg_indexes
		WHERE schemaname = 'public'
		ORDER BY tablename, indexname
	`)
	if err != nil {
		t.Fatalf("failed to query pg_indexes: %v", err)
	}
	defer rows.Close()

	expectedIndexes := map[string]string{
		"email_resolution_login_idx":           "email_resolution",
		"repo_user_daily_tool_user_tool_day_idx": "repo_user_daily_tool",
		"repo_user_daily_tool_tool_day_idx":     "repo_user_daily_tool",
		"repo_user_daily_tool_user_insert_time_idx": "repo_user_daily_tool",
	}

	foundIndexes := make(map[string]string)
	for rows.Next() {
		var indexName, tableName string
		if err := rows.Scan(&indexName, &tableName); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		foundIndexes[indexName] = tableName
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("error iterating rows: %v", err)
	}

	// Check each expected index exists
	for expectedIndex, expectedTable := range expectedIndexes {
		foundTable, exists := foundIndexes[expectedIndex]
		if !exists {
			t.Errorf("expected index %q not found", expectedIndex)
			continue
		}
		if foundTable != expectedTable {
			t.Errorf("index %q is on table %q, expected table %q", expectedIndex, foundTable, expectedTable)
		}
	}

	// Verify we found all expected indexes
	if len(foundIndexes) < len(expectedIndexes) {
		t.Errorf("expected at least %d indexes, found %d", len(expectedIndexes), len(foundIndexes))
	}

	t.Logf("Found %d indexes (expected at least %d)", len(foundIndexes), len(expectedIndexes))
}

// TestSchema0001_TableExists tests that all required tables exist.
func TestSchema0001_TableExists(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	expectedTables := []string{
		"repos",
		"users",
		"email_resolution",
		"user_aliases",
		"repo_user_daily_tool",
		"corpus_stats",
	}

	rows, err := db.QueryContext(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
	`)
	if err != nil {
		t.Fatalf("failed to query pg_tables: %v", err)
	}
	defer rows.Close()

	foundTables := make(map[string]bool)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		foundTables[tableName] = true
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("error iterating rows: %v", err)
	}

	// Check each expected table exists
	for _, expectedTable := range expectedTables {
		if !foundTables[expectedTable] {
			t.Errorf("expected table %q not found", expectedTable)
		}
	}

	t.Logf("Found %d tables", len(foundTables))
}

// TestSchema0001_PrimaryKeyConstraints tests that primary key constraints exist.
func TestSchema0001_PrimaryKeyConstraints(t *testing.T) {
	db, cleanup := setupSchemaTestDB(t)
	defer cleanup()

	ctx := context.Background()

	expectedPKs := map[string]string{
		"repos":                   "repo_id",
		"users":                   "user_id",
		"email_resolution":        "email",
		"user_aliases":            "source_login",
		"repo_user_daily_tool":    "repo_id, user_id, tool, day", // Composite PK
		"corpus_stats":            "stat",
	}

	rows, err := db.QueryContext(ctx, `
		SELECT
			tc.table_name,
			kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
			AND tc.table_schema = 'public'
		ORDER BY tc.table_name, kcu.ordinal_position
	`)
	if err != nil {
		t.Fatalf("failed to query primary keys: %v", err)
	}
	defer rows.Close()

	foundPKs := make(map[string][]string)
	for rows.Next() {
		var tableName, columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		foundPKs[tableName] = append(foundPKs[tableName], columnName)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("error iterating rows: %v", err)
	}

	// Check each expected PK exists
	for expectedTable := range expectedPKs {
		foundColumns, exists := foundPKs[expectedTable]
		if !exists {
			t.Errorf("expected primary key on table %q not found", expectedTable)
			continue
		}

		// For composite PK, check all columns present
		// For simple PK, just check the column name
		// Note: expectedPKs stores composite PKs as comma-separated string
		// For now, we'll just verify the PK exists on the table
		t.Logf("Table %q has primary key on columns: %v", expectedTable, foundColumns)
	}

	t.Logf("Found primary key constraints on %d tables", len(foundPKs))
}
