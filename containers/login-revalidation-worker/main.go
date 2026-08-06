// Login Revalidation Worker — detect renamed/deleted GitHub logins.
//
// This worker periodically samples rows from the email_revalidation table,
// checks login liveness against the GitHub API, and updates the
// email_resolution table when a rename is detected or flags the row when
// a deletion is detected.
//
// The worker respects the shared GitHub API rate limit by using the same
// API_CALL_INTERVAL_SECS mechanism as user-enrichment-worker.
//
// Lifecycle:
// 1. Claim: pick due rows from email_revalidation (next_check_at due or NULL for pending)
// 2. Check: query GitHub API for each login
// 3. Record: update email_revalidation with outcome
// 4. Update: on rename, call identity-ingest-endpoint with new login
// 5. Flag: on deletion, set status='deleted' and stop rechecking
//
// Environment variables:
// - QUEUE_API_URL: queue-api endpoint for email resolution ingest
// - QUEUE_API_INTERNAL_TOKEN: optional bearer token for queue-api auth
// - GITHUB_TOKEN: GitHub API token for checking login liveness
// - POSTGRES_URL: PostgreSQL connection string
// - WORKER_ID: unique identifier for this worker (default: hostname)
// - CLAIM_BATCH: number of rows to claim per cycle (default: 50)
// - IDLE_SLEEP_SECS: seconds to sleep when no work (default: 60)
// - API_CALL_INTERVAL_SECS: seconds between GitHub API calls (default: 6)

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// Config holds worker configuration from environment variables.
type Config struct {
	QueueAPIURL           string
	QueueAPIInternalToken string
	GitHubToken           string
	PostgresURL           string
	WorkerID              string
	ClaimBatch            int
	IdleSleepSecs         int
	APICallIntervalSecs   int
}

// RevalidationRow represents a row from the email_revalidation table.
type RevalidationRow struct {
	Email         string
	Login         string
	LastCheckedAt time.Time
	NextCheckAt   *time.Time
	Status        string
	NewLogin      *string
	CheckError    *string
	CreatedAt     time.Time
}

// GitHubUser represents a GitHub user API response.
type GitHubUser struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	Type      string `json:"type"`
	SiteAdmin bool   `json:"site_admin"`
}

// ResolutionRow represents a row to send to the identity-ingest-endpoint.
type ResolutionRow struct {
	Email      string    `json:"email"`
	GithubLogin string   `json:"github_login"`
	Provider   string   `json:"provider"`
	WorkerID   string   `json:"worker_id"`
	ResolvedAt string   `json:"resolved_at"` // ISO 8601 timestamp
}

func loadConfig() (*Config, error) {
	workerID, err := os.Hostname()
	if err != nil {
		workerID = fmt.Sprintf("worker-%d", time.Now().Unix())
	}

	cfg := &Config{
		QueueAPIURL:           os.Getenv("QUEUE_API_URL"),
		QueueAPIInternalToken: os.Getenv("QUEUE_API_INTERNAL_TOKEN"),
		GitHubToken:           os.Getenv("GITHUB_TOKEN"),
		PostgresURL:           os.Getenv("POSTGRES_URL"),
		WorkerID:              workerID,
		ClaimBatch:            getEnvInt("CLAIM_BATCH", 50),
		IdleSleepSecs:         getEnvInt("IDLE_SLEEP_SECS", 60),
		APICallIntervalSecs:   int(getEnvFloat("API_CALL_INTERVAL_SECS", 6.0)),
	}

	if cfg.QueueAPIURL == "" {
		return nil, fmt.Errorf("QUEUE_API_URL is required")
	}
	if cfg.GitHubToken == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is required")
	}
	if cfg.PostgresURL == "" {
		return nil, fmt.Errorf("POSTGRES_URL is required")
	}

	// Remove trailing slash from QueueAPIURL
	cfg.QueueAPIURL = strings.TrimSuffix(cfg.QueueAPIURL, "/")

	return cfg, nil
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		var intVal int
		if _, err := fmt.Sscanf(val, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		var floatVal float64
		if _, err := fmt.Sscanf(val, "%f", &floatVal); err == nil {
			return floatVal
		}
	}
	return defaultVal
}

func main() {
	flag.Parse()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := sql.Open("postgres", cfg.PostgresURL)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	ctx := context.Background()

	log.Printf("login-revalidation-worker starting id=%s queue=%s batch=%d",
		cfg.WorkerID, cfg.QueueAPIURL, cfg.ClaimBatch)

	// Main worker loop
	ticker := time.NewTicker(time.Duration(cfg.IdleSleepSecs) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Context cancelled, shutting down")
			return
		default:
		}

		rows, err := claimRows(ctx, db, cfg.ClaimBatch)
		if err != nil {
			log.Printf("Failed to claim rows: %v", err)
			time.Sleep(time.Duration(cfg.IdleSleepSecs) * time.Second)
			continue
		}

		if len(rows) == 0 {
			log.Printf("No rows to process, sleeping %ds", cfg.IdleSleepSecs)
			<-ticker.C
			continue
		}

		log.Printf("Claimed %d rows for revalidation", len(rows))

		for _, row := range rows {
			if err := processRow(ctx, db, cfg, row); err != nil {
				log.Printf("Error processing row email=%s login=%s: %v",
					row.Email, row.Login, err)
			}

			// Sleep to respect rate limit
			time.Sleep(time.Duration(cfg.APICallIntervalSecs) * time.Second)
		}

		log.Printf("Batch completed")
	}
}

