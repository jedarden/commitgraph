// Package errors provides structured error types and categorization for the commitgraph project.
package errors

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ErrorCategory represents the type/category of an error.
type ErrorCategory string

const (
	// ValidationError indicates input validation failures
	ValidationError ErrorCategory = "validation_error"
	// ParseError indicates data parsing failures
	ParseError ErrorCategory = "parse_error"
	// DatabaseError indicates database operation failures
	DatabaseError ErrorCategory = "database_error"
	// NetworkError indicates network operation failures
	NetworkError ErrorCategory = "network_error"
	// TimeoutError indicates operation timeout failures
	TimeoutError ErrorCategory = "timeout_error"
	// ClientError indicates HTTP 4xx client errors
	ClientError ErrorCategory = "client_error"
	// ServerError indicates HTTP 5xx server errors
	ServerError ErrorCategory = "server_error"
	// AuthError indicates authentication/authorization failures
	AuthError ErrorCategory = "authentication_error"
	// ConfigError indicates configuration problems
	ConfigError ErrorCategory = "configuration_error"
	// ResourceError indicates resource exhaustion/unavailability
	ResourceError ErrorCategory = "resource_error"
	// ConcurrencyError indicates concurrency/locking issues
	ConcurrencyError ErrorCategory = "concurrency_error"
	// UnknownError indicates uncategorized errors
	UnknownError ErrorCategory = "unknown_error"
)

// SeverityLevel represents the severity of an error.
type SeverityLevel string

const (
	// SeverityCritical indicates complete service failure requiring immediate intervention
	SeverityCritical SeverityLevel = "critical"
	// SeverityHigh indicates significant impact requiring prompt attention
	SeverityHigh SeverityLevel = "high"
	// SeverityMedium indicates limited impact with workarounds
	SeverityMedium SeverityLevel = "medium"
	// SeverityLow indicates minimal impact or edge cases
	SeverityLow SeverityLevel = "low"
	// SeverityInfo indicates informational events (not errors)
	SeverityInfo SeverityLevel = "info"
)

// RetryStrategy represents the retry strategy for an error.
type RetryStrategy string

const (
	// RetryStrategyNone indicates no retry
	RetryStrategyNone RetryStrategy = "none"
	// RetryStrategyLinear indicates linear backoff
	RetryStrategyLinear RetryStrategy = "linear"
	// RetryStrategyExponential indicates exponential backoff
	RetryStrategyExponential RetryStrategy = "exponential"
)

// RetryPolicy defines the retry policy for retryable errors.
type RetryPolicy struct {
	MaxRetries    int           // Maximum number of retry attempts
	InitialDelay  time.Duration // Initial delay before first retry
	MaxDelay      time.Duration // Maximum delay between retries
	Multiplier    float64       // Backoff multiplier (for exponential backoff)
	Strategy      RetryStrategy // Retry strategy to use
}

// DefaultRetryPolicy returns a default retry policy with exponential backoff.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:   3,
		InitialDelay: time.Second * 1,
		MaxDelay:     time.Second * 10,
		Multiplier:   2.0,
		Strategy:    RetryStrategyExponential,
	}
}

// RecoverySuggestion provides actionable recovery suggestions for errors.
type RecoverySuggestion struct {
	Action        string        // Human-readable recovery action
	Steps         []string      // Detailed recovery steps
	Documentation string        // Link to relevant documentation
	Severity      SeverityLevel // Recovery urgency
}

// ErrorContext contains additional contextual information about an error.
type ErrorContext struct {
	UserID      string            // User ID involved in the operation
	RequestID   string            // Request ID for tracing
	SessionID   string            // Session ID for context
	Endpoint    string            // API endpoint involved
	StatusCode  int               // HTTP status code (if applicable)
	Query       string            // Database query (sanitized)
	Package     string            // Package/function where error occurred
	File        string            // File where error occurred
	Line        int               // Line number where error occurred
	Extra       map[string]string // Additional context
}

