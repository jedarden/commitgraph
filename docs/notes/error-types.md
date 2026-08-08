# Error Type Hierarchy and Categorization System

## Overview

This document defines the comprehensive error type hierarchy and categorization system for the commitgraph project. The system provides structured error types, severity levels, and metadata wrapping to make errors distinguishable in logs and便于 monitoring and debugging.

## Error Type Categories

### 1. ValidationError
**Description**: Errors that occur when input validation fails, such as missing required fields, invalid formats, or constraint violations.

**Examples**:
- Empty or missing required fields (email, username)
- Invalid email format
- Constraint violations (duplicate entries)
- Data type mismatches

**Severity**: `medium`

**Retry Policy**: Non-retryable (requires input correction)

---

### 2. ParseError
**Description**: Errors that occur when parsing structured data (JSON, XML, CSV) fails due to malformed content or schema changes.

**Examples**:
- JSON unmarshaling failures
- Invalid character errors
- Schema mismatches
- Format incompatibilities

**Severity**: `high`

**Retry Policy**: Non-retryable (requires data/schema fix)

---

### 3. DatabaseError
**Description**: Errors that occur during database operations, including connection issues, query failures, and transaction problems.

**Subcategories**:
- `database_connection`: Connection establishment failures
- `database_query`: Query execution failures
- `database_transaction`: Transaction commit/rollback failures
- `database_constraint`: Constraint violations
- `database_timeout`: Query timeouts

**Severity**: `high` for connection/constraint errors, `medium` for query/transaction errors

**Retry Policy**: Retryable with exponential backoff for connection/timeout errors; non-retryable for constraint violations

---

### 4. NetworkError
**Description**: Errors that occur during network operations, including DNS resolution failures, connection refused, and timeout errors.

**Subcategories**:
- `network_dns`: DNS resolution failures
- `network_connection`: Connection refused/reset
- `network_timeout`: Operation timeout
- `network_firewall`: Firewall blocking

**Severity**: `high`

**Retry Policy**: Retryable with exponential backoff

---

### 5. TimeoutError
**Description**: Errors that occur when an operation exceeds its configured time limit.

**Examples**:
- HTTP client timeouts
- Database query timeouts
- Context deadline exceeded
- Slow external service responses

**Severity**: `medium`

**Retry Policy**: Retryable with increased timeout

---

### 6. ClientError
**Description**: HTTP 4xx errors indicating client-side request problems.

**Subcategories**:
- `client_error_400`: Bad Request
- `client_error_401`: Unauthorized
- `client_error_403`: Forbidden
- `client_error_404`: Not Found
- `client_error_429`: Too Many Requests

**Severity**: `high` (except 429 which is `medium`)

**Retry Policy**: Non-retryable (except 429 which should retry with backoff)

---

### 7. ServerError
**Description**: HTTP 5xx errors indicating server-side problems.

**Subcategories**:
- `server_error_500`: Internal Server Error
- `server_error_502`: Bad Gateway
- `server_error_503`: Service Unavailable
- `server_error_504`: Gateway Timeout

**Severity**: `medium`

**Retry Policy**: Retryable with exponential backoff

---

### 8. AuthenticationError
**Description**: Errors related to authentication and authorization failures.

**Examples**:
- Invalid credentials
- Token expiration
- Permission denied
- Session invalidation

**Severity**: `high`

**Retry Policy**: Non-retryable (requires credential refresh)

---

### 9. ConfigurationError
**Description**: Errors related to misconfiguration, missing environment variables, or invalid settings.

**Examples**:
- Missing required environment variables
- Invalid configuration values
- File not found (config files)
- Parse errors in configuration

**Severity**: `critical`

**Retry Policy**: Non-retryable (requires configuration fix)

---

### 10. ResourceError
**Description**: Errors related to resource exhaustion or unavailability.

**Subcategories**:
- `resource_memory`: Memory allocation failures
- `resource_disk`: Disk space exhaustion
- `resource_connection`: Connection pool exhaustion
- `resource_quota`: Quota exceeded

**Severity**: `critical`

**Retry Policy**: Conditionally retryable (depends on resource availability)

---

### 11. ConcurrencyError
**Description**: Errors that occur due to concurrent access conflicts, race conditions, or locking issues.

**Examples**:
- Database row locking conflicts
- Optimistic locking failures
- Race conditions
- Deadlock detection

**Severity**: `medium`

**Retry Policy**: Retryable with backoff

---

