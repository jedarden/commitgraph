# Parsing Error Constructor Signatures

This document provides a comprehensive reference for all parsing error constructor signatures found in the commitgraph codebase.

## Overview

The commitgraph project uses a structured error system with multiple layers:
- **Core error types** (`pkg/errors/types.go`) - Base error categories and structured error type
- **Helper constructors** (`pkg/errors/helpers.go`) - Specialized parsing error constructors
- **Ingest logging** (`pkg/ingestlog/logger.go`) - Error classification and logging for ingest operations

## Core Error Types

### 1. `NewError()` - Generic Structured Error Constructor

**Location:** `pkg/errors/types.go:260`

**Signature:**
```go
func NewError(
    typ ErrorCategory,
    severity SeverityLevel,
    message, code, component, operation string,
    opts ...ErrorContextOption
) *StructuredError
```

**Parameters:**
- `typ` (ErrorCategory): The error category (use `ParseError` for parsing errors)
- `severity` (SeverityLevel): Error severity level (SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo)
- `message` (string): Human-readable error message
- `code` (string): Machine-readable error code
- `component` (string): Component/package where error occurred
- `operation` (string): Operation being performed
- `opts` (...ErrorContextOption): Optional context functions (WithCommitSHAOption, WithPositionOption, WithEmailOption, etc.)

**Returns:**
- `*StructuredError`: A new structured error instance

**Example:**
```go
err := errors.NewError(
    errors.ParseError,
    errors.SeverityHigh,
    "failed to parse commit data",
    "PARSE_COMMIT_DATA",
    "pkg/ingest",
    "parseCommit",
    errors.WithCommitSHAOption("abc123"),
    errors.WithPositionOption(1234),
)
```

---

## Helper Constructors

### 2. `ParseErrorfWithCommit()` - Parse Error with Commit SHA

**Location:** `pkg/errors/helpers.go:48`

**Signature:**
```go
func ParseErrorfWithCommit(
    component, operation, dataType, commitSHA, format string,
    args ...interface{}
) *StructuredError
```

**Parameters:**
- `component` (string): Component/package where error occurred
- `operation` (string): Operation being performed
- `dataType` (string): Type of data being parsed (e.g., "JSON", "commit", "tree")
- `commitSHA` (string): Commit SHA associated with the error (can be empty)
- `format` (string): Format string for error message
- `args` (...interface{}): Arguments for format string

**Returns:**
- `*StructuredError`: A new parse error with commit SHA context

**Example:**
```go
err := errors.ParseErrorfWithCommit(
    "pkg/ingest",
    "parseCommit",
    "commit object",
    "abc123def",
    "invalid tree entry at position %d",
    42,
)
```

---

### 3. `ParseErrorf()` - Parse Error (Deprecated)

**Location:** `pkg/errors/helpers.go:76`

**Signature:**
```go
func ParseErrorf(
    component, operation, dataType, format string,
    args ...interface{}
) *StructuredError
```

**Parameters:**
- `component` (string): Component/package where error occurred
- `operation` (string): Operation being performed
- `dataType` (string): Type of data being parsed
- `format` (string): Format string for error message
- `args` (...interface{}): Arguments for format string

**Returns:**
- `*StructuredError`: A new parse error

**Note:** Deprecated - Use `ParseErrorfWithCommit()` to include commit SHA context

**Example:**
```go
err := errors.ParseErrorf(
    "pkg/ingest",
    "parseCommit",
    "JSON",
    "unexpected character at offset %d",
    123,
)
```

---

### 4. `JSONParseErrorWithCommit()` - JSON Parsing Error with Commit SHA

**Location:** `pkg/errors/helpers.go:81`

**Signature:**
```go
func JSONParseErrorWithCommit(
    component, operation, commitSHA string
) *StructuredError
```

**Parameters:**
- `component` (string): Component/package where error occurred
- `operation` (string): Operation being performed
- `commitSHA` (string): Commit SHA associated with the error

**Returns:**
- `*StructuredError`: A new JSON parsing error with commit SHA context

**Example:**
```go
err := errors.JSONParseErrorWithCommit(
    "pkg/ingest",
    "parseCommit",
    "abc123def",
)
```

---

### 5. `JSONParseError()` - JSON Parsing Error (Deprecated)

**Location:** `pkg/errors/helpers.go:87`

**Signature:**
```go
func JSONParseError(component, operation string) *StructuredError
```

**Parameters:**
- `component` (string): Component/package where error occurred
- `operation` (string): Operation being performed

**Returns:**
- `*StructuredError`: A new JSON parsing error

**Note:** Deprecated - Use `JSONParseErrorWithCommit()` to include commit SHA context

**Example:**
```go
err := errors.JSONParseError(
    "pkg/ingest",
    "parseCommit",
)
```

---

## Error Classification

### 6. `classifyError()` - Automatic Error Type Classification

**Location:** `pkg/ingestlog/logger.go:734`

**Signature:**
```go
func classifyError(err error, statusCode int) string
```

**Parameters:**
- `err` (error): The error to classify (can be nil)
- `statusCode` (int): HTTP status code (0 if not applicable)

**Returns:**
- `string`: Error type ("timeout", "network", "parse_error", "client_error", "server_error", "unknown", or "" for nil errors)

**Parse Error Detection:**
The function identifies parse errors by checking for these patterns in error messages:
- "invalid JSON"
- "cannot unmarshal"
- "invalid character"
- "unmarshal"
- "parse error"

**Example:**
```go
err := fmt.Errorf("invalid JSON: unexpected token")
errorType := classifyError(err, 0)
// errorType = "parse_error"
```

---

## Structured Error Type

### 7. `StructuredError` - Main Error Type

**Location:** `pkg/errors/types.go:112`

