# Parsing Error Constructors and Call Sites Catalog

**Created:** 2026-08-07
**Purpose:** Complete inventory of parsing error constructors and their usage in the commitgraph codebase.
**Scope:** All error type definitions, constructor signatures, and call sites.

## Executive Summary

- **Total parsing-related error constructors defined:** 6
- **Constructors with commit SHA support:** 2 (33%)
- **Constructors without commit SHA support:** 4 (67%)
- **Actual usage of structured error constructors:** 0 sites
- **Actual parsing error sites using `fmt.Errorf`:** 27 sites

**Key Finding:** The codebase has a robust structured error system in `pkg/errors/` with commit SHA support, but it is **not being used**. All actual parsing errors use `fmt.Errorf` directly and lack context.

---

## Part 1: Error Constructor Definitions

### Location: `/home/coding/commitgraph/pkg/errors/`

#### File: `types.go`

**Error Category Type:**
```go
type ErrorCategory string

const (
    ValidationError ErrorCategory = "validation_error"
    ParseError      ErrorCategory = "parse_error"
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

**Structured Error Type:**
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
    CommitSHA  string  // ← Commit SHA field available
    Position   int64   // ← Position field available
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

#### File: `helpers.go`

### 1. ParseErrorfWithCommit ✅ HAS COMMIT SHA

**Signature:**
```go
func ParseErrorfWithCommit(component, operation, dataType, commitSHA, format string, args ...interface{}) *StructuredError
```

**Parameters:**
- `component` - Component/package where error occurred
- `operation` - Operation being performed
- `dataType` - Type of data being parsed (e.g., "JSON", "CSV", "timestamp")
- `commitSHA` - **Commit SHA associated with the error** ✅
- `format` - Printf-style format string
- `args` - Format arguments

**Returns:** `*StructuredError` with:
- `Type: ParseError`
- `Severity: SeverityHigh`
- `CommitSHA: commitSHA` (set)
- `Retryable: false`

**Definition:** Line 48 in `/home/coding/commitgraph/pkg/errors/helpers.go`

**Call Sites:** **NONE** (unused in codebase)

---

### 2. ParseErrorf ❌ NO COMMIT SHA (Deprecated)

**Signature:**
```go
func ParseErrorf(component, operation, dataType, format string, args ...interface{}) *StructuredError
```

**Parameters:**
- `component` - Component/package where error occurred
- `operation` - Operation being performed
- `dataType` - Type of data being parsed
- `format` - Printf-style format string
- `args` - Format arguments

**Returns:** `*StructuredError` (calls `ParseErrorfWithCommit` with `commitSHA = ""`)

**Deprecated:** Yes - "Use ParseErrorfWithCommit to include commit SHA context"

**Definition:** Line 76 in `/home/coding/commitgraph/pkg/errors/helpers.go`

**Call Sites:** **NONE** (unused in codebase)

---

### 3. JSONParseErrorWithCommit ✅ HAS COMMIT SHA

**Signature:**
```go
func JSONParseErrorWithCommit(component, operation, commitSHA string) *StructuredError
```

**Parameters:**
- `component` - Component/package where error occurred
- `operation` - Operation being performed
- `commitSHA` - **Commit SHA associated with the error** ✅

**Returns:** `*StructuredError` (calls `ParseErrorfWithCommit` with `dataType = "JSON"`, message = "invalid JSON structure")

**Definition:** Line 81 in `/home/coding/commitgraph/pkg/errors/helpers.go`

**Call Sites:** **NONE** (unused in codebase)

---

### 4. JSONParseError ❌ NO COMMIT SHA (Deprecated)

**Signature:**
```go
func JSONParseError(component, operation string) *StructuredError
```

**Parameters:**
- `component` - Component/package where error occurred
- `operation` - Operation being performed

**Returns:** `*StructuredError` (calls `JSONParseErrorWithCommit` with `commitSHA = ""`)

**Deprecated:** Yes - "Use JSONParseErrorWithCommit to include commit SHA context"

**Definition:** Line 87 in `/home/coding/commitgraph/pkg/errors/helpers.go`

**Call Sites:** **NONE** (unused in codebase)

---

### 5. ValidationErrorf ❌ NO COMMIT SHA

**Signature:**
```go
func ValidationErrorf(component, operation, field, format string, args ...interface{}) *StructuredError
```

**Parameters:**
- `component` - Component/package where error occurred
- `operation` - Operation being performed
- `field` - Field name that failed validation
- `format` - Printf-style format string
- `args` - Format arguments

**Returns:** `*StructuredError` with:
- `Type: ValidationError`
- `Severity: SeverityHigh`
- `Retryable: false`

**Note:** Does not accept `commitSHA` parameter - validation errors are typically not associated with specific commits.

**Definition:** Line 10 in `/home/coding/commitgraph/pkg/errors/helpers.go`

**Call Sites:** **NONE** (unused in codebase)

---

### 6. InvalidFormatError ❌ NO COMMIT SHA

**Signature:**
```go
func InvalidFormatError(component, operation, field, expectedFormat string) *StructuredError
```

**Parameters:**
- `component` - Component/package where error occurred
- `operation` - Operation being performed
- `field` - Field name with invalid format
- `expectedFormat` - Description of expected format

**Returns:** `*StructuredError` (calls `ValidationErrorf`)

**Note:** Wrapper around `ValidationErrorf` for format validation failures.

**Definition:** Line 43 in `/home/coding/commitgraph/pkg/errors/helpers.go`

**Call Sites:** **NONE** (unused in codebase)

---

### 7. RequiredFieldError ❌ NO COMMIT SHA

**Signature:**
```go
func RequiredFieldError(component, operation, field string) *StructuredError
```

**Parameters:**
- `component` - Component/package where error occurred
- `operation` - Operation being performed
- `field` - Field name that is required but missing

**Returns:** `*StructuredError` (calls `ValidationErrorf`)

**Definition:** Line 38 in `/home/coding/commitgraph/pkg/errors/helpers.go`

**Call Sites:** **NONE** (unused in codebase)

---

## Part 2: Generic Error Construction Helpers

### 8. NewError - Generic Structured Error Constructor

**Signature:**
```go
func NewError(typ ErrorCategory, severity SeverityLevel, message, code, component, operation string, opts ...ErrorContextOption) *StructuredError
```

**Parameters:**
- `typ` - Error category
- `severity` - Severity level
- `message` - Error message
- `code` - Error code
- `component` - Component/package
- `operation` - Operation being performed
- `opts` - Functional options for context (including `WithCommitSHAOption`)

**Definition:** Line 260 in `/home/coding/commitgraph/pkg/errors/types.go`

**Functional Options for Context:**
- `WithCommitSHAOption(commitSHA string)` - Sets the `CommitSHA` field
- `WithPositionOption(position int64)` - Sets the `Position` field
- `WithEmailOption(email string)` - Sets the `Email` field
- `WithTraceIDOption(traceID string)` - Sets the `TraceID` field
- `WithRecordKeyOption(recordKey string)` - Sets the `RecordKey` field

**Call Sites:** **NONE** (unused in codebase)

---

### 9. WrapError - Error Wrapping Helper

**Signature:**
```go
func WrapError(cause error, base StructuredError) *StructuredError
```

**Definition:** Line 286 in `/home/coding/commitgraph/pkg/errors/types.go`

**Call Sites:** **NONE** (unused in codebase)

---

### 10. ClassifyError - Automatic Error Classification

**Signature:**
```go
func ClassifyError(err error, opts ClassifyOptions) *StructuredError
```

**Definition:** Line 593 in `/home/coding/commitgraph/pkg/errors/types.go`

**Call Sites:** **NONE** (unused in codebase)

---

## Part 3: Actual Parsing Error Sites (Using `fmt.Errorf`)

**Note:** These are the 27 sites documented in `/home/coding/commitgraph/docs/parsing-error-catalog.md`.
All use `fmt.Errorf` directly and do NOT use the structured error constructors.

### Category 1: Timestamp/Date Parsing Errors (15 sites)

All sites lack commit SHA context.

#### Sites with Email Context (2 sites)
1. `cmd/verify-email-resolution/timestamp_verify.go:72` - Has `src.Email`, `src.ResolvedAt`
2. `cmd/verify-email-resolution/timestamp_verify.go:77` - Has `src.Email`, `targetResolvedAt`

#### Sites without Context (13 sites)
3. `cmd/get-audit-logs/main.go:116`
4. `cmd/get-audit-logs/main.go:122`
5. `cmd/get-audit-logs/main.go:186`
6. `cmd/get-audit-logs/main.go:192`
7. `cmd/get-audit-logs/main.go:217` (parseDate function)
8. `cmd/audit-logs/main.go:92`
9. `cmd/audit-logs/main.go:99`
10. `cmd/audit-logs/main.go:200`
11. `cmd/audit-logs/main.go:206`
12. `cmd/audit-logs/main.go:207`
13. `pkg/handler/audit_logs.go:123`
14. `pkg/handler/audit_logs.go:133`
15. `pkg/handler/audit_logs.go:178`
16. `pkg/handler/audit_logs.go:184`

### Category 2: SQLite Dump Parsing Errors (5 sites)

All sites lack commit SHA, position, and row identifier context.

17. `cmd/load-email-resolution-from-queue-api/main.go:238` - `created_at` parsing
18. `cmd/load-email-resolution-from-queue-api/main.go:243` - `updated_at` parsing
19. `cmd/load-email-resolution-from-queue-api/main.go:308` - Time parsing function
20. `cmd/load-email-resolution-from-queue-api/main.go:196` - CSV values parsing
21. `cmd/load-email-resolution-from-queue-api/main.go:319` - Time parsing warning

### Category 3: YAML/Config Parsing Errors (2 sites)

All sites lack commit SHA, position, and line number context.

22. `cmd/load-admin-aliases/main.go:215` - YAML unmarshal
23. `cmd/load-admin-aliases/main.go:235` - YAML parsing

### Category 4: JSON Parsing Errors (1 site)

Lacks commit SHA, position, and byte offset context.

24. `pkg/warmstart/extract.go:255` - JSON unmarshal with `ErrInvalidConfig`

### Category 5: Integer Parsing Errors (3 sites)

All sites lack commit SHA and position context.

25. `pkg/handler/audit_logs.go:113` - `repo_id` parsing
26. `pkg/handler/audit_logs.go:151` - `limit` parsing
27. `pkg/handler/audit_logs.go:163` - `offset` parsing

---

## Part 4: Context Fields Available in StructuredError

The `StructuredError` type has the following domain-specific fields available:

```go
type StructuredError struct {
    CommitSHA  string   // Commit SHA associated with the error ✅
    Position   int64    // Position/offset in data stream ✅
    Email      string   // Email address involved in the error
    TraceID    string   // Trace ID for distributed tracing
    RecordKey  string   // Record key for database/storage operations
    // ...
}
```

### Methods for Adding Context

```go
// Via WithCommitSHAOption during construction
err := errors.NewError(
    errors.ParseError,
    errors.SeverityHigh,
    "message",
    "code",
    "component",
    "operation",
    errors.WithCommitSHAOption(commitSHA),
)

