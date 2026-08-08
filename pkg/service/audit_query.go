// Package service provides business logic services for audit log queries.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jedarden/commitgraph/pkg/errors"
)

// AuditLogQueryOptions contains optional parameters for querying audit logs.
type AuditLogQueryOptions struct {
	// Pagination
	Limit  int // Maximum number of records to return (default: 100, max: 1000)
	Offset int // Number of records to skip (for offset-based pagination)

	// Filters
	StartTime  *time.Time // Filter by timestamp >= start_time (optional)
	EndTime    *time.Time // Filter by timestamp <= end_time (optional)
	Actor      string     // Filter by actor (optional, exact match)
	EventType  string     // Filter by event_type (optional, exact match: 'exclude' or 'unexclude')
}

// AuditLogRecord represents a structured audit log record returned by QueryAuditLogs.
type AuditLogRecord struct {
	ID               int64      // Unique identifier for the audit entry
	RepoID           int64      // Foreign key reference to the repos table
	Actor            string     // Who performed the action
	Timestamp        time.Time  // When the action was performed
	EventType        string     // Type of event: 'exclude' or 'unexclude'
	OldExcludedAt    *time.Time // Previous excluded_at value before this action
	OldExcludedReason *string    // Previous excluded_reason value before this action
	NewExcludedAt    *time.Time // New excluded_at value after this action
	NewExcludedReason *string    // New excluded_reason value after this action
}

// AuditLogQueryResult contains the results of QueryAuditLogs with pagination metadata.
type AuditLogQueryResult struct {
	Records    []AuditLogRecord // The audit log records
	TotalCount int64            // Total count of matching records (for pagination)
	Limit      int              // The limit used in the query
	Offset     int              // The offset used in the query
}

// AuditLogQuerier provides query functions for audit logs.
type AuditLogQuerier struct {
	db *sql.DB
}

// NewAuditLogQuerier creates a new audit log querier.
func NewAuditLogQuerier(db *sql.DB) *AuditLogQuerier {
	return &AuditLogQuerier{db: db}
}

