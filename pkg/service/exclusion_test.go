package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

// mockRow is a mock implementation of a row scanner for testing.
type mockRow struct {
	scanErr error
}

func (m *mockRow) Scan(dest ...interface{}) error {
	if m.scanErr != nil {
		return m.scanErr
	}
	// For successful scans, set the first dest to 1
	if len(dest) > 0 {
		if ptr, ok := dest[0].(*int); ok {
			*ptr = 1
		}
	}
	return nil
}

// mockQuerier is a mock database Querier for testing.
type mockQuerier struct {
	row RowScanner
}

func (m *mockQuerier) QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner {
	return m.row
}

// mockTransactor is a mock Transactor for testing.
type mockTransactor struct {
	execContextFn func(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	commitFn      func() error
	rollbackFn    func() error
}

func (m *mockTransactor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if m.execContextFn != nil {
		return m.execContextFn(ctx, query, args...)
	}
	return &mockResult{}, nil
}

func (m *mockTransactor) Commit() error {
	if m.commitFn != nil {
		return m.commitFn()
	}
	return nil
}

func (m *mockTransactor) Rollback() error {
	if m.rollbackFn != nil {
		return m.rollbackFn()
	}
	return nil
}

// mockTransactioner is a mock Transactioner for testing.
type mockTransactioner struct {
	row         RowScanner
	beginTxFn   func(ctx context.Context, opts *sql.TxOptions) (Transactor, error)
}

func (m *mockTransactioner) QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner {
	return m.row
}

func (m *mockTransactioner) BeginTx(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
	if m.beginTxFn != nil {
		return m.beginTxFn(ctx, opts)
	}
	return nil, errors.New("mock: BeginTx not implemented")
}

// mockResult is a mock implementation of sql.Result for testing.
type mockResult struct {
	rowsAffected int64
	lastInsertId int64
}

func (m *mockResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

func (m *mockResult) LastInsertId() (int64, error) {
	return m.lastInsertId, nil
}

func TestRepoExists_EmptyInputs(t *testing.T) {
	ctx := context.Background()

	// Create a mock querier that won't be called (empty inputs return early)
	mockDB := &mockQuerier{row: &mockRow{}}
	checker := NewRepoChecker(mockDB)

	tests := []struct {
		name         string
		provider     string
		repoFullName string
		wantResult   bool
	}{
		{
			name:         "empty provider returns false",
			provider:     "",
			repoFullName: "owner/repo",
			wantResult:   false,
		},
		{
			name:         "empty repoFullName returns false",
			provider:     "github",
			repoFullName: "",
			wantResult:   false,
		},
		{
			name:         "both empty returns false",
			provider:     "",
			repoFullName: "",
			wantResult:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.RepoExists(ctx, tt.provider, tt.repoFullName)
			if result != tt.wantResult {
				t.Errorf("RepoExists() = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

func TestRepoExists_ExistingRepo(t *testing.T) {
	ctx := context.Background()

	// Create a mock that successfully scans (repo exists)
	mockRow := &mockRow{scanErr: nil}
	mockDB := &mockQuerier{row: mockRow}

	checker := NewRepoChecker(mockDB)
	result := checker.RepoExists(ctx, "github", "owner/repo")

	if result != true {
		t.Errorf("RepoExists() with existing repo = %v, want true", result)
	}
}

func TestRepoExists_NonExistentRepo(t *testing.T) {
	ctx := context.Background()

	// Create a mock that returns sql.ErrNoRows (repo doesn't exist)
	mockRow := &mockRow{scanErr: sql.ErrNoRows}
	mockDB := &mockQuerier{row: mockRow}

	checker := NewRepoChecker(mockDB)
	result := checker.RepoExists(ctx, "github", "nonexistent/repo")

	if result != false {
		t.Errorf("RepoExists() with nonexistent repo = %v, want false", result)
	}
}

func TestRepoExists_DatabaseError(t *testing.T) {
	ctx := context.Background()

	// Create a mock that returns a database error
	mockRow := &mockRow{scanErr: errors.New("database connection error")}
	mockDB := &mockQuerier{row: mockRow}

	checker := NewRepoChecker(mockDB)
	result := checker.RepoExists(ctx, "github", "owner/repo")

	// Should return false on database errors (fail-safe)
	if result != false {
		t.Errorf("RepoExists() with database error = %v, want false", result)
	}
}

// TestSetRepoExclusion_EmptyReason tests that SetRepoExclusion validates reason is not empty.
// This test passes nil for the database, which will panic if validation doesn't happen first.
func TestSetRepoExclusion_EmptyReason(t *testing.T) {
	ctx := context.Background()

	// Test with empty reason - should fail validation before trying to use db
	err := SetRepoExclusion(ctx, nil, "github", "owner/repo", "")
	if err == nil {
		t.Errorf("SetRepoExclusion() with empty reason should return error, got nil")
	}
	if err.Error() != "exclusion reason cannot be empty" {
		t.Errorf("SetRepoExclusion() wrong error message: %v", err)
	}
}

// TestSetRepoExclusion_RepoNotFound tests that SetRepoExclusion returns error when repo doesn't exist.
func TestSetRepoExclusion_RepoNotFound(t *testing.T) {
	ctx := context.Background()

	// Create a mock database that returns a non-existent repo for the RepoExists check
	mockDB := &mockTransactioner{
		row: &mockRow{scanErr: sql.ErrNoRows},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
			// This shouldn't be called since RepoExists returns false
			return nil, errors.New("should not reach BeginTx when repo doesn't exist")
		},
	}

	err := SetRepoExclusion(ctx, mockDB, "github", "nonexistent/repo", "test reason")
	if err == nil {
		t.Errorf("SetRepoExclusion() with nonexistent repo should return error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("SetRepoExclusion() wrong error for nonexistent repo: %v", err)
	}
}

// TestSetRepoExclusion_Success tests that SetRepoExclusion successfully excludes a repo.
func TestSetRepoExclusion_Success(t *testing.T) {
	ctx := context.Background()

	// Create a successful transaction
	mockTx := &mockTransactor{
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			// Return successful result with 1 row affected
			return &mockResult{rowsAffected: 1, lastInsertId: 0}, nil
		},
		commitFn: func() error {
			return nil
		},
		rollbackFn: func() error {
			return nil
		},
	}

	// Create a mock database that returns an existing repo and provides the successful transaction
	mockDB := &mockTransactioner{
		row: &mockRow{scanErr: nil},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
			// Return our mock transaction
			return mockTx, nil
		},
	}

	err := SetRepoExclusion(ctx, mockDB, "github", "owner/repo", "test reason")
	if err != nil {
		t.Errorf("SetRepoExclusion() with valid inputs should succeed, got error: %v", err)
	}
}

// TestSetRepoExclusion_UpdateError tests that SetRepoExclusion handles update errors properly.
func TestSetRepoExclusion_UpdateError(t *testing.T) {
	ctx := context.Background()

	updateError := errors.New("database update failed")

	// Create a transaction that fails on ExecContext
	mockTx := &mockTransactor{
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return nil, updateError
		},
		commitFn: func() error {
			return nil
		},
		rollbackFn: func() error {
			return nil
		},
	}

	mockDB := &mockTransactioner{
		row: &mockRow{scanErr: nil},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
			return mockTx, nil
		},
	}

	err := SetRepoExclusion(ctx, mockDB, "github", "owner/repo", "test reason")
	if err == nil {
		t.Errorf("SetRepoExclusion() with update error should return error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to update repo exclusion") {
		t.Errorf("SetRepoExclusion() wrong error for update failure: %v", err)
	}
}

// TestSetRepoExclusion_CommitError tests that SetRepoExclusion handles commit errors properly.
func TestSetRepoExclusion_CommitError(t *testing.T) {
	ctx := context.Background()

	commitError := errors.New("transaction commit failed")

	// Create a transaction that fails on Commit
	mockTx := &mockTransactor{
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{rowsAffected: 1, lastInsertId: 0}, nil
		},
		commitFn: func() error {
			return commitError
		},
		rollbackFn: func() error {
			return nil
		},
	}

	mockDB := &mockTransactioner{
		row: &mockRow{scanErr: nil},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
			return mockTx, nil
		},
	}

	err := SetRepoExclusion(ctx, mockDB, "github", "owner/repo", "test reason")
	if err == nil {
		t.Errorf("SetRepoExclusion() with commit error should return error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to commit transaction") {
		t.Errorf("SetRepoExclusion() wrong error for commit failure: %v", err)
	}
}

// TestSetRepoExclusion_BeginTxError tests that SetRepoExclusion handles BeginTx errors properly.
func TestSetRepoExclusion_BeginTxError(t *testing.T) {
	ctx := context.Background()

	beginTxError := errors.New("cannot begin transaction")

	mockDB := &mockTransactioner{
		row: &mockRow{scanErr: nil},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
			return nil, beginTxError
		},
	}

	err := SetRepoExclusion(ctx, mockDB, "github", "owner/repo", "test reason")
	if err == nil {
		t.Errorf("SetRepoExclusion() with BeginTx error should return error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to begin transaction") {
		t.Errorf("SetRepoExclusion() wrong error for BeginTx failure: %v", err)
	}
}

// TestSetRepoExclusion_NoRowsAffected tests that SetRepoExclusion handles the case where no rows are updated.
func TestSetRepoExclusion_NoRowsAffected(t *testing.T) {
	ctx := context.Background()

	// Create a transaction that returns 0 rows affected
	mockTx := &mockTransactor{
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{rowsAffected: 0, lastInsertId: 0}, nil
		},
		commitFn: func() error {
			return nil
		},
		rollbackFn: func() error {
			return nil
		},
	}

	mockDB := &mockTransactioner{
		row: &mockRow{scanErr: nil},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
			return mockTx, nil
		},
	}

	err := SetRepoExclusion(ctx, mockDB, "github", "owner/repo", "test reason")
	if err == nil {
		t.Errorf("SetRepoExclusion() with no rows affected should return error, got nil")
	}
	if !strings.Contains(err.Error(), "no rows updated") {
		t.Errorf("SetRepoExclusion() wrong error for no rows affected: %v", err)
	}
}
