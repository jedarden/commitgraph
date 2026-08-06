// Package pg provides PostgreSQL operations for commitgraph user_aliases.
package pg

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AliasIngester handles user_alias upsert operations.
type AliasIngester struct {
	db DBExecutor
}

// DBExecutor is the database operations interface.
// This matches database/sql's DB and Conn interfaces.
type DBExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

// NewAliasIngester creates a new alias ingester.
// Accepts *sql.DB or *sql.Conn (both implement the DBExecutor interface).
func NewAliasIngester(db DBExecutor) *AliasIngester {
	return &AliasIngester{db: db}
}

// AliasRow represents a single user_aliases row.
type AliasRow struct {
	SourceLogin string    // The login to alias from (PRIMARY KEY)
	TargetLogin string    // The canonical login to alias to
	Reason      string    // 'admin' or 'name-match'
	CreatedAt   time.Time // When this alias was created
}

// UpsertAliases performs a bulk upsert of alias rows.
//
// Uses ON CONFLICT (source_login) DO UPDATE to handle re-runs safely:
// - Updates target_login, reason, and created_at if source_login exists
// - Inserts new row if source_login doesn't exist
//
// This is idempotent - re-running after ConfigMap changes updates existing
// rows rather than erroring or duplicating.
func (a *AliasIngester) UpsertAliases(ctx context.Context, rows []AliasRow) error {
	if len(rows) == 0 {
		return nil
	}

	// Build bulk INSERT with UNNEST for array parameters
	query := `
		INSERT INTO user_aliases (source_login, target_login, reason, created_at)
		SELECT unnest($1::text[]),
		       unnest($2::text[]),
		       unnest($3::text[]),
		       unnest($4::timestamptz[])
		ON CONFLICT (source_login) DO UPDATE
		  SET target_login = excluded.target_login,
		      reason = excluded.reason,
		      created_at = excluded.created_at
	`

	// Build arrays from rows
	sourceLogins := make([]string, len(rows))
	targetLogins := make([]string, len(rows))
	reasons := make([]string, len(rows))
	createdATs := make([]time.Time, len(rows))

	for idx, row := range rows {
		sourceLogins[idx] = row.SourceLogin
		targetLogins[idx] = row.TargetLogin
		reasons[idx] = row.Reason
		createdATs[idx] = row.CreatedAt
	}

	// Execute bulk upsert
	result, err := a.db.ExecContext(ctx, query, sourceLogins, targetLogins, reasons, createdATs)
	if err != nil {
		return fmt.Errorf("bulk upsert failed: %w", err)
	}

	// Log stats
	rowsAffected, _ := result.RowsAffected()
	_ = rowsAffected // silently ignore if we can't get the count

	return nil
}

// GetAdminAliases retrieves all current admin aliases from the database.
// Returns a map of source_login -> target_login for reason='admin' only.
func (a *AliasIngester) GetAdminAliases(ctx context.Context) (map[string]string, error) {
	query := `
		SELECT source_login, target_login
		FROM user_aliases
		WHERE reason = 'admin'
	`

	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query admin aliases failed: %w", err)
	}
	defer rows.Close()

	aliases := make(map[string]string)
	for rows.Next() {
		var sourceLogin, targetLogin string
		if err := rows.Scan(&sourceLogin, &targetLogin); err != nil {
			return nil, fmt.Errorf("scan row failed: %w", err)
		}
		aliases[sourceLogin] = targetLogin
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	return aliases, nil
}

// DeleteAdminAliases removes specific admin aliases by source_login.
// Returns the number of rows deleted.
func (a *AliasIngester) DeleteAdminAliases(ctx context.Context, sourceLogins []string) (int64, error) {
	if len(sourceLogins) == 0 {
		return 0, nil
	}

	query := `
		DELETE FROM user_aliases
		WHERE source_login = ANY($1::text[])
		AND reason = 'admin'
	`

	result, err := a.db.ExecContext(ctx, query, sourceLogins)
	if err != nil {
		return 0, fmt.Errorf("delete failed: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}
