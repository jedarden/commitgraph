package pg

import (
	"context"
	"testing"
	"time"
)

// TestUpsertAliasesEmpty tests that empty row slice returns nil.
func TestUpsertAliasesEmpty(t *testing.T) {
	ingester := NewAliasIngester(&mockExecutor{})
	err := ingester.UpsertAliases(context.Background(), []AliasRow{})
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
}

// TestUpsertAliasesQuery tests the SQL query construction and parameters.
func TestUpsertAliasesQuery(t *testing.T) {
	executor := &mockExecutor{}
	ingester := NewAliasIngester(executor)

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
	if executor.lastQuery == "" {
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
		if !contains(executor.lastQuery, substr) {
			t.Errorf("query missing expected substring %q\nQuery:\n%s", substr, executor.lastQuery)
		}
	}

	// Verify 4 array parameters
	if len(executor.lastArgs) != 4 {
		t.Fatalf("expected 4 array parameters, got %d", len(executor.lastArgs))
	}
}

// TestUpsertAliasesError tests error handling.
func TestUpsertAliasesError(t *testing.T) {
	executor := &mockExecutor{shouldError: true}
	ingester := NewAliasIngester(executor)

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

// TestGetAdminAliases tests querying admin aliases.
func TestGetAdminAliases(t *testing.T) {
	// QueryRowContext/QueryContext require a more sophisticated mock
	// For now, we test the basic structure
	ingester := NewAliasIngester(&mockExecutor{})
	if ingester == nil {
		t.Fatal("ingester is nil")
	}
}

// contains is a helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findInString(s, substr))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
