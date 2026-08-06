package pg

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// TestExclusionOperations tests the exclusion operations using a mock or test DB.
// This is a placeholder for the actual test implementation which would set up
// a test database schema and run the full operation cycle.
func TestExclusionOperations(t *testing.T) {
	// TODO: Set up test database with schema
	// Test cases:
	// 1. Apply exclusion to existing repo
	// 2. Verify exclusion is applied
	// 3. Clear exclusion
	// 4. Verify exclusion is cleared
	// 5. Apply exclusion to non-existent repo (returns 0 rows affected)
	// 6. List exclusions
	// 7. Get exclusion status
	t.Skip("requires test database setup")
}

// mockExec is a mock implementation of Executor for testing.
type mockExec struct{}

func (m *mockExec) ExecContext(ctx context.Context, query string, args ...interface{}) (Result, error) {
	return &mockResult{}, nil
}

func (m *mockExec) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

func (m *mockExec) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

type mockResult struct{}

func (m *mockResult) RowsAffected() (int64, error) {
	return 1, nil
}

// TestNewRepoExcluder tests the constructor.
func TestNewRepoExcluder(t *testing.T) {
	m := &mockExec{}
	excluder := NewRepoExcluder(m)
	if excluder == nil {
		t.Fatal("NewRepoExcluder returned nil")
	}
	if excluder.db != m {
		t.Error("excluder.db not set correctly")
	}
}

// TestExclusionRequestValidation tests request validation.
func TestExclusionRequestValidation(t *testing.T) {
	ctx := context.Background()
	excluder := NewRepoExcluder(&mockExec{})

	now := time.Now()

	tests := []struct {
		name    string
		req     ExclusionRequest
		wantErr bool
	}{
		{
			name: "valid exclude",
			req: ExclusionRequest{
				Provider:      "github",
				RepoFullName:  "owner/repo",
				ExcludedAt:    &now,
				ExcludedReason: "test reason",
				Operator:      "test-operator",
			},
			wantErr: false,
		},
		{
			name: "valid clear",
			req: ExclusionRequest{
				Provider:     "github",
				RepoFullName: "owner/repo",
				ExcludedAt:   nil, // NULL for clear
				Operator:     "test-operator",
			},
			wantErr: false,
		},
		{
			name: "missing provider",
			req: ExclusionRequest{
				RepoFullName:  "owner/repo",
				ExcludedAt:    &now,
				ExcludedReason: "test reason",
				Operator:      "test-operator",
			},
			wantErr: true,
		},
		{
			name: "missing repo_full_name",
			req: ExclusionRequest{
				Provider:      "github",
				ExcludedAt:    &now,
				ExcludedReason: "test reason",
				Operator:      "test-operator",
			},
			wantErr: true,
		},
		{
			name: "missing operator",
			req: ExclusionRequest{
				Provider:      "github",
				RepoFullName:  "owner/repo",
				ExcludedAt:    &now,
				ExcludedReason: "test reason",
			},
			wantErr: true,
		},
		{
			name: "exclude without reason",
			req: ExclusionRequest{
				Provider:     "github",
				RepoFullName: "owner/repo",
				ExcludedAt:   &now,
				// ExcludedReason is empty
				Operator: "test-operator",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := excluder.ApplyExclusion(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyExclusion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
