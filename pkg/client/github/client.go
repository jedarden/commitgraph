// Package github provides a client for checking GitHub user login status.
// This supports the login revalidation worker in detecting renamed and deleted accounts.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LoginStatus represents the possible outcomes of checking a login's liveness.
type LoginStatus string

const (
	// StatusValidated means the login exists and matches the queried login
	StatusValidated LoginStatus = "validated"
	// StatusRenamed means the login was renamed to a different login
	StatusRenamed LoginStatus = "renamed"
	// StatusDeleted means the login no longer exists (account deleted)
	StatusDeleted LoginStatus = "deleted"
	// StatusRetry means a transient error occurred and the check should be retried
	StatusRetry LoginStatus = "retry"
)

// LoginResult represents the result of checking a login's status.
type LoginResult struct {
	Status    LoginStatus // The detected status
	NewLogin  *string     // The new login if Status == StatusRenamed, otherwise nil
	ErrorMsg  *string     // Error message if Status == StatusRetry, otherwise nil
}

// User represents a GitHub user from the API response.
type User struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	Type      string `json:"type"`
	SiteAdmin bool   `json:"site_admin"`
}

// Client defines the interface for checking GitHub login status.
// This allows for test mocks without making live API calls.
type Client interface {
	// CheckLogin checks if a login exists on GitHub and returns its status.
	//
	// Returns:
	//   - LoginResult with Status, optional NewLogin, and optional ErrorMsg
	//   - error if the check itself fails (network issue, malformed response, etc.)
	CheckLogin(ctx context.Context, login string) (*LoginResult, error)
}

// HTTPClient implements Client using the real GitHub API.
type HTTPClient struct {
	token     string
	timeout   time.Duration
	userAgent string
}

// NewHTTPClient creates a new GitHub API client.
//
// Parameters:
//   - token: GitHub personal access token for authentication
//   - timeout: Request timeout (default 15s if 0)
//
// Returns:
//   - A configured Client that makes real HTTP requests to GitHub API
func NewHTTPClient(token string, timeout time.Duration) Client {
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &HTTPClient{
		token:     token,
		timeout:   timeout,
		userAgent: "login-revalidation-worker/1.0",
	}
}

// CheckLogin checks a login against the GitHub API.
func (c *HTTPClient) CheckLogin(ctx context.Context, login string) (*LoginResult, error) {
	client := &http.Client{
		Timeout: c.timeout,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.github.com/users/%s", login), nil)
	if err != nil {
		return &LoginResult{Status: StatusRetry, ErrorMsg: strPtr(err.Error())}, nil
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return &LoginResult{Status: StatusRetry, ErrorMsg: strPtr(err.Error())}, nil
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		// User exists - check if login changed
		var user User
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			return &LoginResult{Status: StatusRetry, ErrorMsg: strPtr(fmt.Sprintf("decode failed: %v", err))}, nil
		}
		if user.Login == login {
			// Login is current
			return &LoginResult{Status: StatusValidated}, nil
		}
		// Login was renamed
		return &LoginResult{Status: StatusRenamed, NewLogin: &user.Login}, nil

	case 404:
		// User not found - deleted
		return &LoginResult{Status: StatusDeleted}, nil

	case 403, 429:
		// Rate limited
		return &LoginResult{Status: StatusRetry, ErrorMsg: strPtr("rate limited")}, nil

	default:
		errMsg := fmt.Sprintf("unexpected status %d", resp.StatusCode)
		return &LoginResult{Status: StatusRetry, ErrorMsg: &errMsg}, nil
	}
}

// strPtr returns a pointer to a string literal.
func strPtr(s string) *string {
	return &s
}
