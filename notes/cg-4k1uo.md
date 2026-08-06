# Error Handling Paths Catalog (cg-4k1uo)

## Overview

This document catalogs all error handling paths in the commitgraph codebase, categorized by operation type and documenting current error formats with required context fields.

## Error Creation Sites

### Summary Statistics
- **Total error creation sites**: 196+ (using `fmt.Errorf` and `errors.New`)
- **Structured error types**: 30+ helper functions in `pkg/errors/helpers.go`
- **Core error categories**: 12 categories defined in `pkg/errors/types.go`

## Error Categories

### 1. Validation Errors (ValidationError)

**Helper Functions:**
- `ValidationErrorf(component, operation, field, format, args...)`
- `ValidationErrorWithRecord(component, operation, field, recordKey, position)`
- `RequiredFieldError(component, operation, field)`
- `InvalidFormatError(component, operation, field, expectedFormat)`

**Current Error Format:**
```go
// ValidationErrorf
message := fmt.Sprintf(format, args...)
if field != "" {
    message = fmt.Sprintf("validation failed for field '%s': %s", field, message)
}

// ValidationErrorWithRecord
message := fmt.Sprintf("validation failed for field '%s'", field)
if recordKey != "" {
    message = fmt.Sprintf("%s for record %s", message, recordKey)
}
if position > 0 {
    message = fmt.Sprintf("%s at position %d", message, position)
}
```

**Usage Sites:**
- `pkg/identity/ingest.go:34` - Email validation
- `pkg/identity/ingest.go:37` - Login validation
- `pkg/identity/ingest.go:43` - Source validation
- `pkg/identity/ingest.go:46` - ResolvedAt validation
- `pkg/pg/repo.go` - Provider/repo validation

**Required Context Fields:**
- `component` - Package/component name
- `operation` - Operation being performed
- `field` - Field that failed validation
- `recordKey` - Record identifier (optional)
- `position` - Position in batch (optional)

### 2. Parse Errors (ParseError)

**Helper Functions:**
- `ParseErrorf(component, operation, dataType, format, args...)`
- `ParseErrorWithRecord(component, operation, dataType, recordKey, position, reason)`
- `JSONParseError(component, operation)`
- `CommitParseError(component, operation, commitSHA, reason)`

**Current Error Format:**
```go
// ParseErrorf
message := fmt.Sprintf(format, args...)
message = fmt.Sprintf("failed to parse %s: %s", dataType, message)

// ParseErrorWithRecord
message := fmt.Sprintf("failed to parse %s", dataType)
if recordKey != "" {
    message = fmt.Sprintf("%s for record %s", message, recordKey)
}
if position > 0 {
    message = fmt.Sprintf("%s at position %d", message, position)
}
if reason != "" {
    message = fmt.Sprintf("%s: %s", message, reason)
}

// CommitParseError
message := fmt.Sprintf("failed to parse commit")
if commitSHA != "" {
    message = fmt.Sprintf("%s for commit %s", message, commitSHA)
}
if reason != "" {
    message = fmt.Sprintf("%s: %s", message, reason)
}
```

**Usage Sites:**
- Currently defined in helpers but no active usage found
- Designed for commit SHA parsing and position handling

**Required Context Fields:**
- `component` - Package/component name
- `operation` - Operation being performed
- `dataType` - Type of data being parsed
- `recordKey` - Record identifier (optional)
- `position` - Position in batch (optional)
- `reason` - Parse failure reason (optional)
- `RecordCommitSHA` - For commit-specific parse errors

### 3. Ingestion Errors (DatabaseError)

**Helper Functions:**
- `DatabaseErrorf(component, operation, query, format, args...)`
- `DatabaseErrorWithEmail(component, operation, query, email, reason)`
- `DatabaseQueryError(component, operation, query, reason)`
- `DatabaseConnectionError(component, operation, dataSource)`

**Current Error Format:**
```go
// DatabaseErrorf
message := fmt.Sprintf(format, args...)

// DatabaseErrorWithEmail
message := fmt.Sprintf("database operation failed")
if email != "" {
    message = fmt.Sprintf("%s for email %s", message, email)
}
if reason != "" {
    message = fmt.Sprintf("%s: %s", message, reason)
}

// Actual usage in pkg/pg/identity.go
return nil, fmt.Errorf("failed to fetch existing email_resolution rows (batch of %d emails, first email %s): %w", len(emails), emails[0], err)
return nil, fmt.Errorf("failed to scan existing email_resolution row for email %s: %w", email, err)
return nil, fmt.Errorf("bulk upsert failed (batch size %d): %w", len(rows), err)
```

**Usage Sites:**
- `pkg/pg/identity.go:124` - Fetch existing rows
- `pkg/pg/identity.go:132` - Scan existing rows
- `pkg/pg/identity.go:145` - Iterate rows
- `pkg/pg/identity.go:213` - Bulk upsert
- `pkg/pg/user_aliases.go` - Bulk upsert and query failures
- `pkg/pg/repo.go` - Exclusion operations

