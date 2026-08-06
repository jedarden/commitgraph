// Package ingestlog provides structured logging for ingest endpoint operations.
package ingestlog

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"time"
)

// Logger writes structured logs for ingest endpoint operations and errors.
type Logger struct {
	output *log.Logger
}

// NewLogger creates a new ingest logger that writes to stderr (or a configured output).
func NewLogger() *Logger {
	return &Logger{
		output: log.New(os.Stderr, "[INGEST-LOG] ", log.LstdFlags|log.Lmicroseconds|log.LUTC),
	}
}

// NewLoggerWithOutput creates a new ingest logger with a custom output writer.
func NewLoggerWithOutput(output *log.Logger) *Logger {
	return &Logger{
		output: output,
	}
}

// ErrorContext contains error details for ingest operations.
type ErrorContext struct {
	Type        string `json:"type"`                  // Type of error (network, timeout, client_error, server_error, parse_error, unknown)
	Message     string `json:"message"`               // Human-readable error message
	StackTrace  string `json:"stack_trace,omitempty"` // Stack trace captured at error time
}

// UserContext contains user identification information.
type UserContext struct {
	Email          string `json:"email"`           // User's email address being resolved
	GithubUsername string `json:"github_username"` // Target GitHub username for resolution
}

// EndpointContext contains HTTP endpoint interaction details.
type EndpointContext struct {
	URL           string `json:"url"`                            // Full HTTP endpoint URL being called
	AttemptNumber int    `json:"attempt_number"`                 // Current retry attempt (1-based)
	StatusCode    int    `json:"status_code,omitempty"`          // HTTP status code received
	ResponseBody  string `json:"response_body,omitempty"`        // Response body content (if available)
}

// LogEntry represents a structured ingest log entry with nested contexts.
type LogEntry struct {
	Timestamp       time.Time       `json:"timestamp"`                   // UTC timestamp when the event occurred
	EventType       string          `json:"event_type"`                  // "retry", "failure", or "success"
	User            UserContext     `json:"user"`                        // User identification context
	Endpoint        EndpointContext `json:"endpoint"`                    // Endpoint interaction context
	Error           ErrorContext    `json:"error,omitempty"`            // Error details (if applicable)
	MaxRetries      int             `json:"max_retries"`                 // Maximum number of retry attempts configured
	RetryDelayMs    int             `json:"retry_delay_ms,omitempty"`   // Delay before next retry in milliseconds
	TotalDurationMs int64           `json:"total_duration_ms,omitempty"` // Total time spent attempting in milliseconds
}

// Event represents a single ingest log event.
type Event struct {
	Timestamp       time.Time `json:"timestamp"`
	EventType       string    `json:"event_type"` // "retry" or "failure"
	Email           string    `json:"email"`
	GithubUsername  string    `json:"github_username"`
	EndpointURL     string    `json:"endpoint_url"`
	AttemptNumber   int       `json:"attempt_number"`
	MaxRetries      int       `json:"max_retries"`
	StatusCode      int       `json:"status_code,omitempty"`         // HTTP status code if available
	ResponseBody    string    `json:"response_body,omitempty"`       // Response body if available
	ErrorType       string    `json:"error_type,omitempty"`          // Type of error (network, timeout, client_error, server_error)
	ErrorMessage    string    `json:"error_message,omitempty"`       // Error message
	Stacktrace      string    `json:"stacktrace,omitempty"`          // Stack trace if available
	RetryDelayMs    int       `json:"retry_delay_ms,omitempty"`      // Delay before next retry in milliseconds
	TotalDurationMs int64     `json:"total_duration_ms,omitempty"`   // Total time spent attempting
}

// LogRetry logs a retry attempt with full context.
func (l *Logger) LogRetry(event Event) error {
	event.EventType = "retry"
	return l.logEvent(event)
}

// LogFailure logs a final failure after all retries exhausted.
func (l *Logger) LogFailure(event Event) error {
	event.EventType = "failure"
	return l.logEvent(event)
}

// LogSuccess logs a successful resolution (optional, for debugging).
func (l *Logger) LogSuccess(event Event) error {
	event.EventType = "success"
	return l.logEvent(event)
}

