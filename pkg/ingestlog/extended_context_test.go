// Package ingestlog tests for extended context validation
package ingestlog

import (
	"encoding/json"
	"testing"
	"time"
)

// TestLogEntry_ExtendedContextValidation verifies that logEntry accepts
// entries with extended context (UserID, SessionID, RequestID) populated
// even when basic fields (Email, GithubUsername) are empty.
// This is the acceptance criteria for bead cg-43bpa.
func TestLogEntry_ExtendedContextValidation(t *testing.T) {
	logger := NewLogger()

	tests := []struct {
		name        string
		entry       LogEntry
		wantErr     bool
		errContains string
	}{
		{
			name: "extended context with UserID only - should pass",
			entry: LogEntry{
				User: UserContext{
					UserID: "user-123",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "extended context with SessionID only - should pass",
			entry: LogEntry{
				User: UserContext{
					SessionID: "session-456",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "extended context with RequestID only - should pass",
			entry: LogEntry{
				User: UserContext{
					RequestID: "request-789",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "extended context with all fields - should pass",
			entry: LogEntry{
				User: UserContext{
					UserID:    "user-123",
					SessionID: "session-456",
					RequestID: "request-789",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "mixed context - extended populated, basic empty - should pass",
			entry: LogEntry{
				User: UserContext{
					UserID:        "user-123",
					SessionID:     "session-456",
					Email:         "",
					GithubUsername: "",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "basic context with email only - should pass",
			entry: LogEntry{
				User: UserContext{
					Email: "user@example.com",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "basic context with github username only - should pass",
			entry: LogEntry{
				User: UserContext{
					GithubUsername: "octocat",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "both basic and extended context - should pass",
			entry: LogEntry{
				User: UserContext{
					UserID:        "user-123",
					Email:         "user@example.com",
					GithubUsername: "octocat",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "no user identification - should fail",
			entry: LogEntry{
				User: UserContext{
					UserID:        "",
					SessionID:     "",
					RequestID:     "",
					Email:         "",
					GithubUsername: "",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 1,
				},
			},
			wantErr:     true,
			errContains: "user identification required",
		},
		{
			name: "missing endpoint URL - should fail",
			entry: LogEntry{
				User: UserContext{
					UserID: "user-123",
				},
				Endpoint: EndpointContext{
					URL:           "",
					AttemptNumber: 1,
				},
			},
			wantErr:     true,
			errContains: "endpoint.url",
		},
		{
			name: "missing attempt number - should fail",
			entry: LogEntry{
				User: UserContext{
					UserID: "user-123",
				},
				Endpoint: EndpointContext{
					URL:           "http://test:8080/resolve",
					AttemptNumber: 0,
				},
			},
			wantErr:     true,
			errContains: "endpoint.attempt_number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := logger.LogRetryWithEntry(&tt.entry)

			if (err != nil) != tt.wantErr {
				t.Errorf("LogRetryWithEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" && err != nil {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

// TestLogEntry_ExtendedContextFullEntry verifies that entries with extended
// context can be fully logged and serialized correctly.
func TestLogEntry_ExtendedContextFullEntry(t *testing.T) {
	logger := NewLogger()

	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		EventType: "retry",
		User: UserContext{
			UserID:        "user-abc-123",
			SessionID:     "session-xyz-789",
			RequestID:     "req-def-456",
			Email:         "user@example.com",
			GithubUsername: "octocat",
		},
		Endpoint: EndpointContext{
			URL:           "http://queue-api:8080/email-resolution/resolve",
			AttemptNumber: 2,
			StatusCode:    500,
			ResponseBody:  `{"error": "internal server error"}`,
		},
		Error: ErrorContext{
			Type:       "server_error",
			Message:    "internal server error",
			StackTrace: "stacktrace here",
		},
		MaxRetries:      4,
		RetryDelayMs:    100,
		TotalDurationMs: 250,
	}

	// Try to log the entry
	err := logger.LogRetryWithEntry(&entry)
	if err != nil {
		t.Fatalf("LogRetryWithEntry failed: %v", err)
	}

	// Verify event type was set
	if entry.EventType != "retry" {
		t.Errorf("EventType = %q, want 'retry'", entry.EventType)
	}

	// Verify timestamp was set
	if entry.Timestamp.IsZero() {
		t.Errorf("Timestamp is zero, want non-zero")
	}

	// Test JSON serialization
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Verify JSON contains extended context fields
	jsonStr := string(jsonBytes)
	if !contains(jsonStr, "user-abc-123") {
		t.Errorf("JSON output does not contain UserID")
	}
	if !contains(jsonStr, "session-xyz-789") {
		t.Errorf("JSON output does not contain SessionID")
	}
	if !contains(jsonStr, "req-def-456") {
		t.Errorf("JSON output does not contain RequestID")
	}

	// Verify unmarshaling preserves extended context
	var unmarshaled LogEntry
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.User.UserID != "user-abc-123" {
		t.Errorf("UserID not preserved: got %q, want 'user-abc-123'", unmarshaled.User.UserID)
	}
	if unmarshaled.User.SessionID != "session-xyz-789" {
		t.Errorf("SessionID not preserved: got %q, want 'session-xyz-789'", unmarshaled.User.SessionID)
	}
	if unmarshaled.User.RequestID != "req-def-456" {
		t.Errorf("RequestID not preserved: got %q, want 'req-def-456'", unmarshaled.User.RequestID)
	}
}
