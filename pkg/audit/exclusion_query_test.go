package audit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestDB creates a test PostgreSQL container and applies migrations.
// Returns the DB connection and a cleanup function.
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	ctx := context.Background()

	// Start PostgreSQL test container
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		wait.ForLog("database system is ready to accept connections"),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Connect to database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Apply schema for exclusion_audit_log
	schema := `
		CREATE TABLE IF NOT EXISTS repos (
			repo_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			provider TEXT NOT NULL,
			repo_full_name TEXT NOT NULL UNIQUE
		);

		CREATE TABLE IF NOT EXISTS exclusion_audit_log (
			id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			repo_id         BIGINT NOT NULL REFERENCES repos(repo_id) ON DELETE CASCADE,
			actor           TEXT NOT NULL,
			timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			event_type      TEXT NOT NULL,
			old_excluded_at     TIMESTAMPTZ,
			old_excluded_reason TEXT,
			new_excluded_at     TIMESTAMPTZ,
			new_excluded_reason TEXT
		);

		CREATE INDEX IF NOT EXISTS exclusion_audit_log_timestamp_idx ON exclusion_audit_log (timestamp DESC);
		CREATE INDEX IF NOT EXISTS exclusion_audit_log_repo_idx ON exclusion_audit_log (repo_id, timestamp DESC);
		CREATE INDEX IF NOT EXISTS exclusion_audit_log_actor_idx ON exclusion_audit_log (actor, timestamp DESC);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		pgContainer.Terminate(ctx)
		t.Fatalf("failed to create schema: %v", err)
	}

	cleanup := func() {
		db.Close()
		pgContainer.Terminate(ctx)
	}

	return db, cleanup
}

// insertTestRepo creates a test repository and returns its ID.
func insertTestRepo(t *testing.T, db *sql.DB, provider, repoFullName string) int64 {
	var repoID int64
	err := db.QueryRow(
		"INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id",
		provider, repoFullName,
	).Scan(&repoID)
	if err != nil {
		t.Fatalf("failed to insert test repo: %v", err)
	}
	return repoID
}

// insertTestAuditLog inserts a test audit log entry.
func insertTestAuditLog(t *testing.T, db *sql.DB, repoID int64, actor, eventType string, timestamp time.Time, newExcludedAt *time.Time, newExcludedReason *string) {
	var oldExcludedAt, oldExcludedReason interface{}
	_, err := db.Exec(`
		INSERT INTO exclusion_audit_log (repo_id, actor, timestamp, event_type, old_excluded_at, old_excluded_reason, new_excluded_at, new_excluded_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, repoID, actor, timestamp, eventType, oldExcludedAt, oldExcludedReason, newExcludedAt, newExcludedReason)
	if err != nil {
		t.Fatalf("failed to insert test audit log: %v", err)
	}
}

func TestQueryExclusionAuditLogs_Basic(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repos
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")
	repo2 := insertTestRepo(t, db, "github", "owner/repo2")

	// Insert test audit logs
	now := time.Now().UTC()
	excludedAt := now.Add(-24 * time.Hour)
	reason := "test reason"

	insertTestAuditLog(t, db, repo1, "admin", "exclude", now, &excludedAt, &reason)
	insertTestAuditLog(t, db, repo2, "admin", "exclude", now.Add(-1*time.Hour), &excludedAt, &reason)

	// Query all logs
	opts := ExclusionAuditQueryOptions{Limit: 10}
	records, err := querier.QueryExclusionAuditLogs(ctx, opts)
	if err != nil {
		t.Fatalf("QueryExclusionAuditLogs failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(records))
	}
}

func TestQueryExclusionAuditLogs_ByRepoID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repos
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")
	repo2 := insertTestRepo(t, db, "github", "owner/repo2")

	// Insert test audit logs
	now := time.Now().UTC()
	excludedAt := now.Add(-24 * time.Hour)
	reason := "test reason"

	insertTestAuditLog(t, db, repo1, "admin", "exclude", now, &excludedAt, &reason)
	insertTestAuditLog(t, db, repo2, "admin", "exclude", now, &excludedAt, &reason)

	// Query logs for repo1 only
	opts := ExclusionAuditQueryOptions{RepoID: repo1, Limit: 10}
	records, err := querier.QueryExclusionAuditLogs(ctx, opts)
	if err != nil {
		t.Fatalf("QueryExclusionAuditLogs failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record for repo1, got %d", len(records))
	}
	if records[0].RepoID != repo1 {
		t.Errorf("Expected RepoID %d, got %d", repo1, records[0].RepoID)
	}
}

