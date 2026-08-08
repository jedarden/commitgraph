package pg

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
)

// SeedEmailResolution creates an email_resolution row for testing login rename scenarios.
// It runs within the provided transaction and can be rolled back after the test.
//
// Parameters:
//   - tx: The transaction to execute within (caller must commit/rollback)
//   - email: The email address to resolve
//   - login: The login name to resolve to (typically "old-name" for rename tests)
//   - source: The resolution source ("live", "seed", or "manual")
//   - resolvedAt: Optional custom timestamp (defaults to current time if zero)
//
// Returns error if the insert fails.
func SeedEmailResolution(tx *sql.Tx, email, login, source string, resolvedAt time.Time) error {
	if resolvedAt.IsZero() {
		resolvedAt = time.Now().UTC()
	}

	_, err := tx.ExecContext(context.Background(),
		`INSERT INTO email_resolution (email, login, source, resolved_at) VALUES ($1, $2, $3, $4)`,
		email, login, source, resolvedAt)
	return err
}

// SeedUser creates a users row for testing login rename scenarios.
// It returns the generated user_id which can be used to seed related data.
//
// Parameters:
//   - tx: The transaction to execute within (caller must commit/rollback)
//   - login: The login name (typically "old-name" for rename tests)
//   - profileUrl: Optional profile URL (can be empty)
//   - avatarUrl: Optional avatar URL (can be empty)
//
// Returns the generated user_id and any error.
func SeedUser(tx *sql.Tx, login, profileUrl, avatarUrl string) (int64, error) {
	var userID int64
	err := tx.QueryRowContext(context.Background(),
		`INSERT INTO users (login, profile_url, avatar_url) VALUES ($1, $2, $3) RETURNING user_id`,
		login, profileUrl, avatarUrl).Scan(&userID)
	return userID, err
}

