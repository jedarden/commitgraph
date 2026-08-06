// Package queueapi provides a client for the queue-api ingest endpoint.
package queueapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/jedarden/commitgraph/pkg/ingestlog"
)

// Client is a client for the queue-api ingest endpoint.
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
	maxRetries int
	logger     *ingestlog.Logger // Optional structured logger
}

// DefaultMaxRetries is the maximum number of retry attempts for ingest endpoint failures.
const DefaultMaxRetries = 4

// NewClient creates a new queue-api client.
//
// Parameters:
//   - baseURL: The base URL of the queue-api (e.g., "http://queue-api:8080")
//   - authToken: Optional bearer token for authentication (can be empty)
//
// Returns:
//   - A configured Client ready for use
func NewClient(baseURL, authToken string) *Client {
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
		authToken:  authToken,
		maxRetries: DefaultMaxRetries,
	}
}

// ResolutionRequest represents a request to the email resolution ingest endpoint.
type ResolutionRequest struct {
	Email       string `json:"email"`
	GithubLogin string `json:"github_login"`
	Source      string `json:"source"`
	ResolvedAt  string `json:"resolved_at"` // ISO 8601 timestamp
}

// PostResolution posts a resolution to the ingest endpoint with retry logic.
//
// This method calls the queue-api's /email-resolution/resolve endpoint with the
// provided email and github username. The source is set to "live" and the
// resolved_at timestamp is set to the current time.
//
// Retry logic:
//   - Implements exponential backoff with: 100ms, 400ms, 900ms, 1600ms delays
//   - Maximum retry attempts: 4 (total max delay: ~3 seconds)
//   - Retries on transient network errors and timeout errors
//   - Does not retry on client errors (4xx) except 408 Request Timeout
//   - Logs all retry attempts with structured error context
//
// The queue claim remains valid during retries since the total retry duration
// is short (~3 seconds max), which is well within the worker's processing window.
//
// Parameters:
//   - ctx: Context for the request (cancellation is respected during retries)
//   - email: The email address to resolve
//   - githubUsername: The resolved GitHub username
//
// Returns:
//   - nil on success
//   - error if all retry attempts are exhausted or a non-retryable error occurs
func (c *Client) PostResolution(ctx context.Context, email, githubUsername string) error {
	// Prepare the request with source="live" and current timestamp
	req := ResolutionRequest{
		Email:       email,
		GithubLogin: githubUsername,
		Source:      "live",
		ResolvedAt:  time.Now().Format(time.RFC3339),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	backoffSequence := []time.Duration{100, 400, 900, 1600} // Exponential backoff: ~3 seconds total

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Log retry attempt with full context
			log.Printf("[QUEUE-INGEST-RETRY] email=%s github_username=%s attempt=%d/%d error=%q",
				email, githubUsername, attempt, c.maxRetries, lastErr)

			// Check if context is cancelled before sleeping
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			default:
			}

			// Sleep for backoff duration before retry
			backoff := backoffSequence[attempt-1]
			time.Sleep(backoff)
		}

		// Create HTTP request
		url := fmt.Sprintf("%s/email-resolution/resolve", c.baseURL)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
		if err != nil {
			lastErr = fmt.Errorf("create request: %w", err)
			continue // Retry on request creation failure
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if c.authToken != "" {
			httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
		}

		// Execute the request
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			// Check if this is a timeout error (should retry)
			var netErr interface{ Timeout() bool }
			if errors.As(err, &netErr) && netErr.Timeout() {
				lastErr = fmt.Errorf("timeout error (retryable): %w", err)
				continue
			}
			// Other network errors (connection refused, DNS failure, etc.) - retry
			lastErr = fmt.Errorf("network error (retryable): %w", err)
			continue
		}

		// Process response
		respBody := resp.Body
		defer respBody.Close()

		// Check status code
		if resp.StatusCode == http.StatusOK {
			return nil // Success
		}

		// Determine if error is retryable based on status code
		if resp.StatusCode == 408 || // Request Timeout
			resp.StatusCode == 429 || // Too Many Requests
			resp.StatusCode >= 500 { // Server errors
			lastErr = fmt.Errorf("server returned retryable status %d", resp.StatusCode)
			continue
		}

		// Client errors (4xx except 408) are not retryable
		lastErr = fmt.Errorf("server returned non-retryable status %d", resp.StatusCode)
		break // Don't retry on client errors
	}

	// All retries exhausted - log structured failure
	log.Printf("[QUEUE-INGEST-FAILURE] email=%s github_username=%s error=%q",
		email, githubUsername, lastErr)

	return fmt.Errorf("post resolution failed after %d attempts: %w", c.maxRetries+1, lastErr)
}
