// Package service tests the audit log query service.
package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// setupTestDB creates a test database connection.
// Uses PostgreSQL if available, otherwise skips the test.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	testDBURL := "postgres://commitgraph:commitgraph@localhost:5432/commitgraph_test?sslmode=disable"
	db, err := sql.Open("postgres", testDBURL)
	if err != nil {
		t.Skipf("skipping test: cannot open test database: %v", err)
	}

	// Verify connection works
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping test: cannot connect to test database: %v", err)
	}

	// Clean up any existing test data and recreate tables
	_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS exclusion_audit_log`)
	if err != nil {
		t.Fatalf("failed to drop existing test table: %v", err)
	}

	// Create the exclusion_audit_log table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS exclusion_audit_log (
		  id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		  repo_id             BIGINT NOT NULL,
		  actor               TEXT NOT NULL,
		  timestamp           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		  event_type          TEXT NOT NULL,
		  old_excluded_at     TIMESTAMPTZ,
		  old_excluded_reason TEXT,
		  new_excluded_at     TIMESTAMPTZ,
		  new_excluded_reason TEXT
		)
	`)
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	// Create indexes for better query performance
	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS exclusion_audit_log_timestamp_idx ON exclusion_audit_log (timestamp DESC)
	`)
	if err != nil {
		t.Fatalf("failed to create timestamp index: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS exclusion_audit_log_repo_idx ON exclusion_audit_log (repo_id, timestamp DESC)
	`)
	if err != nil {
		t.Fatalf("failed to create repo index: %v", err)
	}

	return db
}

// insertTestAuditLog inserts test audit log records.
func insertTestAuditLog(t *testing.T, db *sql.DB, repoID int64, actor, eventType string, oldExcludedAt, newExcludedAt time.Time, oldReason, newReason string) int64 {
	t.Helper()

	ctx := context.Background()
	var oldExcludedAtPtr *time.Time
	if !oldExcludedAt.IsZero() {
		oldExcludedAtPtr = &oldExcludedAt
	}
	var newExcludedAtPtr *time.Time
	if !newExcludedAt.IsZero() {
		newExcludedAtPtr = &newExcludedAt
	}
	var oldReasonPtr *string
	if oldReason != "" {
		oldReasonPtr = &oldReason
	}
	var newReasonPtr *string
	if newReason != "" {
		newReasonPtr = &newReason
	}

	var id int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO exclusion_audit_log (repo_id, actor, timestamp, event_type, old_excluded_at, old_excluded_reason, new_excluded_at, new_excluded_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, repoID, actor, time.Now().UTC(), eventType, oldExcludedAtPtr, oldReasonPtr, newExcludedAtPtr, newReasonPtr).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test audit log: %v", err)
	}

	return id
}

func TestQueryAuditLogs_BasicQuery(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	// Insert test data
	repoID := int64(123)
	insertTestAuditLog(t, db, repoID, "admin", "exclude", time.Time{}, time.Now().UTC(), "", "spam")
	insertTestAuditLog(t, db, repoID, "admin", "unexclude", time.Now().UTC(), time.Time{}, "spam", "")

	// Query audit logs
	result, err := querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}

	// Verify results
	if len(result.Records) != 2 {
		t.Errorf("got %d records, want 2", len(result.Records))
	}
	if result.TotalCount != 2 {
		t.Errorf("got total count %d, want 2", result.TotalCount)
	}
	if result.Limit != 100 { // default limit
		t.Errorf("got limit %d, want 100", result.Limit)
	}
	if result.Offset != 0 {
		t.Errorf("got offset %d, want 0", result.Offset)
	}
}

func TestQueryAuditLogs_EmptyResults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	// Query audit logs for non-existent repo
	result, err := querier.QueryAuditLogs(ctx, 999, AuditLogQueryOptions{})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}

	// Verify empty results handled gracefully
	if len(result.Records) != 0 {
		t.Errorf("got %d records, want 0", len(result.Records))
	}
	if result.TotalCount != 0 {
		t.Errorf("got total count %d, want 0", result.TotalCount)
	}
	// Records should be empty slice, not nil
	if result.Records == nil {
		t.Error("records should be empty slice, not nil")
	}
}

