// Package errors provides helper functions for creating common structured errors.
package errors

import (
	"fmt"
	"time"
)

// ValidationErrorf creates a validation error with formatted message.
func ValidationErrorf(component, operation, field, format string, args ...interface{}) *StructuredError {
	message := fmt.Sprintf(format, args...)
	if field != "" {
		message = fmt.Sprintf("validation failed for field '%s': %s", field, message)
	}

	code := generateErrorCode(component, ValidationError, 0)

	return &StructuredError{
		Type:      ValidationError,
		Severity:  SeverityHigh,
		Message:   message,
		Code:      code,
		Component: component,
		Operation: operation,
		Context: ErrorContext{
			Package: component,
			Extra: map[string]string{
				"field": field,
			},
		},
		Timestamp:  getCurrentTimestamp(),
		Retryable:  false,
		Metadata:   make(map[string]interface{}),
	}
}

// RequiredFieldError creates an error for missing required fields.
func RequiredFieldError(component, operation, field string) *StructuredError {
	return ValidationErrorf(component, operation, field, "field is required")
}

// InvalidFormatError creates an error for invalid field formats.
func InvalidFormatError(component, operation, field, expectedFormat string) *StructuredError {
	return ValidationErrorf(component, operation, field, "invalid format (expected %s)", expectedFormat)
}

// ParseErrorf creates a parse error with formatted message.
func ParseErrorf(component, operation, dataType, format string, args ...interface{}) *StructuredError {
	message := fmt.Sprintf(format, args...)
	message = fmt.Sprintf("failed to parse %s: %s", dataType, message)

	code := generateErrorCode(component, ParseError, 0)

	return &StructuredError{
		Type:      ParseError,
		Severity:  SeverityHigh,
		Message:   message,
		Code:      code,
		Component: component,
		Operation: operation,
		Context: ErrorContext{
			Package: component,
			Extra: map[string]string{
				"data_type": dataType,
			},
		},
		Timestamp:  getCurrentTimestamp(),
		Retryable:  false,
		Metadata:   make(map[string]interface{}),
	}
}

// JSONParseError creates an error for JSON parsing failures.
func JSONParseError(component, operation string) *StructuredError {
	return ParseErrorf(component, operation, "JSON", "invalid JSON structure")
}

// DatabaseErrorf creates a database error with formatted message.
func DatabaseErrorf(component, operation, query, format string, args ...interface{}) *StructuredError {
	message := fmt.Sprintf(format, args...)

	code := generateErrorCode(component, DatabaseError, 0)

	ctx := ErrorContext{
		Package: component,
		Query:   query,
	}
	if query == "" {
		ctx.Query = "[query not provided]"
	}

	return &StructuredError{
		Type:      DatabaseError,
		Severity:  SeverityHigh,
		Message:   message,
		Code:      code,
		Component: component,
		Operation: operation,
		Context:   ctx,
		Timestamp: getCurrentTimestamp(),
		Retryable: true,
		RetryPolicy: RetryPolicy{
			MaxRetries:   3,
			InitialDelay: defaultInitialDelay(),
			MaxDelay:     defaultMaxDelay(),
			Multiplier:   2.0,
			Strategy:     RetryStrategyExponential,
		},
		Metadata: make(map[string]interface{}),
	}
}

// DatabaseConnectionError creates a database connection error.
func DatabaseConnectionError(component, operation, dataSource string) *StructuredError {
	msg := fmt.Sprintf("failed to connect to database: %s", dataSource)
	return DatabaseErrorf(component, operation, "", "%s", msg)
}

// DatabaseQueryError creates a database query execution error.
func DatabaseQueryError(component, operation, query, reason string) *StructuredError {
	return DatabaseErrorf(component, operation, query, "query execution failed: %s", reason)
}

// NetworkErrorf creates a network error with formatted message.
func NetworkErrorf(component, operation, endpoint, format string, args ...interface{}) *StructuredError {
	message := fmt.Sprintf(format, args...)
	message = fmt.Sprintf("network error for endpoint %s: %s", endpoint, message)

	code := generateErrorCode(component, NetworkError, 0)

	return &StructuredError{
		Type:      NetworkError,
		Severity:  SeverityHigh,
		Message:   message,
		Code:      code,
		Component: component,
		Operation: operation,
		Context: ErrorContext{
			Package:  component,
			Endpoint: endpoint,
		},
		Timestamp:  getCurrentTimestamp(),
		Retryable: true,
		RetryPolicy: RetryPolicy{
			MaxRetries:   3,
			InitialDelay: defaultInitialDelay(),
			MaxDelay:     defaultMaxDelay(),
			Multiplier:   2.0,
			Strategy:     RetryStrategyExponential,
		},
		Metadata: make(map[string]interface{}),
	}
}

// ConnectionRefusedError creates a connection refused error.
func ConnectionRefusedError(component, operation, endpoint string) *StructuredError {
	return NetworkErrorf(component, operation, endpoint, "connection refused")
}

