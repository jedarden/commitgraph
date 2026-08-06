// Package ingestlog tests the structured logging functionality.
package ingestlog

import (
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

// TestEventFromError verifies that EventFromError correctly extracts error context.
func TestEventFromError(t *testing.T) {
	tests := []struct {
		name           string
		email          string
		githubUsername string
		endpointURL    string
		err            error
		statusCode     int
		responseBody   string
		attemptNumber  int
		maxRetries     int
		retryDelayMs   int
		totalDurationMs int64
		wantErrorType  string
	}{
		{
			name:           "timeout error",
			email:          "user@example.com",
			githubUsername: "octocat",
			endpointURL:    "http://queue-api:8080/email-resolution/resolve",
			err:            errors.New("context deadline exceeded"),
			statusCode:     0,
			responseBody:   "",
			attemptNumber:  1,
			maxRetries:     4,
			retryDelayMs:   100,
			totalDurationMs: 150,
			wantErrorType:  "timeout",
		},
		{
			name:           "network error - connection refused",
			email:          "user@example.com",
			githubUsername: "octocat",
			endpointURL:    "http://queue-api:8080/email-resolution/resolve",
			err:            &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			statusCode:     0,
			responseBody:   "",
			attemptNumber:  2,
			maxRetries:     4,
			retryDelayMs:   400,
			totalDurationMs: 550,
			wantErrorType:  "network",
		},
		{
			name:           "client error 404",
			email:          "user@example.com",
			githubUsername: "octocat",
			endpointURL:    "http://queue-api:8080/email-resolution/resolve",
			err:            errors.New("server returned non-retryable status 404"),
			statusCode:     404,
			responseBody:   `{"error": "not found"}`,
			attemptNumber:  3,
			maxRetries:     4,
			retryDelayMs:   0,
			totalDurationMs: 1200,
			wantErrorType:  "client_error",
		},
		{
			name:           "server error 500",
			email:          "user@example.com",
			githubUsername: "octocat",
			endpointURL:    "http://queue-api:8080/email-resolution/resolve",
			err:            errors.New("server returned retryable status 500"),
			statusCode:     500,
			responseBody:   `{"error": "internal server error"}`,
			attemptNumber:  1,
			maxRetries:     4,
			retryDelayMs:   100,
			totalDurationMs: 50,
			wantErrorType:  "server_error",
		},
		{
			name:           "parse error",
			email:          "user@example.com",
			githubUsername: "octocat",
			endpointURL:    "http://queue-api:8080/email-resolution/resolve",
			err:            errors.New("invalid character '<' looking for beginning of value"),
			statusCode:     200,
			responseBody:   `<html>not json</html>`,
			attemptNumber:  1,
			maxRetries:     4,
			retryDelayMs:   0,
			totalDurationMs: 20,
			wantErrorType:  "parse_error",
		},
		{
			name:           "unknown error",
			email:          "user@example.com",
			githubUsername: "octocat",
			endpointURL:    "http://queue-api:8080/email-resolution/resolve",
			err:            errors.New("something unexpected happened"),
			statusCode:     0,
			responseBody:   "",
			attemptNumber:  1,
			maxRetries:     4,
			retryDelayMs:   0,
			totalDurationMs: 10,
			wantErrorType:  "unknown",
		},
		{
			name:           "nil error",
			email:          "user@example.com",
			githubUsername: "octocat",
			endpointURL:    "http://queue-api:8080/email-resolution/resolve",
			err:            nil,
			statusCode:     200,
			responseBody:   `{"success": true}`,
			attemptNumber:  1,
			maxRetries:     4,
			retryDelayMs:   0,
			totalDurationMs: 5,
			wantErrorType:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := EventFromError(
				tt.email,
				tt.githubUsername,
				tt.endpointURL,
				tt.err,
				tt.statusCode,
				tt.responseBody,
				tt.attemptNumber,
				tt.maxRetries,
				tt.retryDelayMs,
				tt.totalDurationMs,
			)

			// Verify required fields are populated
			if event.Email != tt.email {
				t.Errorf("Email = %q, want %q", event.Email, tt.email)
			}
			if event.GithubUsername != tt.githubUsername {
				t.Errorf("GithubUsername = %q, want %q", event.GithubUsername, tt.githubUsername)
			}
			if event.EndpointURL != tt.endpointURL {
				t.Errorf("EndpointURL = %q, want %q", event.EndpointURL, tt.endpointURL)
			}

			// Verify retry context
			if event.AttemptNumber != tt.attemptNumber {
				t.Errorf("AttemptNumber = %d, want %d", event.AttemptNumber, tt.attemptNumber)
			}
			if event.MaxRetries != tt.maxRetries {
				t.Errorf("MaxRetries = %d, want %d", event.MaxRetries, tt.maxRetries)
			}
			if event.RetryDelayMs != tt.retryDelayMs {
				t.Errorf("RetryDelayMs = %d, want %d", event.RetryDelayMs, tt.retryDelayMs)
			}
			if event.TotalDurationMs != tt.totalDurationMs {
				t.Errorf("TotalDurationMs = %d, want %d", event.TotalDurationMs, tt.totalDurationMs)
			}

			// Verify endpoint context
			if event.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", event.StatusCode, tt.statusCode)
			}
			if event.ResponseBody != tt.responseBody {
				t.Errorf("ResponseBody = %q, want %q", event.ResponseBody, tt.responseBody)
			}

			// Verify error serialization
			if event.ErrorType != tt.wantErrorType {
				t.Errorf("ErrorType = %q, want %q", event.ErrorType, tt.wantErrorType)
			}
			if tt.err != nil && event.ErrorMessage == "" {
				t.Errorf("ErrorMessage is empty for non-nil error")
			}
			if tt.err != nil && event.Stacktrace == "" {
				t.Errorf("Stacktrace is empty for non-nil error")
			}
			if tt.err == nil && event.ErrorMessage != "" {
				t.Errorf("ErrorMessage = %q for nil error, want empty", event.ErrorMessage)
			}

			// Verify timestamp is set
			if event.Timestamp.IsZero() {
				t.Errorf("Timestamp is zero, want non-zero")
			}
			// Verify timestamp is recent (within last minute)
			if time.Since(event.Timestamp) > time.Minute {
				t.Errorf("Timestamp is too old: %v", event.Timestamp)
			}
		})
	}
}