// QueryAuditLogs retrieves audit logs for a specific repository with pagination and filtering.
//
// Parameters:
//   - ctx: Context for the operation
//   - repoID: Repository ID to query audit logs for
//   - opts: Optional query parameters (pagination, filters)
//
// Returns:
//   - AuditLogQueryResult: Structured result with records and pagination metadata
//   - error: Error if the query fails
//
// The function handles empty results gracefully (returns empty slice with zero count).
// Pagination uses offset-based pagination with configurable limit and offset.
func (q *AuditLogQuerier) QueryAuditLogs(ctx context.Context, repoID int64, opts AuditLogQueryOptions) (*AuditLogQueryResult, error) {
	// Set default limit
	limit := opts.Limit
	if limit <= 0 {
		limit = 100 // default limit
	}
	if limit > 1000 {
		limit = 1000 // max limit to prevent excessive memory use
	}

	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	// Build the WHERE clause dynamically based on filters
	whereClause := "WHERE repo_id = $1"
	argCount := 1
	args := []interface{}{repoID}

	if opts.StartTime != nil {
		argCount++
		whereClause += fmt.Sprintf(" AND timestamp >= $%d", argCount)
		args = append(args, opts.StartTime)
	}

	if opts.EndTime != nil {
		argCount++
		whereClause += fmt.Sprintf(" AND timestamp <= $%d", argCount)
		args = append(args, opts.EndTime)
	}

	if opts.Actor != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND actor = $%d", argCount)
		args = append(args, opts.Actor)
	}

	if opts.EventType != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND event_type = $%d", argCount)
		args = append(args, opts.EventType)
	}

	// Query for total count (with same filters)
	countQuery := `
		SELECT COUNT(*)
		FROM exclusion_audit_log
		` + whereClause

	var totalCount int64
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := q.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		// Log the error at service layer boundary
		log.Printf("[ERROR] service/audit_query.QueryAuditLogs: failed to query audit log count for repo_id=%d: %v", repoID, err)

		// Handle nil case explicitly
		if err == nil {
			err = errors.DatabaseQueryError("service/audit_query", "QueryAuditLogs", countQuery, "unexpected nil error")
			return nil, err
		}

		// Wrap with structured error type preserving original context
		wrappedErr := errors.WrapError(err, *errors.DatabaseQueryError("service/audit_query", "QueryAuditLogs", countQuery, "failed to query audit log count"))
		wrappedErr = wrappedErr.WithRecordKey(fmt.Sprintf("repo_id:%d", repoID))
		return nil, wrappedErr
	}

	// Query for records (with pagination)
	query := `
		SELECT id, repo_id, actor, timestamp, event_type,
		       old_excluded_at, old_excluded_reason, new_excluded_at, new_excluded_reason
		FROM exclusion_audit_log
		` + whereClause + `
		ORDER BY timestamp DESC
		LIMIT $` + fmt.Sprintf("%d", argCount+1) + ` OFFSET $` + fmt.Sprintf("%d", argCount+2)

	args = append(args, limit, offset)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Log the error at service layer boundary
		log.Printf("[ERROR] service/audit_query.QueryAuditLogs: failed to query audit logs for repo_id=%d: %v", repoID, err)

		// Handle nil case explicitly
		if err == nil {
			err = errors.DatabaseQueryError("service/audit_query", "QueryAuditLogs", query, "unexpected nil error")
			return nil, err
		}

		// Wrap with structured error type preserving original context
		wrappedErr := errors.WrapError(err, *errors.DatabaseQueryError("service/audit_query", "QueryAuditLogs", query, "failed to query audit logs"))
		wrappedErr = wrappedErr.WithRecordKey(fmt.Sprintf("repo_id:%d", repoID))
		return nil, wrappedErr
	}
	defer rows.Close()

	var records []AuditLogRecord
	for rows.Next() {
		var rec AuditLogRecord
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
			// Log the error at service layer boundary
			log.Printf("[ERROR] service/audit_query.QueryAuditLogs: failed to scan audit log record for repo_id=%d: %v", repoID, err)

			// Handle nil case explicitly
			if err == nil {
				err = errors.DatabaseQueryError("service/audit_query", "QueryAuditLogs", query, "unexpected nil error during scan")
				return nil, err
			}

			// Wrap with structured error type preserving original context
			wrappedErr := errors.WrapError(err, *errors.DatabaseQueryError("service/audit_query", "QueryAuditLogs", query, "failed to scan audit log record"))
			wrappedErr = wrappedErr.WithRecordKey(fmt.Sprintf("repo_id:%d", repoID))
			return nil, wrappedErr
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		// Log the error at service layer boundary
		log.Printf("[ERROR] service/audit_query.QueryAuditLogs: error iterating audit log records for repo_id=%d: %v", repoID, err)

		// Handle nil case explicitly
		if err == nil {
			err = errors.DatabaseQueryError("service/audit_query", "QueryAuditLogs", query, "unexpected nil error during iteration")
			return nil, err
		}

		// Wrap with structured error type preserving original context
		wrappedErr := errors.WrapError(err, *errors.DatabaseQueryError("service/audit_query", "QueryAuditLogs", query, "error iterating audit log records"))
		wrappedErr = wrappedErr.WithRecordKey(fmt.Sprintf("repo_id:%d", repoID))
		return nil, wrappedErr
	}

	// Handle empty results gracefully
	if records == nil {
		records = []AuditLogRecord{}
	}

	return &AuditLogQueryResult{
		Records:    records,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
	}, nil
}

