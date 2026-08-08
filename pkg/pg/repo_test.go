package pg

import (
	"context"
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

// TestNewRepoExcluder tests the constructor.
func TestNewRepoExcluder(t *testing.T) {
	m := &mockExecutor{}
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
	excluder := NewRepoExcluder(&mockExecutor{})

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

// TestRepoExcluder_GetExclusion_Success tests the GetExclusion method
// when no database error occurs. The mock returns no error but no data,
// which represents the "no exclusion found" case.
func TestRepoExcluder_GetExclusion_Success(t *testing.T) {
	db := &mockExecutor{} // shouldError defaults to false
	excluder := NewRepoExcluder(db)

	excludedAt, reason, err := excluder.GetExclusion(context.Background(), "github", "owner/repo")

	// Success means no error from the database operation
	if err != nil {
		t.Fatalf("GetExclusion() unexpected error: %v", err)
	}

	// No exclusion data found (mock doesn't populate values)
	// This is the "success" case where the query runs but finds no exclusion
	if excludedAt != nil {
		t.Errorf("GetExclusion() excludedAt = %v, want nil (no exclusion found)", excludedAt)
	}

	if reason != "" {
		t.Errorf("GetExclusion() reason = %q, want empty string (no exclusion found)", reason)
	}
}

// TestRepoExcluder_GetExclusion_DatabaseError tests the GetExclusion method
// when the database returns an error (after mock fix).
func TestRepoExcluder_GetExclusion_DatabaseError(t *testing.T) {
	db := &mockExecutor{shouldError: true} // Error flag set
	excluder := NewRepoExcluder(db)

	excludedAt, reason, err := excluder.GetExclusion(context.Background(), "github", "owner/repo")

	// Should get an error from the mock scan
	if err == nil {
		t.Fatal("GetExclusion() expected error, got nil")
	}

	// Error case should return nil values
	if excludedAt != nil {
		t.Errorf("GetExclusion() excludedAt = %v, want nil on error", excludedAt)
	}

	if reason != "" {
		t.Errorf("GetExclusion() reason = %q, want empty string on error", reason)
	}

	// Verify error message contains expected context
	if err.Error() == "" {
		t.Error("GetExclusion() error message is empty")
	}
}
