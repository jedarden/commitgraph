package queueapi

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver for queue-api
)

// TestRepoQueueKindCoexistence tests the widened UNIQUE constraint that allows
// both normal-clone and redetect jobs for the same repository to coexist.
//
// This is a regression test for the constraint widening described in
// docs/plan/plan.md: the original UNIQUE (provider, repo_full_name) constraint
// would have prevented two different job kinds for the same repo from being
// queued simultaneously. The widened constraint UNIQUE (provider, repo_full_name, kind)
// allows this scenario.
//
// Scenario: A catalog version bump triggers a redetect job for a repository that
// already has a pending normal-clone job (e.g., due for its regular rescan).
// Both jobs must be able to exist in the queue without constraint violation.
func TestRepoQueueKindCoexistence(t *testing.T) {
	// Create an in-memory SQLite database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Set up the schema
	if err := setupSchema(db); err != nil {
		t.Fatalf("Failed to set up schema: %v", err)
	}

	// Test data
	provider := "github"
	repoFullName := "test-owner/test-repo"
	now := time.Now().UTC()

	t.Run("insert normal-clone then redetect for same repo", func(t *testing.T) {
		// Step 1: Insert a normal-clone job
		normalCloneID, err := insertJob(db, provider, repoFullName, "normal-clone", now)
		if err != nil {
			t.Fatalf("Failed to insert normal-clone job: %v", err)
		}
		if normalCloneID == 0 {
			t.Fatal("Expected non-zero ID for normal-clone job")
		}

		// Step 2: Insert a redetect job for the SAME repo
		// This should succeed with the widened constraint
		redetectID, err := insertJob(db, provider, repoFullName, "redetect", now)
		if err != nil {
			t.Fatalf("Failed to insert redetect job (constraint violation?): %v", err)
		}
		if redetectID == 0 {
			t.Fatal("Expected non-zero ID for redetect job")
		}

		// Step 3: Verify both jobs exist in the queue
		jobs, err := getJobsByRepo(db, provider, repoFullName)
		if err != nil {
			t.Fatalf("Failed to query jobs: %v", err)
		}

		if len(jobs) != 2 {
			t.Errorf("Expected 2 jobs, got %d", len(jobs))
		}

		// Verify we have one of each kind
		kinds := make(map[string]bool)
		for _, job := range jobs {
			kinds[job.Kind] = true
		}

		if !kinds["normal-clone"] {
			t.Error("Missing normal-clone job")
		}
		if !kinds["redetect"] {
			t.Error("Missing redetect job")
		}
	})

	t.Run("prevent duplicate jobs of same kind", func(t *testing.T) {
		// The constraint should still prevent duplicate jobs of the same kind
		// for the same repo

		// First redetect job
		_, err := insertJob(db, provider, "duplicate-test/repo", "redetect", now)
		if err != nil {
			t.Fatalf("Failed to insert first redetect job: %v", err)
		}

		// Second redetect job for same repo should fail
		_, err = insertJob(db, provider, "duplicate-test/repo", "redetect", now)
		if err == nil {
			t.Error("Expected constraint violation when inserting duplicate redetect job, got nil")
		}
	})

	t.Run("allow different kinds for same repo concurrently", func(t *testing.T) {
		// Multiple different job kinds should coexist for the same repo
		repo := "multi-job/repo"

		kinds := []string{"normal-clone", "redetect", "fork-clone", "mirror-clone"}
		ids := make([]int64, len(kinds))

		for i, kind := range kinds {
			id, err := insertJob(db, provider, repo, kind, now)
			if err != nil {
				t.Fatalf("Failed to insert %s job: %v", kind, err)
			}
			ids[i] = id
		}

		// Verify all jobs exist
		jobs, err := getJobsByRepo(db, provider, repo)
		if err != nil {
			t.Fatalf("Failed to query jobs: %v", err)
		}

		if len(jobs) != len(kinds) {
			t.Errorf("Expected %d jobs, got %d", len(kinds), len(jobs))
		}
	})

	t.Run("different repos can have same job kind", func(t *testing.T) {
		// Different repos can each have their own normal-clone job
		repos := []string{
			"owner1/repo1",
			"owner2/repo2",
			"owner3/repo3",
		}

		for _, repo := range repos {
			_, err := insertJob(db, provider, repo, "normal-clone", now)
			if err != nil {
				t.Fatalf("Failed to insert normal-clone job for %s: %v", repo, err)
			}
		}

		// Verify all repos have their jobs
		for _, repo := range repos {
			jobs, err := getJobsByRepo(db, provider, repo)
			if err != nil {
				t.Fatalf("Failed to query jobs for %s: %v", repo, err)
			}
			if len(jobs) != 1 {
				t.Errorf("Expected 1 job for %s, got %d", repo, len(jobs))
			}
		}
	})
}

