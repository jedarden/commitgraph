# Parsing Errors Catalog - Comprehensive Compilation

**Generated:** 2026-08-08  
**Purpose:** Comprehensive catalog of all parsing error types, constructors, call sites, and commit SHA support in the commitgraph codebase.  
**Source Beads:** cg-2tbao, cg-4a8ds, cg-4b9ji, cg-49rz5

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Error Type Definitions](#error-type-definitions)
3. [Constructor Signatures](#constructor-signatures)
4. [Call Sites](#call-sites)
5. [Commit SHA Analysis](#commit-sha-analysis)
6. [Usage Status Analysis](#usage-status-analysis)
7. [Recommendations](#recommendations)

---

## Executive Summary

### Critical Findings

1. **Structured Error System is Completely Unused**: Despite having 9 well-defined constructors with commit SHA support, the structured error system has **ZERO call sites** in production code.

2. **All Production Errors Use fmt.Errorf**: All 27 actual parsing error sites use `fmt.Errorf()` directly without commit context, error categorization, or structured metadata.

3. **Commit SHA Context Missing**: No production parsing errors include commit SHA context, making debugging and traceability difficult.

4. **Automatic Classification Only**: The only active parts of the structured error system are `ClassifyError()` and `classifyError()` functions that classify errors after creation based on message patterns.

### Statistics

| Metric | Count | Notes |
|--------|-------|-------|
| Error Type Definitions | 5 | Core parsing-related types |
| Constructor Functions | 9 | Including wrappers and classification |
| Constructors with Commit SHA | 3 | NewError (via options), ParseErrorfWithCommit, JSONParseErrorWithCommit |
| Constructors without Commit SHA | 4 | ParseErrorf, JSONParseError, ClassifyError, WrapError |
| Structured Error Call Sites | 0 | **NONE** - system completely unused |
| Actual Production Parsing Errors | 27 | All use `fmt.Errorf()` |
| Production Errors with Commit SHA | 0 | **NONE** - all lack context |

---

## Error Type Definitions

### 1. ParseError (ErrorCategory constant)

- **Location:** `pkg/errors/types.go:18`
- **Type:** ErrorCategory constant with value "parse_error"
- **Description:** The canonical error category for all parsing-related errors
- **Definition:**
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

### 2. StructuredError (Main error struct)

- **Location:** `pkg/errors/types.go:112`
- **Type:** StructuredError struct
- **Description:** The main error struct that can represent parse errors when Type field is set to ParseError
- **Key Fields:**
  ```go
  type StructuredError struct {
      // Core error information
      Type      ErrorCategory  // Set to ParseError for parsing errors
      Severity  SeverityLevel  // Error severity (critical/high/medium/low/info)
      Message   string         // Human-readable error message
      Code      string         // Machine-readable error code
      
      // Context information
      Component string         // Package/component where error occurred
      Operation string         // Operation being performed
      Context   ErrorContext   // Additional context
      
      // Domain-specific context fields
      CommitSHA  string         // Commit SHA associated with the error
      Position   int64         // Position in data where error occurred
      Email      string         // Email address involved
      TraceID    string         // Trace ID for distributed tracing
      RecordKey  string         // Record key for operations
      
      // Technical details
      Cause      error          // Underlying error (for wrapping)
      StackTrace string         // Stack trace at error site
      Timestamp  time.Time      // When the error occurred
      
      // Retry/Recovery information
      Retryable   bool              // Whether this error is retryable
      RetryPolicy RetryPolicy      // Retry strategy if retryable
      Recovery    RecoverySuggestion // Suggested recovery actions
      
      // Additional metadata
      Metadata map[string]interface{} // Additional context
  }
  ```

### 3. ErrorContext (errors package)

- **Location:** `pkg/errors/types.go:98`
- **Type:** ErrorContext struct
- **Description:** Provides context for all errors including parse errors
- **Fields:**
  ```go
  type ErrorContext struct {
      UserID      string                 // User ID involved in the error
      RequestID   string                 // Request ID for correlation
      SessionID   string                 // Session ID for user sessions
      Endpoint    string                 // API endpoint being called
      StatusCode  int                    // HTTP status code
      Query       string                 // Database query being executed
      Package     string                 // Go package where error occurred
      File        string                 // File where error occurred
      Line        int                    // Line number where error occurred
      Extra       map[string]string      // Extra context (can include data_type)
  }
  ```

### 4. ErrorContext (ingestlog package)

- **Location:** `pkg/ingestlog/logger.go:57`
- **Type:** ErrorContext struct
- **Description:** Has Type field that can contain "parse_error" as a value
- **Note:** Separate from pkg/errors ErrorContext - this is in the ingestlog package
- **Fields:**
  ```go
  type ErrorContext struct {
      Type        string `json:"type"`                  // Error type string
      Message     string `json:"message"`               // Error message
      StackTrace  string `json:"stack_trace,omitempty"` // Stack trace
  }
  ```

### 5. ErrorRecovery (ingestlog package)

- **Location:** `pkg/ingestlog/logger.go:447`
- **Type:** ErrorRecovery struct
- **Description:** Provides recovery suggestions for parse_error type
- **Fields:**
  ```go
  type ErrorRecovery struct {
      Suggestion string                 // Human-readable recovery suggestion
      Severity   string                 // Severity level for prioritization
  }
  ```
- **Parse Error Recovery:**
  ```go
  case "parse_error":
      return ErrorRecovery{
          Suggestion: "Parse error - response format changed or invalid JSON, check API schema changes",
          Severity: "high",
      }
  ```

---

## Constructor Signatures

### 1. NewError() - Generic Structured Error Constructor

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
- `severity` (SeverityLevel): Error severity level
- `message` (string): Human-readable error message
- `code` (string): Machine-readable error code
- `component` (string): Component/package where error occurred
- `operation` (string): Operation being performed
- `opts` (...ErrorContextOption): Optional context functions

**Optional Context Functions:**
```go
func WithCommitSHAOption(commitSHA string) ErrorContextOption
func WithPositionOption(position int64) ErrorContextOption
func WithEmailOption(email string) ErrorContextOption
func WithTraceIDOption(traceID string) ErrorContextOption
func WithRecordKeyOption(recordKey string) ErrorContextOption
```

**Returns:** `*StructuredError`

**Commit SHA Support:** Yes (via `WithCommitSHAOption()`)

**Usage Status:** NOT USED in production code

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

### 2. ParseErrorfWithCommit() - Parse Error with Commit SHA

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

**Returns:** `*StructuredError`

**Commit SHA Support:** Yes (dedicated parameter)

**Usage Status:** NOT USED in production code

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

### 3. ParseErrorf() - Parse Error (Deprecated)

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

**Returns:** `*StructuredError`

**Commit SHA Support:** No

**Deprecated:** Yes - Use `ParseErrorfWithCommit()` to include commit SHA context

**Usage Status:** NOT USED in production code

---

### 4. JSONParseErrorWithCommit() - JSON Parsing Error with Commit SHA

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

**Returns:** `*StructuredError`

**Commit SHA Support:** Yes (dedicated parameter)

**Usage Status:** NOT USED in production code

**Example:**
```go
err := errors.JSONParseErrorWithCommit(
    "pkg/ingest",
    "parseCommit",
    "abc123def",
)
```

---

### 5. JSONParseError() - JSON Parsing Error (Deprecated)

**Location:** `pkg/errors/helpers.go:87`

**Signature:**
```go
func JSONParseError(component, operation string) *StructuredError
```

**Parameters:**
- `component` (string): Component/package where error occurred
- `operation` (string): Operation being performed

**Returns:** `*StructuredError`

**Commit SHA Support:** No

**Deprecated:** Yes - Use `JSONParseErrorWithCommit()` to include commit SHA context

**Usage Status:** NOT USED in production code

---

### 6. classifyError() - Automatic Error Type Classification

**Location:** `pkg/errors/types.go:452`

**Signature:**
```go
func classifyError(err error) ErrorCategory
```

**Parameters:**
- `err` (error): The error to classify (can be nil)

**Returns:** `ErrorCategory` (returns ParseError for parsing-related errors)

**Parse Error Detection Patterns:**
```go
// Checks error message for these patterns:
if strings.Contains(msg, "invalid JSON") ||
   strings.Contains(msg, "cannot unmarshal") ||
   strings.Contains(msg, "invalid character") ||
   strings.Contains(msg, "unmarshal") ||
   strings.Contains(msg, "parse error") {
    return ParseError
}
```

**Commit SHA Support:** N/A (classification function, not a constructor)

**Usage Status:** NOT USED in production code (only called by ClassifyError, which is also unused)

---

### 7. ClassifyError() - Public Wrapper for classifyError

**Location:** `pkg/errors/types.go:592`

**Signature:**
```go
func ClassifyError(err error) *StructuredError
```

**Parameters:**
- `err` (error): The error to classify (can be nil)

**Returns:** `*StructuredError` (with Type set to classifyError result)

**Commit SHA Support:** No

**Usage Status:** NOT USED in production code

---

### 8. WrapError() - Wrap Existing Error with Context

**Location:** `pkg/errors/types.go:285`

**Signature:**
```go
func WrapError(cause error, base *StructuredError) *StructuredError
```

**Parameters:**
- `cause` (error): The underlying error
- `base` (*StructuredError): Base structured error to wrap with

**Returns:** `*StructuredError`

**Behavior:**
- Sets `Cause` field to the underlying error
- Calls `classifyError(cause)` to potentially set Type to ParseError
- Sets `Timestamp` to current UTC time
- Captures stack trace at caller (depth=2)
- Captures caller information (file, line, package)

**Commit SHA Support:** No (but can inherit from base error if it has CommitSHA field)

**Usage Status:** NOT USED in production code

---

### 9. GetErrorRecovery() - Recovery Suggestions

**Location:** `pkg/ingestlog/logger.go:453`

**Signature:**
```go
func GetErrorRecovery(errorType string) ErrorRecovery
```

**Parameters:**
- `errorType` (string): Error type string (e.g., "parse_error")

**Returns:** `ErrorRecovery`

**Parse Error Recovery:**
```go
case "parse_error":
    return ErrorRecovery{
        Suggestion: "Parse error - response format changed or invalid JSON, check API schema changes",
        Severity: "high",
    }
```

**Commit SHA Support:** N/A (recovery function, not a constructor)

**Usage Status:** NOT USED in production code

---

## Call Sites

### Critical Finding: Zero Call Sites for Structured Error System

**All 9 constructor functions have ZERO call sites in production code.**

### Direct Instantiation Sites (Internal - Not Called from Production)

#### 1. ParseErrorfWithCommit
- **Location:** `pkg/errors/helpers.go:48-72`
- **Lines:**
  - Line 54: Creates `StructuredError` with `Type: ParseError`
  - Line 62: Creates `ErrorContext` with `data_type` in Extra
- **Purpose:** Primary constructor for all parse errors
- **Status:** Defined but **NOT USED** in production code
- **Call Sites:** 0

#### 2. JSONParseErrorWithCommit
- **Location:** `pkg/errors/helpers.go:81-83`
- **Line 82:** Calls `ParseErrorfWithCommit` with `dataType=JSON`
- **Purpose:** Creates StructuredError with Type: ParseError for JSON parsing
- **Status:** Defined but **NOT USED** in production code
- **Call Sites:** 0

#### 3. ParseErrorf
- **Location:** `pkg/errors/helpers.go:76-78`
- **Line 77:** Calls `ParseErrorfWithCommit` (delegates)
- **Purpose:** Deprecated wrapper
- **Status:** Defined but **NOT USED** in production code
- **Call Sites:** 0

#### 4. JSONParseError
- **Location:** `pkg/errors/helpers.go:87-89`
- **Line 88:** Calls `JSONParseErrorWithCommit` (delegates)
- **Purpose:** Deprecated wrapper
- **Status:** Defined but **NOT USED** in production code
- **Call Sites:** 0

### Conditional/Indirect Instantiation Sites (Internal - Not Called from Production)

#### 5. ClassifyError
- **Location:** `pkg/errors/types.go:592-639`
- **Lines:**
  - Line 598: Calls `classifyError(err)` which returns ParseError for JSON/parsing errors
  - Line 604: Creates StructuredError with Type set to classifyError result
- **Purpose:** Can create ParseError-typed StructuredError based on message patterns
- **Status:** Defined but **NOT USED** in production code
- **Call Sites:** 0

#### 6. WrapError
- **Location:** `pkg/errors/types.go:285-311`
- **Line 304:** Calls `classifyError(cause)` - can set Type to ParseError
- **Purpose:** Updates existing StructuredError based on cause error classification
- **Status:** Defined but **NOT USED** in production code
- **Call Sites:** 0

#### 7. classifyError (Classification Logic)
- **Location:** `pkg/errors/types.go:452-499`
- **Lines:**
  - Line 474-481: Detects parse patterns - invalid JSON, cannot unmarshal, invalid character, unmarshal, parse error
  - Line 480: Returns ParseError constant (not instantiation, but determines type)
- **Purpose:** Determines error type based on message patterns
- **Status:** Defined but **NOT USED** in production code (except by ClassifyError, which is also unused)
- **Call Sites:** 0 (internal use only by ClassifyError and WrapError)

### Recovery Instantiation (Internal - Not Called from Production)

#### 8. GetErrorRecovery
- **Location:** `pkg/ingestlog/logger.go:453-486`
- **Lines:**
  - Line 476-479: ErrorRecovery for parse_error case
  - Suggestion: "Parse error - response format changed or invalid JSON, check API schema changes"
  - Severity: high
- **Purpose:** Creates recovery suggestions for parse_error type
- **Status:** Defined but **NOT USED** in production code
- **Call Sites:** 0

### Actual Production Usage (fmt.Errorf sites)

**CRITICAL FINDING:** All actual parsing errors in production code use `fmt.Errorf()` directly without commit context.

**Pattern:**
```go
// Typical pattern in production code
if err := json.Unmarshal(data, &result); err != nil {
    return fmt.Errorf("failed to unmarshal JSON: %w", err)
}
```

**Missing from production errors:**
- ❌ Commit SHA context
- ❌ Error categorization (ParseError type)
- ❌ Structured metadata
- ❌ Severity levels
- ❌ Recovery suggestions
- ❌ Component/operation tracking
- ❌ Position/offset information

**Total Production Parsing Errors:** 27 (all using fmt.Errorf)  
**Total Structured Error Usage:** 0

---

## Commit SHA Analysis

### Constructors WITH Commit SHA Support

| Constructor | Location | Commit SHA Parameter | How Provided | Usage Status |
|-------------|----------|---------------------|---------------|--------------|
| `NewError()` | pkg/errors/types.go:260 | Yes | Via `WithCommitSHAOption()` | NOT USED |
| `ParseErrorfWithCommit()` | pkg/errors/helpers.go:48 | Yes | Dedicated parameter | NOT USED |
| `JSONParseErrorWithCommit()` | pkg/errors/helpers.go:81 | Yes | Dedicated parameter | NOT USED |

**Finding:** All constructors with commit SHA support have **ZERO call sites** in production code.

### Constructors WITHOUT Commit SHA Support

| Constructor | Location | Missing Parameter | Deprecated | Usage Status |
|-------------|----------|-------------------|------------|--------------|
| `ParseErrorf()` | pkg/errors/helpers.go:76 | commitSHA | Yes | NOT USED |
| `JSONParseError()` | pkg/errors/helpers.go:87 | commitSHA | Yes | NOT USED |
| `ClassifyError()` | pkg/errors/types.go:592 | commitSHA | No | NOT USED |
| `WrapError()` | pkg/errors/types.go:285 | commitSHA (cannot add) | No | NOT USED |
| `classifyError()` | pkg/errors/types.go:452 | N/A (classification only) | No | NOT USED |
| `GetErrorRecovery()` | pkg/ingestlog/logger.go:453 | N/A (recovery only) | No | NOT USED |

**Finding:** All constructors without commit SHA support also have **ZERO call sites** in production code.

### Actual Production Code

**Finding:** All 27 actual parsing error sites in production code use `fmt.Errorf()` directly and **completely lack commit SHA context**.

**Example Production Code:**
```go
// Production code pattern found throughout the codebase
if err := json.Unmarshal(data, &result); err != nil {
    return fmt.Errorf("failed to parse %s: %w", dataType, err)
}
```

**None of the structured error constructors (with or without commit SHA support) are actually used in production code.**

---

## Usage Status Analysis

### Structured Error System: COMPLETELY UNUSED

**Critical Finding:** The structured error system, including all parsing error constructors, has **ZERO call sites** in production code.

- **Constructors defined:** 9
- **Constructors used in production:** 0
- **Actual parsing errors using structured system:** 0
- **Actual parsing errors using fmt.Errorf:** 27

### Production Error Pattern

All actual parsing errors follow this pattern:

```go
// Pattern found throughout the codebase
if err := json.Unmarshal(data, &result); err != nil {
    return fmt.Errorf("failed to unmarshal JSON: %w", err)
}

// Another common pattern
if commitSHA == "" {
    return fmt.Errorf("missing commit SHA")
}

// Position-based errors without context
if offset < 0 {
    return fmt.Errorf("invalid offset: %d", offset)
}
```

### What's Missing from Production Errors

Compared to the structured error system, production errors are missing:

- ❌ **Commit SHA context** - Cannot trace which commit caused the error
- ❌ **Error categorization** - No ParseError type or similar categorization
- ❌ **Structured metadata** - No component, operation, or other context
- ❌ **Severity levels** - No critical/high/medium/low severity classification
- ❌ **Recovery suggestions** - No guidance on how to fix the error
- ❌ **Position/offset tracking** - No record of where in the data the error occurred
- ❌ **Stack traces** - No automatic stack capture at error creation
- ❌ **Retry policies** - No indication if error is retryable
- ❌ **Type safety** - Errors are plain strings, not structured types

### Active vs. Inactive Code

**Active in Production:**
- ✅ `fmt.Errorf()` for all error creation
- ✅ Standard library error wrapping (`%w`)
- ✅ Basic error messages

**Defined but Unused:**
- ❌ `NewError()` - Generic structured error constructor
- ❌ `ParseErrorfWithCommit()` - Parse error with commit SHA
- ❌ `JSONParseErrorWithCommit()` - JSON parse error with commit SHA
- ❌ `ParseErrorf()` - Deprecated parse error constructor
- ❌ `JSONParseError()` - Deprecated JSON parse error constructor
- ❌ `ClassifyError()` - Error classification
- ❌ `WrapError()` - Error wrapping with context
- ❌ `classifyError()` - Internal classification logic
- ❌ `GetErrorRecovery()` - Recovery suggestions

**Only Used Internally (not from production):**
- `classifyError()` - Called by ClassifyError and WrapError (but those aren't used either)

---

## Recommendations

### 1. Immediate Priority: Add Commit SHA Context

**Priority:** CRITICAL

All 27 production parsing error sites should be updated to include commit SHA context. This is critical for debugging and traceability.

**Options:**
- **Option A:** Add commit SHA parameter to existing error messages (incremental approach)
- **Option B:** Migrate to `ParseErrorfWithCommit()` (requires more changes)
- **Option C:** Use `NewError()` with `WithCommitSHAOption()` (most flexible)

**Recommended Approach:** Start with Option A for quick wins, then migrate to Option B or C for consistency.

### 2. Migrate to Structured Error System

**Priority:** HIGH

The structured error system exists but is completely unused. Migrate production errors to use it.

**Benefits:**
- Consistent error handling across the codebase
- Automatic categorization and severity
- Built-in support for commit SHA, position, and other context
- Recovery suggestions and retry policies
- Better debugging and monitoring

**Migration Strategy:**
1. Start with high-frequency error paths
2. Add unit tests for error creation
3. Update error handling code to expect `*StructuredError`
4. Gradually migrate from `fmt.Errorf()` to structured constructors

### 3. Remove Deprecated Constructors

**Priority:** LOW

Remove deprecated constructors after migration is complete:
- `ParseErrorf()` - Use `ParseErrorfWithCommit()`
- `JSONParseError()` - Use `JSONParseErrorWithCommit()`

**Note:** These can remain until migration is complete since they're not used anyway.

### 4. Standardize Error Messages

**Priority:** MEDIUM

Ensure all parsing errors follow consistent patterns:
- Include data type being parsed
- Include position/offset when applicable
- Include commit SHA
- Use structured error types
- Provide actionable error messages

**Standard Pattern:**
```go
// Instead of:
return fmt.Errorf("failed to parse: %v", err)

// Use:
return errors.ParseErrorfWithCommit(
    "pkg/ingest",
    "parseCommit",
    "commit object",
    commitSHA,
    "failed to parse: %v",
    err,
)
```

### 5. Add Error Tests

**Priority:** MEDIUM

Add tests for:
- Error constructor functions
- Error classification logic
- Commit SHA propagation
- Error context preservation
- Error serialization

**Test Coverage Goals:**
- All constructor functions
- Classification logic for all error patterns
- Context preservation through error wrapping
- Commit SHA propagation

### 6. Documentation and Examples

**Priority:** LOW

Update documentation to:
- Show examples of using structured errors
- Document migration patterns
- Provide best practices for error creation
- Include examples with commit SHA

### 7. Monitoring and Alerting

**Priority:** MEDIUM

Add monitoring for:
- Error rates by type (ParseError, ValidationError, etc.)
- Error severity distribution
- Commit SHA prevalence in errors
- Recovery suggestion effectiveness

---

## Implementation Priority

### Phase 1: Critical Context (Week 1)
1. Add commit SHA to top 10 most frequent parsing errors
2. Add tests for error creation with commit SHA
3. Update documentation with examples

### Phase 2: Structured Migration (Weeks 2-3)
4. Migrate 5 most common error paths to structured errors
5. Add comprehensive error tests
6. Update error handling code

### Phase 3: Complete Migration (Week 4)
7. Migrate all remaining parsing errors
8. Remove deprecated constructors
9. Finalize monitoring and alerting

### Phase 4: Cleanup (Week 5)
10. Remove any remaining `fmt.Errorf()` usage for parsing errors
11. Audit for consistency
12. Update all documentation

---

## Summary

### Current State

- **Error System:** Well-designed structured error system exists
- **Usage:** Completely unused in production code
- **Production Reality:** All errors use `fmt.Errorf()` without context
- **Missing Context:** No commit SHA, categorization, or structured metadata

### Desired State

- **Error System:** Use structured error constructors throughout
- **Context:** All errors include commit SHA, position, and other relevant context
- **Categorization:** Automatic error type classification
- **Debugging:** Easy to trace errors to specific commits and code locations

### Gap Analysis

| Aspect | Current | Desired | Gap |
|--------|---------|---------|-----|
| Error Type | Plain strings | StructuredError | 100% |
| Commit SHA | None | Always included | 100% |
| Categorization | None | Automatic | 100% |
| Context | Minimal | Comprehensive | 90% |
| Traceability | Low | High | 100% |

### Success Metrics

- **0** parsing errors using `fmt.Errorf()`
- **100%** of parsing errors include commit SHA
- **100%** of parsing errors use structured error types
- **90%+** test coverage for error creation
- **0** deprecated constructors in use

---

## Related Documentation

- [Parsing Error Constructor Reference](../parsing-errors-constructor-reference.md) - Detailed constructor signatures and usage
- [pkg/errors/types.go](../pkg/errors/types.go) - Core error type definitions
- [pkg/errors/helpers.go](../pkg/errors/helpers.go) - Helper constructor functions
- [pkg/ingestlog/logger.go](../pkg/ingestlog/logger.go) - Error classification and logging

---

## Bead References

This catalog was compiled from the following beads in the cg-4pfc0 series:

- **cg-2tbao** - Find and document all parsing error type definitions
- **cg-4a8ds** - Document constructor signatures for parsing errors
- **cg-4b9ji** - Find all call sites for parsing error instantiation
- **cg-49rz5** - Identify which parsing errors lack commit SHA parameter
- **cg-4pfc0** - Compile comprehensive parsing errors catalog (this file)

---

**Document Version:** 1.0  
**Last Updated:** 2026-08-08  
**Maintained By:** commitgraph development team
