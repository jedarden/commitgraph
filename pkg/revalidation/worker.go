// Package revalidation provides the core logic for the login revalidation worker.
//
// This package is extracted from containers/login-revalidation-worker/main.go
// to make the worker logic testable from other packages.
package revalidation

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jedarden/commitgraph/pkg/client/github"
)

// Row represents a row from the email_revalidation table.
type Row struct {
	Email         string
	Login         string
	LastCheckedAt time.Time
	NextCheckAt   *time.Time
	Status        string
	NewLogin      *string
	CheckError    *string
	CreatedAt     time.Time
}

// Client defines the interface for the queue-api client.
type Client interface {
	PostResolution(ctx context.Context, email, login string) error
}

// Config holds worker configuration.
type Config struct {
	QueueAPIClient Client
	GitHubClient   github.Client
}

// ProcessRow checks a single login against GitHub and updates the database.
// This is the core worker logic extracted from containers/login-revalidation-worker/main.go
// to make it testable from other packages.
func ProcessRow(ctx context.Context, db *sql.DB, cfg *Config, row Row) error {
	now := time.Now()

	// Check GitHub API for login liveness
	result, err := cfg.GitHubClient.CheckLogin(ctx, row.Login)
	if err != nil {
		return fmt.Errorf("check login failed: %w", err)
	}

	switch result.Status {
	case github.StatusValidated:
		// Login is live and current - update for next check in 90 days
		nextCheck := now.Add(90 * 24 * time.Hour)
		if err := updateRevalidation(ctx, db, row.Email, string(result.Status), nil, nextCheck, nil); err != nil {
			return fmt.Errorf("update validated failed: %w", err)
		}

	case github.StatusRenamed:
		// Login was renamed - update email_resolution
		if result.NewLogin == nil {
			return fmt.Errorf("renamed status requires new_login")
		}
		if err := updateEmailResolution(ctx, cfg, row.Email, *result.NewLogin); err != nil {
			return fmt.Errorf("update email_resolution failed: %w", err)
		}
		// Mark as renamed - no further checks needed
		if err := updateRevalidation(ctx, db, row.Email, string(result.Status), result.NewLogin, time.Time{}, nil); err != nil {
			return fmt.Errorf("update renamed failed: %w", err)
		}

	case github.StatusDeleted:
		// Account is gone - stop rechecking
		if err := updateRevalidation(ctx, db, row.Email, string(result.Status), nil, time.Time{}, nil); err != nil {
			return fmt.Errorf("update deleted failed: %w", err)
		}

	case github.StatusRetry:
		// Transient failure - short backoff
		nextCheck := now.Add(5 * time.Minute)
		errMsg := "rate limit or network error"
		if result.ErrorMsg != nil {
			errMsg = *result.ErrorMsg
		}
		if err := updateRevalidation(ctx, db, row.Email, string(result.Status), nil, nextCheck, &errMsg); err != nil {
			return fmt.Errorf("update retry failed: %w", err)
		}

	default:
		return fmt.Errorf("unknown status: %s", result.Status)
	}

	return nil
}

// updateRevalidation updates the email_revalidation table.
func updateRevalidation(ctx context.Context, db *sql.DB, email, status string, newLogin *string, nextCheck time.Time, checkError *string) error {
	query := `
		UPDATE email_revalidation
		SET status = $1,
		    new_login = $2,
		    next_check_at = $3,
		    check_error = $4,
		    last_checked_at = NOW()
		WHERE email = $5
	`

	// Handle NULL values for next_check_at (terminal states)
	var nextCheckPtr *time.Time
	if !nextCheck.IsZero() {
		nextCheckPtr = &nextCheck
	}

	_, err := db.ExecContext(ctx, query, status, newLogin, nextCheckPtr, checkError, email)
	if err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}

	return nil
}

// updateEmailResolution calls the queue-api client with the new login.
func updateEmailResolution(ctx context.Context, cfg *Config, email, newLogin string) error {
	// Use the queue-api client to post the resolution
	return cfg.QueueAPIClient.PostResolution(ctx, email, newLogin)
}
