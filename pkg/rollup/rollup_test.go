package rollup

import (
	"testing"
	"time"
)

func TestQuarantineBounds_IsIncluded(t *testing.T) {
	// Create bounds for 2026-08-05
	today := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	bounds := NewQuarantineBounds(today)

	tests := []struct {
		name        string
		committedAt time.Time
		wantIncluded bool
		description  string
	}{
		{
			name:        "below lower bound",
			committedAt: time.Date(2004, 12, 31, 23, 59, 59, 0, time.UTC),
			wantIncluded: false,
			description:  "2004-12-31 is excluded from rollup",
		},
		{
			name:        "at lower bound inclusive",
			committedAt: time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC),
			wantIncluded: true,
			description:  "2005-01-01 is included in rollup",
		},
		{
			name:        "normal commit date",
			committedAt: time.Date(2024, 6, 15, 12, 30, 0, 0, time.UTC),
			wantIncluded: true,
			description:  "Normal 2024 commit is included",
		},
		{
			name:        "at upper bound inclusive",
			committedAt: time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC),
			wantIncluded: true,
			description:  "today+1 (2026-08-06) is included in rollup",
		},
		{
			name:        "above upper bound",
			committedAt: time.Date(2026, 8, 7, 0, 0, 1, 0, time.UTC),
			wantIncluded: false,
			description:  "today+2 (2026-08-07) is excluded from rollup",
		},
		{
			name:        "far future date 2170 incident",
			committedAt: time.Date(2170, 1, 1, 0, 0, 0, 0, time.UTC),
			wantIncluded: false,
			description:  "2170-dated commit is excluded (reproduces real incident)",
		},
		{
			name:        "edge case exactly at max date midnight",
			committedAt: time.Date(2026, 8, 6, 23, 59, 59, 999999999, time.UTC),
			wantIncluded: true,
			description:  "Last nanosecond of today+1 is included",
		},
		{
			name:        "edge case first nanosecond after max",
			committedAt: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
			wantIncluded: false,
			description:  "First nanosecond of today+2 is excluded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bounds.IsIncluded(tt.committedAt)
			if got != tt.wantIncluded {
				t.Errorf("%s: IsIncluded() = %v, want %v (%s)", tt.name, got, tt.wantIncluded, tt.description)
			}
		})
	}
}

func TestComputeRollup_DateFiltering(t *testing.T) {
	// Create bounds for 2026-08-05
	today := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	bounds := NewQuarantineBounds(today)

	commits := []Commit{
		{
			SHA:         "abc1",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2004, 12, 31, 0, 0, 0, 0, time.UTC),
			Message:     "Too old",
			Tools:       []string{"claude"},
		},
		{
			SHA:         "abc2",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC),
			Message:     "At lower bound",
			Tools:       []string{"claude"},
		},
		{
			SHA:         "abc3",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC),
			Message:     "At upper bound",
			Tools:       []string{"claude"},
		},
		{
			SHA:         "abc4",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
			Message:     "Too new",
			Tools:       []string{"claude"},
		},
		{
			SHA:         "abc5",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2170, 1, 1, 0, 0, 0, 0, time.UTC),
			Message:     "Far future",
			Tools:       []string{"claude"},
		},
	}

	rollup := ComputeRollup(commits, 123, bounds)

	// Should only include 2 commits: abc2, abc3 (abc1, abc4, abc5 excluded by date filter)
	// These are on different days, so they create 2 separate rollup rows
	if len(rollup) != 2 {
		t.Errorf("Expected 2 rollup rows (one per day), got %d", len(rollup))
	}

	// Verify each rollup row has count 1 (different days)
	totalCommits := 0
	for _, row := range rollup {
		totalCommits += row.Count
		if row.Count != 1 {
			t.Errorf("Expected 1 commit per rollup row, got %d", row.Count)
		}
		if row.UserEmail != "user@example.com" {
			t.Errorf("Expected user@example.com, got %s", row.UserEmail)
		}
		if row.Tool != "claude" {
			t.Errorf("Expected tool 'claude', got %s", row.Tool)
		}
	}
	if totalCommits != 2 {
		t.Errorf("Expected 2 total commits aggregated, got %d", totalCommits)
	}
}

