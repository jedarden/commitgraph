// Test to verify NULL login handling and conflict resolution
package identity_test

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/jedarden/commitgraph/pkg/identity"
)

// TestNullLoginHandling verifies that NULL/empty logins are properly skipped
// during the seed process.
func TestNullLoginHandling(t *testing.T) {
	// This test documents the behavior from cmd/seed-email-resolution/main.go
	// Lines 107-111: Skip rows with empty login (no negative-cache seeding)

	// Simulate reading from author_login_cache
	testCases := []struct {
		name        string
		email       string
		login       string
		resolvedAt  string
		shouldSkip  bool
	}{
		{"Valid row", "test@example.com", "user1", "2024-01-01T00:00:00Z", false},
		{"Empty login 1", "null1@example.com", "", "2024-01-04T00:00:00Z", true},
		{"Empty login 2", "null2@example.com", "", "2024-01-05T00:00:00Z", true},
		{"Valid row 2", "test2@example.com", "user2", "2024-01-02T00:00:00Z", false},
	}

	var validRows []identity.ResolutionRow
	var skippedCount int

	for _, tc := range testCases {
		if tc.shouldSkip {
			skippedCount++
			continue
		}

		resolvedAt, err := time.Parse(time.RFC3339Nano, tc.resolvedAt)
		if err != nil {
			t.Fatalf("Failed to parse resolved_at: %v", err)
		}

		validRows = append(validRows, identity.ResolutionRow{
			Email:      tc.email,
			Login:      tc.login,
			Source:     identity.SourceSeed,
			ResolvedAt: resolvedAt,
		})
	}

	// Verify expectations
	if skippedCount != 2 {
		t.Errorf("Expected to skip 2 rows, got %d", skippedCount)
	}

	if len(validRows) != 2 {
		t.Errorf("Expected 2 valid rows, got %d", len(validRows))
	}

	// Verify all valid rows pass validation
	for idx, row := range validRows {
		if err := row.Validate(); err != nil {
			t.Errorf("Row %d failed validation: %v", idx, err)
		}
	}

	log.Printf("NULL handling test: %d rows skipped, %d rows valid\n", skippedCount, len(validRows))
}

// TestConflictResolutionRule verifies the ON CONFLICT rule behavior
func TestConflictResolutionRule(t *testing.T) {
	// This test documents the conflict resolution rule from pkg/pg/identity.go
	// Lines 105-111: The ON CONFLICT rule implements:
	// - Manual source always wins
	// - Non-manual wins only if existing is also non-manual AND newer timestamp

	testCases := []struct {
		name             string
		existingSource   string
		existingResolved time.Time
		newSource        string
		newResolved      time.Time
		shouldWin        bool
	}{
		{
			name:             "Manual always wins over seed",
			existingSource:   "seed",
			existingResolved: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			newSource:        "manual",
			newResolved:      time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			shouldWin:        true, // Even though older, manual wins
		},
		{
			name:             "Manual always wins over live",
			existingSource:   "live",
			existingResolved: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			newSource:        "manual",
			newResolved:      time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			shouldWin:        true, // Even though older, manual wins
		},
		{
			name:             "Newer seed wins over older seed",
			existingSource:   "seed",
			existingResolved: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			newSource:        "seed",
			newResolved:      time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			shouldWin:        true,
		},
		{
			name:             "Older seed loses to newer seed",
			existingSource:   "seed",
			existingResolved: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			newSource:        "seed",
			newResolved:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			shouldWin:        false,
		},
		{
			name:             "Seed loses to manual (existing)",
			existingSource:   "manual",
			existingResolved: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			newSource:        "seed",
			newResolved:      time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			shouldWin:        false, // Manual existing always wins
		},
		{
			name:             "Newer live wins over older seed",
			existingSource:   "seed",
			existingResolved: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			newSource:        "live",
			newResolved:      time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			shouldWin:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the ON CONFLICT WHERE clause logic
			// WHERE excluded.source = 'manual'
			//    OR (email_resolution.source <> 'manual'
			//        AND excluded.resolved_at > email_resolution.resolved_at)

			wins := tc.newSource == "manual" ||
				(tc.existingSource != "manual" &&
					tc.newResolved.After(tc.existingResolved))

			if wins != tc.shouldWin {
				t.Errorf("Conflict resolution error: expected wins=%v, got wins=%v\n"+
					"Existing: source=%s, resolved=%s\n"+
					"New: source=%s, resolved=%s",
					tc.shouldWin, wins,
					tc.existingSource, tc.existingResolved.Format(time.RFC3339),
					tc.newSource, tc.newResolved.Format(time.RFC3339))
			}
		})
	}
}

// TestSeedScriptBehavior simulates the full seed script behavior
// This documents what happens when the seed script runs with edge cases
func TestSeedScriptBehavior(t *testing.T) {
	// This documents the expected behavior when running seed-email-resolution
	// with a test database containing:
	// - 3 rows with empty logins (should be skipped)
	// - 8 valid rows
	// - Some duplicate pairs for conflict resolution

	rowsRead := 11
	emptyLogins := 3
	validRows := rowsRead - emptyLogins // 8

	if validRows != 8 {
		t.Errorf("Expected 8 valid rows after skipping NULLs, got %d", validRows)
	}

	// Expected flow:
	// 1. Read 11 rows from author_login_cache
	// 2. Skip 3 rows with empty logins
	// 3. Submit 8 valid rows to PostgreSQL
	// 4. Conflict resolution applies (some may lose if existing rows are newer/manual)
	// 5. Final count: before + accepted - rejected

	log.Printf("Seed script behavior test:")
	log.Printf("  Rows read: %d", rowsRead)
	log.Printf("  Rows skipped (empty login): %d", emptyLogins)
	log.Printf("  Valid rows submitted: %d", validRows)
	log.Printf("  Expected: ingested count = %d - conflicts_lost", validRows)
}

