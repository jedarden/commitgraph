package pg

import (
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

	// Check for required SQL keywords
	requiredKeywords := []string{
		"INSERT INTO users",
		"SELECT unnest",
		"ON CONFLICT",
		"DO NOTHING",
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
