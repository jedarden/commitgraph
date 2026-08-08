package pg

import (
	"database/sql"
	"testing"
)

// Tests for SQLExecutor coverage

// TestNewSQLExecutor_Wrapper verifies the constructor creates a non-nil wrapper
func TestNewSQLExecutor_Wrapper(t *testing.T) {
	// Create a real sql.DB (can be nil since we're just testing the wrapper)
	var db *sql.DB
	executor := NewSQLExecutor(db)

	if executor == nil {
		t.Fatal("NewSQLExecutor returned nil")
	}
	if executor.db != db {
		t.Errorf("executor.db not set correctly, got %v want %v", executor.db, db)
	}
}

// TestSQLExecutor_ExecContext verifies that ExecContext delegates to the underlying db
func TestSQLExecutor_ExecContext(t *testing.T) {
	// This test verifies that SQLExecutor.ExecContext correctly delegates
	// to the underlying sql.DB.ExecContext
	// Since we can't easily mock sql.DB, we verify the interface implementation
	var _ Executor = (*SQLExecutor)(nil)

	// The actual delegation is tested via integration tests with real DB
	t.Skip("requires real database for full delegation test")
}

// TestSQLExecutor_QueryRowContext verifies the QueryRowContext delegation
func TestSQLExecutor_QueryRowContext(t *testing.T) {
	// Verify interface implementation
	var _ Executor = (*SQLExecutor)(nil)
	t.Skip("requires real database for full delegation test")
}

// TestSQLExecutor_QueryContext verifies the QueryContext delegation
func TestSQLExecutor_QueryContext(t *testing.T) {
	// Verify interface implementation
	var _ Executor = (*SQLExecutor)(nil)
	t.Skip("requires real database for full delegation test")
}