**Required Context Fields:**
- `component` - Package/component name
- `operation` - Operation being performed
- `query` - Database query (sanitized)
- `email` - Email record identifier (optional)
- `reason` - Failure reason (optional)
- `RecordEmail` - For email-specific operations
- `RecordPosition` - For batch operations

### 4. Lookup Errors (DatabaseError/NetworkError)

**Helper Functions:**
- `LookupErrorWithTraceID(component, operation, traceID, recordKey, reason)`
- `ConnectionRefusedError(component, operation, endpoint)`
- `DNSError(component, operation, hostname)`

**Current Error Format:**
```go
// LookupErrorWithTraceID
message := fmt.Sprintf("lookup operation failed")
if traceID != "" {
    message = fmt.Sprintf("%s (trace ID: %s)", message, traceID)
}
if recordKey != "" {
    message = fmt.Sprintf("%s for key %s", message, recordKey)
}
if reason != "" {
    message = fmt.Sprintf("%s: %s", message, reason)
}
```

**Usage Sites:**
- Currently defined in helpers but no active usage found
- Designed for trace ID and record key lookups

**Required Context Fields:**
- `component` - Package/component name
- `operation` - Operation being performed
- `traceID` - Trace identifier
- `recordKey` - Record key for lookup
- `reason` - Failure reason (optional)
- `RecordTraceID` - For trace-specific operations
- `RecordKey` - Generic record key

## Error Context Structure

### ErrorContext Fields (from pkg/errors/types.go)

```go
type ErrorContext struct {
    // Request/Session tracking
    UserID      string            // User ID involved in the operation
    RequestID   string            // Request ID for tracing
    SessionID   string            // Session ID for context

    // HTTP/API context
    Endpoint    string            // API endpoint involved
    StatusCode  int               // HTTP status code (if applicable)

    // Database context
    Query       string            // Database query (sanitized)

    // Code location
    Package     string            // Package/function where error occurred
    File        string            // File where error occurred
    Line        int               // Line number where error occurred

    // Additional context
    Extra       map[string]string // Additional context

    // Record identifiers for debugging
    RecordEmail     string            // Email address for identity resolution records
    RecordCommitSHA string            // Commit SHA for commit records
    RecordTraceID   string            // Trace ID for request tracing
    RecordPosition  int               // Position/index in batch operations
    RecordKey       string            // Generic record key for lookup operations
}
```

## Current Usage Patterns

### Pattern 1: Simple fmt.Errorf (Most Common)

**Location:** Throughout pkg/pg, pkg/identity

```go
// Email validation
return fmt.Errorf("email cannot be empty")

// Field validation with context
return fmt.Errorf("login cannot be empty for email %s", r.Email)

// Database query errors
return nil, fmt.Errorf("failed to fetch existing email_resolution rows (batch of %d emails, first email %s): %w", len(emails), emails[0], err)
```

**Characteristics:**
- Uses standard Go error wrapping (`%w`)
- Includes contextual information inline
- No structured metadata
- Not using the structured error helpers

### Pattern 2: Structured Error Helpers (Not Currently Used)

**Location:** Defined in pkg/errors/helpers.go

```go
// These helpers exist but are NOT currently called in the codebase
ValidationErrorWithRecord("pkg/identity", "ingest", "email", email, position)
ParseErrorWithRecord("pkg/identity", "parse_commit", "commit", sha, position, "invalid hex")
DatabaseErrorWithEmail("pkg/pg", "upsert", query, email, "constraint violation")
LookupErrorWithTraceID("pkg/pg", "lookup", traceID, recordKey, "not found")
```

**Characteristics:**
- Provide structured metadata
- Include context fields for debugging
- Support retry policies
- Currently unused in production code

### Pattern 3: Error Classification (Not Currently Used)

**Location:** pkg/errors/types.go

```go
// ClassifyError function exists but no usage found
ClassifyError(err, ClassifyOptions{
    Component: "pkg/identity",
    Operation: "ingest",
    Context: ErrorContext{
        RecordEmail: email,
        RecordPosition: position,
    },
})
```

**Characteristics:**
- Automatic error categorization
- Stack trace capture
- Retry policy assignment
- Recovery suggestion generation
- Currently unused in production code

## Gap Analysis

### Missing Structured Error Usage

1. **Validation Errors**: Using `fmt.Errorf` instead of `ValidationErrorWithRecord`
2. **Parse Errors**: Commit parsing errors could use `CommitParseError`
3. **Database Errors**: Email operations could use `DatabaseErrorWithEmail`
4. **Lookup Errors**: Trace operations could use `LookupErrorWithTraceID`

### Missing Context Fields

Current error messages include context inline but not in structured fields:

| Current Pattern | Missing Structured Field |
|----------------|-------------------------|
| `"email %s"` → `RecordEmail` | Email identifier |
| `"position %d"` → `RecordPosition` | Batch position |
| `"commit %s"` → `RecordCommitSHA` | Commit identifier |
| `"trace %s"` → `RecordTraceID` | Trace identifier |
| `"key %s"` → `RecordKey` | Generic record key |

## Recommendations

