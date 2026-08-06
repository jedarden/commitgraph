// Package pg provides PostgreSQL implementations for commitgraph.
package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jedarden/commitgraph/pkg/identity"
)

// IdentityIngester implements identity.DB for PostgreSQL.
type IdentityIngester struct {
	db Executor
}

// Executor is the database operations interface.
// This is a subset of database/sql's DB and Conn interfaces,
// allowing for both transactional and non-transactional use.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (Result, error)
}

// Result is the interface returned by ExecContext (subset of sql.Result).
type Result interface {
	RowsAffected() (int64, error)
}

// NewIdentityIngester creates a new PostgreSQL identity ingester.
func NewIdentityIngester(db Executor) *IdentityIngester {
	return &IdentityIngester{db: db}
}

// IngestEmailResolution performs a bulk upsert of email resolution rows
// using the ON CONFLICT rule from plan.md:
//
//	ON CONFLICT (email) DO UPDATE
//	  SET login = excluded.login, source = excluded.source,
//	      resolved_at = excluded.resolved_at
//	  WHERE excluded.source = 'manual'
//	     OR (email_resolution.source <> 'manual'
//	         AND excluded.resolved_at > email_resolution.resolved_at)
//
// This implements the conflict resolution rule:
// - Manual source always wins (overwrites any existing row)
// - Non-manual sources win only if existing row is also non-manual
//   AND the new resolved_at is newer
// - Otherwise the existing row is preserved
//
// The implementation uses a single bulk INSERT with ON CONFLICT DO UPDATE
// and a WHERE clause on the UPDATE to implement the selective conflict rule.
// This is efficient for large batches (349K+ rows from claude-leaderboard seed).
func (i *IdentityIngester) IngestEmailResolution(ctx context.Context, rows []identity.ResolutionRow) error {
	if len(rows) == 0 {
		return nil
	}

	// Build bulk INSERT with UNNEST for array parameters
	// This is the most efficient approach for Postgres: single round-trip,
	// no per-row overhead, supports thousands of rows in one statement.
	query := `
		INSERT INTO email_resolution (email, login, source, resolved_at)
		SELECT unnest($1::text[]),
		       unnest($2::text[]),
		       unnest($3::text[]),
		       unnest($4::timestamptz[])
		ON CONFLICT (email) DO UPDATE
		  SET login = excluded.login,
		      source = excluded.source,
		      resolved_at = excluded.resolved_at
		  WHERE excluded.source = 'manual'
		     OR (email_resolution.source <> 'manual'
		         AND excluded.resolved_at > email_resolution.resolved_at)
	`

	// Build arrays from rows
	emails := make([]string, len(rows))
	logins := make([]string, len(rows))
	sources := make([]string, len(rows))
	resolvedAts := make([]time.Time, len(rows))

	for idx, row := range rows {
		emails[idx] = row.Email
		logins[idx] = row.Login
		sources[idx] = string(row.Source)
		resolvedAts[idx] = row.ResolvedAt
	}

	// Execute bulk upsert
	result, err := i.db.ExecContext(ctx, query, emails, logins, sources, resolvedAts)
	if err != nil {
		return fmt.Errorf("bulk upsert failed: %w", err)
	}

	// Log stats (optional, for observability)
	rowsAffected, _ := result.RowsAffected()
	_ = rowsAffected // silently ignore if we can't get the count

	return nil
}
