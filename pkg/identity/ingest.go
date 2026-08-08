// Package identity provides bulk identity resolution ingest functionality.
//
// The ingest path is the single way all writers (live enrichment worker,
// claude-leaderboard seed, manual curation) write email→login resolutions
// to the email_resolution table, ensuring consistent conflict resolution.
package identity

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"time"

	"github.com/jedarden/commitgraph/pkg/ingestlog"
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

// SkipReason represents why a record was skipped during ingest.
type SkipReason string

const (
	SkipReasonConflictManual SkipReason = "conflict_manual" // Existing manual source won
	SkipReasonConflictOlder  SkipReason = "conflict_older"  // Existing record has newer timestamp
	SkipReasonValidation     SkipReason = "validation"      // Row failed validation
	SkipReasonDatabase       SkipReason = "database"        // Database error during ingest
	SkipReasonOther          SkipReason = "other"           // Other skip reasons
)

// String returns the string representation of the skip reason.
func (r SkipReason) String() string {
	return string(r)
}

// IngestResult describes the outcome of a bulk ingest operation.
type IngestResult struct {
	// Ingested is the number of rows that were written (inserted or updated).
	Ingested int64
	// Skipped is the number of rows that were not written due to conflict resolution.
	Skipped int64
	// SkipDetails provides a breakdown of skip reasons.
	SkipDetails map[SkipReason]int64
}

// Ingester handles bulk upsert of email resolution rows with counter tracking.
type Ingester struct {
	db          DB
	logger      *ingestlog.Logger     // Structured logger for ingest operations
	Processed   int64                 // Total number of records processed (seen)
	Ingested    int64                 // Total number of records successfully written (inserted or updated)
	Skipped     int64                 // Total number of records skipped due to conflict resolution
	SkipDetails map[SkipReason]int64 // Breakdown of skip reasons
}

// DB is the database interface required by Ingester.
// This allows for testing with mocks and supports different database drivers.
type DB interface {
	// IngestEmailResolution performs a bulk upsert of resolution rows.
	// The batch must apply the ON CONFLICT rule:
	//   - Manual source always wins
	//   - Otherwise, the newer resolved_at wins
	// Rows that lose the conflict check are silently skipped.
	// Returns IngestResult with counts of ingested and skipped rows.
	IngestEmailResolution(ctx context.Context, rows []ResolutionRow) (*IngestResult, error)
}

// NewIngester creates a new Ingester.
func NewIngester(db DB) *Ingester {
	return &Ingester{
		db:          db,
		logger:      ingestlog.NewLogger(), // Initialize structured logger
		Processed:   0,                      // Initialize counter to zero
		Ingested:    0,                     // Initialize counter to zero
		Skipped:     0,                     // Initialize counter to zero
		SkipDetails: make(map[SkipReason]int64), // Initialize empty map for skip tracking
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
			// Log validation error with full context
			row := rows[idx]
			i.logValidationError(idx, row, err)
			return fmt.Errorf("row %d: %w", idx, err)
		}
	}

	// Delegate to database implementation
	result, err := i.db.IngestEmailResolution(ctx, rows)
	if err != nil {
		return err
	}
	i.Ingested += result.Ingested
	i.Skipped += result.Skipped
	for reason, count := range result.SkipDetails {
		i.SkipDetails[reason] += count
	}
	return nil
}

// GetProcessed returns the total number of records processed.
func (i *Ingester) GetProcessed() int64 {
	return i.Processed
}

// GetIngested returns the total number of records successfully ingested.
func (i *Ingester) GetIngested() int64 {
	return i.Ingested
}

// GetSkipped returns the total number of records skipped.
func (i *Ingester) GetSkipped() int64 {
	return i.Skipped
}

// GetSkipDetails returns the breakdown of skip reasons.
func (i *Ingester) GetSkipDetails() map[SkipReason]int64 {
	return i.SkipDetails
}

// GetSummary returns a machine-readable, JSON-marshalable snapshot of the
// ingester's counters, suitable for logging at the end of an ingest run.
// Keys: "processed", "ingested", "skipped" (int64), "skip_details"
// (map[string]int64 keyed by skip reason string).
func (i *Ingester) GetSummary() map[string]interface{} {
	skipDetails := make(map[string]int64, len(i.SkipDetails))
	for reason, count := range i.SkipDetails {
		skipDetails[reason.String()] = count
	}
	return map[string]interface{}{
		"processed":    i.Processed,
		"ingested":     i.Ingested,
		"skipped":      i.Skipped,
		"skip_details": skipDetails,
	}
}

// logValidationError logs a validation error with structured context.
// It captures the row index, email, login, and validation error details.
// Falls back to stderr logging if the structured logger fails.
func (i *Ingester) logValidationError(rowIdx int, row ResolutionRow, err error) {
	// Build structured error context
	errorMsg := fmt.Sprintf("validation_error at row %d: email=%q login=%q source=%q error=%s",
		rowIdx, row.Email, row.Login, row.Source, err.Error())

	// Try to use the structured logger first
	if i.logger != nil {
		// Record as a skip for statistics tracking
		i.logger.RecordSkipped(errorMsg)

		// Build a detailed log entry for the validation error
		entry := &ingestlog.LogEntry{
			Timestamp: time.Now().UTC(),
			EventType: "validation_error",
			User: ingestlog.UserContext{
				Email:          row.Email,
				GithubUsername: row.Login,
			},
			Endpoint: ingestlog.EndpointContext{
				Endpoint:      "identity-ingest",
				Method:        "VALIDATE",
				Path:          "row_validation",
				URL:           "internal://ingest/validation",
				AttemptNumber: rowIdx + 1, // Use 1-based indexing for attempts
			},
			Error: ingestlog.ErrorContext{
				Type:        "validation_error",
				Message:     err.Error(),
				StackTrace: string(debug.Stack()),
			},
			MaxRetries:      0, // No retries for validation errors
			RetryDelayMs:    0,
			TotalDurationMs: 0,
			Metadata: ingestlog.RequestMetadata{
				"row_index":    rowIdx,
				"row_source":   string(row.Source),
				"resolved_at":  row.ResolvedAt.UTC().Format(time.RFC3339),
			},
		}

		// Try to log the structured entry
		if logErr := i.logger.LogFailureWithEntry(entry); logErr != nil {
			// Fallback to stderr if structured logging fails
			log.Printf("[INGEST-VALIDATION-ERROR-FALLBACK] %s\n", errorMsg)
			log.Printf("[INGEST-VALIDATION-ERROR-FALLBACK] logging failed: %v\n", logErr)
		}
	} else {
		// Final fallback if logger is not initialized
		log.Printf("[INGEST-VALIDATION-ERROR] %s\n", errorMsg)
	}
}