**Type Definition:**
```go
type StructuredError struct {
    // Core error information
    Type      ErrorCategory
    Severity  SeverityLevel
    Message   string
    Code      string

    // Context information
    Component string
    Operation string
    Context   ErrorContext

    // Domain-specific context fields
    CommitSHA  string
    Position   int64
    Email      string
    TraceID    string
    RecordKey  string

    // Technical details
    Cause      error
    StackTrace string
    Timestamp  time.Time

    // Retry/Recovery information
    Retryable   bool
    RetryPolicy RetryPolicy
    Recovery    RecoverySuggestion

    // Additional metadata
    Metadata map[string]interface{}
}
```

**Constructor Methods:**
- `NewError()` - Creates new structured error
- `WrapError()` - Wraps existing error with context
- `WithContext()` - Adds context information
- `WithMetadata()` - Adds metadata
- `WithRetryPolicy()` - Sets retry policy
- `WithRecovery()` - Sets recovery suggestion
- `WithCommitSHA()` - Sets commit SHA context
- `WithPosition()` - Sets position context
- `WithEmail()` - Sets email context
- `WithTraceID()` - Sets trace ID context
- `WithRecordKey()` - Sets record key context

---

## Constants

### Error Categories

**Location:** `pkg/errors/types.go:12`

```go
const (
    ValidationError ErrorCategory = "validation_error"
    ParseError      ErrorCategory = "parse_error"      // ← Parsing error category
    DatabaseError   ErrorCategory = "database_error"
    NetworkError    ErrorCategory = "network_error"
    TimeoutError    ErrorCategory = "timeout_error"
    ClientError     ErrorCategory = "client_error"
    ServerError     ErrorCategory = "server_error"
    AuthError       ErrorCategory = "authentication_error"
    ConfigError     ErrorCategory = "configuration_error"
    ResourceError   ErrorCategory = "resource_error"
    ConcurrencyError ErrorCategory = "concurrency_error"
    UnknownError    ErrorCategory = "unknown_error"
)
```

### Severity Levels

**Location:** `pkg/errors/types.go:42`

```go
const (
    SeverityCritical SeverityLevel = "critical"
    SeverityHigh     SeverityLevel = "high"
    SeverityMedium   SeverityLevel = "medium"
    SeverityLow      SeverityLevel = "low"
    SeverityInfo     SeverityLevel = "info"
)
```

---

## Context Options

### 8. Functional Options for Context

**Location:** `pkg/errors/types.go:221`

**Available Options:**
```go
func WithCommitSHAOption(commitSHA string) ErrorContextOption
func WithPositionOption(position int64) ErrorContextOption
func WithEmailOption(email string) ErrorContextOption
func WithTraceIDOption(traceID string) ErrorContextOption
func WithRecordKeyOption(recordKey string) ErrorContextOption
```

**Example:**
```go
err := errors.NewError(
    errors.ParseError,
    errors.SeverityHigh,
    "failed to parse",
    "PARSE_ERROR",
    "pkg/ingest",
    "parse",
    errors.WithCommitSHAOption("abc123"),
    errors.WithPositionOption(456),
    errors.WithEmailOption("user@example.com"),
)
```

---

## Error Context Types

### 9. `ErrorContext` - Additional Context Information

**Location:** `pkg/errors/types.go:98`

```go
type ErrorContext struct {
    UserID      string
    RequestID   string
    SessionID   string
    Endpoint    string
    StatusCode  int
    Query       string
    Package     string
    File        string
    Line        int
    Extra       map[string]string
}
```

---

## Usage Patterns

### Pattern 1: Creating a Parse Error with Context

```go
err := errors.ParseErrorfWithCommit(
    "pkg/ingest",
    "parseCommit",
    "commit object",
    commitSHA,
    "invalid tree entry: %v",
    treeEntry,
)
```

### Pattern 2: Creating a JSON Parse Error

```go
if err := json.Unmarshal(data, &result); err != nil {
    return errors.JSONParseErrorWithCommit(
        "pkg/ingest",
        "parseCommit",
        commitSHA,
    )
}
```

### Pattern 3: Creating a Generic Parse Error

```go
err := errors.NewError(
    errors.ParseError,
    errors.SeverityHigh,
    fmt.Sprintf("failed to parse %s at position %d", dataType, position),
    "PARSE_DATA",
    "pkg/parser",
    "parseData",
    errors.WithPositionOption(position),
    errors.WithCommitSHAOption(commitSHA),
)
```

### Pattern 4: Wrapping an Existing Error

```go
if err := parseCommit(data); err != nil {
    baseErr := errors.StructuredError{
        Type:      errors.ParseError,
        Severity:  errors.SeverityHigh,
        Message:   "failed to parse commit",
        Code:      "PARSE_COMMIT",
        Component: "pkg/ingest",
        Operation: "parseCommit",
    }
    return errors.WrapError(err, baseErr)
}
```

---

## Summary Table

| Constructor | Purpose | Commit SHA Support | Deprecated |
|-------------|---------|-------------------|------------|
| `NewError()` | Generic structured error | Via options | No |
| `ParseErrorfWithCommit()` | Parse error with formatting | Yes | No |
| `ParseErrorf()` | Parse error with formatting | No | **Yes** |
| `JSONParseErrorWithCommit()` | JSON parsing error | Yes | No |
| `JSONParseError()` | JSON parsing error | No | **Yes** |

---

## Related Documentation

- [pkg/errors/types.go](../pkg/errors/types.go) - Core error type definitions
- [pkg/errors/helpers.go](../pkg/errors/helpers.go) - Helper constructor functions
- [pkg/ingestlog/logger.go](../pkg/ingestlog/logger.go) - Error classification and logging
- [pkg/warmstart/error.go](../pkg/warmstart/error.go) - Warmstart-specific error types