// StructuredError is the main error type that wraps errors with comprehensive metadata.
type StructuredError struct {
	// Core error information
	Type      ErrorCategory  // Categorized error type
	Severity  SeverityLevel  // Error severity level
	Message   string         // Human-readable error message
	Code      string         // Machine-readable error code

	// Context information
	Component string         // Component/package where error occurred
	Operation string         // Operation being performed
	Context   ErrorContext   // Additional contextual information

	// Domain-specific context fields
	CommitSHA  string // Commit SHA associated with the error
	Position   int64  // Position/offset in data stream
	Email      string // Email address involved in the error
	TraceID    string // Trace ID for distributed tracing
	RecordKey  string // Record key for database/storage operations

	// Technical details
	Cause      error         // Underlying error (for wrapping)
	StackTrace string        // Stack trace at error site
	Timestamp  time.Time     // When the error occurred

	// Retry/Recovery information
	Retryable bool              // Whether this error is retryable
	RetryPolicy RetryPolicy     // Retry strategy if retryable
	Recovery  RecoverySuggestion // Suggested recovery actions

	// Additional metadata
	Metadata map[string]interface{} // Additional context
}

// Error implements the error interface for StructuredError.
func (e *StructuredError) Error() string {
	if e == nil {
		return ""
	}

	base := fmt.Sprintf("[%s] %s", e.Code, e.Message)

	// Append domain-specific context if present
	contexts := []string{}
	if e.CommitSHA != "" {
		contexts = append(contexts, fmt.Sprintf("commit=%s", e.CommitSHA))
	}
	if e.Position > 0 {
		contexts = append(contexts, fmt.Sprintf("position=%d", e.Position))
	}
	if e.Email != "" {
		contexts = append(contexts, fmt.Sprintf("email=%s", e.Email))
	}
	if e.TraceID != "" {
		contexts = append(contexts, fmt.Sprintf("trace=%s", e.TraceID))
	}
	if e.RecordKey != "" {
		contexts = append(contexts, fmt.Sprintf("record=%s", e.RecordKey))
	}

	if len(contexts) > 0 {
		base = fmt.Sprintf("%s [%s]", base, strings.Join(contexts, ", "))
	}

	if e.Cause != nil {
		return fmt.Sprintf("%s: caused by %v", base, e.Cause)
	}
	return base
}

// Unwrap returns the underlying error for error wrapping chains.
func (e *StructuredError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// GetType returns the error category.
func (e *StructuredError) GetType() ErrorCategory {
	if e == nil {
		return UnknownError
	}
	return e.Type
}

// SeverityCode returns the severity level.
func (e *StructuredError) SeverityCode() SeverityLevel {
	if e == nil {
		return SeverityLow
	}
	return e.Severity
}

// ErrorCode returns the machine-readable error code.
func (e *StructuredError) ErrorCode() string {
	if e == nil {
		return ""
	}
	return e.Code
}

// IsRetryable returns whether the error is retryable.
func (e *StructuredError) IsRetryable() bool {
	if e == nil {
		return false
	}
	return e.Retryable
}

// ErrorContextOption is a functional option for NewError that allows setting context fields.
type ErrorContextOption func(*StructuredError)

// WithCommitSHAOption creates an ErrorContextOption that sets the commit SHA.
func WithCommitSHAOption(commitSHA string) ErrorContextOption {
	return func(e *StructuredError) {
		e.CommitSHA = commitSHA
	}
}

// WithPositionOption creates an ErrorContextOption that sets the position.
func WithPositionOption(position int64) ErrorContextOption {
	return func(e *StructuredError) {
		e.Position = position
	}
}

// WithEmailOption creates an ErrorContextOption that sets the email.
func WithEmailOption(email string) ErrorContextOption {
	return func(e *StructuredError) {
		e.Email = email
	}
}

// WithTraceIDOption creates an ErrorContextOption that sets the trace ID.
func WithTraceIDOption(traceID string) ErrorContextOption {
	return func(e *StructuredError) {
		e.TraceID = traceID
	}
}

// WithRecordKeyOption creates an ErrorContextOption that sets the record key.
func WithRecordKeyOption(recordKey string) ErrorContextOption {
	return func(e *StructuredError) {
		e.RecordKey = recordKey
	}
}

// NewError creates a new structured error with the given parameters and optional context.
func NewError(typ ErrorCategory, severity SeverityLevel, message, code, component, operation string, opts ...ErrorContextOption) *StructuredError {
	err := &StructuredError{
		Type:      typ,
		Severity:  severity,
		Message:   message,
		Code:      code,
		Component: component,
		Operation: operation,
		Timestamp: time.Now().UTC(),
		Context: ErrorContext{
			Package: component,
		},
		Metadata:  make(map[string]interface{}),
		Retryable: isRetryableByDefault(typ),
		RetryPolicy: DefaultRetryPolicy(),
	}

	// Apply optional context
	for _, opt := range opts {
		opt(err)
	}

	return err
}

// WrapError wraps an existing error with additional context and structured information.
func WrapError(cause error, base StructuredError) *StructuredError {
	err := &base
	err.Cause = cause
	err.Timestamp = time.Now().UTC()
	err.StackTrace = captureStackTrace(2)

	// Capture caller information
	if pc, file, line, ok := runtime.Caller(2); ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			err.Context.Package = fn.Name()
			err.Context.File = file
			err.Context.Line = line
		}
	}

	// Infer type and severity from cause if not set
	if err.Type == UnknownError && cause != nil {
		err.Type = classifyError(cause)
	}
	if err.Severity == "" && cause != nil {
		err.Severity = inferSeverity(err.Type, cause)
	}

	return err
}

