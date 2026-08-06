package pg

import (
	"context"
	"testing"
	"time"
)

// TestUpsertAliasesEmpty tests that empty row slice returns nil.
func TestUpsertAliasesEmpty(t *testing.T) {
	// Use mockDBExecutor which implements DBExecutor interface
	db := &mockDBExecutor{}
	ingester := NewAliasIngester(db)
	err := ingester.UpsertAliases(context.Background(), []AliasRow{})
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

// TestUpsertAliasesQuery tests the SQL query construction and parameters.
func TestUpsertAliasesQuery(t *testing.T) {
	db := &mockDBExecutor{rowsAffected: 2}
	ingester := NewAliasIngester(db)

	now := time.Now()
	rows := []AliasRow{
		{SourceLogin: "old-johndoe", TargetLogin: "johndoe", Reason: "admin", CreatedAt: now},
		{SourceLogin: "jane-bot", TargetLogin: "jane", Reason: "admin", CreatedAt: now},
	}

	err := ingester.UpsertAliases(context.Background(), rows)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// Verify query was executed
	if db.lastQuery == "" {
		t.Fatal("query was not executed")
	}

	expectedSubstrings := []string{
		"INSERT INTO user_aliases",
		"SELECT unnest",
		"ON CONFLICT (source_login) DO UPDATE",
		"target_login = excluded.target_login",
		"reason = excluded.reason",
		"created_at = excluded.created_at",
	}

	for _, substr := range expectedSubstrings {
		if !contains(db.lastQuery, substr) {
			t.Errorf("query missing expected substring %q\nQuery:\n%s", substr, db.lastQuery)
		}
	}

	// Verify 4 array parameters
	if len(db.lastArgs) != 4 {
		t.Fatalf("expected 4 array parameters, got %d", len(db.lastArgs))
	}
}

// TestUpsertAliasesError tests error handling.
func TestUpsertAliasesError(t *testing.T) {
	db := &mockDBExecutor{shouldError: true}
	ingester := NewAliasIngester(db)

	now := time.Now()
	rows := []AliasRow{
		{SourceLogin: "old-johndoe", TargetLogin: "johndoe", Reason: "admin", CreatedAt: now},
	}

	err := ingester.UpsertAliases(context.Background(), rows)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !contains(err.Error(), "bulk upsert failed") {
		t.Errorf("expected error message to contain 'bulk upsert failed', got %v", err)
	}
}

// TestDeleteAdminAliasesEmpty tests empty sourceLogins returns 0.
func TestDeleteAdminAliasesEmpty(t *testing.T) {
	db := &mockDBExecutor{}
	ingester := NewAliasIngester(db)
	count, err := ingester.DeleteAdminAliases(context.Background(), []string{})
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows deleted for empty slice, got %d", count)
	}
}

// TestDeleteAdminAliasesQuery tests the delete query construction.
func TestDeleteAdminAliasesQuery(t *testing.T) {
	db := &mockDBExecutor{rowsAffected: 3}
	ingester := NewAliasIngester(db)

	sourceLogins := []string{"old-johndoe", "jane-bot"}
	count, err := ingester.DeleteAdminAliases(context.Background(), sourceLogins)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 rows deleted, got %d", count)
	}

	// Verify query was executed
	if db.lastQuery == "" {
		t.Fatal("query was not executed")
	}

	expectedSubstrings := []string{
		"DELETE FROM user_aliases",
		"WHERE source_login = ANY",
		"AND reason = 'admin'",
	}

	for _, substr := range expectedSubstrings {
		if !contains(db.lastQuery, substr) {
			t.Errorf("query missing expected substring %q\nQuery:\n%s", substr, db.lastQuery)
		}
	}

	// Verify 1 parameter (sourceLogins array)
	if len(db.lastArgs) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(db.lastArgs))
	}
}

// TestNewAliasIngester tests the constructor.
func TestNewAliasIngester(t *testing.T) {
	db := &mockDBExecutor{}
	ingester := NewAliasIngester(db)
	if ingester == nil {
		t.Fatal("NewAliasIngester returned nil")
	}
	if ingester.db == nil {
		t.Error("ingester.db is nil")
	}
}
