package pg

import (
	"context"
	"testing"
)

// TestBatchUsersUpsertQuerySyntax verifies the SQL query syntax is valid
// by checking that the query constant is properly formatted.
func TestBatchUsersUpsertQuerySyntax(t *testing.T) {
	// Verify the query constant is defined and non-empty
	if BatchUsersUpsertQuery == "" {
		t.Fatal("BatchUsersUpsertQuery is empty")
	}

	// Basic syntax checks (PostgreSQL-specific patterns)
	query := BatchUsersUpsertQuery

	// Check for required SQL keywords. The upsert uses a no-op
	// "DO UPDATE SET login = excluded.login" rather than "DO NOTHING" so
	// that RETURNING yields a row for every input login, including ones
	// that already existed — Postgres does not emit RETURNING rows for
	// conflicts resolved by DO NOTHING.
	requiredKeywords := []string{
		"INSERT INTO users",
		"SELECT unnest",
		"ON CONFLICT",
		"DO UPDATE SET login = excluded.login",
		"RETURNING",
	}

	for _, keyword := range requiredKeywords {
		if !contains(query, keyword) {
			t.Errorf("Query missing required keyword: %s", keyword)
		}
	}

	// Check for the parameter placeholder
	if !contains(query, "$1") {
		t.Error("Query missing parameter placeholder $1")
	}

	// Check for proper array type annotation
	if !contains(query, "text[]") {
		t.Error("Query missing text[] type annotation for array parameter")
	}
}

// TestUsersSelectByLoginsQuerySyntax verifies the select query syntax is valid.
func TestUsersSelectByLoginsQuerySyntax(t *testing.T) {
	if UsersSelectByLoginsQuery == "" {
		t.Fatal("UsersSelectByLoginsQuery is empty")
	}

	query := UsersSelectByLoginsQuery

	// Check for required SQL keywords
	requiredKeywords := []string{
		"SELECT login, user_id",
		"FROM users",
		"WHERE",
		"ANY($1",
	}

	for _, keyword := range requiredKeywords {
		if !contains(query, keyword) {
			t.Errorf("Query missing required keyword: %s", keyword)
		}
	}
}

// TestBatchUpsertUsersEmpty verifies that an empty (or nil) logins slice
// returns an empty, non-nil map without error and without touching the
// database (a nil *sql.Tx would panic if the guard clause didn't return
// early, so this also proves the empty-slice short-circuit actually runs
// before any query is issued).
func TestBatchUpsertUsersEmpty(t *testing.T) {
	got, err := BatchUpsertUsers(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty map, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %#v", got)
	}

	got, err = BatchUpsertUsers(context.Background(), nil, []string{})
	if err != nil {
		t.Fatalf("expected nil error for empty slice, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %#v", got)
	}
}