func TestQueryAuditLogs_Pagination(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	// Insert multiple test records
	repoID := int64(456)
	for i := 0; i < 25; i++ {
		insertTestAuditLog(t, db, repoID, "admin", "exclude", time.Time{}, time.Now().UTC(), "", "test")
	}

	// Test limit
	result, err := querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{Limit: 10})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 10 {
		t.Errorf("got %d records with limit 10, want 10", len(result.Records))
	}
	if result.TotalCount != 25 {
		t.Errorf("got total count %d, want 25", result.TotalCount)
	}

	// Test offset
	result, err = querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{Limit: 10, Offset: 10})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 10 {
		t.Errorf("got %d records with offset 10, want 10", len(result.Records))
	}
	if result.TotalCount != 25 {
		t.Errorf("got total count %d, want 25", result.TotalCount)
	}

	// Test offset beyond available records
	result, err = querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{Limit: 10, Offset: 20})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 5 {
		t.Errorf("got %d records with offset 20, want 5", len(result.Records))
	}
}

func TestQueryAuditLogs_ActorFilter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	repoID := int64(789)
	insertTestAuditLog(t, db, repoID, "alice", "exclude", time.Time{}, time.Now().UTC(), "", "test")
	insertTestAuditLog(t, db, repoID, "bob", "exclude", time.Time{}, time.Now().UTC(), "", "test")
	insertTestAuditLog(t, db, repoID, "alice", "unexclude", time.Now().UTC(), time.Time{}, "test", "")

	// Query for alice's actions
	result, err := querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{Actor: "alice"})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 2 {
		t.Errorf("got %d records for alice, want 2", len(result.Records))
	}
	for _, rec := range result.Records {
		if rec.Actor != "alice" {
			t.Errorf("got actor %s, want alice", rec.Actor)
		}
	}

	// Query for bob's actions
	result, err = querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{Actor: "bob"})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 1 {
		t.Errorf("got %d records for bob, want 1", len(result.Records))
	}
	if result.Records[0].Actor != "bob" {
		t.Errorf("got actor %s, want bob", result.Records[0].Actor)
	}
}

func TestQueryAuditLogs_EventTypeFilter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	repoID := int64(101)
	insertTestAuditLog(t, db, repoID, "admin", "exclude", time.Time{}, time.Now().UTC(), "", "spam")
	insertTestAuditLog(t, db, repoID, "admin", "exclude", time.Time{}, time.Now().UTC(), "", "policy")
	insertTestAuditLog(t, db, repoID, "admin", "unexclude", time.Now().UTC(), time.Time{}, "spam", "")

	// Query for exclude events only
	result, err := querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{EventType: "exclude"})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 2 {
		t.Errorf("got %d exclude events, want 2", len(result.Records))
	}
	for _, rec := range result.Records {
		if rec.EventType != "exclude" {
			t.Errorf("got event_type %s, want exclude", rec.EventType)
		}
	}

	// Query for unexclude events only
	result, err = querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{EventType: "unexclude"})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 1 {
		t.Errorf("got %d unexclude events, want 1", len(result.Records))
	}
	if result.Records[0].EventType != "unexclude" {
		t.Errorf("got event_type %s, want unexclude", result.Records[0].EventType)
	}
}