// LogRetryWithEntry logs a retry attempt using the new structured LogEntry format.
func (l *Logger) LogRetryWithEntry(entry *LogEntry) error {
	entry.EventType = "retry"
	return l.logEntry(entry)
}

// LogFailureWithEntry logs a final failure using the new structured LogEntry format.
func (l *Logger) LogFailureWithEntry(entry *LogEntry) error {
	entry.EventType = "failure"
	return l.logEntry(entry)
}

// LogSuccessWithEntry logs a successful resolution using the new structured LogEntry format.
func (l *Logger) LogSuccessWithEntry(entry *LogEntry) error {
	entry.EventType = "success"
	return l.logEntry(entry)
}

// logEvent writes a structured log event.
func (l *Logger) logEvent(event Event) error {
	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Validate required fields
	if event.Email == "" {
		return fmt.Errorf("email is required")
	}
	if event.GithubUsername == "" {
		return fmt.Errorf("github_username is required")
	}
	if event.EndpointURL == "" {
		return fmt.Errorf("endpoint_url is required")
	}

	// Serialize to JSON for structured log consumption
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal ingest event: %w", err)
	}

	// Write to log output
	l.output.Println(string(jsonBytes))

	return nil
}

// logEntry writes a structured log entry using the new schema.
func (l *Logger) logEntry(entry *LogEntry) error {
	// Set timestamp if not provided
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	// Validate required fields
	if entry.User.Email == "" {
		return fmt.Errorf("user.email is required")
	}
	if entry.User.GithubUsername == "" {
		return fmt.Errorf("user.github_username is required")
	}
	if entry.Endpoint.URL == "" {
		return fmt.Errorf("endpoint.url is required")
	}
	if entry.Endpoint.AttemptNumber == 0 {
		return fmt.Errorf("endpoint.attempt_number is required")
	}

	// Serialize to JSON for structured log consumption
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal ingest log entry: %w", err)
	}

	// Write to log output
	l.output.Println(string(jsonBytes))

	return nil
}

// LogRetryInline is a convenience function for logging retry attempts without a Logger instance.
func LogRetryInline(email, githubUsername, endpointURL string, attemptNumber, maxRetries int, statusCode int, responseBody, errorType, errorMessage, stacktrace string, retryDelayMs int, totalDurationMs int64) {
	logger := NewLogger()
	event := Event{
		Email:          email,
		GithubUsername: githubUsername,
		EndpointURL:    endpointURL,
		AttemptNumber:  attemptNumber,
		MaxRetries:     maxRetries,
		StatusCode:     statusCode,
		ResponseBody:   responseBody,
		ErrorType:      errorType,
		ErrorMessage:   errorMessage,
		Stacktrace:     stacktrace,
		RetryDelayMs:   retryDelayMs,
		TotalDurationMs: totalDurationMs,
	}
	if err := logger.LogRetry(event); err != nil {
		// If logging fails, at least emit to stderr with a clear error marker
		log.Printf("ERROR: failed to write ingest retry log: %v\n", err)
		log.Printf("ERROR: event was: retry for email=%s github_username=%s\n", email, githubUsername)
	}
}

// LogFailureInline is a convenience function for logging failures without a Logger instance.
func LogFailureInline(email, githubUsername, endpointURL string, attemptNumber, maxRetries int, statusCode int, responseBody, errorType, errorMessage, stacktrace string, totalDurationMs int64) {
	logger := NewLogger()
	event := Event{
		Email:           email,
		GithubUsername:  githubUsername,
		EndpointURL:     endpointURL,
		AttemptNumber:   attemptNumber,
		MaxRetries:      maxRetries,
		StatusCode:      statusCode,
		ResponseBody:    responseBody,
		ErrorType:       errorType,
		ErrorMessage:    errorMessage,
		Stacktrace:      stacktrace,
		TotalDurationMs: totalDurationMs,
	}
	if err := logger.LogFailure(event); err != nil {
		// If logging fails, at least emit to stderr with a clear error marker
		log.Printf("ERROR: failed to write ingest failure log: %v\n", err)
		log.Printf("ERROR: event was: failure for email=%s github_username=%s\n", email, githubUsername)
	}
}

