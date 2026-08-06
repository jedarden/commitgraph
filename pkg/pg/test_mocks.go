package pg

import (
	"context"
	"database/sql"
)

// Common mock implementations for testing

// mockExecutor is a test double that captures SQL execution without a real database.
// It implements the DBExecutor interface for ExecContext only.
// QueryContext and QueryRowContext return nil/empty values since sql.Rows and sql.Row
// are concrete types that cannot be easily mocked.
type mockExecutor struct {
	lastQuery    string
	lastArgs     []interface{}
	rowsAffected int64
	shouldError  bool
}

func (m *mockExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (Result, error) {
	m.lastQuery = query
	m.lastArgs = args
	if m.shouldError {
		return nil, &mockError{err: "test error"}
	}
	return &mockResult{rowsAffected: m.rowsAffected}, nil
}

func (m *mockExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	m.lastQuery = query
	m.lastArgs = args
	if m.shouldError {
		return nil, &mockError{err: "test error"}
	}
	// Return empty mock rows - caller must handle this
	return &mockRows{}, nil
}

func (m *mockExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) Row {
	m.lastQuery = query
	m.lastArgs = args
	// Return nil - caller must handle this
	return &mockRow{}
}

// mockResult implements Result interface for testing.
type mockResult struct {
	rowsAffected int64
}

func (m *mockResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

// mockError implements error interface for testing.
type mockError struct {
	err string
}

func (m *mockError) Error() string {
	return m.err
}

// mockRows implements Rows interface for testing.
type mockRows struct {
	shouldError bool
	currentIndex int
	data         []interface{}
}

func (m *mockRows) Next() bool {
	return false
}

func (m *mockRows) Scan(dest ...interface{}) error {
	if m.shouldError {
		return &mockError{err: "test error"}
	}
	return nil
}

func (m *mockRows) Close() error {
	return nil
}

func (m *mockRows) Err() error {
	if m.shouldError {
		return &mockError{err: "test error"}
	}
	return nil
}

// mockRow implements Row interface for testing.
type mockRow struct {
	shouldError bool
	data        interface{}
}

func (m *mockRow) Scan(dest ...interface{}) error {
	if m.shouldError {
		return &mockError{err: "test error"}
	}
	return nil
}
