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

// CaptureUserContext creates a UserContext struct with validation.
// This helper function accepts user identification parameters and returns
// a populated UserContext struct for use in log entries.
//
// Parameters:
//   - email: User's email address being resolved (required)
//   - githubUsername: Target GitHub username for resolution (required)
//
// Returns:
//   - A populated UserContext struct
//   - An error if validation fails (empty email or githubUsername)
func CaptureUserContext(email, githubUsername string) (UserContext, error) {
	// Validate required fields
	if email == "" {
		return UserContext{}, fmt.Errorf("email is required for UserContext")
	}
	if githubUsername == "" {
		return UserContext{}, fmt.Errorf("github_username is required for UserContext")
	}

	return UserContext{
		Email:          email,
		GithubUsername: githubUsername,
	}, nil
}

// CaptureEndpointContext creates an EndpointContext struct with validation.
// This helper function accepts endpoint interaction parameters and returns
// a populated EndpointContext struct for use in log entries.
//
// Parameters:
//   - url: Full HTTP endpoint URL being called (required)
//   - attemptNumber: Current retry attempt (1-based, required)
//   - statusCode: HTTP status code received (0 if not applicable, defaults to 0)
//   - responseBody: Response body content (empty if not available, defaults to empty string)
//
// Returns:
//   - A populated EndpointContext struct
//   - An error if validation fails (empty URL or zero/negative attempt number)
func CaptureEndpointContext(url string, attemptNumber int, statusCode int, responseBody string) (EndpointContext, error) {
	// Validate required fields
	if url == "" {
		return EndpointContext{}, fmt.Errorf("url is required for EndpointContext")
	}
	if attemptNumber <= 0 {
		return EndpointContext{}, fmt.Errorf("attempt_number must be positive (got %d)", attemptNumber)
	}

	// Apply default values for optional fields
	if statusCode == 0 {
		statusCode = 0 // Explicitly keep as 0 (no status code available)
	}

	// Truncate response body if it exceeds reasonable size limit (10KB)
	const maxResponseBodySize = 10 * 1024
	if len(responseBody) > maxResponseBodySize {
		responseBody = responseBody[:maxResponseBodySize] + "... (truncated)"
	}

	return EndpointContext{
		URL:           url,
		AttemptNumber: attemptNumber,
		StatusCode:    statusCode,
		ResponseBody:  responseBody,
	}, nil
}

// LogIngestError is the main logging function that integrates all components
// and writes formatted log entries for ingest endpoint operations.
//
// This function brings together error serialization (from cg-2iff2), context
// capture helpers (from cg-4zz54), and the Logger to create a complete
// ingest error logging solution.
//
// Parameters:
//   - logger: The Logger instance to write the log entry (can be nil, will use default)
//   - email: User's email address being resolved (required)
//   - githubUsername: Target GitHub username for resolution (required)
//   - endpointURL: Full HTTP endpoint URL being called (required)
//   - err: The error that occurred (can be nil for success cases)
//   - statusCode: HTTP status code received (0 if not applicable)
//   - responseBody: Response body content (empty if not available)
//   - attemptNumber: Current retry attempt (1-based, required)
//   - maxRetries: Maximum number of retry attempts configured (required)
//   - retryDelayMs: Delay before next retry in milliseconds (0 for final failure)
//   - totalDurationMs: Total time spent attempting in milliseconds
//   - eventType: Type of event ("retry", "failure", or "success")
//
// Returns:
//   - error: Any error that occurred during logging (nil indicates success)
//
// The function handles logging failures gracefully by returning the error
// to the caller while ensuring the log entry is properly formatted and written.
func LogIngestError(logger *Logger, email, githubUsername, endpointURL string, err error, statusCode int, responseBody string, attemptNumber, maxRetries, retryDelayMs int, totalDurationMs int64, eventType string) error {
	// Use default logger if none provided
	if logger == nil {
		logger = NewLogger()
	}

	// Capture user context using the context capture helper (from cg-4zz54)
	userCtx, userErr := CaptureUserContext(email, githubUsername)
	if userErr != nil {
		return fmt.Errorf("failed to capture user context: %w", userErr)
	}

	// Capture endpoint context using the context capture helper (from cg-4zz54)
	endpointCtx, endpointErr := CaptureEndpointContext(endpointURL, attemptNumber, statusCode, responseBody)
	if endpointErr != nil {
		return fmt.Errorf("failed to capture endpoint context: %w", endpointErr)
	}

	// Serialize error using the error serialization helper (from cg-2iff2)
	errorCtx := SerializeError(err)

	// Assemble the complete LogEntry struct with all captured context
	entry := &LogEntry{
		Timestamp: time.Now().UTC(),
		EventType: eventType,
		User:      userCtx,
		Endpoint:  endpointCtx,
		Error:     errorCtx,
		MaxRetries:      maxRetries,
		RetryDelayMs:    retryDelayMs,
		TotalDurationMs: totalDurationMs,
	}

	// Write the log entry using the appropriate method based on event type
	var logErr error
	switch eventType {
	case "retry":
		logErr = logger.LogRetryWithEntry(entry)
	case "failure":
		logErr = logger.LogFailureWithEntry(entry)
	case "success":
		logErr = logger.LogSuccessWithEntry(entry)
	default:
		// Default to retry for unknown event types
		logErr = logger.LogRetryWithEntry(entry)
	}

	// Handle logging failures gracefully - return the error to the caller
	if logErr != nil {
		return fmt.Errorf("failed to write ingest log entry: %w", logErr)
	}

	return nil
}