func TestQueryExclusionAuditLogs_ByActor(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repo
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")

	// Insert test audit logs from different actors
	now := time.Now().UTC()
	excludedAt := now.Add(-24 * time.Hour)
	reason := "test reason"

	insertTestAuditLog(t, db, repo1, "admin", "exclude", now, &excludedAt, &reason)
	insertTestAuditLog(t, db, repo1, "operator", "exclude", now.Add(-1*time.Hour), &excludedAt, &reason)

	// Query logs by actor
	opts := ExclusionAuditQueryOptions{Actor: "admin", Limit: 10}
	records, err := querier.QueryExclusionAuditLogs(ctx, opts)
	if err != nil {
		t.Fatalf("QueryExclusionAuditLogs failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record by admin, got %d", len(records))
	}
	if records[0].Actor != "admin" {
		t.Errorf("Expected Actor 'admin', got '%s'", records[0].Actor)
	}
}

func TestQueryExclusionAuditLogs_ByEventType(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repo
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")

	// Insert test audit logs with different event types
	now := time.Now().UTC()
	excludedAt := now.Add(-24 * time.Hour)
	reason := "test reason"

	insertTestAuditLog(t, db, repo1, "admin", "exclude", now, &excludedAt, &reason)
	insertTestAuditLog(t, db, repo1, "admin", "unexclude", now.Add(-1*time.Hour), nil, nil)

	// Query logs by event type
	opts := ExclusionAuditQueryOptions{EventType: "exclude", Limit: 10}
	records, err := querier.QueryExclusionAuditLogs(ctx, opts)
	if err != nil {
		t.Fatalf("QueryExclusionAuditLogs failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 exclude event, got %d", len(records))
	}
	if records[0].EventType != "exclude" {
		t.Errorf("Expected EventType 'exclude', got '%s'", records[0].EventType)
	}
}

func TestQueryExclusionAuditLogs_ByDateRange(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repo
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")

	// Insert test audit logs with different timestamps
	now := time.Now().UTC()
	excludedAt := now.Add(-24 * time.Hour)
	reason := "test reason"

	oldTime := now.Add(-30 * 24 * time.Hour) // 30 days ago
	newTime := now.Add(-1 * 24 * time.Hour) // 1 day ago

	insertTestAuditLog(t, db, repo1, "admin", "exclude", oldTime, &excludedAt, &reason)
	insertTestAuditLog(t, db, repo1, "admin", "exclude", newTime, &excludedAt, &reason)

	// Query logs within date range (last 7 days)
	startDate := now.Add(-7 * 24 * time.Hour)
	opts := ExclusionAuditQueryOptions{StartDate: startDate, Limit: 10}
	records, err := querier.QueryExclusionAuditLogs(ctx, opts)
	if err != nil {
		t.Fatalf("QueryExclusionAuditLogs failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record within date range, got %d", len(records))
	}
}

func TestQueryExclusionAuditLogs_Pagination(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repo
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")

	// Insert multiple audit logs
	now := time.Now().UTC()
	excludedAt := now.Add(-24 * time.Hour)
	reason := "test reason"

	for i := 0; i < 5; i++ {
		timestamp := now.Add(time.Duration(-i) * time.Hour)
		insertTestAuditLog(t, db, repo1, "admin", "exclude", timestamp, &excludedAt, &reason)
	}

	// Query first page with limit 2
	opts := ExclusionAuditQueryOptions{Offset: 0, Limit: 2}
	records, err := querier.QueryExclusionAuditLogs(ctx, opts)
	if err != nil {
		t.Fatalf("QueryExclusionAuditLogs failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records on first page, got %d", len(records))
	}

	// Query second page
	opts = ExclusionAuditQueryOptions{Offset: 2, Limit: 2}
	records, err = querier.QueryExclusionAuditLogs(ctx, opts)
	if err != nil {
		t.Fatalf("QueryExclusionAuditLogs failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Expected 2 records on second page, got %d", len(records))
	}
}

func TestCountExclusionAuditLogs(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repos
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")
	repo2 := insertTestRepo(t, db, "github", "owner/repo2")

	// Insert test audit logs
	now := time.Now().UTC()
	excludedAt := now.Add(-24 * time.Hour)
	reason := "test reason"

	insertTestAuditLog(t, db, repo1, "admin", "exclude", now, &excludedAt, &reason)
	insertTestAuditLog(t, db, repo1, "admin", "unexclude", now.Add(-1*time.Hour), nil, nil)
	insertTestAuditLog(t, db, repo2, "admin", "exclude", now, &excludedAt, &reason)

	// Count all logs
	opts := ExclusionAuditQueryOptions{}
	count, err := querier.CountExclusionAuditLogs(ctx, opts)
	if err != nil {
		t.Fatalf("CountExclusionAuditLogs failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected count of 3, got %d", count)
	}

	// Count by repo
	opts = ExclusionAuditQueryOptions{RepoID: repo1}
	count, err = querier.CountExclusionAuditLogs(ctx, opts)
	if err != nil {
		t.Fatalf("CountExclusionAuditLogs failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected count of 2 for repo1, got %d", count)
	}
}