// WithContext adds context information to the error.
func (e *StructuredError) WithContext(ctx ErrorContext) *StructuredError {
	if e == nil {
		return nil
	}

	// Merge context
	if e.Context.Extra == nil {
		e.Context.Extra = make(map[string]string)
	}
	for k, v := range ctx.Extra {
		e.Context.Extra[k] = v
	}

	// Set non-empty fields
	if ctx.UserID != "" {
		e.Context.UserID = ctx.UserID
	}
	if ctx.RequestID != "" {
		e.Context.RequestID = ctx.RequestID
	}
	if ctx.SessionID != "" {
		e.Context.SessionID = ctx.SessionID
	}
	if ctx.Endpoint != "" {
		e.Context.Endpoint = ctx.Endpoint
	}
	if ctx.StatusCode != 0 {
		e.Context.StatusCode = ctx.StatusCode
	}
	if ctx.Query != "" {
		e.Context.Query = ctx.Query
	}

	return e
}

// WithMetadata adds metadata to the error.
func (e *StructuredError) WithMetadata(metadata map[string]interface{}) *StructuredError {
	if e == nil {
		return nil
	}

	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	for k, v := range metadata {
		e.Metadata[k] = v
	}

	return e
}

// WithRetryPolicy sets the retry policy for the error.
func (e *StructuredError) WithRetryPolicy(policy RetryPolicy) *StructuredError {
	if e == nil {
		return nil
	}

	e.RetryPolicy = policy
	e.Retryable = policy.MaxRetries > 0

	return e
}

// WithRecovery sets the recovery suggestion for the error.
func (e *StructuredError) WithRecovery(recovery RecoverySuggestion) *StructuredError {
	if e == nil {
		return nil
	}

	e.Recovery = recovery
	return e
}

// WithCommitSHA sets the commit SHA context on the error.
func (e *StructuredError) WithCommitSHA(commitSHA string) *StructuredError {
	if e == nil {
		return nil
	}

	e.CommitSHA = commitSHA
	return e
}

// WithPosition sets the position context on the error.
func (e *StructuredError) WithPosition(position int64) *StructuredError {
	if e == nil {
		return nil
	}

	e.Position = position
	return e
}

// WithEmail sets the email context on the error.
func (e *StructuredError) WithEmail(email string) *StructuredError {
	if e == nil {
		return nil
	}

	e.Email = email
	return e
}

// WithTraceID sets the trace ID context on the error.
func (e *StructuredError) WithTraceID(traceID string) *StructuredError {
	if e == nil {
		return nil
	}

	e.TraceID = traceID
	return e
}

// WithRecordKey sets the record key context on the error.
func (e *StructuredError) WithRecordKey(recordKey string) *StructuredError {
	if e == nil {
		return nil
	}

	e.RecordKey = recordKey
	return e
}

// isRetryableByDefault determines if an error type is retryable by default.
func isRetryableByDefault(typ ErrorCategory) bool {
	switch typ {
	case NetworkError, TimeoutError, ServerError, ConcurrencyError:
		return true
	case DatabaseError:
		return true // Database errors are generally retryable
	case ValidationError, ParseError, ClientError, AuthError, ConfigError, ResourceError:
		return false
	default:
		return false
	}
}

