// Package rollup provides rollup computation for commitgraph.
//
// The rollup aggregates AI-tool-tagged commits by (user, repo, tool, day)
// while applying date quarantine filtering to exclude out-of-range commits
// from the rollup table while preserving them in the raw Parquet artifact.
package rollup

import (
	"time"
)

// QuarantineBounds defines the valid date range for rollup computation.
// Commits with committed_at outside this range are excluded from the rollup
// but preserved in the raw Parquet artifact.
type QuarantineBounds struct {
	// MinDate is the lower bound (inclusive): 2005-01-01 UTC
	MinDate time.Time
	// MaxDate is the upper bound (inclusive): today+1 UTC
	MaxDate time.Time
}

// NewQuarantineBounds creates bounds for the current date.
// MinDate is fixed at 2005-01-01 UTC.
// MaxDate is the current UTC date + 1 day.
func NewQuarantineBounds(today time.Time) QuarantineBounds {
	// Normalize today to UTC midnight
	todayUTC := today.UTC()
	todayUTC = time.Date(todayUTC.Year(), todayUTC.Month(), todayUTC.Day(), 0, 0, 0, 0, time.UTC)

	// MaxDate is the end of today + 1 day (inclusive upper bound)
	// We use 23:59:59.999999999 to include the entire day
	maxDate := todayUTC.AddDate(0, 0, 1)
	maxDate = time.Date(maxDate.Year(), maxDate.Month(), maxDate.Day(), 23, 59, 59, 999999999, time.UTC)

	// MinDate is fixed at 2005-01-01 00:00:00 UTC (inclusive lower bound)
	minDate := time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC)

	return QuarantineBounds{
		MinDate: minDate,
		MaxDate: maxDate,
	}
}

// IsIncluded returns true if the given committed_at falls within the
// quarantine bounds [MinDate, MaxDate] (inclusive on both ends).
// MinDate includes the entire day starting at 2005-01-01 00:00:00 UTC.
// MaxDate includes the entire day ending at today+1 23:59:59.999999999 UTC.
func (qb QuarantineBounds) IsIncluded(committedAt time.Time) bool {
	// Normalize committedAt to UTC for comparison
	committedUTC := committedAt.UTC()

	// Check inclusive bounds: MinDate <= committedAt <= MaxDate
	return !committedUTC.Before(qb.MinDate) && !committedUTC.After(qb.MaxDate)
}

// Commit represents a single commit for rollup computation.
type Commit struct {
	SHA         string    // Commit SHA
	AuthorEmail string    // Author email (for identity resolution)
	AuthorName  string    // Author name
	CommittedAt time.Time // Commit date (UTC)
	Message     string    // Commit message
	Tools       []string  // Detected AI tools (empty if no AI tool detected)
}

// RollupRow represents a single rollup aggregation row.
type RollupRow struct {
	UserEmail string    // Author email (resolved to user_id later)
	RepoID    int64     // Repository ID
	Tool      string    // AI tool name
	Day       time.Time // Day (UTC, midnight)
	Count     int       // Number of commits
	// InsertTime is set by database DEFAULT transaction_timestamp()
	// and is NOT managed by application code.
}

// ComputeRollup computes (user, repo, tool, day, count) aggregations
// from the given commits, applying date quarantine filtering.
//
// Commits with committed_at outside the quarantine bounds are excluded
// from the rollup entirely. The caller is responsible for preserving
// the raw commit data (including unclamped committed_at) in the Parquet artifact.
//
// Parameters:
//   - commits: All commits for a repo (may include out-of-range dates)
//   - repoID: Repository ID for rollup rows
//   - bounds: Quarantine date bounds
//
// Returns:
//   - Rollup rows for commits within the date bounds
func ComputeRollup(commits []Commit, repoID int64, bounds QuarantineBounds) []RollupRow {
	// Group commits by (user_email, tool, day)
	rollupMap := make(map[string]RollupRow)

	for _, commit := range commits {
		// Skip commits with no AI tools detected
		if len(commit.Tools) == 0 {
			continue
		}

		// Apply date quarantine filter
		if !bounds.IsIncluded(commit.CommittedAt) {
			// Exclude from rollup but still write to Parquet (caller's responsibility)
			continue
		}

		// Normalize committed_at to day (UTC midnight)
		day := commit.CommittedAt.UTC()
		day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)

		// Create rollup entry for each detected tool
		for _, tool := range commit.Tools {
			key := commit.AuthorEmail + "|" + tool + "|" + day.Format(time.RFC3339)

			existing, found := rollupMap[key]
			if found {
				// Increment count
				existing.Count++
				rollupMap[key] = existing
			} else {
				// Create new rollup row
				// Note: InsertTime is set by database DEFAULT transaction_timestamp()
				rollupMap[key] = RollupRow{
					UserEmail: commit.AuthorEmail,
					RepoID:    repoID,
					Tool:      tool,
					Day:       day,
					Count:     1,
				}
			}
		}
	}

	// Convert map to slice
	result := make([]RollupRow, 0, len(rollupMap))
	for _, row := range rollupMap {
		result = append(result, row)
	}

	return result
}