### 12. UnknownError
**Description**: Errors that cannot be classified into any specific category.

**Severity**: `low`

**Retry Policy**: Case-by-case basis

---

## Severity Levels

### Critical
**Definition**: Errors that cause complete service failure or data corruption and require immediate intervention.

**Characteristics**:
- Service is completely down
- Data integrity at risk
- Requires immediate human intervention

**Examples**:
- Configuration errors preventing startup
- Resource exhaustion (memory, disk)
- Database connection failures

**Action**: Immediate alerting and intervention required

---

### High
**Definition**: Errors that significantly impact functionality but don't cause complete service failure.

**Characteristics**:
- Major features unavailable
- User experience severely degraded
- Requires prompt attention

**Examples**:
- Network connectivity issues
- Authentication failures
- Parse errors blocking data processing

**Action**: Prompt investigation and remediation required

---

### Medium
**Definition**: Errors that impact functionality but have workarounds or limited scope.

**Characteristics**:
- Some features unavailable
- Partial user impact
- Can be worked around temporarily

**Examples**:
- Timeout errors (retriable)
- Individual server errors
- Concurrency conflicts

**Action**: Investigation and remediation within normal operational window

---

### Low
**Definition**: Errors that have minimal impact or are edge cases with clear workarounds.

**Characteristics**:
- Limited user impact
- Clear documentation/workaround exists
- Doesn't block core functionality

**Examples**:
- Unknown error types
- Occasional transient failures
- Edge case handling

**Action**: Monitor and investigate during regular maintenance

---

### Info
**Definition**: Not errors but informational events that may be useful for debugging or monitoring.

**Characteristics**:
- No negative impact
- Useful for debugging
- Performance monitoring

**Examples**:
- Successful operations with warnings
- Performance metrics
- State transitions

**Action**: No action required, logging only

---

## Enhanced Error Struct Design

### StructuredError
The main error type that wraps errors with comprehensive metadata:

```go
type StructuredError struct {
    // Core error information
    Type         ErrorCategory   // Categorized error type
    Severity     SeverityLevel   // Error severity level
    Message      string          // Human-readable error message
    Code         string          // Machine-readable error code
    
    // Context information
    Component    string          // Component/package where error occurred
    Operation    string          // Operation being performed
    Context      ErrorContext    // Additional contextual information
    
    // Technical details
    Cause        error           // Underlying error (for wrapping)
    StackTrace   string          // Stack trace at error site
    Timestamp    time.Time       // When the error occurred
    
    // Retry/Recovery information
    Retryable    bool            // Whether this error is retryable
    RetryPolicy  RetryPolicy     // Retry strategy if retryable
    Recovery     RecoverySuggestion // Suggested recovery actions
    
    // Additional metadata
    Metadata     map[string]interface{} // Additional context
}
```

### ErrorCategory
```go
type ErrorCategory string

const (
    ValidationError    ErrorCategory = "validation_error"
    ParseError        ErrorCategory = "parse_error"
    DatabaseError     ErrorCategory = "database_error"
    NetworkError      ErrorCategory = "network_error"
    TimeoutError      ErrorCategory = "timeout_error"
    ClientError       ErrorCategory = "client_error"
    ServerError       ErrorCategory = "server_error"
    AuthError         ErrorCategory = "authentication_error"
    ConfigError       ErrorCategory = "configuration_error"
    ResourceError     ErrorCategory = "resource_error"
    ConcurrencyError  ErrorCategory = "concurrency_error"
    UnknownError      ErrorCategory = "unknown_error"
)
```

### SeverityLevel
```go
type SeverityLevel string

const (
    SeverityCritical SeverityLevel = "critical"
    SeverityHigh     SeverityLevel = "high"
    SeverityMedium   SeverityLevel = "medium"
    SeverityLow      SeverityLevel = "low"
    SeverityInfo     SeverityLevel = "info"
)
```

### RetryPolicy
```go
type RetryPolicy struct {
    MaxRetries    int           // Maximum number of retry attempts
    InitialDelay  time.Duration // Initial delay before first retry
    MaxDelay      time.Duration // Maximum delay between retries
    Multiplier    float64       // Backoff multiplier (for exponential backoff)
    Strategy      RetryStrategy // Retry strategy to use
}

type RetryStrategy string

const (
    RetryStrategyNone         RetryStrategy = "none"
    RetryStrategyLinear      RetryStrategy = "linear"
    RetryStrategyExponential RetryStrategy = "exponential"
)
```

