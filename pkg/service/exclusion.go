// Package service provides business logic services for commitgraph operations.
package service

import (
	"context"
	"database/sql"
)

// RowScanner is the interface for scanning row results.
// This makes testing easier by allowing mock implementations.
type RowScanner interface {
	Scan(dest ...interface{}) error
}

// Querier is the database interface for queries.
// This matches database/sql's DB and Conn interfaces.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner
}

// RepoChecker validates repo existence and related business rules.
type RepoChecker struct {
	db Querier
}

// NewRepoChecker creates a new RepoChecker.
func NewRepoChecker(db Querier) *RepoChecker {
	return &RepoChecker{db: db}
}

// RepoExists checks if a repo exists in the repos table by provider and full_name.
//
// Returns false if:
// - provider is empty
// - repoFullName is empty
// - repo is not found in the database
// - database query returns an error
//
// This is a validation helper - it returns false on any error condition
// to fail-safe rather than propagate errors.
func (r *RepoChecker) RepoExists(ctx context.Context, provider, repoFullName string) bool {
	// Handle empty inputs - return false
	if provider == "" || repoFullName == "" {
		return false
	}

	query := `
		SELECT 1
		FROM repos
		WHERE provider = $1 AND repo_full_name = $2
		LIMIT 1
	`

	var exists int
	err := r.db.QueryRowContext(ctx, query, provider, repoFullName).Scan(&exists)

	// Return false on any error (repo not found or query error)
	if err != nil {
		return false
	}

	return true
}

// sqlRowScanner wraps *sql.Row to implement RowScanner.
type sqlRowScanner struct {
	row *sql.Row
}

func (s *sqlRowScanner) Scan(dest ...interface{}) error {
	return s.row.Scan(dest...)
}

// SQLQuerier wraps *sql.DB to implement Querier.
type SQLQuerier struct {
	db *sql.DB
}

func (s *SQLQuerier) QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner {
	return &sqlRowScanner{row: s.db.QueryRowContext(ctx, query, args...)}
}

// NewRepoCheckerFromDB creates a RepoChecker from a *sql.DB.
func NewRepoCheckerFromDB(db *sql.DB) *RepoChecker {
	return NewRepoChecker(&SQLQuerier{db: db})
}
