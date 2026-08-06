// Package audit provides query functions for the exclusion audit log.
package audit

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ExclusionAuditRecord represents a row from the exclusion_audit_log table.
type ExclusionAuditRecord struct {
	ID                 int64
	RepoID             int64
	Actor              string
	Timestamp          time.Time
	EventType          string // 'exclude' or 'unexclude'
	OldExcludedAt      *time.Time
	OldExcludedReason  *string
	NewExcludedAt      *time.Time
	NewExcludedReason  *string
}

// ExclusionAuditQueryOptions provides filtering and pagination for audit queries.
type ExclusionAuditQueryOptions struct {
	// RepoID filters to a specific repository (0 = all repos)
	RepoID int64

	// Actor filters to a specific actor (empty = all actors)
	Actor string

	// EventType filters to a specific event type ('exclude' or 'unexclude', empty = all)
	EventType string

	// DateRange filters by timestamp (zero values = no filter)
	StartDate time.Time
	EndDate   time.Time

	// Pagination controls
	Offset int // Offset for pagination (0 = first page)
	Limit  int // Limit results (0 = use default of 100)
}

// ExclusionAuditQuerier provides query functions for the exclusion_audit_log table.
type ExclusionAuditQuerier struct {
	db *sql.DB
}

// NewExclusionAuditQuerier creates a new exclusion audit log querier.
func NewExclusionAuditQuerier(db *sql.DB) *ExclusionAuditQuerier {
	return &ExclusionAuditQuerier{db: db}
}

// QueryExclusionAuditLogs retrieves audit log records with filtering and pagination.
//
// Returns audit records matching the provided options, ordered by timestamp descending
// (most recent first). Use Offset and Limit in options for pagination.
func (q *ExclusionAuditQuerier) QueryExclusionAuditLogs(ctx context.Context, opts ExclusionAuditQueryOptions) ([]ExclusionAuditRecord, error) {
	// Build the base query
	baseQuery := `
		SELECT id, repo_id, actor, timestamp, event_type,
		       old_excluded_at, old_excluded_reason,
		       new_excluded_at, new_excluded_reason
		FROM exclusion_audit_log
		WHERE 1=1
	`

	// Build WHERE clauses based on options
	args := []interface{}{}
	argPos := 1

	if opts.RepoID != 0 {
		baseQuery += fmt.Sprintf(" AND repo_id = $%d", argPos)
		args = append(args, opts.RepoID)
		argPos++
	}

	if opts.Actor != "" {
		baseQuery += fmt.Sprintf(" AND actor = $%d", argPos)
		args = append(args, opts.Actor)
		argPos++
	}

	if opts.EventType != "" {
		baseQuery += fmt.Sprintf(" AND event_type = $%d", argPos)
		args = append(args, opts.EventType)
		argPos++
	}

	if !opts.StartDate.IsZero() {
		baseQuery += fmt.Sprintf(" AND timestamp >= $%d", argPos)
		args = append(args, opts.StartDate)
		argPos++
	}

	if !opts.EndDate.IsZero() {
		baseQuery += fmt.Sprintf(" AND timestamp <= $%d", argPos)
		args = append(args, opts.EndDate)
		argPos++
	}

	// Add ordering and pagination
	baseQuery += " ORDER BY timestamp DESC"

	// Apply limit with default of 100
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	baseQuery += fmt.Sprintf(" LIMIT $%d", argPos)
	args = append(args, limit)
	argPos++

	// Apply offset if specified
	if opts.Offset > 0 {
		baseQuery += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, opts.Offset)
	}

	rows, err := q.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query exclusion audit logs: %w", err)
	}
	defer rows.Close()

	var records []ExclusionAuditRecord
	for rows.Next() {
		var rec ExclusionAuditRecord
		err := rows.Scan(
			&rec.ID,
			&rec.RepoID,
			&rec.Actor,
			&rec.Timestamp,
			&rec.EventType,
			&rec.OldExcludedAt,
			&rec.OldExcludedReason,
			&rec.NewExcludedAt,
			&rec.NewExcludedReason,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan exclusion audit record: %w", err)
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating exclusion audit records: %w", err)
	}

	return records, nil
}

