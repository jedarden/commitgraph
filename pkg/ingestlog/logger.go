// Package ingestlog provides structured logging for ingest endpoint operations.
package ingestlog

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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