// classifyError classifies an error into an ErrorCategory based on its message and type.
func classifyError(err error) ErrorCategory {
	if err == nil {
		return UnknownError
	}

	errMsg := err.Error()

	// Check for timeout errors
	if contains(errMsg, "deadline exceeded") || contains(errMsg, "timeout") {
		return TimeoutError
	}

	// Check for network errors
	if contains(errMsg, "connection refused") ||
		contains(errMsg, "connection reset") ||
		contains(errMsg, "no such host") ||
		contains(errMsg, "dial") ||
		contains(errMsg, "network") {
		return NetworkError
	}

	// Check for parse errors
	if contains(errMsg, "invalid JSON") ||
		contains(errMsg, "cannot unmarshal") ||
		contains(errMsg, "invalid character") ||
		contains(errMsg, "unmarshal") ||
		contains(errMsg, "parse error") {
		return ParseError
	}

	// Check for database errors
	if contains(errMsg, "database") ||
		contains(errMsg, "sql") ||
		contains(errMsg, "constraint") ||
		contains(errMsg, "transaction") ||
		contains(errMsg, "connection") {
		return DatabaseError
	}

	// Check for authentication/authorization errors
	if contains(errMsg, "unauthorized") ||
		contains(errMsg, "forbidden") ||
		contains(errMsg, "authentication") ||
		contains(errMsg, "token") {
		return AuthError
	}

	// Check for resource errors
	if contains(errMsg, "out of memory") ||
		contains(errMsg, "disk full") ||
		contains(errMsg, "no space") ||
		contains(errMsg, "resource") {
		return ResourceError
	}

	// Check for concurrency errors
	if contains(errMsg, "deadlock") ||
		contains(errMsg, "lock") ||
		contains(errMsg, "concurrent") ||
		contains(errMsg, "race") {
		return ConcurrencyError
	}

	// Check for validation errors
	if contains(errMsg, "required") ||
		contains(errMsg, "invalid") ||
		contains(errMsg, "validation") {
		return ValidationError
	}

	// Check for configuration errors
	if contains(errMsg, "config") ||
		contains(errMsg, "environment") ||
		contains(errMsg, "setting") {
		return ConfigError
	}

	return UnknownError
}

// inferSeverity infers the severity level based on error type and content.
func inferSeverity(typ ErrorCategory, err error) SeverityLevel {
	switch typ {
	case ConfigError, ResourceError:
		return SeverityCritical
	case AuthError, NetworkError, ParseError, DatabaseError:
		return SeverityHigh
	case TimeoutError, ServerError, ConcurrencyError:
		return SeverityMedium
	case ValidationError, ClientError:
		return SeverityHigh // Most validation/client errors need attention
	default:
		return SeverityLow
	}
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// captureStackTrace captures the current stack trace as a formatted string.
func captureStackTrace(depth int) string {
	const maxDepth = 32
	pcs := make([]uintptr, maxDepth)

	// Skip the specified number of frames
	n := runtime.Callers(depth+1, pcs)
	if n == 0 {
		return ""
	}

	pcs = pcs[:n]

	frames := runtime.CallersFrames(pcs)
	var sb strings.Builder

	for {
		frame, more := frames.Next()

		// Format: file:line function
		sb.WriteString(fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function))

		if !more {
			break
		}
	}

	return sb.String()
}

// ClassifyOptions provides options for error classification.
type ClassifyOptions struct {
	Component   string
	Operation   string
	StatusCode  int
	Context     ErrorContext
}