### 1. Adopt Structured Error Helpers

Replace `fmt.Errorf` calls with appropriate structured error helpers:

```go
// Before
return fmt.Errorf("email cannot be empty")

// After
return RequiredFieldError("pkg/identity", "ingest", "email")

// Before
return fmt.Errorf("login cannot be empty for email %s", r.Email)

// After
return ValidationErrorWithRecord("pkg/identity", "ingest", "login", r.Email, 0)
```

### 2. Use Error Context Fields

Move contextual information from inline messages to structured fields:

```go
// Before
return fmt.Errorf("validation failed for row %d (email %s): %w", idx, rows[idx].Email, err)

// After
baseErr := ValidationErrorWithRecord("pkg/identity", "ingest", "email", rows[idx].Email, idx)
return WrapError(err, *baseErr)
```

### 3. Standardize Error Messages

Use consistent error message formats across all operations:

| Operation Type | Message Format |
|---------------|----------------|
| Validation | `"validation failed for field '%s' for record %s at position %d: %s"` |
| Parse | `"failed to parse %s for record %s at position %d: %s"` |
| Database | `"database operation failed for email %s (query: %s): %s"` |
| Lookup | `"lookup operation failed (trace ID: %s) for key %s: %s"` |

### 4. Implement Error Classification

Use `ClassifyError` for automatic error categorization:

```go
// Wrap database errors with automatic classification
if err != nil {
    return ClassifyError(err, ClassifyOptions{
        Component: "pkg/pg",
        Operation: "IngestEmailResolution",
        Context: ErrorContext{
            RecordEmail: email,
            RecordPosition: idx,
        },
    })
}
```

## Operation Type Categorization

### Parsing Errors

**Operations:**
- Commit SHA parsing and validation
- Position handling in batch operations
- JSON parsing for dump files

**Required Context:**
- `RecordCommitSHA` - Commit identifier
- `RecordPosition` - Batch position
- `data_type` - Type of data being parsed
- `reason` - Specific parse failure reason

### Ingestion Errors

**Operations:**
- Email validation (empty, format)
- Login validation (empty)
- Source validation (live, seed, manual)
- Timestamp validation (resolved_at)
- Database upsert failures
- Query execution failures

**Required Context:**
- `RecordEmail` - Email address
- `RecordPosition` - Batch position
- `Query` - Database query (sanitized)
- `reason` - Specific failure reason

### Lookup Errors

**Operations:**
- Trace ID lookups
- Record key retrievals
- Email resolution queries
- User alias lookups

**Required Context:**
- `RecordTraceID` - Trace identifier
- `RecordKey` - Generic record key
- `RecordEmail` - Email address (for email lookups)
- `Query` - Database query (sanitized)

## Appendix: Complete Error Helper List

### Validation Errors (4 functions)
1. `ValidationErrorf` - Formatted validation error
2. `ValidationErrorWithRecord` - Validation with record identifier
3. `RequiredFieldError` - Missing required field
4. `InvalidFormatError` - Invalid field format

### Parse Errors (4 functions)
1. `ParseErrorf` - Formatted parse error
2. `ParseErrorWithRecord` - Parse error with record identifier
3. `JSONParseError` - JSON parsing failure
4. `CommitParseError` - Commit-specific parse error

### Database Errors (4 functions)
1. `DatabaseErrorf` - Formatted database error
2. `DatabaseErrorWithEmail` - Database error with email context
3. `DatabaseConnectionError` - Connection failure
4. `DatabaseQueryError` - Query execution failure

### Network Errors (2 functions)
1. `NetworkErrorf` - Formatted network error
2. `ConnectionRefusedError` - Connection refused

### Timeout Errors (3 functions)
1. `TimeoutErrorf` - Formatted timeout error
2. `HTTPTimeoutError` - HTTP request timeout
3. `DatabaseTimeoutError` - Database query timeout

### HTTP Errors (1 function)
1. `HTTPError` - HTTP status code errors

### Authentication Errors (3 functions)
1. `AuthErrorf` - Formatted auth error
2. `UnauthorizedError` - 401 Unauthorized
3. `ForbiddenError` - 403 Forbidden
4. `TokenExpiredError` - Token expiration

### Configuration Errors (2 functions)
1. `ConfigErrorf` - Formatted config error
2. `MissingConfigError` - Missing configuration
3. `InvalidConfigError` - Invalid configuration value

### Resource Errors (3 functions)
1. `ResourceErrorf` - Formatted resource error
2. `MemoryExhaustedError` - Memory exhaustion
3. `DiskSpaceExhaustedError` - Disk space exhaustion
4. `ConnectionPoolExhaustedError` - Connection pool exhaustion

### Concurrency Errors (2 functions)
1. `ConcurrencyErrorf` - Formatted concurrency error
2. `DeadlockError` - Deadlock detection
3. `LockConflictError` - Lock conflict

### Lookup Errors (1 function)
1. `LookupErrorWithTraceID` - Lookup with trace identifier

## Total Count: 31+ structured error helper functions currently defined but minimally used in production code.