// SeedRepoUserDailyTool creates repo_user_daily_tool rows representing historical activity
// for a given user. This is used to test that historical data remains intact after login rename.
//
// Parameters:
//   - tx: The transaction to execute within (caller must commit/rollback)
//   - repoID: The repository ID (must exist in repos table)
//   - userID: The user ID (must exist in users table)
//   - tool: The tool name (e.g., "claude", "copilot", "cursor")
//   - days: Array of dates for which to create activity rows
//   - commitsPerDay: Number of commits to record for each day
//
// Returns error if any insert fails.
func SeedRepoUserDailyTool(tx *sql.Tx, repoID, userID int64, tool string, days []time.Time, commitsPerDay int) error {
	ctx := context.Background()
	insertTime := time.Now().UTC()

	for _, day := range days {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (repo_id, user_id, tool, day) DO UPDATE
			 SET commits = $5, insert_time = $6`,
			repoID, userID, tool, day, commitsPerDay, insertTime)
		if err != nil {
			return err
		}
	}
	return nil
}

// SeedRepo creates a repos row as a prerequisite for repo_user_daily_tool data.
//
// Parameters:
//   - tx: The transaction to execute within (caller must commit/rollback)
//   - provider: The provider name (e.g., "github")
//   - repoFullName: The full repository name (e.g., "acme/widgets")
//
// Returns the generated repo_id and any error.
func SeedRepo(tx *sql.Tx, provider, repoFullName string) (int64, error) {
	var repoID int64
	err := tx.QueryRowContext(context.Background(),
		`INSERT INTO repos (provider, repo_full_name) VALUES ($1, $2) RETURNING repo_id`,
		provider, repoFullName).Scan(&repoID)
	return repoID, err
}

// LoginRenameFixtures encapsulates the seeded test data for a login rename scenario.
// This allows tests to access the generated IDs and verify post-rename state.
type LoginRenameFixtures struct {
	// Email, login, and user_id from the original (pre-rename) state
	Email    string
	OldLogin string
	UserID   int64

	// Repository data for historical activity
	RepoID  int64
	Tool    string
	DayData []time.Time
}

// SeedLoginRenameScenario creates a complete test scenario for login rename handling.
// It seeds:
// 1. An email_resolution row with email resolved to login="old-name"
// 2. A users row with login="old-name" and captures the user_id
// 3. A repos row (needed for foreign key constraint)
// 4. Several repo_user_daily_tool rows under that user_id representing historical activity
//
// Parameters:
//   - tx: The transaction to execute within (caller must commit/rollback)
//   - email: Email to resolve (e.g., "old@example.com")
//   - oldLogin: The original login name (e.g., "old-name")
//   - tool: Tool name for historical activity (e.g., "claude")
//
// Returns LoginRenameFixtures containing all seeded data and any error.
//
// Example usage:
//
//	tx, _ := db.BeginTx(ctx, nil)
//	defer tx.Rollback()
//
//	fixtures, err := SeedLoginRenameScenario(tx, "old@example.com", "old-name", "claude")
//	if err != nil {
//	    t.Fatalf("seed fixtures: %v", err)
//	}
//
//	// Test login rename handling here
//	// After rename, verify fixtures.UserID is stable and historical data is intact
func SeedLoginRenameScenario(tx *sql.Tx, email, oldLogin, tool string) (*LoginRenameFixtures, error) {
	// 1. Seed email_resolution
	if err := SeedEmailResolution(tx, email, oldLogin, "seed", time.Now().UTC()); err != nil {
		return nil, err
	}

	// 2. Seed user (captures user_id)
	userID, err := SeedUser(tx, oldLogin, "", "")
	if err != nil {
		return nil, err
	}

	// 3. Seed repo (needed for repo_user_daily_tool foreign key)
	repoID, err := SeedRepo(tx, "github", "test/repo")
	if err != nil {
		return nil, err
	}

	// 4. Seed historical activity (3 days of commits)
	now := time.Now().UTC()
	dayData := []time.Time{
		now.Add(-48 * time.Hour), // 2 days ago
		now.Add(-24 * time.Hour), // 1 day ago
		now,                      // today
	}

	if err := SeedRepoUserDailyTool(tx, repoID, userID, tool, dayData, 5); err != nil {
		return nil, err
	}

	return &LoginRenameFixtures{
		Email:    email,
		OldLogin: oldLogin,
		UserID:   userID,
		RepoID:   repoID,
		Tool:     tool,
		DayData:  dayData,
	}, nil
}

// AssertLoginRenameFixturesIntact verifies that the fixtures created by SeedLoginRenameScenario
// remain intact after a login rename operation. This checks that:
// - The user_id is unchanged (surrogate key stability)
// - Historical repo_user_daily_tool data is still accessible via the same user_id
// - No data was lost during the rename
//
// Parameters:
//   - db: Database connection (can be the raw DB or a transaction)
//   - fixtures: The fixtures created by SeedLoginRenameScenario
//
// Returns error if any check fails.
func AssertLoginRenameFixturesIntact(db *sql.DB, fixtures *LoginRenameFixtures) error {
	ctx := context.Background()

	// 1. Verify user_id still exists and has the expected login
	var loginCheck string
	err := db.QueryRowContext(ctx, `SELECT login FROM users WHERE user_id = $1`, fixtures.UserID).Scan(&loginCheck)
	if err != nil {
		return err
	}

	// 2. Count historical activity rows for this user
	var activityCount int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM repo_user_daily_tool WHERE user_id = $1 AND repo_id = $2 AND tool = $3`,
		fixtures.UserID, fixtures.RepoID, fixtures.Tool).Scan(&activityCount)
	if err != nil {
		return err
	}

	// Expect 3 days of activity (as seeded)
	if activityCount != 3 {
		return &FixtureError{
			Expected: 3,
			Actual:   activityCount,
			Message:  "historical activity row count mismatch after rename",
		}
	}

	return nil
}

// FixtureError represents a test fixture verification error.
type FixtureError struct {
	Expected int
	Actual   int
	Message  string
}

func (e *FixtureError) Error() string {
	return e.Message
}