// TestClassifyError verifies error classification logic.
func TestClassifyError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		wantType   string
	}{
		{
			name:       "timeout error",
			err:        errors.New("context deadline exceeded"),
			statusCode: 0,
			wantType:   "timeout",
		},
		{
			name:       "network connection refused",
			err:        errors.New("connection refused"),
			statusCode: 0,
			wantType:   "network",
		},
		{
			name:       "network connection reset",
			err:        errors.New("connection reset by peer"),
			statusCode: 0,
			wantType:   "network",
		},
		{
			name:       "DNS error",
			err:        errors.New("no such host"),
			statusCode: 0,
			wantType:   "network",
		},
		{
			name:       "parse error invalid",
			err:        errors.New("invalid JSON"),
			statusCode: 0,
			wantType:   "parse_error",
		},
		{
			name:       "parse error unmarshal",
			err:        errors.New("cannot unmarshal"),
			statusCode: 0,
			wantType:   "parse_error",
		},
		{
			name:       "client error from status code",
			err:        errors.New("some error"),
			statusCode: 404,
			wantType:   "client_error",
		},
		{
			name:       "server error from status code",
			err:        errors.New("some error"),
			statusCode: 503,
			wantType:   "server_error",
		},
		{
			name:       "unknown error",
			err:        errors.New("something unexpected"),
			statusCode: 0,
			wantType:   "unknown",
		},
		{
			name:       "nil error",
			err:        nil,
			statusCode: 0,
			wantType:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType := classifyError(tt.err, tt.statusCode)
			if gotType != tt.wantType {
				t.Errorf("classifyError() = %q, want %q", gotType, tt.wantType)
			}
		})
	}
}

// TestLogRetry verifies LogRetry creates retry events.
func TestLogRetry(t *testing.T) {
	logger := NewLogger()
	event := Event{
		Email:          "test@example.com",
		GithubUsername: "testuser",
		EndpointURL:    "http://test:8080/resolve",
		AttemptNumber:  1,
		MaxRetries:     4,
		StatusCode:     429,
		ErrorType:      "client_error",
		ErrorMessage:   "rate limit exceeded",
		RetryDelayMs:   1000,
	}

	err := logger.LogRetry(event)
	if err != nil {
		t.Fatalf("LogRetry failed: %v", err)
	}

	if event.EventType != "retry" {
		t.Errorf("EventType = %q, want 'retry'", event.EventType)
	}
}