// TestRepoQueueConstraintWidening documents the specific behavior this test protects.
//
// The constraint widening from UNIQUE (provider, repo_full_name) to
// UNIQUE (provider, repo_full_name, kind) enables the following workflow:
//
// 1. A repository is already in the queue with a pending normal-clone job
// 2. A catalog version bump occurs (new tool signature added)
// 3. The system enqueues a redetect job for the same repository
// 4. Both jobs coexist in the queue without constraint violation
// 5. Workers process each job independently
//
// Without the widened constraint, step 3 would fail with a UNIQUE constraint violation.
func TestRepoQueueConstraintWidening(t *testing.T) {
	t.Log("Constraint widening behavior:")
	t.Log("")
	t.Log("Original constraint: UNIQUE (provider, repo_full_name)")
	t.Log("  - Only one job per repo regardless of kind")
	t.Log("  - Blocked scenario: repo with both normal-clone AND redetect pending")
	t.Log("")
	t.Log("Widened constraint: UNIQUE (provider, repo_full_name, kind)")
	t.Log("  - One job per (repo, kind) combination")
	t.Log("  - Allows scenario: normal-clone + redetect for same repo")
	t.Log("")
	t.Log("Example collision scenario from plan.md:")
	t.Log("  - Catalog version bumps (e.g., new Co-Authored-By pattern)")
	t.Log("  - Redetect jobs queued for repos affected by new detection")
	t.Log("  - Some repos also due for normal rescan (independent timing)")
	t.Log("  - Both jobs must coexist; widening prevents collision")
}

// job represents a row in the repo_queue table
type job struct {
	ID            int64
	Provider      string
	RepoFullName  string
	Kind          string
	Status        string
	ClaimedAt     *time.Time
	CompletedAt   *time.Time
	ErrorMessage  *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// setupSchema creates the repo_queue table in the test database
func setupSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS repo_queue (
	  id              INTEGER PRIMARY KEY AUTOINCREMENT,
	  provider        TEXT NOT NULL,
	  repo_full_name  TEXT NOT NULL,
	  kind            TEXT NOT NULL DEFAULT 'normal-clone',
	  status          TEXT NOT NULL DEFAULT 'pending',
	  claimed_at      TIMESTAMP,
	  completed_at    TIMESTAMP,
	  error_message   TEXT,
	  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	  updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	  CONSTRAINT repo_queue_provider_repo_kind UNIQUE (provider, repo_full_name, kind)
	);

	CREATE INDEX IF NOT EXISTS repo_queue_status_idx ON repo_queue (status);
	CREATE INDEX IF NOT EXISTS repo_queue_kind_idx ON repo_queue (kind);
	CREATE INDEX IF NOT EXISTS repo_queue_created_at_idx ON repo_queue (created_at);
	CREATE INDEX IF NOT EXISTS repo_queue_provider_repo_idx ON repo_queue (provider, repo_full_name);
	`

	_, err := db.Exec(schema)
	return err
}

// insertJob inserts a new job into the repo_queue table
func insertJob(db *sql.DB, provider, repoFullName, kind string, createdAt time.Time) (int64, error) {
	query := `
	INSERT INTO repo_queue (provider, repo_full_name, kind, status, created_at, updated_at)
	VALUES (?, ?, ?, 'pending', ?, ?)
	`

	result, err := db.Exec(query, provider, repoFullName, kind, createdAt, createdAt)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// getJobsByRepo retrieves all jobs for a specific repository
func getJobsByRepo(db *sql.DB, provider, repoFullName string) ([]job, error) {
	query := `
	SELECT id, provider, repo_full_name, kind, status,
	       claimed_at, completed_at, error_message,
	       created_at, updated_at
	FROM repo_queue
	WHERE provider = ? AND repo_full_name = ?
	ORDER BY created_at
	`

	rows, err := db.Query(query, provider, repoFullName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []job
	for rows.Next() {
		var j job
		err := rows.Scan(
			&j.ID,
			&j.Provider,
			&j.RepoFullName,
			&j.Kind,
			&j.Status,
			&j.ClaimedAt,
			&j.CompletedAt,
			&j.ErrorMessage,
			&j.CreatedAt,
			&j.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}

	return jobs, nil
}