func TestGetActiveExclusions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repo
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")

	// Insert: exclude, then unexclude
	now := time.Now().UTC()
	excludedAt := now.Add(-24 * time.Hour)
	reason := "test reason"

	insertTestAuditLog(t, db, repo1, "admin", "exclude", now.Add(-2*time.Hour), &excludedAt, &reason)
	insertTestAuditLog(t, db, repo1, "admin", "unexclude", now.Add(-1*time.Hour), nil, nil)

	// Get active exclusions
	records, err := querier.GetActiveExclusions(ctx)
	if err != nil {
		t.Fatalf("GetActiveExclusions failed: %v", err)
	}

	// Repo1 should not be active (was unexcluded)
	for _, rec := range records {
		if rec.RepoID == repo1 {
			t.Errorf("Repo1 should not be in active exclusions (was unexcluded)")
		}
	}

	// Now add another repo that's still excluded
	repo2 := insertTestRepo(t, db, "github", "owner/repo2")
	insertTestAuditLog(t, db, repo2, "admin", "exclude", now, &excludedAt, &reason)

	records, err = querier.GetActiveExclusions(ctx)
	if err != nil {
		t.Fatalf("GetActiveExclusions failed: %v", err)
	}

	// Should have at least repo2
	found := false
	for _, rec := range records {
		if rec.RepoID == repo2 {
			found = true
			if rec.EventType != "exclude" {
				t.Errorf("Expected EventType 'exclude', got '%s'", rec.EventType)
			}
			if rec.NewExcludedAt == nil {
				t.Errorf("Expected NewExcludedAt to be set for active exclusion")
			}
		}
	}
	if !found {
		t.Errorf("Repo2 should be in active exclusions")
	}
}

func TestGetRepoAuditHistory(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repos
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")
	repo2 := insertTestRepo(t, db, "github", "owner/repo2")

	// Insert audit logs for both repos
	now := time.Now().UTC()
	excludedAt := now.Add(-24 * time.Hour)
	reason := "test reason"

	insertTestAuditLog(t, db, repo1, "admin", "exclude", now, &excludedAt, &reason)
	insertTestAuditLog(t, db, repo2, "admin", "exclude", now, &excludedAt, &reason)

	// Get history for repo1
	records, err := querier.GetRepoAuditHistory(ctx, repo1, 0, 10)
	if err != nil {
		t.Fatalf("GetRepoAuditHistory failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record for repo1, got %d", len(records))
	}
	if records[0].RepoID != repo1 {
		t.Errorf("Expected RepoID %d, got %d", repo1, records[0].RepoID)
	}
}

func TestGetActorAuditHistory(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repo
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")

	// Insert audit logs from different actors
	now := time.Now().UTC()
	excludedAt := now.Add(-24 * time.Hour)
	reason := "test reason"

	insertTestAuditLog(t, db, repo1, "admin", "exclude", now, &excludedAt, &reason)
	insertTestAuditLog(t, db, repo1, "operator", "exclude", now.Add(-1*time.Hour), &excludedAt, &reason)

	// Get history for admin
	records, err := querier.GetActorAuditHistory(ctx, "admin", 0, 10)
	if err != nil {
		t.Fatalf("GetActorAuditHistory failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 record for admin, got %d", len(records))
	}
	if records[0].Actor != "admin" {
		t.Errorf("Expected Actor 'admin', got '%s'", records[0].Actor)
	}
}

func TestGetLongstandingExclusions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	querier := NewExclusionAuditQuerier(db)
	ctx := context.Background()

	// Create test repos
	repo1 := insertTestRepo(t, db, "github", "owner/repo1")
	repo2 := insertTestRepo(t, db, "github", "owner/repo2")

	// Insert audit logs
	now := time.Now().UTC()
	reason := "test reason"

	// Repo1: excluded 10 days ago (should not appear if minDuration is 30 days)
	excludedAt1 := now.Add(-10 * 24 * time.Hour)
	insertTestAuditLog(t, db, repo1, "admin", "exclude", now.Add(-10*24*time.Hour), &excludedAt1, &reason)

	// Repo2: excluded 40 days ago (should appear)
	excludedAt2 := now.Add(-40 * 24 * time.Hour)
	insertTestAuditLog(t, db, repo2, "admin", "exclude", now.Add(-40*24*time.Hour), &excludedAt2, &reason)

	// Get longstanding exclusions (> 30 days)
	minDuration := 30 * 24 * time.Hour
	exclusions, err := querier.GetLongstandingExclusions(ctx, minDuration)
	if err != nil {
		t.Fatalf("GetLongstandingExclusions failed: %v", err)
	}

	// Should have at least repo2
	found := false
	for _, exc := range exclusions {
		if exc.RepoID == repo2 {
			found = true
			if exc.Duration < minDuration {
				t.Errorf("Expected duration >= %v, got %v", minDuration, exc.Duration)
			}
		}
		if exc.RepoID == repo1 {
			t.Errorf("Repo1 should not be in longstanding exclusions (only 10 days)")
		}
	}
	if !found {
		t.Errorf("Repo2 should be in longstanding exclusions")
	}
}