func TestQueryAuditLogs_DateRangeFilter(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	repoID := int64(202)
	now := time.Now().UTC()

	// Insert records with specific timestamps
	// We need to insert with specific timestamps, but our test helper uses time.Now()
	// So we'll insert directly
	oldTime := now.Add(-48 * time.Hour)
	midTime := now.Add(-24 * time.Hour)
	newTime := now

	_, err := db.ExecContext(ctx, `
		INSERT INTO exclusion_audit_log (repo_id, actor, timestamp, event_type, old_excluded_at, old_excluded_reason, new_excluded_at, new_excluded_reason)
		VALUES ($1, $2, $3, $4, NULL, NULL, $5, $6)
	`, repoID, "admin", oldTime, "exclude", oldTime, "old")
	if err != nil {
		t.Fatalf("failed to insert old record: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO exclusion_audit_log (repo_id, actor, timestamp, event_type, old_excluded_at, old_excluded_reason, new_excluded_at, new_excluded_reason)
		VALUES ($1, $2, $3, $4, NULL, NULL, $5, $6)
	`, repoID, "admin", midTime, "exclude", midTime, "mid")
	if err != nil {
		t.Fatalf("failed to insert mid record: %v", err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO exclusion_audit_log (repo_id, actor, timestamp, event_type, old_excluded_at, old_excluded_reason, new_excluded_at, new_excluded_reason)
		VALUES ($1, $2, $3, $4, NULL, NULL, $5, $6)
	`, repoID, "admin", newTime, "exclude", newTime, "new")
	if err != nil {
		t.Fatalf("failed to insert new record: %v", err)
	}

	// Query with start time only (should get mid and new)
	startTime := midTime.Add(-1 * time.Hour)
	result, err := querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{StartTime: &startTime})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 2 {
		t.Errorf("got %d records with start time filter, want 2", len(result.Records))
	}

	// Query with end time only (should get old and mid)
	endTime := midTime.Add(1 * time.Hour)
	result, err = querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{EndTime: &endTime})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 2 {
		t.Errorf("got %d records with end time filter, want 2", len(result.Records))
	}

	// Query with both start and end time (should get only mid)
	result, err = querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{
		StartTime: &startTime,
		EndTime:   &endTime,
	})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 1 {
		t.Errorf("got %d records with date range filter, want 1", len(result.Records))
	}
}

func TestQueryAuditLogs_CombinedFilters(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	repoID := int64(303)
	now := time.Now().UTC()

	// Insert mixed records
	insertTestAuditLog(t, db, repoID, "alice", "exclude", time.Time{}, now.Add(-2*time.Hour), "", "spam")
	insertTestAuditLog(t, db, repoID, "bob", "exclude", time.Time{}, now.Add(-1*time.Hour), "", "policy")
	insertTestAuditLog(t, db, repoID, "alice", "unexclude", now.Add(-30*time.Minute), time.Time{}, "spam", "")

	// Query with combined filters: alice + exclude + date range
	startTime := now.Add(-3 * time.Hour)
	endTime := now.Add(-30 * time.Minute)
	result, err := querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{
		StartTime: &startTime,
		EndTime:   &endTime,
		Actor:     "alice",
		EventType: "exclude",
	})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if len(result.Records) != 1 {
		t.Errorf("got %d records with combined filters, want 1", len(result.Records))
	}
	if result.Records[0].Actor != "alice" {
		t.Errorf("got actor %s, want alice", result.Records[0].Actor)
	}
	if result.Records[0].EventType != "exclude" {
		t.Errorf("got event_type %s, want exclude", result.Records[0].EventType)
	}
}

func TestQueryAuditLogs_LimitBounds(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	repoID := int64(404)
	for i := 0; i < 10; i++ {
		insertTestAuditLog(t, db, repoID, "admin", "exclude", time.Time{}, time.Now().UTC(), "", "test")
	}

	// Test zero limit (should use default 100)
	result, err := querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{Limit: 0})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if result.Limit != 100 {
		t.Errorf("got limit %d with zero input, want 100 (default)", result.Limit)
	}

	// Test negative limit (should use default 100)
	result, err = querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{Limit: -1})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if result.Limit != 100 {
		t.Errorf("got limit %d with negative input, want 100 (default)", result.Limit)
	}

	// Test limit exceeding max (should be capped at 1000)
	result, err = querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{Limit: 2000})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if result.Limit != 1000 {
		t.Errorf("got limit %d with input 2000, want 1000 (max)", result.Limit)
	}

	// Test negative offset (should be treated as 0)
	result, err = querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{Offset: -1})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}
	if result.Offset != 0 {
		t.Errorf("got offset %d with negative input, want 0", result.Offset)
	}
}