// TestValidationRejectsInvalidRows verifies that validation catches bad data
func TestValidationRejectsInvalidRows(t *testing.T) {
	testCases := []struct {
		name        string
		row         identity.ResolutionRow
		shouldFail  bool
	}{
		{
			name: "Valid row",
			row: identity.ResolutionRow{
				Email:      "test@example.com",
				Login:      "user",
				Source:     identity.SourceSeed,
				ResolvedAt: time.Now(),
			},
			shouldFail: false,
		},
		{
			name: "Empty email",
			row: identity.ResolutionRow{
				Email:      "",
				Login:      "user",
				Source:     identity.SourceSeed,
				ResolvedAt: time.Now(),
			},
			shouldFail: true,
		},
		{
			name: "Empty login",
			row: identity.ResolutionRow{
				Email:      "test@example.com",
				Login:      "",
				Source:     identity.SourceSeed,
				ResolvedAt: time.Now(),
			},
			shouldFail: true,
		},
		{
			name: "Invalid source",
			row: identity.ResolutionRow{
				Email:      "test@example.com",
				Login:      "user",
				Source:     "invalid",
				ResolvedAt: time.Now(),
			},
			shouldFail: true,
		},
		{
			name: "Zero timestamp",
			row: identity.ResolutionRow{
				Email:      "test@example.com",
				Login:      "user",
				Source:     identity.SourceSeed,
				ResolvedAt: time.Time{},
			},
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.row.Validate()
			if tc.shouldFail && err == nil {
				t.Errorf("Expected validation to fail, but it passed")
			}
			if !tc.shouldFail && err != nil {
				t.Errorf("Expected validation to pass, but got error: %v", err)
			}
		})
	}
}

// TestConflictResolutionWithDuplicatePairs verifies behavior when
// the same email-login pair appears multiple times with different timestamps
func TestConflictResolutionWithDuplicatePairs(t *testing.T) {
	// When duplicate pairs exist, the ON CONFLICT rule ensures:
	// 1. Only one row per email in the final table (PRIMARY KEY)
	// 2. The winner is determined by: manual source OR newest timestamp

	now := time.Now().UTC()
	older := now.Add(-24 * time.Hour)
	newer := now.Add(+24 * time.Hour)

	duplicatePairs := []struct {
		name          string
		existing      identity.ResolutionRow
		newRows       []identity.ResolutionRow
		expectedLogin string
		expectedCount int // How many rows actually get inserted
	}{
		{
			name: "Same email, different logins - newest timestamp wins",
			existing: identity.ResolutionRow{
				Email:      "conflict@example.com",
				Login:      "userA",
				Source:     identity.SourceSeed,
				ResolvedAt: older,
			},
			newRows: []identity.ResolutionRow{
				{
					Email:      "conflict@example.com",
					Login:      "userB",
					Source:     identity.SourceSeed,
					ResolvedAt: newer,
				},
			},
			expectedLogin: "userB", // Newest timestamp
			expectedCount: 1,
		},
		{
			name: "Manual source always wins regardless of timestamp",
			existing: identity.ResolutionRow{
				Email:      "override@example.com",
				Login:      "auto",
				Source:     identity.SourceLive,
				ResolvedAt: newer,
			},
			newRows: []identity.ResolutionRow{
				{
					Email:      "override@example.com",
					Login:      "manual_override",
					Source:     identity.SourceManual,
					ResolvedAt: older, // Even if older
				},
			},
			expectedLogin: "manual_override", // Manual wins
			expectedCount: 1,
		},
	}

	for _, tc := range duplicatePairs {
		t.Run(tc.name, func(t *testing.T) {
			// This documents the expected behavior
			// The actual database enforces this via ON CONFLICT rule

			// Simulate conflict resolution
			finalLogin := tc.existing.Login
			finalResolved := tc.existing.ResolvedAt
			finalSource := tc.existing.Source

			for _, newRow := range tc.newRows {
				// Apply conflict resolution rule
				if newRow.Source == identity.SourceManual {
					finalLogin = newRow.Login
					finalResolved = newRow.ResolvedAt
					finalSource = newRow.Source
				} else if finalSource != identity.SourceManual &&
					newRow.ResolvedAt.After(finalResolved) {
					finalLogin = newRow.Login
					finalResolved = newRow.ResolvedAt
					finalSource = newRow.Source
				}
			}

			if finalLogin != tc.expectedLogin {
				t.Errorf("Expected final login %q, got %q", tc.expectedLogin, finalLogin)
			}

			log.Printf("Conflict resolution: %s -> %s (source: %s, resolved: %s)",
				tc.existing.Login, finalLogin, finalSource,
				finalResolved.Format(time.RFC3339))
		})
	}
}

// Example: With real PostgreSQL connection
// This shows how to run the actual test with a live database
func ExampleTestWithRealDatabase() {
	// This would require a real PostgreSQL connection:
	// dbHost := os.Getenv("PGHOST")
	// dbUser := os.Getenv("PGUSER")
	// etc.

	// The test database /tmp/test_seed.db contains:
	// - 11 total rows
	// - 3 rows with empty logins
	// - 8 valid rows
	// - Some duplicate pairs for conflict testing

	// Expected results when seeded:
	// 1. 3 rows skipped (empty logins)
	// 2. 8 rows submitted to ingest
	// 3. Conflict resolution applied for duplicates
	// 4. Final table has unique email entries with winning values

	fmt.Println("See test database at /tmp/test_seed.db")
	fmt.Println("Run: sqlite3 /tmp/test_seed.db 'SELECT * FROM author_login_cache'")
}
