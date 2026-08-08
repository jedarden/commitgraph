// Package queueapi provides a client for the queue-api ingest endpoint.
package queueapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
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

// SetLogger sets a custom ingest logger for the client.
// If not set, a default logger writing to stderr is used.
func (c *Client) SetLogger(logger *ingestlog.Logger) {
	c.logger = logger
}

// log returns the client's logger, lazily initializing a default one if the
// Client was constructed as a zero value (e.g. `&Client{...}` in tests)
// rather than via NewClient. This keeps PostResolution safe to call on any
// Client value instead of panicking on a nil logger.
func (c *Client) log() *ingestlog.Logger {
	if c.logger == nil {
		c.logger = ingestlog.NewLogger()
	}
	return c.logger
}

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
		logger:     ingestlog.NewLogger(),
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
	// Record that this record is entering the ingest flow
	c.log().RecordProcessed()

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
	var lastStatusCode int
	var lastResponseBody string
	backoffSequence := []time.Duration{100, 400, 900, 1600} // Exponential backoff: ~3 seconds total
	startTime := time.Now()

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Log retry attempt with full structured context
			backoff := backoffSequence[attempt-1]
			totalDurationMs := time.Since(startTime).Milliseconds()

			// Create structured event for this retry
			event := ingestlog.EventFromError(
				email,
				githubUsername,
				fmt.Sprintf("%s/email-resolution/resolve", c.baseURL),
				lastErr,
				0, // statusCode (not available in retry case before request)
				"", // responseBody (not available for request creation failures)
				attempt,
				c.maxRetries,
				int(backoff.Milliseconds()),
				totalDurationMs,
			)

			if err := c.log().LogRetry(&event); err != nil {
				// Fallback to basic log if structured logging fails
				log.Printf("[QUEUE-INGEST-RETRY] email=%s github_username=%s attempt=%d/%d error=%q (structured logging failed: %v)",
					email, githubUsername, attempt, c.maxRetries, lastErr, err)
			}

			// Check if context is cancelled before sleeping
			select {
			case <-ctx.Done():
				// Log the failure before returning due to context cancellation
				totalDurationMs := time.Since(startTime).Milliseconds()
				event := ingestlog.EventFromError(
					email,
					githubUsername,
					fmt.Sprintf("%s/email-resolution/resolve", c.baseURL),
					ctx.Err(),
					lastStatusCode,
					lastResponseBody,
					attempt, // Current attempt when cancelled
					c.maxRetries,
					0, // No retry delay on cancellation
					totalDurationMs,
				)

				if err := c.log().LogFailure(&event); err != nil {
					// Fallback to basic log if structured logging fails
					log.Printf("[QUEUE-INGEST-FAILURE] email=%s github_username=%s error=%q (structured logging failed: %v)",
						email, githubUsername, ctx.Err(), err)
				}

				return fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			default:
			}

			// Sleep for backoff duration before retry
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

		// Read response body for error logging
		bodyBytes, _ := io.ReadAll(respBody)
		lastResponseBody = string(bodyBytes)
		lastStatusCode = resp.StatusCode

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
	totalDurationMs := time.Since(startTime).Milliseconds()
	event := ingestlog.EventFromError(
		email,
		githubUsername,
		fmt.Sprintf("%s/email-resolution/resolve", c.baseURL),
		lastErr,
		lastStatusCode,
		lastResponseBody,
		c.maxRetries, // This was the final attempt
		c.maxRetries,
		0, // No retry delay on final failure
		totalDurationMs,
	)

	if err := c.log().LogFailure(&event); err != nil {
		// Fallback to basic log if structured logging fails
		log.Printf("[QUEUE-INGEST-FAILURE] email=%s github_username=%s error=%q (structured logging failed: %v)",
			email, githubUsername, lastErr, err)
	}

	return fmt.Errorf("post resolution failed after %d attempts: %w", c.maxRetries+1, lastErr)
}