// Via method chaining
err := errors.ParseErrorfWithCommit("component", "operation", "JSON", commitSHA, "format").
    WithPosition(offset).
    WithRecordKey(recordKey).
    WithEmail(email)
```

---

## Part 5: Gap Analysis

### What We Have vs. What We Use

| Component | Defined | Used | Gap |
|-----------|---------|------|-----|
| Structured error types | ✅ Yes | ❌ No | Complete lack of adoption |
| Commit SHA field | ✅ Available | ❌ Never set | Infrastructure exists, unused |
| ParseErrorfWithCommit | ✅ Defined | ❌ 0 call sites | No adoption |
| JSONParseErrorWithCommit | ✅ Defined | ❌ 0 call sites | No adoption |
| Position context | ✅ Available | ❌ Never set | Infrastructure exists, unused |
| RecordKey context | ✅ Available | ❌ Never set | Infrastructure exists, unused |

### Migration Path

To add commit SHA support to existing parsing errors:

1. **High Priority** - SQLite dump parsing (5 sites):
   - Add row number/position tracking
   - Add commit SHA when parsing commit-related data
   - Use `ParseErrorfWithCommit` instead of `fmt.Errorf`

2. **Medium Priority** - JSON/YAML parsing (3 sites):
   - Add line/byte offset tracking
   - Use `JSONParseErrorWithCommit` for JSON errors
   - Add commit SHA when parsing commit metadata

3. **Low Priority** - Date/integer parsing (19 sites):
   - These are typically CLI inputs, not commit data
   - Position context still useful for bulk operations
   - Can use `ValidationErrorf` or `InvalidFormatError`

---

## Part 6: Recommended Next Steps

### Immediate Actions

1. **Audit:** No action needed - the structured error system is well-designed

2. **Documentation:** Update inline comments in `pkg/errors/helpers.go` to clarify:
   - When to use `ParseErrorfWithCommit` vs `ParseErrorf`
   - What types of parsing should include commit SHA
   - Examples of proper usage

3. **Migration Plan:** Create phased migration from `fmt.Errorf` to structured errors:
   - Phase 1: Add structured error usage to high-volume parsing paths (SQLite dump)
   - Phase 2: Add to JSON/YAML parsing paths
   - Phase 3: Add to validation/date parsing paths
   - Phase 4: Remove deprecated constructors

### Code Changes Required

None required for this bead (documentation only). Subsequent beads should:
1. Migrate parsing error sites to use `ParseErrorfWithCommit`
2. Add commit SHA tracking to parsing paths
3. Add position/offset tracking for bulk operations
4. Update deprecated constructor call sites

---

## Appendix A: Constructor Summary Table

| Constructor | Commit SHA? | Status | Call Sites | File Location |
|-------------|-------------|--------|------------|---------------|
| `ParseErrorfWithCommit` | ✅ Yes | Active | 0 | `pkg/errors/helpers.go:48` |
| `ParseErrorf` | ❌ No | Deprecated | 0 | `pkg/errors/helpers.go:76` |
| `JSONParseErrorWithCommit` | ✅ Yes | Active | 0 | `pkg/errors/helpers.go:81` |
| `JSONParseError` | ❌ No | Deprecated | 0 | `pkg/errors/helpers.go:87` |
| `ValidationErrorf` | ❌ No | Active | 0 | `pkg/errors/helpers.go:10` |
| `InvalidFormatError` | ❌ No | Active | 0 | `pkg/errors/helpers.go:43` |
| `RequiredFieldError` | ❌ No | Active | 0 | `pkg/errors/helpers.go:38` |
| `NewError` | ✅ Via option | Active | 0 | `pkg/errors/types.go:260` |
| `WrapError` | ✅ Via base | Active | 0 | `pkg/errors/types.go:286` |
| `ClassifyError` | ❌ No | Active | 0 | `pkg/errors/types.go:593` |

---

## Appendix B: Related Documentation

- `/home/coding/commitgraph/docs/parsing-error-catalog.md` - Runtime parsing error sites (27 sites)
- `/home/coding/commitgraph/pkg/errors/types.go` - Core error type definitions
- `/home/coding/commitgraph/pkg/errors/helpers.go` - Error constructor functions

---

**End of Catalog**