// DNSError creates a DNS resolution error.
func DNSError(component, operation, hostname string) *StructuredError {
	return NetworkErrorf(component, operation, hostname, "DNS resolution failed")
}

// TimeoutErrorf creates a timeout error with formatted message.
func TimeoutErrorf(component, operation, target, format string, args ...interface{}) *StructuredError {
	message := fmt.Sprintf(format, args...)
	message = fmt.Sprintf("timeout error for %s: %s", target, message)

	code := generateErrorCode(component, TimeoutError, 0)

	return &StructuredError{
		Type:      TimeoutError,
		Severity:  SeverityMedium,
		Message:   message,
		Code:      code,
		Component: component,
		Operation: operation,
		Context: ErrorContext{
			Package: component,
		},
		Timestamp:  getCurrentTimestamp(),
		Retryable: true,
		RetryPolicy: RetryPolicy{
			MaxRetries:   2,
			InitialDelay: defaultInitialDelay(),
			MaxDelay:     defaultMaxDelay(),
			Multiplier:   1.5,
			Strategy:     RetryStrategyLinear,
		},
		Metadata: make(map[string]interface{}),
	}
}

// HTTPTimeoutError creates an HTTP timeout error.
func HTTPTimeoutError(component, operation, url string, timeoutSeconds int) *StructuredError {
	return TimeoutErrorf(component, operation, url, "HTTP request timed out after %d seconds", timeoutSeconds)
}

// DatabaseTimeoutError creates a database timeout error.
func DatabaseTimeoutError(component, operation, query string, timeoutSeconds int) *StructuredError {
	return TimeoutErrorf(component, operation, "database query", "query timed out after %d seconds", timeoutSeconds)
}

// HTTPError creates an HTTP error based on status code.
func HTTPError(component, operation, url string, statusCode int, responseBody string) *StructuredError {
	var typ ErrorCategory
	var severity SeverityLevel
	var retryable bool

	switch {
	case statusCode >= 400 && statusCode < 500:
		typ = ClientError
		severity = SeverityHigh
		retryable = (statusCode == 429) // Only 429 (Too Many Requests) is retryable
	case statusCode >= 500 && statusCode < 600:
		typ = ServerError
		severity = SeverityMedium
		retryable = true
	default:
		typ = UnknownError
		severity = SeverityLow
		retryable = false
	}

	code := generateErrorCode(component, typ, statusCode)
	message := fmt.Sprintf("HTTP %d error for %s", statusCode, url)
	if responseBody != "" {
		message = fmt.Sprintf("%s: %s", message, responseBody)
	}

	retryPolicy := DefaultRetryPolicy()
	if !retryable {
		retryPolicy = RetryPolicy{Strategy: RetryStrategyNone}
	}

	return &StructuredError{
		Type:      typ,
		Severity:  severity,
		Message:   message,
		Code:      code,
		Component: component,
		Operation: operation,
		Context: ErrorContext{
			Package:     component,
			Endpoint:    url,
			StatusCode:  statusCode,
			Extra: map[string]string{
				"response_body": responseBody,
			},
		},
		Timestamp:  getCurrentTimestamp(),
		Retryable:  retryable,
		RetryPolicy: retryPolicy,
		Metadata:   make(map[string]interface{}),
	}
}

// AuthErrorf creates an authentication/authorization error.
func AuthErrorf(component, operation, format string, args ...interface{}) *StructuredError {
	message := fmt.Sprintf(format, args...)

	code := generateErrorCode(component, AuthError, 0)

	return &StructuredError{
		Type:      AuthError,
		Severity:  SeverityHigh,
		Message:   message,
		Code:      code,
		Component: component,
		Operation: operation,
		Context: ErrorContext{
			Package: component,
		},
		Timestamp:  getCurrentTimestamp(),
		Retryable: false,
		Metadata:  make(map[string]interface{}),
	}
}

// UnauthorizedError creates a 401 Unauthorized error.
func UnauthorizedError(component, operation string) *StructuredError {
	return AuthErrorf(component, operation, "authentication required")
}

// ForbiddenError creates a 403 Forbidden error.
func ForbiddenError(component, operation, resource string) *StructuredError {
	return AuthErrorf(component, operation, "access forbidden to resource: %s", resource)
}

// TokenExpiredError creates a token expiration error.
func TokenExpiredError(component, operation string) *StructuredError {
	return AuthErrorf(component, operation, "authentication token has expired")
}

// ConfigErrorf creates a configuration error with formatted message.
func ConfigErrorf(component, format string, args ...interface{}) *StructuredError {
	message := fmt.Sprintf(format, args...)

	code := generateErrorCode(component, ConfigError, 0)

	return &StructuredError{
		Type:      ConfigError,
		Severity:  SeverityCritical,
		Message:   message,
		Code:      code,
		Component: component,
		Context: ErrorContext{
			Package: component,
		},
		Timestamp:  getCurrentTimestamp(),
		Retryable: false,
		Metadata:  make(map[string]interface{}),
	}
}