// ToLogEntry converts a legacy Event to the new structured LogEntry format.
func (e *Event) ToLogEntry() LogEntry {
	return LogEntry{
		Timestamp: e.Timestamp,
		EventType: e.EventType,
		User: UserContext{
			Email:          e.Email,
			GithubUsername: e.GithubUsername,
		},
		Endpoint: EndpointContext{
			URL:           e.EndpointURL,
			AttemptNumber: e.AttemptNumber,
			StatusCode:    e.StatusCode,
			ResponseBody:  e.ResponseBody,
		},
		Error: ErrorContext{
			Type:       e.ErrorType,
			Message:    e.ErrorMessage,
			StackTrace: e.Stacktrace,
		},
		MaxRetries:      e.MaxRetries,
		RetryDelayMs:    e.RetryDelayMs,
		TotalDurationMs: e.TotalDurationMs,
	}
}

// ToEvent converts a LogEntry to the legacy Event format for backward compatibility.
func (le *LogEntry) ToEvent() Event {
	return Event{
		Timestamp:       le.Timestamp,
		EventType:       le.EventType,
		Email:           le.User.Email,
		GithubUsername:  le.User.GithubUsername,
		EndpointURL:     le.Endpoint.URL,
		AttemptNumber:   le.Endpoint.AttemptNumber,
		MaxRetries:      le.MaxRetries,
		StatusCode:      le.Endpoint.StatusCode,
		ResponseBody:    le.Endpoint.ResponseBody,
		ErrorType:       le.Error.Type,
		ErrorMessage:    le.Error.Message,
		Stacktrace:      le.Error.StackTrace,
		RetryDelayMs:    le.RetryDelayMs,
		TotalDurationMs: le.TotalDurationMs,
	}
}

// classifyError classifies an error into a type category for logging purposes.
// It analyzes both the error message and HTTP status code to determine the appropriate error classification.
//
// Error types:
//   - "timeout": Deadline exceeded or timeout errors
//   - "network": Connection refused, reset, DNS failures, and other network issues
//   - "parse_error": JSON unmarshaling or parsing errors
//   - "client_error": 4xx HTTP status codes (non-retryable client errors)
//   - "server_error": 5xx HTTP status codes (retryable server errors)
//   - "unknown": Unclassified errors
//   - "": Nil errors (no error)
//
// Parameters:
//   - err: The error that occurred (can be nil)
//   - statusCode: HTTP status code received (0 if not applicable)
//
// Returns:
//   - Error type string as described above
func classifyError(err error, statusCode int) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// Check for timeout errors
	if contains(errMsg, "deadline exceeded") || contains(errMsg, "timeout") {
		return "timeout"
	}

	// Check for network errors
	if contains(errMsg, "connection refused") ||
	   contains(errMsg, "connection reset") ||
	   contains(errMsg, "no such host") ||
	   contains(errMsg, "dial") ||
	   contains(errMsg, "network") {
		return "network"
	}

	// Check for parse errors
	if contains(errMsg, "invalid JSON") ||
	   contains(errMsg, "cannot unmarshal") ||
	   contains(errMsg, "invalid character") ||
	   contains(errMsg, "unmarshal") ||
	   contains(errMsg, "parse error") {
		return "parse_error"
	}

	// Classify based on HTTP status code
	if statusCode >= 400 && statusCode < 500 {
		return "client_error"
	}
	if statusCode >= 500 && statusCode < 600 {
		return "server_error"
	}

	// Unknown error type
	return "unknown"
}

// contains checks if a string contains a substring (case-insensitive helper).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