// ClassifyError classifies an error and creates a StructuredError with appropriate metadata.
func ClassifyError(err error, opts ClassifyOptions) *StructuredError {
	if err == nil {
		return nil
	}

	typ := classifyError(err)
	severity := inferSeverity(typ, err)

	// Generate error code
	code := generateErrorCode(opts.Component, typ, opts.StatusCode)

	structuredErr := &StructuredError{
		Type:      typ,
		Severity:  severity,
		Message:   err.Error(),
		Code:      code,
		Component: opts.Component,
		Operation: opts.Operation,
		Context:   opts.Context,
		Cause:     err,
		Timestamp: time.Now().UTC(),
		Metadata:  make(map[string]interface{}),
		Retryable: isRetryableByDefault(typ),
	}

	// Set retry policy based on HTTP status code
	if opts.StatusCode >= 400 && opts.StatusCode < 500 {
		structuredErr.Retryable = false
		if opts.StatusCode == 429 {
			structuredErr.Retryable = true
		}
	}

	// Capture stack trace
	structuredErr.StackTrace = captureStackTrace(2)

	// Capture caller information
	if pc, file, line, ok := runtime.Caller(2); ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			structuredErr.Context.Package = fn.Name()
			structuredErr.Context.File = file
			structuredErr.Context.Line = line
		}
	}

	// Add recovery suggestion
	structuredErr.Recovery = getRecoverySuggestion(typ, opts.StatusCode)

	return structuredErr
}

// generateErrorCode generates a machine-readable error code.
func generateErrorCode(component string, typ ErrorCategory, statusCode int) string {
	// Format: [COMPONENT]_[ERROR_TYPE]_[SPECIFIC]
	// Example: USER_DATABASE_QUERY_TIMEOUT

	// Extract component name (last part of path)
	parts := strings.Split(component, "/")
	componentName := ""
	if len(parts) > 0 {
		componentName = parts[len(parts)-1]
	}

	// Convert error type to code
	typeCode := strings.ReplaceAll(string(typ), "_error", "")
	typeCode = strings.ToUpper(typeCode)

	// Build base code
	baseCode := fmt.Sprintf("%s_%s", componentName, typeCode)

	// Add status code suffix for HTTP errors
	if statusCode >= 400 && statusCode < 600 {
		baseCode = fmt.Sprintf("%s_%d", baseCode, statusCode)
	}

	return strings.ToUpper(baseCode)
}

// getRecoverySuggestion returns recovery suggestions based on error type and status code.
func getRecoverySuggestion(typ ErrorCategory, statusCode int) RecoverySuggestion {
	switch typ {
	case TimeoutError:
		return RecoverySuggestion{
			Action:   "Increase timeout or retry with backoff",
			Steps:    []string{"Check network latency", "Verify service availability", "Increase timeout configuration"},
			Severity: SeverityMedium,
		}
	case NetworkError:
		return RecoverySuggestion{
			Action:   "Verify network connectivity and endpoint availability",
			Steps:    []string{"Check DNS resolution", "Verify firewall rules", "Confirm endpoint is reachable"},
			Severity: SeverityHigh,
		}
	case DatabaseError:
		return RecoverySuggestion{
			Action:   "Check database connectivity and query execution",
			Steps:    []string{"Verify database connection", "Check query syntax", "Review database logs"},
			Severity: SeverityHigh,
		}
	case ParseError:
		return RecoverySuggestion{
			Action:   "Verify data format and schema compatibility",
			Steps:    []string{"Check input data format", "Verify schema hasn't changed", "Validate against schema"},
			Severity: SeverityHigh,
		}
	case ClientError:
		return RecoverySuggestion{
			Action:   "Fix client request format or authentication",
			Steps:    []string{"Validate request format", "Check authentication credentials", "Verify request parameters"},
			Severity: SeverityHigh,
		}
	case ServerError:
		return RecoverySuggestion{
			Action:   "Retry with exponential backoff",
			Steps:    []string{"Implement retry logic", "Check service status", "Monitor error rate"},
			Severity: SeverityMedium,
		}
	case AuthError:
		return RecoverySuggestion{
			Action:   "Refresh authentication credentials",
			Steps:    []string{"Verify credentials are valid", "Refresh tokens", "Check permissions"},
			Severity: SeverityHigh,
		}
	case ConfigError:
		return RecoverySuggestion{
			Action:   "Fix configuration issues",
			Steps:    []string{"Review configuration files", "Check environment variables", "Verify required settings"},
			Severity: SeverityCritical,
		}
	case ResourceError:
		return RecoverySuggestion{
			Action:   "Address resource exhaustion",
			Steps:    []string{"Check resource availability", "Increase resource limits", "Scale capacity"},
			Severity: SeverityCritical,
		}
	default:
		return RecoverySuggestion{
			Action:   "Investigate error details",
			Steps:    []string{"Review error message", "Check stack trace", "Verify operation context"},
			Severity: SeverityLow,
		}
	}
}
