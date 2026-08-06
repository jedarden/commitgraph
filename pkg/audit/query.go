// Package audit provides query functions for the audit log.
package audit

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AuditRecord represents a row from the audit_log table.
type AuditRecord struct {
	ID             int64
	Timestamp      time.Time
	Operation      string
	Provider       string
	RepoFullName   string
	Operator       string
	Reason         string
	IncidentID     string
	ExcludedBefore *time.Time
	ReasonBefore   string
	ExcludedAfter  *time.Time
	ReasonAfter    string
	RowsAffected   int
}

// Querier provides query functions for the audit log.
type Querier struct {
	db *sql.DB
}

// NewQuerier creates a new audit log querier.
func NewQuerier(db *sql.DB) *Querier {
	return &Querier{db: db}
}

// GetAuditHistory retrieves audit history for a specific repository.
//
// Returns audit records for the given repository, ordered by timestamp descending
// (most recent first). Use limit to control the number of records returned.
func (q *Querier) GetAuditHistory(ctx context.Context, provider, repoFullName string, limit int) ([]AuditRecord, error) {
	if limit <= 0 {
		limit = 100 // default limit
	}

	query := `
		SELECT id, timestamp, operation, provider, repo_full_name, operator, reason, incident_id,
		       excluded_before, reason_before, excluded_after, reason_after, rows_affected
		FROM audit_log
		WHERE provider = $1 AND repo_full_name = $2
		ORDER BY timestamp DESC
		LIMIT $3
	`

	rows, err := q.db.QueryContext(ctx, query, provider, repoFullName, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit history: %w", err)
	}
	defer rows.Close()

	var records []AuditRecord
	for rows.Next() {
		var rec AuditRecord
		err := rows.Scan(
			&rec.ID,
			&rec.Timestamp,
			&rec.Operation,
			&rec.Provider,
			&rec.RepoFullName,
			&rec.Operator,
			&rec.Reason,
			&rec.IncidentID,
			&rec.ExcludedBefore,
			&rec.ReasonBefore,
			&rec.ExcludedAfter,
			&rec.ReasonAfter,
			&rec.RowsAffected,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit record: %w", err)
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit records: %w", err)
	}

	return records, nil
}

// GetRecentAuditLog retrieves recent audit events across all repositories.
//
// Returns audit records ordered by timestamp descending (most recent first).
// Use limit to control the number of records returned.
func (q *Querier) GetRecentAuditLog(ctx context.Context, limit int) ([]AuditRecord, error) {
	if limit <= 0 {
		limit = 100 // default limit
	}

	query := `
		SELECT id, timestamp, operation, provider, repo_full_name, operator, reason, incident_id,
		       excluded_before, reason_before, excluded_after, reason_after, rows_affected
		FROM audit_log
		ORDER BY timestamp DESC
		LIMIT $1
	`

	rows, err := q.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent audit log: %w", err)
	}
	defer rows.Close()

	var records []AuditRecord
	for rows.Next() {
		var rec AuditRecord
		err := rows.Scan(
			&rec.ID,
			&rec.Timestamp,
			&rec.Operation,
			&rec.Provider,
			&rec.RepoFullName,
			&rec.Operator,
			&rec.Reason,
			&rec.IncidentID,
			&rec.ExcludedBefore,
			&rec.ReasonBefore,
			&rec.ExcludedAfter,
			&rec.ReasonAfter,
			&rec.RowsAffected,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit record: %w", err)
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit records: %w", err)
	}

	return records, nil
}

// LongstandingExclusion represents a repository that has been excluded for a long time.
type LongstandingExclusion struct {
	Provider      string
	RepoFullName  string
	ExcludedAt    time.Time
	Reason        string
	LastAuditTime time.Time
	Duration      time.Duration // how long it's been excluded
	Operator      string        // who applied the exclusion
}

// GetLongstandingExclusions finds repositories that have been excluded for longer
// than the specified duration without a clear event.
//
// This is the key function for periodic alerting on the "reactive exclusion" residual
// risk described in plan.md's threat model. It surfaces repos that may have been
// excluded and forgotten, requiring review.
func (q *Querier) GetLongstandingExclusions(ctx context.Context, minDuration time.Duration) ([]LongstandingExclusion, error) {
	query := `
		WITH latest_exclude AS (
			SELECT DISTINCT ON (provider, repo_full_name)
				provider, repo_full_name, excluded_after, reason_after, operator, timestamp
			FROM audit_log
			WHERE operation = 'exclude' AND excluded_after IS NOT NULL
			ORDER BY provider, repo_full_name, timestamp DESC
		),
		has_cleared AS (
			SELECT DISTINCT ON (provider, repo_full_name)
				provider, repo_full_name
			FROM audit_log
			WHERE operation = 'clear' AND excluded_after IS NULL
			ORDER BY provider, repo_full_name, timestamp DESC
		)
		SELECT
			le.provider, le.repo_full_name, le.excluded_after, le.reason_after,
			le.operator, le.timestamp
		FROM latest_exclude le
		LEFT JOIN has_cleared hc ON le.provider = hc.provider AND le.repo_full_name = hc.repo_full_name
		WHERE hc.provider IS NULL  -- has not been cleared
		  AND le.excluded_after < NOW() - ($1::interval)
		ORDER BY le.excluded_after ASC
	`

	rows, err := q.db.QueryContext(ctx, query, fmt.Sprintf("%d seconds", int(minDuration.Seconds())))
	if err != nil {
		return nil, fmt.Errorf("failed to query longstanding exclusions: %w", err)
	}
	defer rows.Close()

	var exclusions []LongstandingExclusion
	now := time.Now()

	for rows.Next() {
		var le LongstandingExclusion
		var excludedAt time.Time
		var auditTimestamp time.Time

		err := rows.Scan(
			&le.Provider,
			&le.RepoFullName,
			&excludedAt,
			&le.Reason,
			&le.Operator,
			&auditTimestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan longstanding exclusion: %w", err)
		}

		le.ExcludedAt = excludedAt
		le.LastAuditTime = auditTimestamp
		le.Duration = now.Sub(excludedAt)

		exclusions = append(exclusions, le)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating longstanding exclusions: %w", err)
	}

	return exclusions, nil
}

// GetOperatorHistory retrieves audit history for a specific operator.
//
// Returns audit records performed by the given operator, ordered by timestamp
// descending (most recent first). Use limit to control the number of records returned.
func (q *Querier) GetOperatorHistory(ctx context.Context, operator string, limit int) ([]AuditRecord, error) {
	if limit <= 0 {
		limit = 100 // default limit
	}

	query := `
		SELECT id, timestamp, operation, provider, repo_full_name, operator, reason, incident_id,
		       excluded_before, reason_before, excluded_after, reason_after, rows_affected
		FROM audit_log
		WHERE operator = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`

	rows, err := q.db.QueryContext(ctx, query, operator, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query operator history: %w", err)
	}
	defer rows.Close()

	var records []AuditRecord
	for rows.Next() {
		var rec AuditRecord
		err := rows.Scan(
			&rec.ID,
			&rec.Timestamp,
			&rec.Operation,
			&rec.Provider,
			&rec.RepoFullName,
			&rec.Operator,
			&rec.Reason,
			&rec.IncidentID,
			&rec.ExcludedBefore,
			&rec.ReasonBefore,
			&rec.ExcludedAfter,
			&rec.ReasonAfter,
			&rec.RowsAffected,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit record: %w", err)
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit records: %w", err)
	}

	return records, nil
}

// GetActiveExclusions retrieves all currently active exclusions from the audit log.
//
// This returns repos that have an exclude event without a subsequent clear event.
func (q *Querier) GetActiveExclusions(ctx context.Context) ([]AuditRecord, error) {
	query := `
		WITH ranked_events AS (
			SELECT
				id, timestamp, operation, provider, repo_full_name, operator, reason, incident_id,
				excluded_before, reason_before, excluded_after, reason_after, rows_affected,
				ROW_NUMBER() OVER (PARTITION BY provider, repo_full_name ORDER BY timestamp DESC) as rn
			FROM audit_log
		)
		SELECT
			id, timestamp, operation, provider, repo_full_name, operator, reason, incident_id,
			excluded_before, reason_before, excluded_after, reason_after, rows_affected
		FROM ranked_events
		WHERE rn = 1 AND operation = 'exclude'
		ORDER BY timestamp DESC
	`

	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active exclusions: %w", err)
	}
	defer rows.Close()

	var records []AuditRecord
	for rows.Next() {
		var rec AuditRecord
		err := rows.Scan(
			&rec.ID,
			&rec.Timestamp,
			&rec.Operation,
			&rec.Provider,
			&rec.RepoFullName,
			&rec.Operator,
			&rec.Reason,
			&rec.IncidentID,
			&rec.ExcludedBefore,
			&rec.ReasonBefore,
			&rec.ExcludedAfter,
			&rec.ReasonAfter,
			&rec.RowsAffected,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit record: %w", err)
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit records: %w", err)
	}

	return records, nil
}
