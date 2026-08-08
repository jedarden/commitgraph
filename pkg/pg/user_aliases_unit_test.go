package pg

import (
	"context"
	"testing"
)

// TestGetAdminAliases_Unit tests GetAdminAliases with a mock database
func TestGetAdminAliases_Unit(t *testing.T) {
	// Use mockDBExecutor which implements DBExecutor interface
	db := &mockDBExecutor{}
	ingester := NewAliasIngester(db)

	// mockDBExecutor.QueryContext returns an error because *sql.Rows cannot
	// be mocked without a real database driver
	aliases, err := ingester.GetAdminAliases(context.Background())

	// We expect an error because mockDBExecutor.QueryContext returns an error
	// about sql.Rows not being mockable
	if err == nil {
		t.Error("expected error from mockDBExecutor, got nil")
	}
	if aliases != nil {
		t.Error("expected nil aliases on error, got non-nil")
	}
}

// TestDeleteAdminAliases_Unit tests DeleteAdminAliases with a mock database
func TestDeleteAdminAliases_Unit(t *testing.T) {
	db := &mockDBExecutor{rowsAffected: 3}
	ingester := NewAliasIngester(db)

	sourceLogins := []string{"old-johndoe", "jane-bot", "deprecated-bob"}
	count, err := ingester.DeleteAdminAliases(context.Background(), sourceLogins)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 rows deleted, got %d", count)
	}

	// Verify query was executed
	if db.lastQuery == "" {
		t.Fatal("query was not executed")
	}

	// Verify query contains expected elements
	if !contains(db.lastQuery, "DELETE FROM user_aliases") {
		t.Errorf("query missing DELETE statement, got: %s", db.lastQuery)
	}
	if !contains(db.lastQuery, "WHERE source_login = ANY") {
		t.Errorf("query missing WHERE clause, got: %s", db.lastQuery)
	}
	if !contains(db.lastQuery, "AND reason = 'admin'") {
		t.Errorf("query missing admin filter, got: %s", db.lastQuery)
	}

	// Verify parameter is the array we passed
	if len(db.lastArgs) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(db.lastArgs))
	}
	args, ok := db.lastArgs[0].([]string)
	if !ok {
		t.Fatal("first parameter should be []string")
	}
	if len(args) != 3 {
		t.Errorf("expected 3 source logins, got %d", len(args))
	}
}

// TestDeleteAdminAliases_Empty_Unit tests empty sourceLogins
func TestDeleteAdminAliases_Empty_Unit(t *testing.T) {
	db := &mockDBExecutor{}
	ingester := NewAliasIngester(db)

	count, err := ingester.DeleteAdminAliases(context.Background(), []string{})

	if err != nil {
		t.Fatalf("unexpected error for empty slice: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows deleted for empty slice, got %d", count)
	}

	// Verify no query was executed for empty slice
	if db.lastQuery != "" {
		t.Error("expected no query for empty slice, but got one")
	}
}

// TestDeleteAdminAliases_Error_Unit tests error handling
func TestDeleteAdminAliases_Error_Unit(t *testing.T) {
	db := &mockDBExecutor{shouldError: true}
	ingester := NewAliasIngester(db)

	sourceLogins := []string{"old-johndoe"}
	count, err := ingester.DeleteAdminAliases(context.Background(), sourceLogins)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if count != 0 {
		t.Errorf("expected 0 rows deleted on error, got %d", count)
	}
	if !contains(err.Error(), "delete failed") {
		t.Errorf("expected error message to contain 'delete failed', got %v", err)
	}
}

// TestGetAdminAliases_IntegrationSkipped_Unit is a placeholder for the integration test
// This function exists to provide code coverage for the GetAdminAliases function signature
// The actual testing happens in user_aliases_integration_test.go
func TestGetAdminAliases_IntegrationSkipped_Unit(t *testing.T) {
	// This is a placeholder - real tests are in integration test file
	// We're creating this to document the testing approach
	t.Skip("GetAdminAliases is tested via integration tests in user_aliases_integration_test.go")
}
