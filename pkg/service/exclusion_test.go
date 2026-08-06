package service

import (
	"context"
	"database/sql"
	"errors"
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