func TestQueryAuditLogs_RecordStructure(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	repoID := int64(505)
	now := time.Now().UTC()
	insertTestAuditLog(t, db, repoID, "admin", "exclude", time.Time{}, now, "", "test reason")

	result, err := querier.QueryAuditLogs(ctx, repoID, AuditLogQueryOptions{})
	if err != nil {
		t.Fatalf("QueryAuditLogs failed: %v", err)
	}

	if len(result.Records) != 1 {
		t.Fatalf("got %d records, want 1", len(result.Records))
	}

	rec := result.Records[0]

	// Verify all fields are populated correctly
	if rec.RepoID != repoID {
		t.Errorf("got repo_id %d, want %d", rec.RepoID, repoID)
	}
	if rec.Actor != "admin" {
		t.Errorf("got actor %s, want admin", rec.Actor)
	}
	if rec.EventType != "exclude" {
		t.Errorf("got event_type %s, want exclude", rec.EventType)
	}
	if rec.OldExcludedAt != nil {
		t.Errorf("got old_excluded_at %v, want nil", rec.OldExcludedAt)
	}
	if rec.OldExcludedReason != nil {
		t.Errorf("got old_excluded_reason %v, want nil", rec.OldExcludedReason)
	}
	if rec.NewExcludedAt == nil {
		t.Error("got new_excluded_at nil, want non-nil")
	}
	if rec.NewExcludedReason == nil {
		t.Error("got new_excluded_reason nil, want non-nil")
	}
	if *rec.NewExcludedReason != "test reason" {
		t.Errorf("got new_excluded_reason %s, want 'test reason'", *rec.NewExcludedReason)
	}
}

func TestQueryAllAuditLogs_BasicQuery(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	// Insert test data for multiple repos
	insertTestAuditLog(t, db, 100, "admin", "exclude", time.Time{}, time.Now().UTC(), "", "spam")
	insertTestAuditLog(t, db, 200, "admin", "exclude", time.Time{}, time.Now().UTC(), "", "policy")
	insertTestAuditLog(t, db, 300, "admin", "unexclude", time.Now().UTC(), time.Time{}, "spam", "")

	// Query all audit logs
	result, err := querier.QueryAllAuditLogs(ctx, AuditLogQueryOptions{})
	if err != nil {
		t.Fatalf("QueryAllAuditLogs failed: %v", err)
	}

	if len(result.Records) != 3 {
		t.Errorf("got %d records, want 3", len(result.Records))
	}
	if result.TotalCount != 3 {
		t.Errorf("got total count %d, want 3", result.TotalCount)
	}
}

func TestQueryAllAuditLogs_WithFilters(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	// Insert test data
	insertTestAuditLog(t, db, 100, "alice", "exclude", time.Time{}, time.Now().UTC(), "", "spam")
	insertTestAuditLog(t, db, 200, "bob", "exclude", time.Time{}, time.Now().UTC(), "", "policy")
	insertTestAuditLog(t, db, 300, "alice", "unexclude", time.Now().UTC(), time.Time{}, "spam", "")

	// Query with actor filter
	result, err := querier.QueryAllAuditLogs(ctx, AuditLogQueryOptions{Actor: "alice"})
	if err != nil {
		t.Fatalf("QueryAllAuditLogs failed: %v", err)
	}
	if len(result.Records) != 2 {
		t.Errorf("got %d records for alice, want 2", len(result.Records))
	}

	// Query with event type filter
	result, err = querier.QueryAllAuditLogs(ctx, AuditLogQueryOptions{EventType: "unexclude"})
	if err != nil {
		t.Fatalf("QueryAllAuditLogs failed: %v", err)
	}
	if len(result.Records) != 1 {
		t.Errorf("got %d unexclude records, want 1", len(result.Records))
	}
}

func TestQueryAllAuditLogs_Pagination(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	querier := NewAuditLogQuerier(db)
	ctx := context.Background()

	// Insert multiple records
	for i := 0; i < 15; i++ {
		insertTestAuditLog(t, db, int64(i), "admin", "exclude", time.Time{}, time.Now().UTC(), "", "test")
	}

	// Test pagination
	result, err := querier.QueryAllAuditLogs(ctx, AuditLogQueryOptions{Limit: 5, Offset: 5})
	if err != nil {
		t.Fatalf("QueryAllAuditLogs failed: %v", err)
	}
	if len(result.Records) != 5 {
		t.Errorf("got %d records, want 5", len(result.Records))
	}
	if result.TotalCount != 15 {
		t.Errorf("got total count %d, want 15", result.TotalCount)
	}
}