// MissingConfigError creates an error for missing configuration values.
func MissingConfigError(component, configKey string) *StructuredError {
	return ConfigErrorf(component, "missing required configuration: %s", configKey)
}

// InvalidConfigError creates an error for invalid configuration values.
func InvalidConfigError(component, configKey, reason string) *StructuredError {
	return ConfigErrorf(component, "invalid configuration value for %s: %s", configKey, reason)
}

// ResourceErrorf creates a resource error with formatted message.
func ResourceErrorf(component, operation, resourceType, format string, args ...interface{}) *StructuredError {
	message := fmt.Sprintf(format, args...)
	message = fmt.Sprintf("resource error (%s): %s", resourceType, message)

	code := generateErrorCode(component, ResourceError, 0)

	return &StructuredError{
		Type:      ResourceError,
		Severity:  SeverityCritical,
		Message:   message,
		Code:      code,
		Component: component,
		Operation: operation,
		Context: ErrorContext{
			Package: component,
			Extra: map[string]string{
				"resource_type": resourceType,
			},
		},
		Timestamp:  getCurrentTimestamp(),
		Retryable: false,
		Metadata:  make(map[string]interface{}),
	}
}

// MemoryExhaustedError creates a memory exhaustion error.
func MemoryExhaustedError(component, operation string) *StructuredError {
	return ResourceErrorf(component, operation, "memory", "memory exhausted")
}

// DiskSpaceExhaustedError creates a disk space exhaustion error.
func DiskSpaceExhaustedError(component, operation, path string) *StructuredError {
	return ResourceErrorf(component, operation, "disk", "disk space exhausted at %s", path)
}

// ConnectionPoolExhaustedError creates a connection pool exhaustion error.
func ConnectionPoolExhaustedError(component, operation, poolName string) *StructuredError {
	return ResourceErrorf(component, operation, "connection_pool", "connection pool exhausted: %s", poolName)
}

// ConcurrencyErrorf creates a concurrency error with formatted message.
func ConcurrencyErrorf(component, operation, format string, args ...interface{}) *StructuredError {
	message := fmt.Sprintf(format, args...)

	code := generateErrorCode(component, ConcurrencyError, 0)

	return &StructuredError{
		Type:      ConcurrencyError,
		Severity:  SeverityMedium,
		Message:   message,
		Code:      code,
		Component: component,
		Operation: operation,
		Context: ErrorContext{
			Package: component,
		},
		Timestamp:  getCurrentTimestamp(),
		Retryable: true,
		RetryPolicy: RetryPolicy{
			MaxRetries:   3,
			InitialDelay: defaultInitialDelay(),
			MaxDelay:     defaultMaxDelay(),
			Multiplier:   2.0,
			Strategy:     RetryStrategyExponential,
		},
		Metadata: make(map[string]interface{}),
	}
}

// DeadlockError creates a deadlock detection error.
func DeadlockError(component, operation, resource string) *StructuredError {
	return ConcurrencyErrorf(component, operation, "deadlock detected on resource: %s", resource)
}

// LockConflictError creates a lock conflict error.
func LockConflictError(component, operation, resource string) *StructuredError {
	return ConcurrencyErrorf(component, operation, "lock conflict on resource: %s", resource)
}

// GetHTTPStatus returns the HTTP status code for an error, if applicable.
func GetHTTPStatus(err error) int {
	if structuredErr, ok := err.(*StructuredError); ok {
		return structuredErr.Context.StatusCode
	}
	return 0
}

// IsHTTPClientError returns true if the error is an HTTP 4xx client error.
func IsHTTPClientError(err error) bool {
	if structuredErr, ok := err.(*StructuredError); ok {
		return structuredErr.Context.StatusCode >= 400 && structuredErr.Context.StatusCode < 500
	}
	return false
}

// IsHTTPServerError returns true if the error is an HTTP 5xx server error.
func IsHTTPServerError(err error) bool {
	if structuredErr, ok := err.(*StructuredError); ok {
		return structuredErr.Context.StatusCode >= 500 && structuredErr.Context.StatusCode < 600
	}
	return false
}

// IsRetryable returns true if the error is retryable.
func IsRetryable(err error) bool {
	if structuredErr, ok := err.(*StructuredError); ok {
		return structuredErr.Retryable
	}
	return false
}

// GetSeverity returns the severity level of an error.
func GetSeverity(err error) SeverityLevel {
	if structuredErr, ok := err.(*StructuredError); ok {
		return structuredErr.Severity
	}
	return SeverityLow
}

// GetType returns the error category of an error.
func GetType(err error) ErrorCategory {
	if structuredErr, ok := err.(*StructuredError); ok {
		return structuredErr.Type
	}
	return UnknownError
}

// getCurrentTimestamp returns the current UTC timestamp.
func getCurrentTimestamp() time.Time {
	return time.Now().UTC()
}

// defaultInitialDelay returns the default initial retry delay.
func defaultInitialDelay() time.Duration {
	return time.Second * 1
}

// defaultMaxDelay returns the default maximum retry delay.
func defaultMaxDelay() time.Duration {
	return time.Second * 10
}