// claimRows claims rows from email_revalidation that need checking.
func claimRows(ctx context.Context, db *sql.DB, batch int) ([]RevalidationRow, error) {
	// Claim rows where:
	// - status is 'pending' (never checked)
	// - OR next_check_at is due (past or NULL for retry status)
	// - status is not 'deleted' or 'renamed' (terminal states)
	query := `
		SELECT email, login, last_checked_at, next_check_at, status, new_login, check_error, created_at
		FROM email_revalidation
		WHERE status = 'pending'
		   OR (next_check_at IS NOT NULL AND next_check_at <= NOW())
		   OR (status = 'retry' AND next_check_at IS NULL)
		ORDER BY
			CASE WHEN status = 'pending' THEN 0 ELSE 1 END,
			last_checked_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := db.QueryContext(ctx, query, batch)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var result []RevalidationRow
	for rows.Next() {
		var r RevalidationRow
		if err := rows.Scan(
			&r.Email, &r.Login, &r.LastCheckedAt, &r.NextCheckAt,
			&r.Status, &r.NewLogin, &r.CheckError, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		result = append(result, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return result, nil
}

// processRow checks a single login against GitHub and updates the database.
func processRow(ctx context.Context, db *sql.DB, cfg *Config, row RevalidationRow) error {
	now := time.Now()

	// Check GitHub API for login liveness
	status, newLogin, checkError := checkLogin(ctx, cfg.GitHubToken, row.Login)

	switch status {
	case "validated":
		// Login is live and current - update for next check in 90 days
		nextCheck := now.Add(90 * 24 * time.Hour)
		if err := updateRevalidation(ctx, db, row.Email, status, nil, nextCheck, nil); err != nil {
			return fmt.Errorf("update validated failed: %w", err)
		}
		log.Printf("Validated email=%s login=%s", row.Email, row.Login)

	case "renamed":
		// Login was renamed - update email_resolution
		if newLogin == nil {
			return fmt.Errorf("renamed status requires new_login")
		}
		if err := updateEmailResolution(ctx, cfg, row.Email, *newLogin); err != nil {
			return fmt.Errorf("update email_resolution failed: %w", err)
		}
		// Mark as renamed - no further checks needed
		if err := updateRevalidation(ctx, db, row.Email, status, newLogin, time.Time{}, nil); err != nil {
			return fmt.Errorf("update renamed failed: %w", err)
		}
		log.Printf("Renamed email=%s old_login=%s new_login=%s", row.Email, row.Login, *newLogin)

	case "deleted":
		// Account is gone - stop rechecking
		if err := updateRevalidation(ctx, db, row.Email, status, nil, time.Time{}, nil); err != nil {
			return fmt.Errorf("update deleted failed: %w", err)
		}
		log.Printf("Deleted email=%s login=%s", row.Email, row.Login)

	case "retry":
		// Transient failure - short backoff
		nextCheck := now.Add(5 * time.Minute)
		errMsg := "rate limit or network error"
		if checkError != nil {
			errMsg = *checkError
		}
		if err := updateRevalidation(ctx, db, row.Email, status, nil, nextCheck, &errMsg); err != nil {
			return fmt.Errorf("update retry failed: %w", err)
		}
		log.Printf("Retry email=%s login=%s error=%s", row.Email, row.Login, errMsg)

	default:
		return fmt.Errorf("unknown status: %s", status)
	}

	return nil
}

// checkLogin checks a login against GitHub API.
// Returns (status, new_login, error_message)
// status can be: "validated", "renamed", "deleted", "retry"
func checkLogin(ctx context.Context, githubToken, login string) (string, *string, *string) {
	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.github.com/users/%s", login), nil)
	if err != nil {
		errMsg := err.Error()
		return "retry", nil, &errMsg
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", githubToken))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		errMsg := err.Error()
		return "retry", nil, &errMsg
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		// User exists - check if login changed
		var user GitHubUser
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			errMsg := fmt.Sprintf("decode failed: %v", err)
			return "retry", nil, &errMsg
		}
		if user.Login == login {
			// Login is current
			return "validated", nil, nil
		}
		// Login was renamed
		return "renamed", &user.Login, nil

	case 404:
		// User not found - deleted
		return "deleted", nil, nil

	case 403, 429:
		// Rate limited
		errMsg := "rate limited"
		return "retry", nil, &errMsg

	default:
		errMsg := fmt.Sprintf("unexpected status %d", resp.StatusCode)
		return "retry", nil, &errMsg
	}
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

// updateEmailResolution calls the identity-ingest-endpoint with the new login.
func updateEmailResolution(ctx context.Context, cfg *Config, email, newLogin string) error {
	// Call the identity-ingest-endpoint
	// POST /email-resolution/resolve with source='live'
	row := ResolutionRow{
		Email:       email,
		GithubLogin: newLogin,
		Provider:    "github",
		WorkerID:    cfg.WorkerID,
		ResolvedAt:  time.Now().Format(time.RFC3339),
	}

	body, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.QueueAPIURL+"/email-resolution/resolve", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if cfg.QueueAPIInternalToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.QueueAPIInternalToken))
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return nil
}