### RecoverySuggestion
```go
type RecoverySuggestion struct {
    Action        string   // Human-readable recovery action
    Steps         []string // Detailed recovery steps
    Documentation string   // Link to relevant documentation
    Severity      SeverityLevel // Recovery urgency
}
```

## Error Creation Patterns

### Direct Construction
```go
err := &StructuredError{
    Type:      ValidationError,
    Severity:  SeverityMedium,
    Message:   "email field is required",
    Component: "user-service",
    Operation: "create-user",
    Retryable: false,
}
```

### Wrapping Existing Errors
```go
originalErr := someOperation()
err := WrapError(originalErr, StructuredError{
    Type:      DatabaseError,
    Severity:  SeverityHigh,
    Component: "user-repository",
    Operation: "save-user",
    Retryable: true,
    RetryPolicy: RetryPolicy{
        MaxRetries:   3,
        InitialDelay: time.Second * 1,
        MaxDelay:     time.Second * 10,
        Multiplier:   2.0,
        Strategy:    RetryStrategyExponential,
    },
})
```

### Classification Helper
```go
err := ClassifyError(originalErr, ClassifyOptions{
    Component:  "api-handler",
    Operation:  "process-request",
    StatusCode: 500,
    Context:    ErrorContext{...},
})
```

## Logging Integration

### Structured Logging Format
The error system integrates with the existing ingestlog package to provide structured logging:

```json
{
  "timestamp": "2026-08-06T15:30:45Z",
  "event_type": "error",
  "error": {
    "type": "database_error",
    "severity": "high",
    "code": "DB_QUERY_FAILED",
    "message": "failed to execute query: connection pool exhausted",
    "component": "user-repository",
    "operation": "find-user",
    "retryable": true,
    "retry_policy": {
      "max_retries": 3,
      "strategy": "exponential"
    },
    "recovery": {
      "action": "Increase connection pool size or retry with backoff",
      "severity": "medium"
    }
  },
  "metadata": {
    "query": "SELECT * FROM users WHERE id = ?",
    "pool_size": 10,
    "active_connections": 10
  }
}
```

## Error Code System

### Code Format
Error codes follow the pattern: `[COMPONENT]_[ERROR_TYPE]_[SPECIFIC]`

### Examples
- `USER_VALIDATION_MISSING_FIELD`: User validation missing required field
- `DB_QUERY_TIMEOUT`: Database query timeout
- `NET_CONNECTION_REFUSED`: Network connection refused
- `PARSE_JSON_INVALID`: JSON parsing failed
- `AUTH_TOKEN_EXPIRED`: Authentication token expired

## Monitoring and Alerting

### Severity-based Alerting
- **Critical**: Immediate paging (24/7)
- **High**: Alert within 15 minutes (24/7)
- **Medium**: Alert within 1 hour (business hours)
- **Low**: Daily digest
- **Info**: No alerting, logging only

### Error Rate Monitoring
Monitor error rates by type and component:
- Errors per minute by type
- Error percentage by operation
- Retry success rates
- Recovery suggestion effectiveness

## Best Practices

### When to Create New Error Types
1. When the error requires different handling logic
2. When the error has different retry characteristics
3. When the error needs different monitoring/alerting
4. When the error represents a distinct failure mode

### Error Context Enrichment
Always include relevant context:
- What operation was being performed
- What component/service was involved
- Relevant IDs (user ID, request ID, etc.)
- Current state information
- Configuration values (sanitized)

### Error Wrapping Guidelines
1. Preserve the original error cause
2. Add context at each wrapping layer
3. Don't double-wrap already structured errors
4. Maintain error chain for debugging

## Exit Code Mapping

### Overview
The commitgraph project uses standardized exit codes for CLI commands to enable programmatic error handling and monitoring. Exit codes follow the Unix convention where 0 indicates success and non-zero values indicate various failure modes.

### Exit Code Constants

```go
const (
    ExitCodeSuccess            = 0  // Successful execution
    ExitCodeError              = 1  // General error (default/fallback)
    ExitCodeInvalidInput       = 2  // Invalid command-line arguments or input
    ExitCodeDatabaseError      = 3  // Database connection or query errors
    ExitCodeNetworkError       = 4  // Network connectivity errors
    ExitCodeTimeoutError       = 5  // Operation timeout
    ExitCodeAuthenticationError = 6 // Authentication/authorization failures
    ExitCodeConfigError        = 7  // Configuration problems
    ExitCodeValidationError    = 8  // Data validation failures
    ExitCodeParseError         = 9  // Data parsing failures
    ExitCodeResourceError      = 10 // Resource exhaustion/unavailability
    ExitCodeConcurrencyError   = 11 // Concurrency/locking issues
    ExitCodeServerError        = 12 // Server-side errors (HTTP 5xx)
)
```