// RequestMetadata contains optional metadata for ingest requests.
type RequestMetadata map[string]interface{}

// ExtendedUserContext contains enhanced user identification information.
type ExtendedUserContext struct {
	UserID    string `json:"user_id"`    // User's unique identifier
	SessionID string `json:"session_id"` // Current session identifier
	RequestID string `json:"request_id"` // Current request identifier
	Email     string `json:"email"`      // User's email address (optional)
	Username  string `json:"username"`   // User's username (optional)
}

// ExtendedEndpointContext contains detailed HTTP endpoint interaction information.
type ExtendedEndpointContext struct {
	Endpoint     string `json:"endpoint"`              // Endpoint identifier (e.g., "github-username-resolution")
	Method       string `json:"method"`                // HTTP method (GET, POST, etc.)
	Path         string `json:"path"`                  // Request path
	URL          string `json:"url"`                   // Full HTTP endpoint URL
	StatusCode   int    `json:"status_code"`           // HTTP status code received
	ResponseBody string `json:"response_body,omitempty"` // Response body content (if available)
}

// LogIngestErrorExtended is the enhanced logging function that accepts structured context.
//
// This function brings together error serialization (from cg-2iff2), context
// capture helpers (from cg-4zz54), and the Logger to create a complete
// ingest error logging solution with enhanced user and endpoint context.
//
// Parameters:
//   - logger: The Logger instance to write the log entry (can be nil, will use default)
//   - err: The error that occurred (error interface, can be nil for success cases)
//   - userCtx: Extended user context containing userID, sessionID, requestID, and optional email/username
//   - endpointCtx: Extended endpoint context containing endpoint, method, path, URL, and response details
//   - metadata: Optional metadata map for additional context (can be nil)
//
// Returns:
//   - error: Any error that occurred during logging (nil indicates success)
//
// The function handles logging failures gracefully by returning the error
// to the caller while ensuring the log entry is properly formatted and written.
//
// Integration Points (TODO):
//   - cg-2iff2: Error serialization and type classification
//   - cg-4zz54: User and endpoint context capture helpers
//   - Future: Integration with monitoring and alerting systems
//   - Future: Integration with distributed tracing systems
func LogIngestErrorExtended(logger *Logger, err error, userCtx ExtendedUserContext, endpointCtx ExtendedEndpointContext, metadata RequestMetadata) error {
	// TODO: Implement logger default initialization
	// Use default logger if none provided
	if logger == nil {
		logger = NewLogger()
	}

	// TODO: Validate required user context fields
	if userCtx.UserID == "" {
		return fmt.Errorf("user_id is required in user context")
	}
	if userCtx.SessionID == "" {
		return fmt.Errorf("session_id is required in user context")
	}
	if userCtx.RequestID == "" {
		return fmt.Errorf("request_id is required in user context")
	}

	// TODO: Validate required endpoint context fields
	if endpointCtx.Endpoint == "" {
		return fmt.Errorf("endpoint is required in endpoint context")
	}
	if endpointCtx.Method == "" {
		return fmt.Errorf("method is required in endpoint context")
	}
	if endpointCtx.Path == "" {
		return fmt.Errorf("path is required in endpoint context")
	}

	// TODO: Integrate with error serialization from cg-2iff2
	// Serialize error using the error serialization helper
	errorCtx := SerializeError(err)

	// TODO: Build extended log entry with metadata integration
	// Assemble the complete LogEntry struct with all captured context
	entry := &LogEntry{
		Timestamp: time.Now().UTC(),
		EventType: "error", // Default to error type, can be extended
		User: UserContext{
			Email:          userCtx.Email,
			GithubUsername: userCtx.Username,
		},
		Endpoint: EndpointContext{
			URL:           endpointCtx.URL,
			AttemptNumber: 1, // Default, can be parameterized
			StatusCode:    endpointCtx.StatusCode,
			ResponseBody:  endpointCtx.ResponseBody,
		},
		Error:           errorCtx,
		MaxRetries:      0, // Placeholder
		RetryDelayMs:    0, // Placeholder
		TotalDurationMs: 0, // Placeholder
	}

	// TODO: Integrate metadata into log entry for extended context
	// Metadata can be added to the log entry as additional context
	if metadata != nil && len(metadata) > 0 {
		// TODO: Serialize metadata and attach to log entry
		// For now, we'll skip this as it requires LogEntry schema extension
		_ = metadata // Placeholder to avoid unused variable warning
	}

	// TODO: Implement event type detection based on error and context
	// Determine event type based on error presence and other context
	eventType := "failure"
	if err == nil {
		eventType = "success"
	}
	entry.EventType = eventType

	// TODO: Add integration point for monitoring system hooks
	// This is where we would hook into external monitoring systems

	// TODO: Add integration point for distributed tracing
	// This is where we would integrate with OpenTelemetry or similar

	// Write the log entry using the appropriate method based on event type
	var logErr error
	switch eventType {
	case "retry":
		logErr = logger.LogRetryWithEntry(entry)
	case "failure":
		logErr = logger.LogFailureWithEntry(entry)
	case "success":
		logErr = logger.LogSuccessWithEntry(entry)
	default:
		// Default to failure for unknown event types
		logErr = logger.LogFailureWithEntry(entry)
	}

	// Handle logging failures gracefully - return the error to the caller
	if logErr != nil {
		return fmt.Errorf("failed to write ingest log entry: %w", logErr)
	}

	return nil
}