// containsSubstring is a simple substring check helper.
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// LogEntryFromError creates a LogEntry from error context, extracting error details.
// This function accepts all required context parameters and serializes error information
// (type, message, stack trace) into a structured LogEntry for logging.
//
// Parameters:
//   - email: User's email address being resolved
//   - githubUsername: Target GitHub username for resolution
//   - endpointURL: Full HTTP endpoint URL being called
//   - err: The error that occurred (can be nil)
//   - statusCode: HTTP status code received (0 if not applicable)
//   - responseBody: Response body content (empty if not available)
//   - attemptNumber: Current retry attempt (1-based)
//   - maxRetries: Maximum number of retry attempts
//   - retryDelayMs: Delay before next retry in milliseconds (0 for final failure)
//   - totalDurationMs: Total time spent attempting in milliseconds
//
// Returns:
//   - A populated LogEntry struct with error context extracted from the error
func LogEntryFromError(email, githubUsername, endpointURL string, err error, statusCode int, responseBody string, attemptNumber, maxRetries, retryDelayMs int, totalDurationMs int64) LogEntry {
	errorType := classifyError(err, statusCode)
	errorMessage := ""
	stacktrace := ""

	if err != nil {
		errorMessage = err.Error()
		// Capture stack trace for debugging
		stacktrace = string(debug.Stack())
	}

	return LogEntry{
		Timestamp: time.Now().UTC(),
		User: UserContext{
			Email:          email,
			GithubUsername: githubUsername,
		},
		Endpoint: EndpointContext{
			URL:           endpointURL,
			AttemptNumber: attemptNumber,
			StatusCode:    statusCode,
			ResponseBody:  responseBody,
		},
		Error: ErrorContext{
			Type:       errorType,
			Message:    errorMessage,
			StackTrace: stacktrace,
		},
		MaxRetries:      maxRetries,
		RetryDelayMs:    retryDelayMs,
		TotalDurationMs: totalDurationMs,
	}
}

// LogRetryInlineWithEntry is a convenience function for logging retry attempts using the new LogEntry format.
func LogRetryInlineWithEntry(entry LogEntry) {
	logger := NewLogger()
	if err := logger.LogRetryWithEntry(&entry); err != nil {
		// If logging fails, at least emit to stderr with a clear error marker
		log.Printf("ERROR: failed to write ingest retry log entry: %v\n", err)
		log.Printf("ERROR: entry was: retry for email=%s github_username=%s\n", entry.User.Email, entry.User.GithubUsername)
	}
}

// LogFailureInlineWithEntry is a convenience function for logging failures using the new LogEntry format.
func LogFailureInlineWithEntry(entry LogEntry) {
	logger := NewLogger()
	if err := logger.LogFailureWithEntry(&entry); err != nil {
		// If logging fails, at least emit to stderr with a clear error marker
		log.Printf("ERROR: failed to write ingest failure log entry: %v\n", err)
		log.Printf("ERROR: entry was: failure for email=%s github_username=%s\n", entry.User.Email, entry.User.GithubUsername)
	}
}

// EventFromError creates an Event from error context, extracting error details.
// This function accepts all required context parameters and serializes error information
// (type, message, stack trace) into a structured Event for logging.
//
// Parameters:
//   - email: User's email address being resolved
//   - githubUsername: Target GitHub username for resolution
//   - endpointURL: Full HTTP endpoint URL being called
//   - err: The error that occurred (can be nil)
//   - statusCode: HTTP status code received (0 if not applicable)
//   - responseBody: Response body content (empty if not available)
//   - attemptNumber: Current retry attempt (1-based)
//   - maxRetries: Maximum number of retry attempts
//   - retryDelayMs: Delay before next retry in milliseconds (0 for final failure)
//   - totalDurationMs: Total time spent attempting in milliseconds
//
// Returns:
//   - A populated Event struct with error context extracted from the error
func EventFromError(email, githubUsername, endpointURL string, err error, statusCode int, responseBody string, attemptNumber, maxRetries, retryDelayMs int, totalDurationMs int64) Event {
	errorType := classifyError(err, statusCode)
	errorMessage := ""
	stacktrace := ""

	if err != nil {
		errorMessage = err.Error()
		// Capture stack trace for debugging
		stacktrace = string(debug.Stack())
	}

	return Event{
		Timestamp:       time.Now().UTC(),
		Email:           email,
		GithubUsername:  githubUsername,
		EndpointURL:     endpointURL,
		AttemptNumber:   attemptNumber,
		MaxRetries:      maxRetries,
		StatusCode:      statusCode,
		ResponseBody:    responseBody,
		ErrorType:       errorType,
		ErrorMessage:    errorMessage,
		Stacktrace:      stacktrace,
		RetryDelayMs:    retryDelayMs,
		TotalDurationMs: totalDurationMs,
	}
}