// CountExclusionAuditLogs returns the total count of records matching the filter options.
// This is useful for pagination UI to show total pages.
func (q *ExclusionAuditQuerier) CountExclusionAuditLogs(ctx context.Context, opts ExclusionAuditQueryOptions) (int64, error) {
	// Build the count query (same WHERE clauses as QueryExclusionAuditLogs)
	baseQuery := "SELECT COUNT(*) FROM exclusion_audit_log WHERE 1=1"

	args := []interface{}{}
	argPos := 1

	if opts.RepoID != 0 {
		baseQuery += fmt.Sprintf(" AND repo_id = $%d", argPos)
		args = append(args, opts.RepoID)
		argPos++
	}

	if opts.Actor != "" {
		baseQuery += fmt.Sprintf(" AND actor = $%d", argPos)
		args = append(args, opts.Actor)
		argPos++
	}

	if opts.EventType != "" {
		baseQuery += fmt.Sprintf(" AND event_type = $%d", argPos)
		args = append(args, opts.EventType)
		argPos++
	}

	if !opts.StartDate.IsZero() {
		baseQuery += fmt.Sprintf(" AND timestamp >= $%d", argPos)
		args = append(args, opts.StartDate)
		argPos++
	}

	if !opts.EndDate.IsZero() {
		baseQuery += fmt.Sprintf(" AND timestamp <= $%d", argPos)
		args = append(args, opts.EndDate)
		argPos++
	}

	var count int64
	err := q.db.QueryRowContext(ctx, baseQuery, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count exclusion audit logs: %w", err)
	}

	return count, nil
}

// GetActiveExclusions retrieves all currently active exclusions from the exclusion audit log.
//
// This returns repos that have an exclude event without a subsequent unexclude event.
// Returns the most recent audit record for each actively excluded repo.
func (q *ExclusionAuditQuerier) GetActiveExclusions(ctx context.Context) ([]ExclusionAuditRecord, error) {
	query := `
		WITH ranked_events AS (
			SELECT
				id, repo_id, actor, timestamp, event_type,
				old_excluded_at, old_excluded_reason,
				new_excluded_at, new_excluded_reason,
				ROW_NUMBER() OVER (PARTITION BY repo_id ORDER BY timestamp DESC) as rn
			FROM exclusion_audit_log
		)
		SELECT
			id, repo_id, actor, timestamp, event_type,
			old_excluded_at, old_excluded_reason,
			new_excluded_at, new_excluded_reason
		FROM ranked_events
		WHERE rn = 1 AND event_type = 'exclude' AND new_excluded_at IS NOT NULL
		ORDER BY timestamp DESC
	`

	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active exclusions: %w", err)
	}
	defer rows.Close()

	var records []ExclusionAuditRecord
	for rows.Next() {
		var rec ExclusionAuditRecord
		err := rows.Scan(
			&rec.ID,
			&rec.RepoID,
			&rec.Actor,
			&rec.Timestamp,
			&rec.EventType,
			&rec.OldExcludedAt,
			&rec.OldExcludedReason,
			&rec.NewExcludedAt,
			&rec.NewExcludedReason,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan exclusion audit record: %w", err)
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating exclusion audit records: %w", err)
	}

	return records, nil
}

// GetRepoAuditHistory retrieves audit history for a specific repository.
//
// Returns audit records for the given repository ID, ordered by timestamp descending
// (most recent first). Use offset and limit for pagination.
func (q *ExclusionAuditQuerier) GetRepoAuditHistory(ctx context.Context, repoID int64, offset, limit int) ([]ExclusionAuditRecord, error) {
	opts := ExclusionAuditQueryOptions{
		RepoID: repoID,
		Offset: offset,
		Limit:  limit,
	}
	return q.QueryExclusionAuditLogs(ctx, opts)
}

