package pg

import (
	"context"
	"database/sql"
)

// Common mock implementations for testing

// mockExecutor is a test double that captures SQL execution without a real database.
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
	return &mockRows{}, nil
}

func (m *mockExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) Row {
	m.lastQuery = query
	m.lastArgs = args
	return &mockRow{shouldError: m.shouldError}
}

// mockResult implements Result interface for testing.
type mockResult struct {
	rowsAffected int64
}

func (m *mockResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

func (m *mockResult) LastInsertId() (int64, error) {
	return 0, nil
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
	closed bool
}

func (m *mockRows) Next() bool {
	return false
}

func (m *mockRows) Scan(dest ...interface{}) error {
	return sql.ErrNoRows
}

func (m *mockRows) Close() error {
	m.closed = true
	return nil
}

func (m *mockRows) Err() error {
	return nil
}

// mockRow implements Row interface for testing.
type mockRow struct {
	shouldError bool
}

func (m *mockRow) Scan(dest ...interface{}) error {
	if m.shouldError {
		return &mockError{err: "test error"}
	}
	return sql.ErrNoRows
}
