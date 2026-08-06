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
	stats  *AggregateStats
}

// NewLogger creates a new ingest logger that writes to stderr (or a configured output).
func NewLogger() *Logger {
	return &Logger{
		output: log.New(os.Stderr, "[INGEST-LOG] ", log.LstdFlags|log.Lmicroseconds|log.LUTC),
		stats:  NewAggregateStats(),
	}
}

// NewLoggerWithOutput creates a new ingest logger with a custom output writer.
func NewLoggerWithOutput(output *log.Logger) *Logger {
	return &Logger{
		output: output,
		stats:  NewAggregateStats(),
	}
}

// AggregateStats tracks aggregate statistics for ingest operations.
type AggregateStats struct {
	TotalProcessed int // Total records attempted
	TotalSkipped   int // Total records skipped (e.g., empty login, validation failures)
	TotalIngested  int // Total records successfully ingested
	TotalRetries   int // Total retry attempts
	TotalFailures  int // Total final failures (after all retries)
	StartTime      time.Time
	LastUpdateTime time.Time
}

// NewAggregateStats creates a new AggregateStats instance.
func NewAggregateStats() *AggregateStats {
	return &AggregateStats{
		StartTime:      time.Now().UTC(),
		LastUpdateTime: time.Now().UTC(),
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
	UserID        string `json:"user_id"`        // User's unique identifier
	SessionID     string `json:"session_id"`     // User's current session identifier
	RequestID     string `json:"request_id"`     // Current request identifier
	Email         string `json:"email"`           // User's email address being resolved
	GithubUsername string `json:"github_username"` // Target GitHub username for resolution
}

// EndpointContext contains HTTP endpoint interaction details.
type EndpointContext struct {
	Endpoint      string `json:"endpoint"`                       // Endpoint identifier (e.g., "github-username-resolution")
	Method        string `json:"method"`                         // HTTP method (GET, POST, etc.)
	Path          string `json:"path"`                           // Request path
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
	Metadata        RequestMetadata `json:"metadata,omitempty"`          // Optional metadata for additional context
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
	l.stats.TotalIngested++
	l.stats.TotalProcessed++
	l.stats.LastUpdateTime = time.Now().UTC()
	return l.logEntry(entry)
}

// RecordSkipped records a skipped record (e.g., empty login, validation failure).
func (l *Logger) RecordSkipped(reason string) {
	l.stats.TotalSkipped++
	l.stats.LastUpdateTime = time.Now().UTC()
	l.output.Printf("SKIP: %s | Total skipped: %d\n", reason, l.stats.TotalSkipped)
}

// RecordRetry records a retry attempt.
func (l *Logger) RecordRetry(entry *LogEntry) error {
	l.stats.TotalRetries++
	l.stats.LastUpdateTime = time.Now().UTC()
	return l.LogRetryWithEntry(entry)
}

// RecordFailure records a final failure after all retries exhausted.
func (l *Logger) RecordFailure(entry *LogEntry) error {
	l.stats.TotalFailures++
	l.stats.LastUpdateTime = time.Now().UTC()
	return l.LogFailureWithEntry(entry)
}

// RecordProcessed records a record as it enters the ingest flow.
// This increments the TotalProcessed counter and updates the LastUpdateTime.
func (l *Logger) RecordProcessed() {
	l.stats.TotalProcessed++
	l.stats.LastUpdateTime = time.Now().UTC()
}

// GetStats returns the current aggregate statistics.
func (l *Logger) GetStats() *AggregateStats {
	return l.stats
}

// LogStats logs the current aggregate statistics in a formatted summary.
func (l *Logger) LogStats(title string) {
	elapsed := l.stats.LastUpdateTime.Sub(l.stats.StartTime)
	rate := float64(l.stats.TotalProcessed) / elapsed.Seconds()

	l.output.Println("\n=== " + title + " ===")
	l.output.Printf("Records processed:    %d\n", l.stats.TotalProcessed)
	l.output.Printf("Records skipped:      %d (%.1f%%)\n", l.stats.TotalSkipped,
		float64(l.stats.TotalSkipped)/float64(l.stats.TotalProcessed)*100)
	l.output.Printf("Records ingested:     %d (%.1f%%)\n", l.stats.TotalIngested,
		float64(l.stats.TotalIngested)/float64(l.stats.TotalProcessed)*100)
	l.output.Printf("Retry attempts:       %d\n", l.stats.TotalRetries)
	l.output.Printf("Final failures:       %d\n", l.stats.TotalFailures)
	l.output.Printf("Elapsed time:         %v\n", elapsed.Round(time.Millisecond))
	l.output.Printf("Average rate:         %.2f records/sec\n", rate)
}

// BatchProgress represents progress information for batch processing.
type BatchProgress struct {
	BatchNum        int           // Current batch number (1-based)
	TotalBatches    int           // Total number of batches
	ProcessedRows   int           // Total rows processed so far
	TotalRows       int           // Total rows to process
	BatchElapsed    time.Duration // Time taken for this batch
	TotalElapsed    time.Duration // Total elapsed time
}

// LogBatchProgress logs batch processing progress with rate and ETA calculations.
// This is suitable for monitoring large production runs (e.g., 349,425 records).
func (l *Logger) LogBatchProgress(progress BatchProgress) {
	percentComplete := float64(progress.ProcessedRows) / float64(progress.TotalRows) * 100

	// Calculate average rate
	avgRate := float64(progress.ProcessedRows) / progress.TotalElapsed.Seconds()

	// Estimate time remaining
	rowsRemaining := progress.TotalRows - progress.ProcessedRows
	etaSeconds := float64(rowsRemaining) / avgRate
	eta := time.Duration(etaSeconds) * time.Second

	l.output.Printf("  Progress: %d/%d batches (%d rows, %.1f%%) | Rate: %.0f rows/sec | ETA: %v (batch took: %v)\n",
		progress.BatchNum, progress.TotalBatches, progress.ProcessedRows, percentComplete,
		avgRate, eta.Round(time.Second), progress.BatchElapsed.Round(time.Millisecond))

	l.stats.LastUpdateTime = time.Now().UTC()
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

	// Validate endpoint fields (always required)
	if entry.Endpoint.URL == "" {
		return fmt.Errorf("endpoint.url is required")
	}
	if entry.Endpoint.AttemptNumber == 0 {
		return fmt.Errorf("endpoint.attempt_number is required")
	}

	// Validate user identification - accept either basic context or extended context
	// Basic context: Email and GithubUsername (legacy behavior)
	// Extended context: At least one of UserID, SessionID, or RequestID
	hasBasicContext := entry.User.Email != "" || entry.User.GithubUsername != ""
	hasExtendedContext := entry.User.UserID != "" || entry.User.SessionID != "" || entry.User.RequestID != ""

	if !hasBasicContext && !hasExtendedContext {
		return fmt.Errorf("user identification required: either email/github_username or user_id/session_id/request_id must be provided")
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

// ErrorRecovery provides recovery suggestions for different error types.
type ErrorRecovery struct {
	Suggestion string // Actionable recovery suggestion
	Severity   string // "low", "medium", "high"
}

// GetErrorRecovery returns actionable recovery suggestions based on error type.
func GetErrorRecovery(errorType string, statusCode int) ErrorRecovery {
	switch errorType {
	case "timeout":
		return ErrorRecovery{
			Suggestion: "Network timeout - increase client timeout, check network connectivity, or retry with exponential backoff",
			Severity:   "medium",
		}
	case "network":
		return ErrorRecovery{
			Suggestion: "Network error - verify endpoint availability, check DNS resolution, inspect firewall rules",
			Severity:   "high",
		}
	case "client_error":
		return ErrorRecovery{
			Suggestion: fmt.Sprintf("Client error %d - validate request format, check authentication, verify payload structure", statusCode),
			Severity:   "high",
		}
	case "server_error":
		return ErrorRecovery{
			Suggestion: fmt.Sprintf("Server error %d - service unavailable or overloaded, implement retry with exponential backoff", statusCode),
			Severity:   "medium",
		}
	case "parse_error":
		return ErrorRecovery{
			Suggestion: "Parse error - response format changed or invalid JSON, check API schema changes",
			Severity:   "high",
		}
	default:
		return ErrorRecovery{
			Suggestion: "Unknown error - check service logs, verify request payload, inspect stack trace for details",
			Severity:   "low",
		}
	}
}

// LogErrorWithRecovery logs an error with actionable recovery suggestions.
func (l *Logger) LogErrorWithRecovery(entry *LogEntry) error {
	// Get recovery suggestion based on error type
	recovery := GetErrorRecovery(entry.Error.Type, entry.Endpoint.StatusCode)

	// Log the original error
	var err error
	switch entry.EventType {
	case "retry":
		err = l.LogRetryWithEntry(entry)
	case "failure":
		err = l.LogFailureWithEntry(entry)
	default:
		err = l.logEntry(entry)
	}

	if err != nil {
		return err
	}

	// Log recovery suggestion
	l.output.Printf("RECOVERY [%s severity]: %s\n", recovery.Severity, recovery.Suggestion)

	return nil
}

// StatusReport contains comprehensive status information for periodic reporting.
type StatusReport struct {
	Title           string       // Report title
	Logger          *Logger      // Logger instance with stats
	ProcessedRows   int          // Total rows processed
	TotalRows       int          // Total rows to process
	CurrentBatch    int          // Current batch number
	TotalBatches    int          // Total number of batches
	LastError       string       // Last error message
	LastErrorTime   time.Time    // Time of last error
	ErrorCount      int          // Number of errors since last report
	IncludeProgress bool         // Whether to include progress percentage
}

// LogStatusReport logs a comprehensive status report suitable for periodic monitoring.
// This provides production-ready visibility into long-running operations.
func (l *Logger) LogStatusReport(report StatusReport) {
	l.output.Println("\n=== " + report.Title + " ===")
	l.output.Printf("Timestamp: %s\n", time.Now().UTC().Format(time.RFC3339))

	// Aggregate statistics
	l.output.Printf("Records processed:   %d", l.stats.TotalProcessed)
	if report.IncludeProgress && report.TotalRows > 0 {
		percentComplete := float64(report.ProcessedRows) / float64(report.TotalRows) * 100
		l.output.Printf(" (%.1f%% of %d total)", percentComplete, report.TotalRows)
	}
	l.output.Println()

	l.output.Printf("Records skipped:     %d\n", l.stats.TotalSkipped)
	l.output.Printf("Records ingested:    %d\n", l.stats.TotalIngested)
	l.output.Printf("Retry attempts:      %d\n", l.stats.TotalRetries)
	l.output.Printf("Final failures:      %d\n", l.stats.TotalFailures)

	// Progress information
	if report.CurrentBatch > 0 && report.TotalBatches > 0 {
		l.output.Printf("Batch progress:     %d/%d batches completed\n", report.CurrentBatch, report.TotalBatches)
	}

	// Error summary
	if report.ErrorCount > 0 {
		l.output.Printf("Recent errors:      %d since last report\n", report.ErrorCount)
		if report.LastError != "" {
			l.output.Printf("Last error:         %s (at %s)\n", report.LastError, report.LastErrorTime.Format(time.RFC3339))
		}
	}

	// Performance metrics
	elapsed := time.Since(l.stats.StartTime)
	l.output.Printf("Elapsed time:       %v\n", elapsed.Round(time.Millisecond))
	if l.stats.TotalProcessed > 0 {
		rate := float64(l.stats.TotalProcessed) / elapsed.Seconds()
		l.output.Printf("Average rate:       %.2f records/sec\n", rate)
	}

	// ETA calculation
	if report.TotalRows > 0 && report.ProcessedRows > 0 {
		rowsRemaining := report.TotalRows - report.ProcessedRows
		avgRate := float64(report.ProcessedRows) / elapsed.Seconds()
		etaSeconds := float64(rowsRemaining) / avgRate
		eta := time.Duration(etaSeconds) * time.Second
		l.output.Printf("Estimated complete: %v\n", time.Now().UTC().Add(eta).Format(time.RFC3339))
		l.output.Printf("ETA remaining:      %v\n", eta.Round(time.Second))
	}

	l.output.Println("===")
}

// PeriodicReporter provides automatic periodic status reporting.
type PeriodicReporter struct {
	logger       *Logger
	report       StatusReport
	interval     time.Duration
	stopChan     chan struct{}
	lastReport   time.Time
	errorBuffer  []string
	maxErrors    int
}

// NewPeriodicReporter creates a new periodic reporter.
func NewPeriodicReporter(logger *Logger, report StatusReport, interval time.Duration) *PeriodicReporter {
	return &PeriodicReporter{
		logger:      logger,
		report:      report,
		interval:    interval,
		stopChan:    make(chan struct{}),
		lastReport:  time.Now(),
		errorBuffer: make([]string, 0, 10),
		maxErrors:   10,
	}
}

// Start begins periodic status reporting in a background goroutine.
func (pr *PeriodicReporter) Start() {
	ticker := time.NewTicker(pr.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				pr.logger.LogStatusReport(pr.report)
				pr.lastReport = time.Now()
			case <-pr.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the periodic reporter and logs a final status report.
func (pr *PeriodicReporter) Stop() {
	close(pr.stopChan)
	pr.logger.LogStatusReport(pr.report)
}

// AddError records an error for inclusion in the next status report.
func (pr *PeriodicReporter) AddError(errorMsg string) {
	if len(pr.errorBuffer) >= pr.maxErrors {
		// Remove oldest error
		pr.errorBuffer = pr.errorBuffer[1:]
	}
	pr.errorBuffer = append(pr.errorBuffer, errorMsg)
	pr.report.ErrorCount++
	pr.report.LastError = errorMsg
	pr.report.LastErrorTime = time.Now()
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

// CaptureUserID creates a userID string with validation.
// This helper function accepts a userID parameter and returns it with validation.
//
// Parameters:
//   - userID: User's unique identifier (optional, can be empty string)
//
// Returns:
//   - The userID string (empty string if not provided)
//   - nil (always succeeds - userID is optional)
func CaptureUserID(userID string) string {
	// userID is optional - return empty string if not provided
	if userID == "" {
		return ""
	}
	return userID
}

// CaptureSessionID creates a sessionID string with validation.
// This helper function accepts a sessionID parameter and returns it with validation.
//
// Parameters:
//   - sessionID: User's current session identifier (optional, can be empty string)
//
// Returns:
//   - The sessionID string (empty string if not provided)
func CaptureSessionID(sessionID string) string {
	// sessionID is optional - return empty string if not provided
	if sessionID == "" {
		return ""
	}
	return sessionID
}

// CaptureRequestID creates a requestID string with validation.
// This helper function accepts a requestID parameter and returns it with validation.
//
// Parameters:
//   - requestID: Current request identifier (optional, can be empty string)
//
// Returns:
//   - The requestID string (empty string if not provided)
func CaptureRequestID(requestID string) string {
	// requestID is optional - return empty string if not provided
	if requestID == "" {
		return ""
	}
	return requestID
}

// CaptureEndpointName creates an endpoint string with validation.
// This helper function accepts an endpoint parameter and returns it with validation.
//
// Parameters:
//   - endpoint: Endpoint identifier (e.g., "github-username-resolution", required)
//
// Returns:
//   - The endpoint string
//   - An error if validation fails (empty endpoint)
func CaptureEndpointName(endpoint string) (string, error) {
	// endpoint is required - return error if not provided
	if endpoint == "" {
		return "", fmt.Errorf("endpoint is required for EndpointContext")
	}
	return endpoint, nil
}

// CaptureMethod creates a method string with validation.
// This helper function accepts an HTTP method parameter and returns it with validation.
//
// Parameters:
//   - method: HTTP method (GET, POST, etc., required)
//
// Returns:
//   - The method string
//   - An error if validation fails (empty method)
func CaptureMethod(method string) (string, error) {
	// method is required - return error if not provided
	if method == "" {
		return "", fmt.Errorf("method is required for EndpointContext")
	}
	return method, nil
}

// CapturePath creates a path string with validation.
// This helper function accepts a path parameter and returns it with validation.
//
// Parameters:
//   - path: Request path (required)
//
// Returns:
//   - The path string
//   - An error if validation fails (empty path)
func CapturePath(path string) (string, error) {
	// path is required - return error if not provided
	if path == "" {
		return "", fmt.Errorf("path is required for EndpointContext")
	}
	return path, nil
}

// CaptureEndpointContext creates an EndpointContext struct with validation.
// This helper function accepts endpoint interaction parameters and returns
// a populated EndpointContext struct for use in log entries.
//
// Parameters:
//   - endpoint: Endpoint identifier (e.g., "github-username-resolution", required)
//   - method: HTTP method (GET, POST, etc., required)
//   - path: Request path (required)
//   - url: Full HTTP endpoint URL being called (required)
//   - attemptNumber: Current retry attempt (1-based, required)
//   - statusCode: HTTP status code received (0 if not applicable, defaults to 0)
//   - responseBody: Response body content (empty if not available, defaults to empty string)
//
// Returns:
//   - A populated EndpointContext struct
//   - An error if validation fails (empty endpoint, method, path, url or zero/negative attempt number)
func CaptureEndpointContext(endpoint, method, path, url string, attemptNumber int, statusCode int, responseBody string) (EndpointContext, error) {
	// Validate required fields
	if endpoint == "" {
		return EndpointContext{}, fmt.Errorf("endpoint is required for EndpointContext")
	}
	if method == "" {
		return EndpointContext{}, fmt.Errorf("method is required for EndpointContext")
	}
	if path == "" {
		return EndpointContext{}, fmt.Errorf("path is required for EndpointContext")
	}
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
		Endpoint:      endpoint,
		Method:        method,
		Path:          path,
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
//   - userID: User's unique identifier (optional, can be empty string)
//   - sessionID: User's current session identifier (optional, can be empty string)
//   - requestID: Current request identifier (optional, can be empty string)
//   - endpoint: Endpoint identifier (e.g., "github-username-resolution", required)
//   - method: HTTP method (GET, POST, etc., required)
//   - path: Request path (required)
//   - endpointURL: Full HTTP endpoint URL being called (required)
//   - err: The error that occurred (can be nil for success cases)
//   - statusCode: HTTP status code received (0 if not applicable)
//   - responseBody: Response body content (empty if not available)
//   - attemptNumber: Current retry attempt (1-based, required)
//   - maxRetries: Maximum number of retry attempts configured (required)
//   - retryDelayMs: Delay before next retry in milliseconds (0 for final failure)
//   - totalDurationMs: Total time spent attempting in milliseconds
//   - eventType: Type of event ("retry", "failure", or "success")
//   - metadata: Optional metadata map for additional context (can be nil)
//
// Returns:
//   - error: Any error that occurred during logging (nil indicates success)
//
// The function handles logging failures gracefully by returning the error
// to the caller while ensuring the log entry is properly formatted and written.
func LogIngestError(logger *Logger, email, githubUsername, userID, sessionID, requestID, endpoint, method, path, endpointURL string, err error, statusCode int, responseBody string, attemptNumber, maxRetries, retryDelayMs int, totalDurationMs int64, eventType string, metadata RequestMetadata) error {
	// Use default logger if none provided
	if logger == nil {
		logger = NewLogger()
	}

	// Validate metadata keys to prevent collisions with LogEntry fields
	if err := ValidateMetadataKeys(metadata); err != nil {
		return fmt.Errorf("metadata validation failed: %w", err)
	}

	// Capture userID using the userID capture helper
	capturedUserID := CaptureUserID(userID)

	// Capture sessionID using the sessionID capture helper
	capturedSessionID := CaptureSessionID(sessionID)

	// Capture requestID using the requestID capture helper
	capturedRequestID := CaptureRequestID(requestID)

	// Capture user context using the context capture helper (from cg-4zz54)
	userCtx, userErr := CaptureUserContext(email, githubUsername)
	if userErr != nil {
		return fmt.Errorf("failed to capture user context: %w", userErr)
	}

	// Store the captured userID in the user context
	userCtx.UserID = capturedUserID

	// Store the captured sessionID in the user context
	userCtx.SessionID = capturedSessionID

	// Store the captured requestID in the user context
	userCtx.RequestID = capturedRequestID

	// Capture endpoint name using the endpoint capture helper
	capturedEndpoint, endpointErr := CaptureEndpointName(endpoint)
	if endpointErr != nil {
		return fmt.Errorf("failed to capture endpoint name: %w", endpointErr)
	}

	// Capture method using the method capture helper
	capturedMethod, methodErr := CaptureMethod(method)
	if methodErr != nil {
		return fmt.Errorf("failed to capture method: %w", methodErr)
	}

	// Capture path using the path capture helper
	capturedPath, pathErr := CapturePath(path)
	if pathErr != nil {
		return fmt.Errorf("failed to capture path: %w", pathErr)
	}

	// Capture endpoint context using the context capture helper (from cg-4zz54)
	endpointCtx, endpointErr := CaptureEndpointContext(capturedEndpoint, capturedMethod, capturedPath, endpointURL, attemptNumber, statusCode, responseBody)
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
		Metadata:        metadata,
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

// reservedLogEntryFields contains the names of fields that are reserved in LogEntry
// and cannot be used as metadata keys to prevent key collisions.
var reservedLogEntryFields = map[string]bool{
	"timestamp":         true,
	"event_type":        true,
	"user":              true,
	"endpoint":          true,
	"error":             true,
	"max_retries":       true,
	"retry_delay_ms":    true,
	"total_duration_ms": true,
	"metadata":          true,
}

// ValidateMetadataKeys validates that metadata keys do not collide with reserved LogEntry fields.
// This prevents metadata from overwriting core LogEntry fields during JSON marshaling.
//
// Parameters:
//   - metadata: The metadata map to validate (can be nil or empty)
//
// Returns:
//   - error: An error if a reserved key is found, nil otherwise
//
// The reserved keys correspond to the top-level JSON fields in LogEntry:
//   - timestamp, event_type, user, endpoint, error, max_retries, retry_delay_ms, total_duration_ms, metadata
//
// Example:
//   metadata := RequestMetadata{"batch_id": "123"} // valid
//   metadata := RequestMetadata{"user": "collision"} // returns error
func ValidateMetadataKeys(metadata RequestMetadata) error {
	if metadata == nil {
		return nil
	}

	for key := range metadata {
		if reservedLogEntryFields[key] {
			return fmt.Errorf("metadata key '%s' is reserved and cannot be used", key)
		}
	}

	return nil
}

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
