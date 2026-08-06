package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

// mockRow is a mock implementation of a row scanner for testing.
type mockRow struct {
	scanErr error
	// Values to return on scan (for multi-column queries)
	scanValues []interface{}
}

func (m *mockRow) Scan(dest ...interface{}) error {
	if m.scanErr != nil {
		return m.scanErr
	}
	// If scanValues are provided, use them
	if m.scanValues != nil && len(m.scanValues) > 0 {
		for i, val := range m.scanValues {
			if i >= len(dest) {
				break
			}
			switch v := val.(type) {
			case int:
				if ptr, ok := dest[i].(*int64); ok {
					*ptr = int64(v)
				}
			case int64:
				if ptr, ok := dest[i].(*int64); ok {
					*ptr = v
				}
			case string:
				if ptr, ok := dest[i].(*string); ok {
					*ptr = v
				}
			case *string:
				if ptr, ok := dest[i].(**string); ok {
					*ptr = v
				}
			case time.Time:
				if ptr, ok := dest[i].(*time.Time); ok {
					*ptr = v
				}
			case *time.Time:
				if ptr, ok := dest[i].(**time.Time); ok {
					*ptr = v
				}
			case nil:
				// Handle nil pointers for nullable fields
				// The dest should already be a pointer to a pointer
			}
		}
		return nil
	}
	// For successful scans without specific values, set the first dest to 1
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
	execContextFn     func(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	queryRowContextFn func(ctx context.Context, query string, args ...interface{}) RowScanner
	commitFn          func() error
	rollbackFn        func() error
}

func (m *mockTransactor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if m.execContextFn != nil {
		return m.execContextFn(ctx, query, args...)
	}
	return &mockResult{}, nil
}

func (m *mockTransactor) QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner {
	if m.queryRowContextFn != nil {
		return m.queryRowContextFn(ctx, query, args...)
	}
	return &mockRow{}
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

// TestClearRepoExclusion_RepoNotFound tests that ClearRepoExclusion returns error when repo doesn't exist.
func TestClearRepoExclusion_RepoNotFound(t *testing.T) {
	ctx := context.Background()

	// Create a mock database that returns a non-existent repo for the RepoExists check
	mockDB := &mockTransactioner{
		row: &mockRow{scanErr: sql.ErrNoRows},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
			// This shouldn't be called since RepoExists returns false
			return nil, errors.New("should not reach BeginTx when repo doesn't exist")
		},
	}

	err := ClearRepoExclusion(ctx, mockDB, "github", "nonexistent/repo")
	if err == nil {
		t.Errorf("ClearRepoExclusion() with nonexistent repo should return error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("ClearRepoExclusion() wrong error for nonexistent repo: %v", err)
	}
}

// TestClearRepoExclusion_Success tests that ClearRepoExclusion successfully clears exclusion from a repo.
func TestClearRepoExclusion_Success(t *testing.T) {
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

	err := ClearRepoExclusion(ctx, mockDB, "github", "owner/repo")
	if err != nil {
		t.Errorf("ClearRepoExclusion() with valid inputs should succeed, got error: %v", err)
	}
}

// TestClearRepoExclusion_NonExcludedRepo tests that ClearRepoExclusion succeeds when clearing a non-excluded repo.
// This is a no-op operation that should succeed (1 row affected).
func TestClearRepoExclusion_NonExcludedRepo(t *testing.T) {
	ctx := context.Background()

	// Create a successful transaction (clearing NULL to NULL is still 1 row affected)
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

	err := ClearRepoExclusion(ctx, mockDB, "github", "owner/repo")
	if err != nil {
		t.Errorf("ClearRepoExclusion() with non-excluded repo should succeed (no-op), got error: %v", err)
	}
}

// TestClearRepoExclusion_UpdateError tests that ClearRepoExclusion handles update errors properly.
func TestClearRepoExclusion_UpdateError(t *testing.T) {
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

	err := ClearRepoExclusion(ctx, mockDB, "github", "owner/repo")
	if err == nil {
		t.Errorf("ClearRepoExclusion() with update error should return error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to clear repo exclusion") {
		t.Errorf("ClearRepoExclusion() wrong error for update failure: %v", err)
	}
}

// TestClearRepoExclusion_CommitError tests that ClearRepoExclusion handles commit errors properly.
func TestClearRepoExclusion_CommitError(t *testing.T) {
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

	err := ClearRepoExclusion(ctx, mockDB, "github", "owner/repo")
	if err == nil {
		t.Errorf("ClearRepoExclusion() with commit error should return error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to commit transaction") {
		t.Errorf("ClearRepoExclusion() wrong error for commit failure: %v", err)
	}
}

// TestClearRepoExclusion_BeginTxError tests that ClearRepoExclusion handles BeginTx errors properly.
func TestClearRepoExclusion_BeginTxError(t *testing.T) {
	ctx := context.Background()

	beginTxError := errors.New("cannot begin transaction")

	mockDB := &mockTransactioner{
		row: &mockRow{scanErr: nil},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
			return nil, beginTxError
		},
	}

	err := ClearRepoExclusion(ctx, mockDB, "github", "owner/repo")
	if err == nil {
		t.Errorf("ClearRepoExclusion() with BeginTx error should return error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to begin transaction") {
		t.Errorf("ClearRepoExclusion() wrong error for BeginTx failure: %v", err)
	}
}

// TestClearRepoExclusion_NoRowsAffected tests that ClearRepoExclusion handles the case where no rows are updated.
func TestClearRepoExclusion_NoRowsAffected(t *testing.T) {
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

	err := ClearRepoExclusion(ctx, mockDB, "github", "owner/repo")
	if err == nil {
		t.Errorf("ClearRepoExclusion() with no rows affected should return error, got nil")
	}
	if !strings.Contains(err.Error(), "no rows updated") {
		t.Errorf("ClearRepoExclusion() wrong error for no rows affected: %v", err)
	}
}

// TestSetRepoExclusion_EmptyProvider tests that SetRepoExclusion validates provider is not empty.
func TestSetRepoExclusion_EmptyProvider(t *testing.T) {
	ctx := context.Background()

	// Test with empty provider - should fail validation before trying to use db
	err := SetRepoExclusion(ctx, nil, "", "owner/repo", "test reason")
	if err == nil {
		t.Errorf("SetRepoExclusion() with empty provider should return error, got nil")
	}
	if !strings.Contains(err.Error(), "provider cannot be empty") {
		t.Errorf("SetRepoExclusion() wrong error message: %v", err)
	}
}

// TestSetRepoExclusion_InvalidProviderFormat tests that SetRepoExclusion validates provider format.
func TestSetRepoExclusion_InvalidProviderFormat(t *testing.T) {
	ctx := context.Background()

	invalidProviders := []string{
		"GITHUB",    // uppercase
		"Git-Hub",   // mixed case with hyphen
		"github_",   // trailing underscore
		"github.com", // contains dots
		"git hub",   // contains space
		"123!",      // contains special chars
	}

	for _, provider := range invalidProviders {
		t.Run(provider, func(t *testing.T) {
			err := SetRepoExclusion(ctx, nil, provider, "owner/repo", "test reason")
			if err == nil {
				t.Errorf("SetRepoExclusion() with invalid provider '%s' should return error, got nil", provider)
			}
			if !strings.Contains(err.Error(), "provider must be lowercase alphanumeric") {
				t.Errorf("SetRepoExclusion() wrong error for invalid provider '%s': %v", provider, err)
			}
		})
	}
}

// TestSetRepoExclusion_ValidProviders tests that valid providers are accepted.
func TestSetRepoExclusion_ValidProviders(t *testing.T) {
	ctx := context.Background()

	validProviders := []string{
		"github",
		"gitlab",
		"bitbucket",
		"gitea",
		"sourcehut",
	}

	for _, provider := range validProviders {
		t.Run(provider, func(t *testing.T) {
			// Create a successful transaction
			mockTx := &mockTransactor{
				execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
					return &mockResult{rowsAffected: 1, lastInsertId: 0}, nil
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

			err := SetRepoExclusion(ctx, mockDB, provider, "owner/repo", "test reason")
			if err != nil {
				t.Errorf("SetRepoExclusion() with valid provider '%s' should succeed, got error: %v", provider, err)
			}
		})
	}
}

// TestSetRepoExclusion_EmptyRepoFullName tests that SetRepoExclusion validates repoFullName is not empty.
func TestSetRepoExclusion_EmptyRepoFullName(t *testing.T) {
	ctx := context.Background()

	// Test with empty repoFullName - should fail validation before trying to use db
	err := SetRepoExclusion(ctx, nil, "github", "", "test reason")
	if err == nil {
		t.Errorf("SetRepoExclusion() with empty repoFullName should return error, got nil")
	}
	if !strings.Contains(err.Error(), "repository full name cannot be empty") {
		t.Errorf("SetRepoExclusion() wrong error message: %v", err)
	}
}

// TestSetRepoExclusion_MalformedRepoFullName tests that SetRepoExclusion validates repoFullName format.
func TestSetRepoExclusion_MalformedRepoFullName(t *testing.T) {
	ctx := context.Background()

	malformedNames := []struct {
		name        string
		repoFullName string
		expectedErr string
	}{
		{
			name:        "missing repo",
			repoFullName: "owner",
			expectedErr: "must be in 'owner/repo' format",
		},
		{
			name:        "extra slash",
			repoFullName: "owner/repo/extra",
			expectedErr: "must be in 'owner/repo' format",
		},
		{
			name:        "empty owner",
			repoFullName: "/repo",
			expectedErr: "repository owner cannot be empty",
		},
		{
			name:        "empty repo name",
			repoFullName: "owner/",
			expectedErr: "repository name cannot be empty",
		},
		{
			name:        "no slash",
			repoFullName: "ownerrepo",
			expectedErr: "must be in 'owner/repo' format",
		},
	}

	for _, tc := range malformedNames {
		t.Run(tc.name, func(t *testing.T) {
			err := SetRepoExclusion(ctx, nil, "github", tc.repoFullName, "test reason")
			if err == nil {
				t.Errorf("SetRepoExclusion() with malformed repoFullName '%s' should return error, got nil", tc.repoFullName)
			}
			if !strings.Contains(err.Error(), tc.expectedErr) {
				t.Errorf("SetRepoExclusion() wrong error for malformed repoFullName '%s': %v", tc.repoFullName, err)
			}
		})
	}
}

// TestSetRepoExclusion_ValidRepoFullName tests that valid repoFullName formats are accepted.
func TestSetRepoExclusion_ValidRepoFullName(t *testing.T) {
	ctx := context.Background()

	validNames := []string{
		"owner/repo",
		"user123/my-project",
		"orgname/repo_name",
		"test/repo123",
	}

	for _, repoFullName := range validNames {
		t.Run(repoFullName, func(t *testing.T) {
			// Create a successful transaction
			mockTx := &mockTransactor{
				execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
					return &mockResult{rowsAffected: 1, lastInsertId: 0}, nil
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

			err := SetRepoExclusion(ctx, mockDB, "github", repoFullName, "test reason")
			if err != nil {
				t.Errorf("SetRepoExclusion() with valid repoFullName '%s' should succeed, got error: %v", repoFullName, err)
			}
		})
	}
}

// TestClearRepoExclusion_EmptyProvider tests that ClearRepoExclusion validates provider is not empty.
func TestClearRepoExclusion_EmptyProvider(t *testing.T) {
	ctx := context.Background()

	// Test with empty provider - should fail validation before trying to use db
	err := ClearRepoExclusion(ctx, nil, "", "owner/repo")
	if err == nil {
		t.Errorf("ClearRepoExclusion() with empty provider should return error, got nil")
	}
	if !strings.Contains(err.Error(), "provider cannot be empty") {
		t.Errorf("ClearRepoExclusion() wrong error message: %v", err)
	}
}

// TestClearRepoExclusion_InvalidProviderFormat tests that ClearRepoExclusion validates provider format.
func TestClearRepoExclusion_InvalidProviderFormat(t *testing.T) {
	ctx := context.Background()

	invalidProviders := []string{
		"GITHUB",
		"git-hub",
		"github.com",
		"git hub",
	}

	for _, provider := range invalidProviders {
		t.Run(provider, func(t *testing.T) {
			err := ClearRepoExclusion(ctx, nil, provider, "owner/repo")
			if err == nil {
				t.Errorf("ClearRepoExclusion() with invalid provider '%s' should return error, got nil", provider)
			}
			if !strings.Contains(err.Error(), "provider must be lowercase alphanumeric") {
				t.Errorf("ClearRepoExclusion() wrong error for invalid provider '%s': %v", provider, err)
			}
		})
	}
}

// TestClearRepoExclusion_EmptyRepoFullName tests that ClearRepoExclusion validates repoFullName is not empty.
func TestClearRepoExclusion_EmptyRepoFullName(t *testing.T) {
	ctx := context.Background()

	// Test with empty repoFullName - should fail validation before trying to use db
	err := ClearRepoExclusion(ctx, nil, "github", "")
	if err == nil {
		t.Errorf("ClearRepoExclusion() with empty repoFullName should return error, got nil")
	}
	if !strings.Contains(err.Error(), "repository full name cannot be empty") {
		t.Errorf("ClearRepoExclusion() wrong error message: %v", err)
	}
}

// TestClearRepoExclusion_MalformedRepoFullName tests that ClearRepoExclusion validates repoFullName format.
func TestClearRepoExclusion_MalformedRepoFullName(t *testing.T) {
	ctx := context.Background()

	malformedNames := []struct {
		name        string
		repoFullName string
		expectedErr string
	}{
		{
			name:        "missing repo",
			repoFullName: "owner",
			expectedErr: "must be in 'owner/repo' format",
		},
		{
			name:        "extra slash",
			repoFullName: "owner/repo/extra",
			expectedErr: "must be in 'owner/repo' format",
		},
		{
			name:        "empty owner",
			repoFullName: "/repo",
			expectedErr: "repository owner cannot be empty",
		},
		{
			name:        "empty repo name",
			repoFullName: "owner/",
			expectedErr: "repository name cannot be empty",
		},
	}

	for _, tc := range malformedNames {
		t.Run(tc.name, func(t *testing.T) {
			err := ClearRepoExclusion(ctx, nil, "github", tc.repoFullName)
			if err == nil {
				t.Errorf("ClearRepoExclusion() with malformed repoFullName '%s' should return error, got nil", tc.repoFullName)
			}
			if !strings.Contains(err.Error(), tc.expectedErr) {
				t.Errorf("ClearRepoExclusion() wrong error for malformed repoFullName '%s': %v", tc.repoFullName, err)
			}
		})
	}
}

// TestSetRepoExclusion_RollbackOnError tests that SetRepoExclusion properly rolls back on error.
func TestSetRepoExclusion_RollbackOnError(t *testing.T) {
	ctx := context.Background()

	rollbackCalled := false
	updateError := errors.New("database update failed")

	// Create a transaction that fails on ExecContext and tracks rollback
	mockTx := &mockTransactor{
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return nil, updateError
		},
		commitFn: func() error {
			return nil
		},
		rollbackFn: func() error {
			rollbackCalled = true
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
		t.Errorf("SetRepoExclusion() should return error, got nil")
	}
	if !rollbackCalled {
		t.Errorf("SetRepoExclusion() should call rollback on error, but it wasn't called")
	}
}

// TestClearRepoExclusion_RollbackOnError tests that ClearRepoExclusion properly rolls back on error.
func TestClearRepoExclusion_RollbackOnError(t *testing.T) {
	ctx := context.Background()

	rollbackCalled := false
	updateError := errors.New("database update failed")

	// Create a transaction that fails on ExecContext and tracks rollback
	mockTx := &mockTransactor{
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return nil, updateError
		},
		commitFn: func() error {
			return nil
		},
		rollbackFn: func() error {
			rollbackCalled = true
			return nil
		},
	}

	mockDB := &mockTransactioner{
		row: &mockRow{scanErr: nil},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
			return mockTx, nil
		},
	}

	err := ClearRepoExclusion(ctx, mockDB, "github", "owner/repo")
	if err == nil {
		t.Errorf("ClearRepoExclusion() should return error, got nil")
	}
	if !rollbackCalled {
		t.Errorf("ClearRepoExclusion() should call rollback on error, but it wasn't called")
	}
}

// TestSetRepoExclusion_RollbackOnCommitError tests that SetRepoExclusion rolls back on commit failure.
func TestSetRepoExclusion_RollbackOnCommitError(t *testing.T) {
	ctx := context.Background()

	rollbackCalled := false
	commitError := errors.New("commit failed")

	// Create a transaction that succeeds on update but fails on commit
	mockTx := &mockTransactor{
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{rowsAffected: 1, lastInsertId: 0}, nil
		},
		commitFn: func() error {
			return commitError
		},
		rollbackFn: func() error {
			rollbackCalled = true
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
		t.Errorf("SetRepoExclusion() should return error, got nil")
	}
	if !rollbackCalled {
		t.Errorf("SetRepoExclusion() should call rollback on commit error, but it wasn't called")
	}
}

// TestClearRepoExclusion_RollbackOnCommitError tests that ClearRepoExclusion rolls back on commit failure.
func TestClearRepoExclusion_RollbackOnCommitError(t *testing.T) {
	ctx := context.Background()

	rollbackCalled := false
	commitError := errors.New("commit failed")

	// Create a transaction that succeeds on update but fails on commit
	mockTx := &mockTransactor{
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{rowsAffected: 1, lastInsertId: 0}, nil
		},
		commitFn: func() error {
			return commitError
		},
		rollbackFn: func() error {
			rollbackCalled = true
			return nil
		},
	}

	mockDB := &mockTransactioner{
		row: &mockRow{scanErr: nil},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
			return mockTx, nil
		},
	}

	err := ClearRepoExclusion(ctx, mockDB, "github", "owner/repo")
	if err == nil {
		t.Errorf("ClearRepoExclusion() should return error, got nil")
	}
	if !rollbackCalled {
		t.Errorf("ClearRepoExclusion() should call rollback on commit error, but it wasn't called")
	}
}

// Note on Concurrency Testing:
// Comprehensive concurrent exclusion testing would require a real database to test
// transaction isolation and locking behavior. The current mock-based testing approach
// cannot properly simulate concurrent database operations. Database-level concurrency
// (e.g., race conditions, deadlocks) should be tested in integration tests with an
// actual database connection. The transaction tests above verify that rollback is called
// appropriately on errors, which is the service-level responsibility.

// TestSetRepoExclusionWithActor_AuditRecording tests that SetRepoExclusionWithActor
// properly records the audit log with before and after states.
func TestSetRepoExclusionWithActor_AuditRecording(t *testing.T) {
	ctx := context.Background()

	var capturedAuditParams struct {
		repoID            int64
		actor             string
		eventType         string
		oldExcludedAt     *time.Time
		oldExcludedReason *string
		newExcludedAt     *time.Time
		newExcludedReason *string
	}

	// Create a transaction that tracks what was passed to RecordExclusionAudit
	mockTx := &mockTransactor{
		queryRowContextFn: func(ctx context.Context, query string, args ...interface{}) RowScanner {
			// Return mock row with repo data
			// repo_id = 123, excluded_at = NULL, excluded_reason = NULL
			return &mockRow{
				scanValues: []interface{}{
					int64(123),
					(*time.Time)(nil), // NULL excluded_at
					(*string)(nil),    // NULL excluded_reason
				},
			}
		},
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{rowsAffected: 1, lastInsertId: 0}, nil
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

	// Create a mock RecordExclusionAudit function
	originalRecordExclusionAudit := RecordExclusionAudit
	defer func() { RecordExclusionAudit = originalRecordExclusionAudit }()

	RecordExclusionAudit = func(
		ctx context.Context,
		tx Transactor,
		repoID int64,
		actor string,
		eventType string,
		oldExcludedAt *time.Time,
		oldExcludedReason *string,
		newExcludedAt *time.Time,
		newExcludedReason *string,
	) error {
		capturedAuditParams.repoID = repoID
		capturedAuditParams.actor = actor
		capturedAuditParams.eventType = eventType
		capturedAuditParams.oldExcludedAt = oldExcludedAt
		capturedAuditParams.oldExcludedReason = oldExcludedReason
		capturedAuditParams.newExcludedAt = newExcludedAt
		capturedAuditParams.newExcludedReason = newExcludedReason
		return nil
	}

	actor := "test-admin"
	reason := "policy violation"

	err := SetRepoExclusionWithActor(ctx, mockDB, "github", "owner/repo", reason, actor)
	if err != nil {
		t.Errorf("SetRepoExclusionWithActor() should succeed, got error: %v", err)
	}

	// Verify audit parameters
	if capturedAuditParams.repoID != 123 {
		t.Errorf("Expected repo_id 123, got %d", capturedAuditParams.repoID)
	}
	if capturedAuditParams.actor != actor {
		t.Errorf("Expected actor '%s', got '%s'", actor, capturedAuditParams.actor)
	}
	if capturedAuditParams.eventType != "exclude" {
		t.Errorf("Expected event_type 'exclude', got '%s'", capturedAuditParams.eventType)
	}
	if capturedAuditParams.oldExcludedAt != nil {
		t.Errorf("Expected old excluded_at to be nil, got %v", capturedAuditParams.oldExcludedAt)
	}
	if capturedAuditParams.oldExcludedReason != nil {
		t.Errorf("Expected old excluded_reason to be nil, got %v", capturedAuditParams.oldExcludedReason)
	}
	if capturedAuditParams.newExcludedAt == nil {
		t.Errorf("Expected new excluded_at to be non-nil")
	}
	if capturedAuditParams.newExcludedReason == nil {
		t.Errorf("Expected new excluded_reason to be non-nil")
	}
	if *capturedAuditParams.newExcludedReason != reason {
		t.Errorf("Expected new excluded_reason '%s', got '%s'", reason, *capturedAuditParams.newExcludedReason)
	}
}

// TestSetRepoExclusionWithActor_AuditRecordingFromPrevious tests that SetRepoExclusionWithActor
// correctly captures the previous exclusion state when re-excluding an already-excluded repo.
func TestSetRepoExclusionWithActor_AuditRecordingFromPrevious(t *testing.T) {
	ctx := context.Background()

	var capturedAuditParams struct {
		repoID            int64
		actor             string
		eventType         string
		oldExcludedAt     *time.Time
		oldExcludedReason *string
		newExcludedAt     *time.Time
		newExcludedReason *string
	}

	// Previous exclusion state
	oldTime := time.Now().Add(-24 * time.Hour)
	oldReason := "old policy violation"

	// Create a transaction that returns a previously excluded repo
	mockTx := &mockTransactor{
		queryRowContextFn: func(ctx context.Context, query string, args ...interface{}) RowScanner {
			// Return mock row with repo data showing previous exclusion
			return &mockRow{
				scanValues: []interface{}{
					int64(456),
					&oldTime,  // previous excluded_at
					&oldReason, // previous excluded_reason
				},
			}
		},
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{rowsAffected: 1, lastInsertId: 0}, nil
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

	// Create a mock RecordExclusionAudit function
	originalRecordExclusionAudit := RecordExclusionAudit
	defer func() { RecordExclusionAudit = originalRecordExclusionAudit }()

	RecordExclusionAudit = func(
		ctx context.Context,
		tx Transactor,
		repoID int64,
		actor string,
		eventType string,
		oldExcludedAt *time.Time,
		oldExcludedReason *string,
		newExcludedAt *time.Time,
		newExcludedReason *string,
	) error {
		capturedAuditParams.repoID = repoID
		capturedAuditParams.actor = actor
		capturedAuditParams.eventType = eventType
		capturedAuditParams.oldExcludedAt = oldExcludedAt
		capturedAuditParams.oldExcludedReason = oldExcludedReason
		capturedAuditParams.newExcludedAt = newExcludedAt
		capturedAuditParams.newExcludedReason = newExcludedReason
		return nil
	}

	actor := "admin"
	reason := "new policy violation"

	err := SetRepoExclusionWithActor(ctx, mockDB, "github", "owner/repo", reason, actor)
	if err != nil {
		t.Errorf("SetRepoExclusionWithActor() should succeed, got error: %v", err)
	}

	// Verify audit parameters captured the old state
	if capturedAuditParams.repoID != 456 {
		t.Errorf("Expected repo_id 456, got %d", capturedAuditParams.repoID)
	}
	if capturedAuditParams.oldExcludedAt == nil {
		t.Errorf("Expected old excluded_at to be non-nil, got nil")
	} else if !capturedAuditParams.oldExcludedAt.Equal(oldTime) {
		t.Errorf("Expected old excluded_at %v, got %v", oldTime, *capturedAuditParams.oldExcludedAt)
	}
	if capturedAuditParams.oldExcludedReason == nil {
		t.Errorf("Expected old excluded_reason to be non-nil, got nil")
	} else if *capturedAuditParams.oldExcludedReason != oldReason {
		t.Errorf("Expected old excluded_reason '%s', got '%s'", oldReason, *capturedAuditParams.oldExcludedReason)
	}
	if capturedAuditParams.newExcludedReason == nil {
		t.Errorf("Expected new excluded_reason to be non-nil")
	} else if *capturedAuditParams.newExcludedReason != reason {
		t.Errorf("Expected new excluded_reason '%s', got '%s'", reason, *capturedAuditParams.newExcludedReason)
	}
}

// TestSetRepoExclusionWithActor_SelectError tests error handling when the SELECT query fails.
func TestSetRepoExclusionWithActor_SelectError(t *testing.T) {
	ctx := context.Background()

	selectError := errors.New("database select failed")

	// Create a transaction that fails on QueryRowContext
	mockTx := &mockTransactor{
		queryRowContextFn: func(ctx context.Context, query string, args ...interface{}) RowScanner {
			return &mockRow{scanErr: selectError}
		},
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{rowsAffected: 1, lastInsertId: 0}, nil
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

	err := SetRepoExclusionWithActor(ctx, mockDB, "github", "owner/repo", "test reason", "admin")
	if err == nil {
		t.Errorf("SetRepoExclusionWithActor() with select error should return error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to query current repo state") {
		t.Errorf("SetRepoExclusionWithActor() wrong error for select failure: %v", err)
	}
}

// TestSetRepoExclusion_WithSystemActor tests that SetRepoExclusion uses "system" as the actor.
func TestSetRepoExclusion_WithSystemActor(t *testing.T) {
	ctx := context.Background()

	var capturedActor string

	// Create a transaction
	mockTx := &mockTransactor{
		queryRowContextFn: func(ctx context.Context, query string, args ...interface{}) RowScanner {
			return &mockRow{
				scanValues: []interface{}{
					int64(789),
					(*time.Time)(nil),
					(*string)(nil),
				},
			}
		},
		execContextFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return &mockResult{rowsAffected: 1, lastInsertId: 0}, nil
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

	// Create a mock RecordExclusionAudit function
	originalRecordExclusionAudit := RecordExclusionAudit
	defer func() { RecordExclusionAudit = originalRecordExclusionAudit }()

	RecordExclusionAudit = func(
		ctx context.Context,
		tx Transactor,
		repoID int64,
		actor string,
		eventType string,
		oldExcludedAt *time.Time,
		oldExcludedReason *string,
		newExcludedAt *time.Time,
		newExcludedReason *string,
	) error {
		capturedActor = actor
		return nil
	}

	err := SetRepoExclusion(ctx, mockDB, "github", "owner/repo", "test reason")
	if err != nil {
		t.Errorf("SetRepoExclusion() should succeed, got error: %v", err)
	}

	if capturedActor != "system" {
		t.Errorf("Expected actor 'system', got '%s'", capturedActor)
	}
}