// TestLogFailure verifies LogFailure creates failure events.
func TestLogFailure(t *testing.T) {
	logger := NewLogger()
	event := Event{
		Email:           "test@example.com",
		GithubUsername:  "testuser",
		EndpointURL:     "http://test:8080/resolve",
		AttemptNumber:   4,
		MaxRetries:      4,
		StatusCode:      500,
		ErrorType:       "server_error",
		ErrorMessage:    "internal server error",
		TotalDurationMs: 5000,
	}

	err := logger.LogFailure(event)
	if err != nil {
		t.Fatalf("LogFailure failed: %v", err)
	}

	if event.EventType != "failure" {
		t.Errorf("EventType = %q, want 'failure'", event.EventType)
	}
}

// TestLogEventValidation verifies required field validation.
func TestLogEventValidation(t *testing.T) {
	logger := NewLogger()

	tests := []struct {
		name        string
		event       Event
		wantErr     bool
		errContains string
	}{
		{
			name: "missing email",
			event: Event{
				GithubUsername: "testuser",
				EndpointURL:    "http://test:8080/resolve",
			},
			wantErr:     true,
			errContains: "email",
		},
		{
			name: "missing github username",
			event: Event{
				Email:       "test@example.com",
				EndpointURL: "http://test:8080/resolve",
			},
			wantErr:     true,
			errContains: "github_username",
		},
		{
			name: "missing endpoint URL",
			event: Event{
				Email:          "test@example.com",
				GithubUsername: "testuser",
			},
			wantErr:     true,
			errContains: "endpoint_url",
		},
		{
			name: "valid event",
			event: Event{
				Email:          "test@example.com",
				GithubUsername: "testuser",
				EndpointURL:    "http://test:8080/resolve",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := logger.LogRetry(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("LogRetry error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContains != "" && err != nil {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// TestEventTimestamp verifies timestamp auto-population.
func TestEventTimestamp(t *testing.T) {
	logger := NewLogger()
	before := time.Now().UTC()

	event := Event{
		Email:          "test@example.com",
		GithubUsername: "testuser",
		EndpointURL:    "http://test:8080/resolve",
		// Timestamp is zero - should be auto-populated
	}

	err := logger.LogSuccess(event)
	if err != nil {
		t.Fatalf("LogSuccess failed: %v", err)
	}

	if event.Timestamp.IsZero() {
		t.Errorf("Timestamp was not auto-populated, is zero")
	}
	if event.Timestamp.Before(before) {
		t.Errorf("Timestamp %v is before test start %v", event.Timestamp, before)
	}
}

// TestLogRetryInline verifies inline retry logging.
func TestLogRetryInline(t *testing.T) {
	// This test just verifies it doesn't panic - actual output goes to stderr
	LogRetryInline(
		"test@example.com",
		"testuser",
		"http://test:8080/resolve",
		1, 4,
		429, "rate limited",
		"client_error", "rate limit exceeded",
		"stacktrace here",
		1000, 1500,
	)
}

// TestLogFailureInline verifies inline failure logging.
func TestLogFailureInline(t *testing.T) {
	// This test just verifies it doesn't panic - actual output goes to stderr
	LogFailureInline(
		"test@example.com",
		"testuser",
		"http://test:8080/resolve",
		4, 4,
		500, "internal error",
		"server_error", "server error",
		"stacktrace here",
		5000,
	)
}

// TestLogEntryStructure verifies the LogEntry struct has all required fields.
func TestLogEntryStructure(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		EventType: "retry",
		User: UserContext{
			Email:          "test@example.com",
			GithubUsername: "testuser",
		},
		Endpoint: EndpointContext{
			URL:           "http://test:8080/resolve",
			AttemptNumber: 1,
			StatusCode:    429,
			ResponseBody:  `{"error": "rate limit"}`,
		},
		Error: ErrorContext{
			Type:       "client_error",
			Message:    "rate limit exceeded",
			StackTrace: "stacktrace here",
		},
		MaxRetries:      4,
		RetryDelayMs:    1000,
		TotalDurationMs: 1500,
	}

	// Verify LogEntry top-level fields
	if entry.EventType != "retry" {
		t.Errorf("EventType = %q, want 'retry'", entry.EventType)
	}
	if entry.MaxRetries != 4 {
		t.Errorf("MaxRetries = %d, want 4", entry.MaxRetries)
	}
	if entry.RetryDelayMs != 1000 {
		t.Errorf("RetryDelayMs = %d, want 1000", entry.RetryDelayMs)
	}
	if entry.TotalDurationMs != 1500 {
		t.Errorf("TotalDurationMs = %d, want 1500", entry.TotalDurationMs)
	}
}

// TestErrorContextStructure verifies ErrorContext has all required fields.
func TestErrorContextStructure(t *testing.T) {
	errorCtx := ErrorContext{
		Type:       "timeout",
		Message:    "context deadline exceeded",
		StackTrace: "stacktrace content",
	}

	// Verify ErrorContext fields
	if errorCtx.Type != "timeout" {
		t.Errorf("ErrorContext.Type = %q, want 'timeout'", errorCtx.Type)
	}
	if errorCtx.Message != "context deadline exceeded" {
		t.Errorf("ErrorContext.Message = %q, want 'context deadline exceeded'", errorCtx.Message)
	}
	if errorCtx.StackTrace != "stacktrace content" {
		t.Errorf("ErrorContext.StackTrace = %q, want 'stacktrace content'", errorCtx.StackTrace)
	}
}

// TestUserContextStructure verifies UserContext has all required fields.
func TestUserContextStructure(t *testing.T) {
	userCtx := UserContext{
		Email:          "user@example.com",
		GithubUsername: "octocat",
	}

	// Verify UserContext fields
	if userCtx.Email != "user@example.com" {
		t.Errorf("UserContext.Email = %q, want 'user@example.com'", userCtx.Email)
	}
	if userCtx.GithubUsername != "octocat" {
		t.Errorf("UserContext.GithubUsername = %q, want 'octocat'", userCtx.GithubUsername)
	}
}

// TestEndpointContextStructure verifies EndpointContext has all required fields.
func TestEndpointContextStructure(t *testing.T) {
	endpointCtx := EndpointContext{
		URL:           "http://queue-api:8080/email-resolution/resolve",
		AttemptNumber: 3,
		StatusCode:    500,
		ResponseBody:  `{"error": "internal server error"}`,
	}

	// Verify EndpointContext fields
	if endpointCtx.URL != "http://queue-api:8080/email-resolution/resolve" {
		t.Errorf("EndpointContext.URL = %q, want 'http://queue-api:8080/email-resolution/resolve'", endpointCtx.URL)
	}
	if endpointCtx.AttemptNumber != 3 {
		t.Errorf("EndpointContext.AttemptNumber = %d, want 3", endpointCtx.AttemptNumber)
	}
	if endpointCtx.StatusCode != 500 {
		t.Errorf("EndpointContext.StatusCode = %d, want 500", endpointCtx.StatusCode)
	}
	if endpointCtx.ResponseBody != `{"error": "internal server error"}` {
		t.Errorf("EndpointContext.ResponseBody = %q, want '{\"error\": \"internal server error\"}'", endpointCtx.ResponseBody)
	}
}

// TestLogEntryJSONSerialization verifies proper JSON tags on all structs.
func TestLogEntryJSONSerialization(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
		EventType: "failure",
		User: UserContext{
			Email:          "test@example.com",
			GithubUsername: "testuser",
		},
		Endpoint: EndpointContext{
			URL:           "http://test:8080/resolve",
			AttemptNumber: 4,
			StatusCode:    500,
			ResponseBody:  "error body",
		},
		Error: ErrorContext{
			Type:       "server_error",
			Message:    "internal error",
			StackTrace: "stacktrace",
		},
		MaxRetries:      4,
		RetryDelayMs:    0,
		TotalDurationMs: 5000,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal LogEntry: %v", err)
	}

	// Unmarshal back to struct
	var unmarshaled LogEntry
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal LogEntry: %v", err)
	}

	// Verify all fields are preserved
	if unmarshaled.User.Email != entry.User.Email {
		t.Errorf("User.Email not preserved: got %q, want %q", unmarshaled.User.Email, entry.User.Email)
	}
	if unmarshaled.User.GithubUsername != entry.User.GithubUsername {
		t.Errorf("User.GithubUsername not preserved: got %q, want %q", unmarshaled.User.GithubUsername, entry.User.GithubUsername)
	}
	if unmarshaled.Endpoint.URL != entry.Endpoint.URL {
		t.Errorf("Endpoint.URL not preserved: got %q, want %q", unmarshaled.Endpoint.URL, entry.Endpoint.URL)
	}
	if unmarshaled.Endpoint.AttemptNumber != entry.Endpoint.AttemptNumber {
		t.Errorf("Endpoint.AttemptNumber not preserved: got %d, want %d", unmarshaled.Endpoint.AttemptNumber, entry.Endpoint.AttemptNumber)
	}
	if unmarshaled.Endpoint.StatusCode != entry.Endpoint.StatusCode {
		t.Errorf("Endpoint.StatusCode not preserved: got %d, want %d", unmarshaled.Endpoint.StatusCode, entry.Endpoint.StatusCode)
	}
	if unmarshaled.Endpoint.ResponseBody != entry.Endpoint.ResponseBody {
		t.Errorf("Endpoint.ResponseBody not preserved: got %q, want %q", unmarshaled.Endpoint.ResponseBody, entry.Endpoint.ResponseBody)
	}
	if unmarshaled.Error.Type != entry.Error.Type {
		t.Errorf("Error.Type not preserved: got %q, want %q", unmarshaled.Error.Type, entry.Error.Type)
	}
	if unmarshaled.Error.Message != entry.Error.Message {
		t.Errorf("Error.Message not preserved: got %q, want %q", unmarshaled.Error.Message, entry.Error.Message)
	}
	if unmarshaled.Error.StackTrace != entry.Error.StackTrace {
		t.Errorf("Error.StackTrace not preserved: got %q, want %q", unmarshaled.Error.StackTrace, entry.Error.StackTrace)
	}
}

// TestLogEntryFromError verifies LogEntryFromError creates correct LogEntry.
func TestLogEntryFromError(t *testing.T) {
	err := errors.New("context deadline exceeded")
	entry := LogEntryFromError(
		"user@example.com",
		"octocat",
		"http://queue-api:8080/email-resolution/resolve",
		err,
		0,
		"",
		2,
		4,
		200,
		350,
	)

	// Verify UserContext
	if entry.User.Email != "user@example.com" {
		t.Errorf("User.Email = %q, want 'user@example.com'", entry.User.Email)
	}
	if entry.User.GithubUsername != "octocat" {
		t.Errorf("User.GithubUsername = %q, want 'octocat'", entry.User.GithubUsername)
	}

	// Verify EndpointContext
	if entry.Endpoint.URL != "http://queue-api:8080/email-resolution/resolve" {
		t.Errorf("Endpoint.URL = %q, want 'http://queue-api:8080/email-resolution/resolve'", entry.Endpoint.URL)
	}
	if entry.Endpoint.AttemptNumber != 2 {
		t.Errorf("Endpoint.AttemptNumber = %d, want 2", entry.Endpoint.AttemptNumber)
	}

	// Verify ErrorContext
	if entry.Error.Type != "timeout" {
		t.Errorf("Error.Type = %q, want 'timeout'", entry.Error.Type)
	}
	if entry.Error.Message == "" {
		t.Errorf("Error.Message is empty, want non-empty")
	}
	if entry.Error.StackTrace == "" {
		t.Errorf("Error.StackTrace is empty, want non-empty")
	}

	// Verify metadata
	if entry.MaxRetries != 4 {
		t.Errorf("MaxRetries = %d, want 4", entry.MaxRetries)
	}
	if entry.RetryDelayMs != 200 {
		t.Errorf("RetryDelayMs = %d, want 200", entry.RetryDelayMs)
	}
	if entry.TotalDurationMs != 350 {
		t.Errorf("TotalDurationMs = %d, want 350", entry.TotalDurationMs)
	}

	// Verify timestamp is set
	if entry.Timestamp.IsZero() {
		t.Errorf("Timestamp is zero, want non-zero")
	}
}

// TestLogRetryWithEntry verifies LogRetryWithEntry creates retry events.
func TestLogRetryWithEntry(t *testing.T) {
	logger := NewLogger()
	entry := LogEntry{
		User: UserContext{
			Email:          "test@example.com",
			GithubUsername: "testuser",
		},
		Endpoint: EndpointContext{
			URL:           "http://test:8080/resolve",
			AttemptNumber: 1,
			StatusCode:    429,
		},
		Error: ErrorContext{
			Type:    "client_error",
			Message: "rate limit exceeded",
		},
		MaxRetries:   4,
		RetryDelayMs: 1000,
	}

	err := logger.LogRetryWithEntry(&entry)
	if err != nil {
		t.Fatalf("LogRetryWithEntry failed: %v", err)
	}

	if entry.EventType != "retry" {
		t.Errorf("EventType = %q, want 'retry'", entry.EventType)
	}
}

// TestLogFailureWithEntry verifies LogFailureWithEntry creates failure events.
func TestLogFailureWithEntry(t *testing.T) {
	logger := NewLogger()
	entry := LogEntry{
		User: UserContext{
			Email:          "test@example.com",
			GithubUsername: "testuser",
		},
		Endpoint: EndpointContext{
			URL:           "http://test:8080/resolve",
			AttemptNumber: 4,
			StatusCode:    500,
		},
		Error: ErrorContext{
			Type:    "server_error",
			Message: "internal server error",
		},
		MaxRetries:      4,
		TotalDurationMs: 5000,
	}

	err := logger.LogFailureWithEntry(&entry)
	if err != nil {
		t.Fatalf("LogFailureWithEntry failed: %v", err)
	}

	if entry.EventType != "failure" {
		t.Errorf("EventType = %q, want 'failure'", entry.EventType)
	}
}

// TestLogEntryValidation verifies required field validation for LogEntry.
func TestLogEntryValidation(t *testing.T) {
	logger := NewLogger()

	tests := []struct {
		name        string
		entry       LogEntry
		wantErr     bool
		errContains string
	}{
		{
			name: "missing user email",
			entry: LogEntry{
				User: UserContext{
					GithubUsername: "testuser",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr:     true,
			errContains: "user.email",
		},
		{
			name: "missing github username",
			entry: LogEntry{
				User: UserContext{
					Email: "test@example.com",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr:     true,
			errContains: "user.github_username",
		},
		{
			name: "missing endpoint URL",
			entry: LogEntry{
				User: UserContext{
					Email:          "test@example.com",
					GithubUsername: "testuser",
				},
				Endpoint: EndpointContext{
					AttemptNumber: 1,
				},
			},
			wantErr:     true,
			errContains: "endpoint.url",
		},
		{
			name: "missing attempt number",
			entry: LogEntry{
				User: UserContext{
					Email:          "test@example.com",
					GithubUsername: "testuser",
				},
				Endpoint: EndpointContext{
					URL: "http://test:8080/resolve",
				},
			},
			wantErr:     true,
			errContains: "endpoint.attempt_number",
		},
		{
			name: "valid entry",
			entry: LogEntry{
				User: UserContext{
					Email:          "test@example.com",
					GithubUsername: "testuser",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := logger.LogRetryWithEntry(&tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("LogRetryWithEntry error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContains != "" && err != nil {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// TestEventToLogEntryConversion verifies conversion between Event and LogEntry.
func TestEventToLogEntryConversion(t *testing.T) {
	event := Event{
		Timestamp:       time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
		EventType:       "retry",
		Email:           "user@example.com",
		GithubUsername:  "octocat",
		EndpointURL:     "http://test:8080/resolve",
		AttemptNumber:   2,
		MaxRetries:      4,
		StatusCode:      429,
		ResponseBody:    `{"error": "rate limit"}`,
		ErrorType:       "client_error",
		ErrorMessage:    "rate limit exceeded",
		Stacktrace:      "stacktrace here",
		RetryDelayMs:    1000,
		TotalDurationMs: 1500,
	}

	// Convert Event to LogEntry
	entry := event.ToLogEntry()

	// Verify all fields are correctly mapped
	if entry.User.Email != event.Email {
		t.Errorf("User.Email = %q, want %q", entry.User.Email, event.Email)
	}
	if entry.User.GithubUsername != event.GithubUsername {
		t.Errorf("User.GithubUsername = %q, want %q", entry.User.GithubUsername, event.GithubUsername)
	}
	if entry.Endpoint.URL != event.EndpointURL {
		t.Errorf("Endpoint.URL = %q, want %q", entry.Endpoint.URL, event.EndpointURL)
	}
	if entry.Endpoint.AttemptNumber != event.AttemptNumber {
		t.Errorf("Endpoint.AttemptNumber = %d, want %d", entry.Endpoint.AttemptNumber, event.AttemptNumber)
	}
	if entry.Endpoint.StatusCode != event.StatusCode {
		t.Errorf("Endpoint.StatusCode = %d, want %d", entry.Endpoint.StatusCode, event.StatusCode)
	}
	if entry.Endpoint.ResponseBody != event.ResponseBody {
		t.Errorf("Endpoint.ResponseBody = %q, want %q", entry.Endpoint.ResponseBody, event.ResponseBody)
	}
	if entry.Error.Type != event.ErrorType {
		t.Errorf("Error.Type = %q, want %q", entry.Error.Type, event.ErrorType)
	}
	if entry.Error.Message != event.ErrorMessage {
		t.Errorf("Error.Message = %q, want %q", entry.Error.Message, event.ErrorMessage)
	}
	if entry.Error.StackTrace != event.Stacktrace {
		t.Errorf("Error.StackTrace = %q, want %q", entry.Error.StackTrace, event.Stacktrace)
	}
	if entry.MaxRetries != event.MaxRetries {
		t.Errorf("MaxRetries = %d, want %d", entry.MaxRetries, event.MaxRetries)
	}
	if entry.RetryDelayMs != event.RetryDelayMs {
		t.Errorf("RetryDelayMs = %d, want %d", entry.RetryDelayMs, event.RetryDelayMs)
	}
	if entry.TotalDurationMs != event.TotalDurationMs {
		t.Errorf("TotalDurationMs = %d, want %d", entry.TotalDurationMs, event.TotalDurationMs)
	}

	// Convert back to Event
	recoveredEvent := entry.ToEvent()

	// Verify round-trip preserves all fields
	if recoveredEvent.Email != event.Email {
		t.Errorf("Round-trip Email = %q, want %q", recoveredEvent.Email, event.Email)
	}
	if recoveredEvent.GithubUsername != event.GithubUsername {
		t.Errorf("Round-trip GithubUsername = %q, want %q", recoveredEvent.GithubUsername, event.GithubUsername)
	}
	if recoveredEvent.EndpointURL != event.EndpointURL {
		t.Errorf("Round-trip EndpointURL = %q, want %q", recoveredEvent.EndpointURL, event.EndpointURL)
	}
}

// TestCaptureUserContext verifies CaptureUserContext creates valid UserContext.
func TestCaptureUserContext(t *testing.T) {
	tests := []struct {
		name           string
		email          string
		githubUsername string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid user context",
			email:          "user@example.com",
			githubUsername: "octocat",
			wantErr:        false,
		},
		{
			name:           "empty email",
			email:          "",
			githubUsername: "octocat",
			wantErr:        true,
			errContains:    "email",
		},
		{
			name:           "empty github username",
			email:          "user@example.com",
			githubUsername: "",
			wantErr:        true,
			errContains:    "github_username",
		},
		{
			name:           "both empty",
			email:          "",
			githubUsername: "",
			wantErr:        true,
			errContains:    "email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userCtx, err := CaptureUserContext(tt.email, tt.githubUsername)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CaptureUserContext expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("CaptureUserContext unexpected error: %v", err)
			}

			// Verify UserContext fields are populated
			if userCtx.Email != tt.email {
				t.Errorf("UserContext.Email = %q, want %q", userCtx.Email, tt.email)
			}
			if userCtx.GithubUsername != tt.githubUsername {
				t.Errorf("UserContext.GithubUsername = %q, want %q", userCtx.GithubUsername, tt.githubUsername)
			}
		})
	}
}

// TestCaptureEndpointContext verifies CaptureEndpointContext creates valid EndpointContext.
func TestCaptureEndpointContext(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		attemptNumber int
		statusCode    int
		responseBody  string
		wantErr       bool
		errContains   string
	}{
		{
			name:          "valid endpoint context with status code",
			url:           "http://queue-api:8080/email-resolution/resolve",
			attemptNumber: 1,
			statusCode:    200,
			responseBody:  `{"success": true}`,
			wantErr:       false,
		},
		{
			name:          "valid endpoint context without status code",
			url:           "http://queue-api:8080/email-resolution/resolve",
			attemptNumber: 2,
			statusCode:    0,
			responseBody:  "",
			wantErr:       false,
		},
		{
			name:          "empty url",
			url:           "",
			attemptNumber: 1,
			statusCode:    200,
			responseBody:  "response",
			wantErr:       true,
			errContains:  "url",
		},
		{
			name:          "zero attempt number",
			url:           "http://queue-api:8080/resolve",
			attemptNumber: 0,
			statusCode:    200,
			responseBody:  "response",
			wantErr:       true,
			errContains:  "attempt_number",
		},
		{
			name:          "negative attempt number",
			url:           "http://queue-api:8080/resolve",
			attemptNumber: -1,
			statusCode:    200,
			responseBody:  "response",
			wantErr:       true,
			errContains:  "attempt_number",
		},
		{
			name:          "large response body gets truncated",
			url:           "http://queue-api:8080/resolve",
			attemptNumber: 1,
			statusCode:    500,
			responseBody:  string(make([]byte, 11*1024)), // 11KB response
			wantErr:       false,
		},
		{
			name:          "exactly 10KB response body",
			url:           "http://queue-api:8080/resolve",
			attemptNumber: 1,
			statusCode:    200,
			responseBody:  string(make([]byte, 10*1024)), // Exactly 10KB
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointCtx, err := CaptureEndpointContext(tt.url, tt.attemptNumber, tt.statusCode, tt.responseBody)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CaptureEndpointContext expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("CaptureEndpointContext unexpected error: %v", err)
			}

			// Verify EndpointContext fields are populated
			if endpointCtx.URL != tt.url {
				t.Errorf("EndpointContext.URL = %q, want %q", endpointCtx.URL, tt.url)
			}
			if endpointCtx.AttemptNumber != tt.attemptNumber {
				t.Errorf("EndpointContext.AttemptNumber = %d, want %d", endpointCtx.AttemptNumber, tt.attemptNumber)
			}
			if endpointCtx.StatusCode != tt.statusCode {
				t.Errorf("EndpointContext.StatusCode = %d, want %d", endpointCtx.StatusCode, tt.statusCode)
			}

			// Verify response body truncation
			if tt.name == "large response body gets truncated" {
				if len(endpointCtx.ResponseBody) >= len(tt.responseBody) {
					t.Errorf("Response body was not truncated: got %d bytes, want < %d bytes",
						len(endpointCtx.ResponseBody), len(tt.responseBody))
				}
				if !contains(endpointCtx.ResponseBody, "... (truncated)") {
					t.Errorf("Truncated response body does not contain truncation marker")
				}
			}
			if tt.name == "exactly 10KB response body" {
				if len(endpointCtx.ResponseBody) != len(tt.responseBody) {
					t.Errorf("Response body at exactly 10KB should not be truncated: got %d bytes, want %d bytes",
						len(endpointCtx.ResponseBody), len(tt.responseBody))
				}
			}
		})
	}
}

// TestCaptureContextIntegration verifies integration of capture functions with LogEntry.
func TestCaptureContextIntegration(t *testing.T) {
	email := "user@example.com"
	githubUsername := "octocat"
	url := "http://queue-api:8080/email-resolution/resolve"
	attemptNumber := 2
	statusCode := 500
	responseBody := `{"error": "internal server error"}`

	// Capture contexts
	userCtx, err := CaptureUserContext(email, githubUsername)
	if err != nil {
		t.Fatalf("CaptureUserContext failed: %v", err)
	}

	endpointCtx, err := CaptureEndpointContext(url, attemptNumber, statusCode, responseBody)
	if err != nil {
		t.Fatalf("CaptureEndpointContext failed: %v", err)
	}

	// Create LogEntry using captured contexts
	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		EventType: "retry",
		User:      userCtx,
		Endpoint:  endpointCtx,
		Error: ErrorContext{
			Type:    "server_error",
			Message: "internal server error",
		},
		MaxRetries:      4,
		RetryDelayMs:    200,
		TotalDurationMs: 350,
	}

	// Verify LogEntry is properly populated
	if entry.User.Email != email {
		t.Errorf("User.Email = %q, want %q", entry.User.Email, email)
	}
	if entry.User.GithubUsername != githubUsername {
		t.Errorf("User.GithubUsername = %q, want %q", entry.User.GithubUsername, githubUsername)
	}
	if entry.Endpoint.URL != url {
		t.Errorf("Endpoint.URL = %q, want %q", entry.Endpoint.URL, url)
	}
	if entry.Endpoint.AttemptNumber != attemptNumber {
		t.Errorf("Endpoint.AttemptNumber = %d, want %d", entry.Endpoint.AttemptNumber, attemptNumber)
	}
	if entry.Endpoint.StatusCode != statusCode {
		t.Errorf("Endpoint.StatusCode = %d, want %d", entry.Endpoint.StatusCode, statusCode)
	}
	if entry.Endpoint.ResponseBody != responseBody {
		t.Errorf("Endpoint.ResponseBody = %q, want %q", entry.Endpoint.ResponseBody, responseBody)
	}

	// Verify LogEntry can be logged
	logger := NewLogger()
	err = logger.LogRetryWithEntry(&entry)
	if err != nil {
		t.Errorf("LogRetryWithEntry failed: %v", err)
	}
}