func TestComputeRollup_NoToolCommitsExcluded(t *testing.T) {
	today := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	bounds := NewQuarantineBounds(today)

	commits := []Commit{
		{
			SHA:         "abc1",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			Message:     "No AI tool",
			Tools:       []string{}, // No AI tools detected
		},
	}

	rollup := ComputeRollup(commits, 123, bounds)

	if len(rollup) != 0 {
		t.Errorf("Expected 0 rollup rows (no AI tools), got %d", len(rollup))
	}
}

func TestComputeRollup_MultipleToolsPerCommit(t *testing.T) {
	today := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	bounds := NewQuarantineBounds(today)

	commits := []Commit{
		{
			SHA:         "abc1",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			Message:     "Multiple tools",
			Tools:       []string{"claude", "cursor"}, // Two tools
		},
	}

	rollup := ComputeRollup(commits, 123, bounds)

	// Should create 2 rollup rows (one per tool)
	if len(rollup) != 2 {
		t.Errorf("Expected 2 rollup rows (one per tool), got %d", len(rollup))
	}
}

func TestComputeRollup_AggregationByDay(t *testing.T) {
	today := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	bounds := NewQuarantineBounds(today)

	// Same day, different times - should aggregate
	commits := []Commit{
		{
			SHA:         "abc1",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC),
			Message:     "Morning",
			Tools:       []string{"claude"},
		},
		{
			SHA:         "abc2",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2024, 6, 15, 18, 45, 0, 0, time.UTC),
			Message:     "Evening",
			Tools:       []string{"claude"},
		},
	}

	rollup := ComputeRollup(commits, 123, bounds)

	if len(rollup) != 1 {
		t.Errorf("Expected 1 rollup row (aggregated by day), got %d", len(rollup))
	}

	if len(rollup) > 0 && rollup[0].Count != 2 {
		t.Errorf("Expected 2 commits aggregated, got %d", rollup[0].Count)
	}
}

func TestComputeRollup_Synthetic2170Fixture(t *testing.T) {
	today := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	bounds := NewQuarantineBounds(today)

	// Synthetic fixture repo with one 2170-dated commit
	commits := []Commit{
		{
			SHA:         "future-abc",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2170, 1, 1, 0, 0, 0, 0, time.UTC),
			Message:     "Future commit",
			Tools:       []string{"claude"},
		},
	}

	rollup := ComputeRollup(commits, 123, bounds)

	// 2170-dated commit should produce zero rollup rows
	if len(rollup) != 0 {
		t.Errorf("Expected 0 rollup rows for 2170-dated commit, got %d", len(rollup))
	}
}

func TestComputeRollup_ParquetPreservation(t *testing.T) {
	today := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	bounds := NewQuarantineBounds(today)

	commits := []Commit{
		{
			SHA:         "abc1",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2004, 12, 31, 0, 0, 0, 0, time.UTC), // Out of range
			Message:     "Old commit",
			Tools:       []string{"claude"},
		},
		{
			SHA:         "abc2",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2170, 1, 1, 0, 0, 0, 0, time.UTC), // Out of range
			Message:     "Future commit",
			Tools:       []string{"claude"},
		},
		{
			SHA:         "abc3",
			AuthorEmail: "user@example.com",
			AuthorName:  "Test User",
			CommittedAt: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC), // In range
			Message:     "Normal commit",
			Tools:       []string{"claude"},
		},
	}

	rollup := ComputeRollup(commits, 123, bounds)

	// Rollup should only include in-range commits
	if len(rollup) != 1 {
		t.Errorf("Expected 1 rollup row (only in-range commit), got %d", len(rollup))
	}

	// Verify that original commits still exist for Parquet writing
	// This is the caller's responsibility - we just don't filter the input
	if len(commits) != 3 {
		t.Errorf("Input commits should be preserved for Parquet, got %d", len(commits))
	}

	// Verify all original committed_at values are present
	expectedDates := []time.Time{
		time.Date(2004, 12, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2170, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
	}

	for i, commit := range commits {
		if !commit.CommittedAt.Equal(expectedDates[i]) {
			t.Errorf("Commit %d: expected committed_at %v, got %v", i, expectedDates[i], commit.CommittedAt)
		}
	}
}