### Error Category to Exit Code Mapping

| Error Category         | Exit Code | Description                          |
|------------------------|-----------|--------------------------------------|
| ValidationError        | 8         | Input validation failures            |
| ParseError             | 9         | Data parsing failures                |
| DatabaseError          | 3         | Database operation failures          |
| NetworkError           | 4         | Network operation failures           |
| TimeoutError           | 5         | Operation timeout failures           |
| ClientError            | 2         | HTTP 4xx client errors (bad input)   |
| ServerError            | 12        | HTTP 5xx server errors                |
| AuthError              | 6         | Authentication/authorization failures |
| ConfigError            | 7         | Configuration problems               |
| ResourceError          | 10        | Resource exhaustion/unavailability    |
| ConcurrencyError       | 11        | Concurrency/locking issues            |
| UnknownError           | 1         | Uncategorized errors (fallback)       |

### Severity to Exit Code Mapping

| Severity Level   | Example Exit Codes | Usage                                |
|------------------|--------------------|--------------------------------------|
| SeverityInfo     | 0                  | Success, no errors                   |
| SeverityLow      | 1, 2, 8, 9         | User input errors, validation issues |
| SeverityMedium   | 5, 11, 12          | Retriable errors (timeouts, server) |
| SeverityHigh     | 3, 4, 6           | System errors requiring attention    |
| SeverityCritical | 7, 10             | Critical failures (config, resources)|

### Usage in CLI Commands

```go
import "github.com/jedarden/commitgraph/pkg/errors"

func main() {
    cliHandler := errors.NewCLIHandler("my-program")
    
    // Handle validation errors
    if requiredField == "" {
        cliHandler.HandleError(errors.RequiredFlagError("field"))
        // Exits with code 8 (ExitCodeValidationError)
    }
    
    // Handle database errors
    if err != nil {
        cliHandler.HandleError(errors.WrapDatabaseConnectionError(err))
        // Exits with code 3 (ExitCodeDatabaseError)
    }
    
    // Success
    cliHandler.Success("Operation completed")
    // Exits with code 0 (ExitCodeSuccess)
}
```

### Programmatic Error Handling

Exit codes enable shell scripts and automation to detect and handle errors:

```bash
#!/bin/bash
# Example shell script using exit codes

my-program || exit_code=$?

case $exit_code in
    0) echo "Success" ;;
    2) echo "Invalid input - check arguments" ;;
    3) echo "Database error - check connection" ;;
    4) echo "Network error - check connectivity" ;;
    8) echo "Validation error - check input format" ;;
    *) echo "Unknown error (code $exit_code)" ;;
esac
```

### Exit Code Rationale

The exit code scheme follows these principles:

1. **Standard conventions**: 0 = success, 1 = general error, 2 = usage/input errors
2. **Categorization**: Related errors share codes (e.g., all DB errors = 3)
3. **Automation-friendly**: Codes enable programmatic error handling
4. **Debugging aids**: Codes map to specific error categories for troubleshooting
5. **Compatibility**: Follows POSIX/Unix conventions for shell integration

### Design Decisions

**Why code 2 for client errors?**
- Follows Unix convention: exit code 2 indicates "command line usage errors"
- Maps 4xx client errors to user-correctable input problems
- Distinguishes user input errors from system errors (3+)

**Why code 3 for database errors?**
- Database is a critical dependency; deserves dedicated code
- Enables monitoring systems to track DB issues separately
- Common failure mode that operators need to identify quickly

**Why code 12 for server errors?**
- Separates server-side failures (5xx) from other errors
- Allows retry logic to distinguish server (retryable) vs client (non-retryable)
- Highest numbered code indicates "most severe" (server infrastructure down)

## Migration Notes

### From Existing Error System
The existing error types in `pkg/ingestlog/logger.go` will be mapped to the new hierarchy:

- `timeout` → `TimeoutError`
- `network` → `NetworkError`
- `parse_error` → `ParseError`
- `client_error` → `ClientError`
- `server_error` → `ServerError`
- `unknown` → `UnknownError`

### Backward Compatibility
The existing `ErrorContext` struct will be enhanced to include the new fields while maintaining backward compatibility through converter functions.