// GetActorAuditHistory retrieves audit history for a specific actor.
//
// Returns audit records performed by the given actor, ordered by timestamp
// descending (most recent first). Use offset and limit for pagination.
func (q *ExclusionAuditQuerier) GetActorAuditHistory(ctx context.Context, actor string, offset, limit int) ([]ExclusionAuditRecord, error) {
	opts := ExclusionAuditQueryOptions{
		Actor:  actor,
		Offset: offset,
		Limit:  limit,
	}
	return q.QueryExclusionAuditLogs(ctx, opts)
}

// LongstandingExclusionV2 represents a repository that has been excluded for a long time.
// This version uses the exclusion_audit_log table with repo_id foreign key.
type LongstandingExclusionV2 struct {
	RepoID           int64
	Provider         string
	RepoFullName     string
	ExcludedAt       time.Time
	Reason           string
	LastAuditTime    time.Time
	Duration         time.Duration // how long it's been excluded
	Actor            string        // who applied the exclusion
}

// GetLongstandingExclusions finds repositories that have been excluded for longer
// than the specified duration without an unexclude event.
//
// This is the key function for periodic alerting on the "reactive exclusion" residual
// risk described in plan.md's threat model. It surfaces repos that may have been
// excluded and forgotten, requiring review.
//
// Joins with repos table to get provider and repo_full_name for display.
func (q *ExclusionAuditQuerier) GetLongstandingExclusions(ctx context.Context, minDuration time.Duration) ([]LongstandingExclusionV2, error) {
	query := `
		WITH latest_exclude AS (
			SELECT DISTINCT ON (repo_id)
				repo_id, new_excluded_at, new_excluded_reason, actor, timestamp
			FROM exclusion_audit_log
			WHERE event_type = 'exclude' AND new_excluded_at IS NOT NULL
			ORDER BY repo_id, timestamp DESC
		),
		has_unexcluded AS (
			SELECT DISTINCT ON (repo_id)
				repo_id
			FROM exclusion_audit_log
			WHERE event_type = 'unexclude' AND new_excluded_at IS NULL
			ORDER BY repo_id, timestamp DESC
		)
		SELECT
			le.repo_id, r.provider, r.repo_full_name,
			le.new_excluded_at, le.new_excluded_reason, le.actor, le.timestamp
		FROM latest_exclude le
		INNER JOIN repos r ON le.repo_id = r.repo_id
		LEFT JOIN has_unexcluded hu ON le.repo_id = hu.repo_id
		WHERE hu.repo_id IS NULL  -- has not been unexcluded
		  AND le.new_excluded_at < NOW() - ($1::interval)
		ORDER BY le.new_excluded_at ASC
	`

	rows, err := q.db.QueryContext(ctx, query, fmt.Sprintf("%d seconds", int(minDuration.Seconds())))
	if err != nil {
		return nil, fmt.Errorf("failed to query longstanding exclusions: %w", err)
	}
	defer rows.Close()

	var exclusions []LongstandingExclusionV2
	now := time.Now()

	for rows.Next() {
		var le LongstandingExclusionV2
		var excludedAt time.Time
		var auditTimestamp time.Time
		var reason *string

		err := rows.Scan(
			&le.RepoID,
			&le.Provider,
			&le.RepoFullName,
			&excludedAt,
			&reason,
			&le.Actor,
			&auditTimestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan longstanding exclusion: %w", err)
		}

		le.ExcludedAt = excludedAt
		le.LastAuditTime = auditTimestamp
		le.Duration = now.Sub(excludedAt)
		if reason != nil {
			le.Reason = *reason
		}

		exclusions = append(exclusions, le)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating longstanding exclusions: %w", err)
	}

	return exclusions, nil
}
