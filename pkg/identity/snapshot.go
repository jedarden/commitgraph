// Package identity provides snapshot capture utilities for email resolution tables.
package identity

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// EmailResolutionSnapshot captures the complete state of the email_resolution table.
// This includes the row count, a cryptographic hash of all data, and optionally
// the full row data for debugging and detailed comparison.
type EmailResolutionSnapshot struct {
	// RowCount is the total number of rows in the email_resolution table
	RowCount int

	// Hash is the SHA-256 checksum of all row data (sorted by email)
	// The hash includes all columns: email, login, source, resolved_at
	Hash string

	// Rows contains the full row data for debugging and detailed analysis.
	// This field is optional and can be nil for snapshots that only need
	// hash-based comparison.
	Rows []ResolutionRow
}

// captureSnapshotOptions configures the behavior of CaptureSnapshot.
type captureSnapshotOptions struct {
	includeFullRows bool
}

// CaptureSnapshotOption is a functional option for CaptureSnapshot.
type CaptureSnapshotOption func(*captureSnapshotOptions)

// WithFullRowData enables capturing the complete row data in the snapshot.
// This is useful for debugging and detailed analysis but uses more memory.
func WithFullRowData() CaptureSnapshotOption {
	return func(opts *captureSnapshotOptions) {
		opts.includeFullRows = true
	}
}

// CaptureSnapshot reads all rows from the email_resolution table and returns
// a snapshot containing the row count, cryptographic hash, and optionally
// the full row data.
//
// The hash is computed by:
// 1. Reading all rows sorted by email (ensures consistent ordering)
// 2. Concatenating row data in a stable format
// 3. Computing SHA-256 of the concatenated data
//
// This ensures the hash is sensitive to changes in any column (email, login,
// source, resolved_at) regardless of database storage order.
//
// Parameters:
//   - db: Database connection (can be PostgreSQL or SQLite)
//   - opts: Optional configuration (e.g., WithFullRowData())
//
// Returns an error if:
//   - The database query fails
//   - A row cannot be scanned
//   - The timestamp format is invalid
func CaptureSnapshot(db *sql.DB, opts ...CaptureSnapshotOption) (*EmailResolutionSnapshot, error) {
	// Parse options
	options := &captureSnapshotOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Query all rows, sorted by email for consistent hashing
	// We sort by all columns to ensure deterministic ordering
	rows, err := db.Query(`
		SELECT email, login, source, resolved_at
		FROM email_resolution
		ORDER BY email, login, source, resolved_at
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query email_resolution: %w", err)
	}
	defer rows.Close()

	// Prepare hash writer
	hasher := sha256.New()
	var allRows []ResolutionRow
	count := 0

	// Read and process each row
	for rows.Next() {
		var email, login, source string
		var resolvedAt time.Time

		if err := rows.Scan(&email, &login, &source, &resolvedAt); err != nil {
			return nil, fmt.Errorf("failed to scan row %d: %w", count, err)
		}

		// Parse source to ensure it's valid
		sourceEnum := Source(source)
		if sourceEnum != SourceLive && sourceEnum != SourceSeed && sourceEnum != SourceManual {
			return nil, fmt.Errorf("invalid source %q for email %s", source, email)
		}

		// Format row data for hashing in a stable, canonical format
		// Format: email|login|source|resolved_at\n
		// Using RFC3339Nano ensures consistent timestamp representation
		rowData := fmt.Sprintf("%s|%s|%s|%s\n", email, login, source, resolvedAt.Format(time.RFC3339Nano))
		if _, err := hasher.Write([]byte(rowData)); err != nil {
			return nil, fmt.Errorf("failed to write to hash: %w", err)
		}

		// Store full row data if requested
		if options.includeFullRows {
			allRows = append(allRows, ResolutionRow{
				Email:      email,
				Login:      login,
				Source:     sourceEnum,
				ResolvedAt: resolvedAt,
			})
		}

		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Compute final hash
	hashBytes := hasher.Sum(nil)
	hash := hex.EncodeToString(hashBytes)

	return &EmailResolutionSnapshot{
		RowCount: count,
		Hash:     hash,
		Rows:     allRows,
	}, nil
}

// CompareSnapshots compares two snapshots and returns whether they are
// byte-for-byte identical.
//
// Returns:
//   - (true, nil) if snapshots are identical (same row count and hash)
//   - (false, error) if snapshots differ, with detailed error message
//   - (false, error) if either snapshot is nil
func CompareSnapshots(a, b *EmailResolutionSnapshot) (bool, error) {
	// Check for nil snapshots
	if a == nil && b == nil {
		return true, nil
	}
	if a == nil {
		return false, fmt.Errorf("first snapshot is nil")
	}
	if b == nil {
		return false, fmt.Errorf("second snapshot is nil")
	}

	// Compare row counts
	if a.RowCount != b.RowCount {
		return false, fmt.Errorf("row count differs: %d vs %d", a.RowCount, b.RowCount)
	}

	// Compare hashes
	if a.Hash != b.Hash {
		return false, fmt.Errorf("data hash differs:\n  first:  %s\n  second: %s", a.Hash, b.Hash)
	}

	// All checks passed - snapshots are identical
	return true, nil
}
