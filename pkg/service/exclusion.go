// Package service provides business logic services for commitgraph operations.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
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

// Execer is the database interface for executing statements.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// Transactor is the interface for database transactions.
type Transactor interface {
	Execer
	Querier
	Commit() error
	Rollback() error
}

// Transactioner is the interface for beginning transactions.
type Transactioner interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Transactor, error)
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

// SQLTx wraps *sql.Tx to implement Transactor.
type SQLTx struct {
	tx *sql.Tx
}

func (s *SQLTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.tx.ExecContext(ctx, query, args...)
}

func (s *SQLTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner {
	return &sqlRowScanner{row: s.tx.QueryRowContext(ctx, query, args...)}
}

func (s *SQLTx) Commit() error {
	return s.tx.Commit()
}

func (s *SQLTx) Rollback() error {
	return s.tx.Rollback()
}

// SQLDB wraps *sql.DB to implement Transactioner.
type SQLDB struct {
	db *sql.DB
}

// NewSQLDB creates a new SQLDB from *sql.DB for use with service functions.
func NewSQLDB(db *sql.DB) *SQLDB {
	return &SQLDB{db: db}
}

func (s *SQLDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) RowScanner {
	return &sqlRowScanner{row: s.db.QueryRowContext(ctx, query, args...)}
}

func (s *SQLDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Transactor, error) {
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &SQLTx{tx: tx}, nil
}

// NewRepoCheckerFromDB creates a RepoChecker from a *sql.DB.
func NewRepoCheckerFromDB(db *sql.DB) *RepoChecker {
	return NewRepoChecker(&SQLQuerier{db: db})
}

// validateProvider validates the provider format.
// Returns error if provider is empty or contains invalid characters.
// Valid providers should be lowercase alphanumeric names (e.g., "github", "gitlab").
func validateProvider(provider string) error {
	if provider == "" {
		return fmt.Errorf("provider cannot be empty")
	}

	// Provider should be lowercase alphanumeric
	matched, err := regexp.MatchString("^[a-z0-9]+$", provider)
	if err != nil {
		return fmt.Errorf("failed to validate provider format: %w", err)
	}
	if !matched {
		return fmt.Errorf("provider must be lowercase alphanumeric (e.g., 'github', 'gitlab'), got: %s", provider)
	}

	return nil
}

// validateRepoFullName validates the repoFullName format.
// Returns error if repoFullName is empty or not in owner/repo format.
// Valid format is "owner/repo" where both owner and repo are non-empty.
func validateRepoFullName(repoFullName string) error {
	if repoFullName == "" {
		return fmt.Errorf("repository full name cannot be empty")
	}

	// Check for owner/repo format
	parts := strings.Split(repoFullName, "/")
	if len(parts) != 2 {
		return fmt.Errorf("repository full name must be in 'owner/repo' format, got: %s", repoFullName)
	}

	owner, repo := parts[0], parts[1]
	if owner == "" {
		return fmt.Errorf("repository owner cannot be empty in 'owner/repo' format")
	}
	if repo == "" {
		return fmt.Errorf("repository name cannot be empty in 'owner/repo' format")
	}

	return nil
}

// SetRepoExclusion sets the exclusion status for a repository.
//
// This function performs the following operations within a database transaction:
// 1. Validates that the repo exists (using RepoExists)
// 2. Validates that the reason is not empty
// 3. Sets excluded_at to NOW() and excluded_reason to the provided reason
//
// Parameters:
//   - ctx: Context for the operation
//   - db: Database connection (will be used to create a transaction)
//   - provider: Repository provider (e.g., "github")
//   - repoFullName: Repository full name (e.g., "owner/repo")
//   - reason: Human-readable reason for exclusion (must not be empty)
//
// Returns:
//   - nil on success
//   - error if validation fails or database operation fails
//
// The function uses a database transaction to ensure atomicity:
// - On success, the transaction is committed
// - On error, the transaction is rolled back
func SetRepoExclusion(ctx context.Context, db Transactioner, provider, repoFullName, reason string) error {
	return SetRepoExclusionWithActor(ctx, db, provider, repoFullName, reason, "system")
}

// SetRepoExclusionWithActor sets the exclusion status for a repository with a specific actor.
//
// This function performs the following operations within a database transaction:
// 1. Validates that the repo exists (using RepoExists)
// 2. Validates that the reason is not empty
// 3. Queries the current exclusion state (excluded_at, excluded_reason, repo_id) BEFORE updating
// 4. Sets excluded_at to NOW() and excluded_reason to the provided reason
// 5. Records an audit log entry with the before and after states
//
// Parameters:
//   - ctx: Context for the operation
//   - db: Database connection (will be used to create a transaction)
//   - provider: Repository provider (e.g., "github")
//   - repoFullName: Repository full name (e.g., "owner/repo")
//   - reason: Human-readable reason for exclusion (must not be empty)
//   - actor: Who performed the action (e.g., 'admin', 'system')
//
// Returns:
//   - nil on success
//   - error if validation fails or database operation fails
//
// The function uses a database transaction to ensure atomicity:
// - On success, the transaction is committed
// - On error, the transaction is rolled back
func SetRepoExclusionWithActor(ctx context.Context, db Transactioner, provider, repoFullName, reason, actor string) error {
	// Validate provider format
	if err := validateProvider(provider); err != nil {
		return err
	}

	// Validate repoFullName format
	if err := validateRepoFullName(repoFullName); err != nil {
		return err
	}

	// Validate reason is not empty
	if reason == "" {
		return fmt.Errorf("exclusion reason cannot be empty")
	}

	// Check if repo exists
	checker := NewRepoChecker(db)
	if !checker.RepoExists(ctx, provider, repoFullName) {
		return fmt.Errorf("repository %s/%s not found", provider, repoFullName)
	}

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback happens on error
	defer tx.Rollback()

	// Capture the current exclusion state BEFORE updating
	// We need repo_id for the audit log, and the current excluded_at and excluded_reason
	var repoID int64
	var oldExcludedAt *time.Time
	var oldExcludedReason *string

	selectQuery := `
		SELECT id, excluded_at, excluded_reason
		FROM repos
		WHERE provider = $1 AND repo_full_name = $2
	`

	selectRow := tx.QueryRowContext(ctx, selectQuery, provider, repoFullName)
	if err := selectRow.Scan(&repoID, &oldExcludedAt, &oldExcludedReason); err != nil {
		return fmt.Errorf("failed to query current repo state: %w", err)
	}

	// Update the repo with exclusion information
	updateQuery := `
		UPDATE repos
		SET excluded_at = NOW(),
		    excluded_reason = $1
		WHERE provider = $2 AND repo_full_name = $3
	`

	result, err := tx.ExecContext(ctx, updateQuery, reason, provider, repoFullName)
	if err != nil {
		return fmt.Errorf("failed to update repo exclusion: %w", err)
	}

	// Verify that exactly one row was affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no rows updated - repo may have been deleted")
	}

	// Record the audit log entry with before and after states
	newExcludedAt := time.Now()
	newExcludedReason := &reason

	if err := RecordExclusionAudit(
		ctx,
		tx,
		repoID,
		actor,
		"exclude",
		oldExcludedAt,
		oldExcludedReason,
		&newExcludedAt,
		newExcludedReason,
	); err != nil {
		return fmt.Errorf("failed to record exclusion audit: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ClearRepoExclusion clears the exclusion status for a repository.
//
// This function performs the following operations within a database transaction:
// 1. Validates that the repo exists (using RepoExists)
// 2. Sets excluded_at to NULL and excluded_reason to NULL
//
// Parameters:
//   - ctx: Context for the operation
//   - db: Database connection (will be used to create a transaction)
//   - provider: Repository provider (e.g., "github")
//   - repoFullName: Repository full name (e.g., "owner/repo")
//
// Returns:
//   - nil on success
//   - error if validation fails or database operation fails
//
// The function uses a database transaction to ensure atomicity:
// - On success, the transaction is committed
// - On error, the transaction is rolled back
//
// Note: Clearing exclusion on a repo that is not currently excluded is
// considered a no-op and will succeed (1 row affected).
func ClearRepoExclusion(ctx context.Context, db Transactioner, provider, repoFullName string) error {
	// Validate provider format
	if err := validateProvider(provider); err != nil {
		return err
	}

	// Validate repoFullName format
	if err := validateRepoFullName(repoFullName); err != nil {
		return err
	}

	// Check if repo exists
	checker := NewRepoChecker(db)
	if !checker.RepoExists(ctx, provider, repoFullName) {
		return fmt.Errorf("repository %s/%s not found", provider, repoFullName)
	}

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback happens on error
	defer tx.Rollback()

	// Update the repo to clear exclusion information
	query := `
		UPDATE repos
		SET excluded_at = NULL,
		    excluded_reason = NULL
		WHERE provider = $1 AND repo_full_name = $2
	`

	result, err := tx.ExecContext(ctx, query, provider, repoFullName)
	if err != nil {
		return fmt.Errorf("failed to clear repo exclusion: %w", err)
	}

	// Verify that exactly one row was affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no rows updated - repo may have been deleted")
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// recordExclusionAuditImpl is the actual implementation of RecordExclusionAudit.
// This is a separate function to allow mocking in tests.
func recordExclusionAuditImpl(
	ctx context.Context,
	tx Transactor,
	repoID int64,
	actor string,
	eventType string,
	oldExcludedAt *time.Time,
	oldExcludedReason *string,
	newExcludedAt *time.Time,
	newExcludedReason *string,
) error {
	query := `
		INSERT INTO exclusion_audit_log (
			repo_id,
			actor,
			event_type,
			old_excluded_at,
			old_excluded_reason,
			new_excluded_at,
			new_excluded_reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := tx.ExecContext(ctx, query,
		repoID,
		actor,
		eventType,
		oldExcludedAt,
		oldExcludedReason,
		newExcludedAt,
		newExcludedReason,
	)

	if err != nil {
		return fmt.Errorf("failed to insert exclusion audit log: %w", err)
	}

	return nil
}

// RecordExclusionAudit is a variable that holds the current implementation.
// This allows tests to mock the function for verification.
var RecordExclusionAudit = func(
	ctx context.Context,
	tx Transactor,
	repoID int64,
	actor string,
	eventType string,
	oldExcludedAt *time.Time,
	oldExcludedReason *string,
	newExcludedAt *time.Time,
	newExcludedReason *string,
) error {
	return recordExclusionAuditImpl(ctx, tx, repoID, actor, eventType, oldExcludedAt, oldExcludedReason, newExcludedAt, newExcludedReason)
}
