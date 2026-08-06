// Package identity provides bulk identity resolution ingest functionality.
//
// The ingest path is the single way all writers (live enrichment worker,
// claude-leaderboard seed, manual curation) write email→login resolutions
// to the email_resolution table, ensuring consistent conflict resolution.
package identity

import (
	"context"
	"fmt"
	"time"
)

// ResolutionRow represents a single email→login resolution row.
type ResolutionRow struct {
	Email      string    // Email address (primary key)
	Login      string    // Resolved GitHub login
	Source     Source    // Source of this resolution: live, seed, or manual
	ResolvedAt time.Time // When this resolution was made
}

// Source represents the provenance of an identity resolution.
type Source string

const (
	SourceLive   Source = "live"   // Resolved by live enrichment worker
	SourceSeed   Source = "seed"   // From claude-leaderboard frozen cache
	SourceManual Source = "manual" // Hand-curated by operator
)

// Validate checks if the row has valid fields.
func (r *ResolutionRow) Validate() error {
	if r.Email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	if r.Login == "" {
		return fmt.Errorf("login cannot be empty")
	}
	switch r.Source {
	case SourceLive, SourceSeed, SourceManual:
		// Valid sources
	default:
		return fmt.Errorf("invalid source: %q (must be live, seed, or manual)", r.Source)
	}
	if r.ResolvedAt.IsZero() {
		return fmt.Errorf("resolved_at cannot be zero")
	}
	return nil
}

// Ingester handles bulk upsert of email resolution rows.
type Ingester struct {
	db        DB
	Processed int64 // Total number of records processed (seen)
}

// DB is the database interface required by Ingester.
// This allows for testing with mocks and supports different database drivers.
type DB interface {
	// IngestEmailResolution performs a bulk upsert of resolution rows.
	// The batch must apply the ON CONFLICT rule:
	//   - Manual source always wins
	//   - Otherwise, the newer resolved_at wins
	// Rows that lose the conflict check are silently skipped.
	IngestEmailResolution(ctx context.Context, rows []ResolutionRow) error
}

// NewIngester creates a new Ingester.
func NewIngester(db DB) *Ingester {
	return &Ingester{
		db:        db,
		Processed: 0, // Initialize counter to zero
	}
}

// IngestResolution performs a bulk upsert of email resolution rows.
// It validates all rows first, then delegates to the database implementation.
//
// The batch must use the ON CONFLICT rule from the plan:
//   ON CONFLICT (email) DO UPDATE
//     SET login = excluded.login, source = excluded.source,
//         resolved_at = excluded.resolved_at
//     WHERE excluded.source = 'manual'
//        OR (email_resolution.source <> 'manual'
//            AND excluded.resolved_at > email_resolution.resolved_at)
//
// This means:
// - A manual source always wins (overwrites any existing row)
// - A non-manual source wins only if the existing row is also non-manual
//   AND has an older resolved_at timestamp
// - Otherwise the existing row is left unchanged
//
// Rows that lose the conflict check are silently skipped - this is
// upsert semantics, not an all-or-nothing batch failure.
//
// Returns an error if validation fails or the database operation fails.
// A partial failure (some rows succeed, some fail) returns an error.
func (i *Ingester) IngestResolution(ctx context.Context, rows []ResolutionRow) error {
	if len(rows) == 0 {
		return nil
	}

	// Track total records processed
	i.Processed += int64(len(rows))

	// Validate all rows first
	for idx := range rows {
		if err := rows[idx].Validate(); err != nil {
			return fmt.Errorf("row %d: %w", idx, err)
		}
	}

	// Delegate to database implementation
	return i.db.IngestEmailResolution(ctx, rows)
}

// GetProcessed returns the total number of records processed.
func (i *Ingester) GetProcessed() int64 {
	return i.Processed
}