// QueryAllAuditLogs retrieves audit logs across all repositories with pagination and filtering.
//
// This is similar to QueryAuditLogs but doesn't filter by repo_id.
// Use with caution - may return many records.
//
// Parameters:
//   - ctx: Context for the operation
//   - opts: Optional query parameters (pagination, filters)
//
// Returns:
//   - AuditLogQueryResult: Structured result with records and pagination metadata
//   - error: Error if the query fails
func (q *AuditLogQuerier) QueryAllAuditLogs(ctx context.Context, opts AuditLogQueryOptions) (*AuditLogQueryResult, error) {
	// Set default limit
	limit := opts.Limit
	if limit <= 0 {
		limit = 100 // default limit
	}
	if limit > 1000 {
		limit = 1000 // max limit to prevent excessive memory use
	}

	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	// Build the WHERE clause dynamically based on filters
	whereClause := "WHERE 1=1"
	argCount := 0
	args := []interface{}{}

	if opts.StartTime != nil {
		argCount++
		whereClause += fmt.Sprintf(" AND timestamp >= $%d", argCount)
		args = append(args, opts.StartTime)
	}

	if opts.EndTime != nil {
		argCount++
		whereClause += fmt.Sprintf(" AND timestamp <= $%d", argCount)
		args = append(args, opts.EndTime)
	}

	if opts.Actor != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND actor = $%d", argCount)
		args = append(args, opts.Actor)
	}

	if opts.EventType != "" {
		argCount++
		whereClause += fmt.Sprintf(" AND event_type = $%d", argCount)
		args = append(args, opts.EventType)
	}

	// Query for total count (with same filters)
	countQuery := `
		SELECT COUNT(*)
		FROM exclusion_audit_log
		` + whereClause

	var totalCount int64
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := q.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		// Log the error at service layer boundary
		log.Printf("[ERROR] service/audit_query.QueryAllAuditLogs: failed to query audit log count: %v", err)

		// Handle nil case explicitly
		if err == nil {
			err = errors.DatabaseQueryError("service/audit_query", "QueryAllAuditLogs", countQuery, "unexpected nil error")
			return nil, err
		}

		// Wrap with structured error type preserving original context
		wrappedErr := errors.WrapError(err, *errors.DatabaseQueryError("service/audit_query", "QueryAllAuditLogs", countQuery, "failed to query audit log count"))
		return nil, wrappedErr
	}

	// Query for records (with pagination)
	query := `
		SELECT id, repo_id, actor, timestamp, event_type,
		       old_excluded_at, old_excluded_reason, new_excluded_at, new_excluded_reason
		FROM exclusion_audit_log
		` + whereClause + `
		ORDER BY timestamp DESC
		LIMIT $` + fmt.Sprintf("%d", argCount+1) + ` OFFSET $` + fmt.Sprintf("%d", argCount+2)

	args = append(args, limit, offset)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		// Log the error at service layer boundary
		log.Printf("[ERROR] service/audit_query.QueryAllAuditLogs: failed to query audit logs: %v", err)

		// Handle nil case explicitly
		if err == nil {
			err = errors.DatabaseQueryError("service/audit_query", "QueryAllAuditLogs", query, "unexpected nil error")
			return nil, err
		}

		// Wrap with structured error type preserving original context
		wrappedErr := errors.WrapError(err, *errors.DatabaseQueryError("service/audit_query", "QueryAllAuditLogs", query, "failed to query audit logs"))
		return nil, wrappedErr
	}
	defer rows.Close()

	var records []AuditLogRecord
	for rows.Next() {
		var rec AuditLogRecord
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
			// Log the error at service layer boundary
			log.Printf("[ERROR] service/audit_query.QueryAllAuditLogs: failed to scan audit log record: %v", err)

			// Handle nil case explicitly
			if err == nil {
				err = errors.DatabaseQueryError("service/audit_query", "QueryAllAuditLogs", query, "unexpected nil error during scan")
				return nil, err
			}

			// Wrap with structured error type preserving original context
			wrappedErr := errors.WrapError(err, *errors.DatabaseQueryError("service/audit_query", "QueryAllAuditLogs", query, "failed to scan audit log record"))
			return nil, wrappedErr
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		// Log the error at service layer boundary
		log.Printf("[ERROR] service/audit_query.QueryAllAuditLogs: error iterating audit log records: %v", err)

		// Handle nil case explicitly
		if err == nil {
			err = errors.DatabaseQueryError("service/audit_query", "QueryAllAuditLogs", query, "unexpected nil error during iteration")
			return nil, err
		}

		// Wrap with structured error type preserving original context
		wrappedErr := errors.WrapError(err, *errors.DatabaseQueryError("service/audit_query", "QueryAllAuditLogs", query, "error iterating audit log records"))
		return nil, wrappedErr
	}

	// Handle empty results gracefully
	if records == nil {
		records = []AuditLogRecord{}
	}

	return &AuditLogQueryResult{
		Records:    records,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
	}, nil
}
