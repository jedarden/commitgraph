// Package queueapi provides a client for the queue-api ingest endpoint.
package queueapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client is a client for the queue-api ingest endpoint.
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
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
	}
}

// ResolutionRequest represents a request to the email resolution ingest endpoint.
type ResolutionRequest struct {
	Email       string `json:"email"`
	GithubLogin string `json:"github_login"`
	Source      string `json:"source"`
	ResolvedAt  string `json:"resolved_at"` // ISO 8601 timestamp
}

// PostResolution posts a resolution to the ingest endpoint.
//
// This method calls the queue-api's /email-resolution/resolve endpoint with the
// provided email and github username. The source is set to "live" and the
// resolved_at timestamp is set to the current time.
//
// Parameters:
//   - ctx: Context for the request
//   - email: The email address to resolve
//   - githubUsername: The resolved GitHub username
//
// Returns:
//   - nil on success
//   - error if the request fails or returns a non-200 status
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

	// Create HTTP request
	url := fmt.Sprintf("%s/email-resolution/resolve", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	}

	// Execute the request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
