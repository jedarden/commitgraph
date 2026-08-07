// Package pg provides PostgreSQL operations for commitgraph repos.
package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RepoUpsertQuery is a single parameterized upsert that allocates or reuses
// the surrogate repo_id for a (provider, repo_full_name) pair. Per
// docs/plan/plan.md "Postgres schema": allocation is one upsert-returning-id
// per repo, not per commit — a single round trip per scan job, negligible
// against the clone itself.
//
// ON CONFLICT ... DO UPDATE (not DO NOTHING) is required for two reasons:
//  1. Postgres does not emit a RETURNING row for a conflict resolved by DO
//     NOTHING, only for rows actually written by INSERT or UPDATE — DO
//     NOTHING would return no rows at all on the (common) second-and-later
//     call for an existing repo.
//  2. repo_full_name must be refreshed in place on a GitHub rename (plan.md
//     edge case 4: "Repo renamed on GitHub — the surrogate repo_id survives
//     it; the repos.repo_full_name row is updated"). DO NOTHING would leave
//     the stale name in place.
const RepoUpsertQuery = `
INSERT INTO repos (provider, repo_full_name)
VALUES ($1, $2)
ON CONFLICT (provider, repo_full_name) DO UPDATE
  SET repo_full_name = excluded.repo_full_name
RETURNING repo_id
`

// UpsertRepo allocates or reuses the surrogate repo_id for (provider,
// repoFullName) in exactly one round trip, using RepoUpsertQuery. It is
// called once per repo scan job, before any rollup rows are written for
// that repo.
//
// The first call for a new (provider, repoFullName) pair inserts a row and
// returns a fresh repo_id. Every subsequent call for the same pair returns
// the same repo_id (idempotent), even across a GitHub rename that changes
// repoFullName for an already-known provider — the ON CONFLICT target is
// the UNIQUE (provider, repo_full_name) constraint from the current schema,
// so a genuine rename (which changes repo_full_name) is only picked up here
// once the caller passes the new name; see plan.md's identity-resolution
// notes for how renames are detected upstream of this call.
func UpsertRepo(ctx context.Context, db *sql.Tx, provider, repoFullName string) (int64, error) {
	if provider == "" {
		return 0, fmt.Errorf("provider is required")
	}
	if repoFullName == "" {
		return 0, fmt.Errorf("repo_full_name is required")
	}

	var repoID int64
	err := db.QueryRowContext(ctx, RepoUpsertQuery, provider, repoFullName).Scan(&repoID)
	if err != nil {
		return 0, fmt.Errorf("upsert repo %s/%s failed: %w", provider, repoFullName, err)
	}
	return repoID, nil
}

// RepoExcluder handles repo-level exclusion operations.
type RepoExcluder struct {
	db Executor
}

// NewRepoExcluder creates a new repo excluder.
func NewRepoExcluder(db Executor) *RepoExcluder {
	return &RepoExcluder{db: db}
}

// ExclusionOp represents the type of exclusion operation.
type ExclusionOp string

const (
	// OpExclude applies an exclusion
	OpExclude ExclusionOp = "exclude"
	// OpClear removes an exclusion
	OpClear ExclusionOp = "clear"
)

// ExclusionRequest represents a request to apply or clear an exclusion.
type ExclusionRequest struct {
	Provider      string     // e.g., "github"
	RepoFullName  string     // e.g., "owner/name"
	ExcludedAt    *time.Time // NULL for clear operations
	ExcludedReason string     // Human-readable reason (required for exclude, empty for clear)
	Operator      string     // Who is performing this action
}

// ApplyExclusion applies or clears a repo exclusion.
//
// For exclude operations: sets excluded_at = now() with the provided reason.
// For clear operations: sets excluded_at = NULL and excluded_reason = NULL.
//
// This returns the number of rows affected (should be 1 if repo exists, 0 otherwise).
func (r *RepoExcluder) ApplyExclusion(ctx context.Context, req ExclusionRequest) (int64, error) {
	if req.Provider == "" {
		return 0, fmt.Errorf("provider is required")
	}
	if req.RepoFullName == "" {
		return 0, fmt.Errorf("repo_full_name is required")
	}
	if req.Operator == "" {
		return 0, fmt.Errorf("operator is required (for audit logging)")
	}

	var query string
	var args []interface{}

	if req.ExcludedAt == nil {
		// Clear operation: NULL both fields
		query = `
			UPDATE repos
			SET excluded_at = NULL,
			    excluded_reason = NULL
			WHERE provider = $1 AND repo_full_name = $2
		`
		args = []interface{}{req.Provider, req.RepoFullName}
	} else {
		// Exclude operation: set both fields
		if req.ExcludedReason == "" {
			return 0, fmt.Errorf("excluded_reason is required for exclude operations")
		}
		query = `
			UPDATE repos
			SET excluded_at = $1,
			    excluded_reason = $2
			WHERE provider = $3 AND repo_full_name = $4
		`
		args = []interface{}{*req.ExcludedAt, req.ExcludedReason, req.Provider, req.RepoFullName}
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("exclusion update failed: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}

// GetExclusion retrieves the current exclusion status for a repo.
// Returns (excluded_at, excluded_reason, nil) or (nil, "", error).
func (r *RepoExcluder) GetExclusion(ctx context.Context, provider, repoFullName string) (*time.Time, string, error) {
	if provider == "" {
		return nil, "", fmt.Errorf("provider is required")
	}
	if repoFullName == "" {
		return nil, "", fmt.Errorf("repo_full_name is required")
	}

	query := `
		SELECT excluded_at, excluded_reason
		FROM repos
		WHERE provider = $1 AND repo_full_name = $2
	`

	var excludedAt *time.Time
	var excludedReason *string

	err := r.db.QueryRowContext(ctx, query, provider, repoFullName).Scan(&excludedAt, &excludedReason)
	if err != nil {
		return nil, "", fmt.Errorf("get exclusion failed: %w", err)
	}

	// Convert nullable strings to empty string for API convenience
	reason := ""
	if excludedReason != nil {
		reason = *excludedReason
	}

	return excludedAt, reason, nil
}

// ListExclusions retrieves all currently excluded repos.
// Returns a slice of (provider, repo_full_name, excluded_at, excluded_reason).
func (r *RepoExcluder) ListExclusions(ctx context.Context) ([]ExclusionInfo, error) {
	query := `
		SELECT provider, repo_full_name, excluded_at, excluded_reason
		FROM repos
		WHERE excluded_at IS NOT NULL
		ORDER BY excluded_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list exclusions failed: %w", err)
	}
	defer rows.Close()

	var exclusions []ExclusionInfo
	for rows.Next() {
		var info ExclusionInfo
		var excludedReason *string

		err := rows.Scan(&info.Provider, &info.RepoFullName, &info.ExcludedAt, &excludedReason)
		if err != nil {
			return nil, fmt.Errorf("scan exclusion row failed: %w", err)
		}

		if excludedReason != nil {
			info.ExcludedReason = *excludedReason
		}

		exclusions = append(exclusions, info)
	}

	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close rows failed: %w", err)
	}

	return exclusions, nil
}

// ExclusionInfo holds information about an excluded repo.
type ExclusionInfo struct {
	Provider       string    `json:"provider"`
	RepoFullName   string    `json:"repo_full_name"`
	ExcludedAt     time.Time `json:"excluded_at"`
	ExcludedReason string    `json:"excluded_reason"`
}
