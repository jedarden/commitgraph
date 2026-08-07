# Parsing Errors Catalog

**Date:** 2026-08-07  
**Repository:** commitgraph  
**Scope:** Production code only (excludes test files)  
**Purpose:** Comprehensive catalog of parsing error types, constructors, call sites, and gap analysis for commit SHA parameter support

---

## Overview

This catalog documents all parsing-related error handling in the commitgraph codebase. It consolidates findings from four discovery tasks:

1. **cg-2i71q:** Error type definitions discovery
2. **cg-4v7y8:** Constructor signature documentation
3. **cg-66gv7:** Call site catalog
4. **cg-jn62f:** Commit SHA gap analysis

### Key Findings Summary

- **Total error types documented:** 4 main error structures
- **Total constructors documented:** 14 (8 core + 6 warmstart-specific)
- **Total call sites cataloged:** 12
- **Constructors with commit SHA support:** 3 (ParseErrorfWithCommit, JSONParseErrorWithCommit, NewError)
- **Constructors needing commit SHA:** 6
- **Call sites needing updates:** 8
- **Files analyzed:** 7 production files

---

## 1. Error Type Definitions

### 1.1 Core Error System (pkg/errors/types.go)

#### StructuredError

**File:** `pkg/errors/types.go`  
**Lines:** 63-134

**Purpose:** Centralized error type for all categorized errors including parse errors

**Structure:**
```go
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
    CommitSHA  string        // Commit SHA associated with the error
    Position   int64         // Position/offset in data stream
    Email      string        // Email address involved in the error
    TraceID    string        // Trace ID for distributed tracing
    RecordKey  string        // Record key for database/storage operations

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
```

**Related Types:**
- `ErrorCategory` (enum) - includes `ParseError` constant
- `ErrorContext` - supporting context structure
- `SeverityLevel` - enum (critical, high, medium, low, info)

**Classification Logic:**
The `classifyError()` function automatically categorizes errors as parse errors when the error message contains:
- "invalid JSON"
- "cannot unmarshal"
- "invalid character"
- "unmarshal"
- "parse error"

---

### 1.2 Parse Helper Functions (pkg/errors/helpers.go)

**File:** `pkg/errors/helpers.go`

#### ParseErrorfWithCommit

**Line:** 48-72  
**Status:** ✅ Has commit SHA parameter  
**Call Sites:** 0 (defined but unused)

**Purpose:** Creates a parse error with formatted message and commit SHA context

**Signature:**
```go
func ParseErrorfWithCommit(
    component string,
    operation string,
    dataType string,
    commitSHA string,
    format string,
    args ...interface{}
) *StructuredError
```

**Parameters:**
- `component` (string): Component/package where error occurred
- `operation` (string): Operation being performed
- `dataType` (string): Type of data being parsed (e.g., "JSON", "XML")
- `commitSHA` (string): Commit SHA for context
- `format` (string): Format string for message
- `args` (...interface{}): Format arguments (variadic)

**Return:** `*StructuredError`

**Default Values:**
- Type: ParseError
- Severity: SeverityHigh
- Retryable: false
- Timestamp: Current UTC time
- Message: "failed to parse {dataType}: {formatted message}"

---

#### JSONParseErrorWithCommit

**Line:** 81-93  
**Status:** ✅ Has commit SHA parameter  
**Call Sites:** 0 (defined but unused)

**Purpose:** Creates an error specifically for JSON parsing failures with commit SHA

**Signature:**
```go
func JSONParseErrorWithCommit(
    component string,
    operation string,
    commitSHA string
) *StructuredError
```

**Parameters:**
- `component` (string): Component/package where error occurred
- `operation` (string): Operation being performed
- `commitSHA` (string): Commit SHA for context

**Return:** `*StructuredError`

**Default Values:**
- Type: ParseError
- Severity: SeverityHigh
- Retryable: false
- dataType: "JSON"
- Message: "failed to parse JSON: invalid JSON structure"

---

### 1.3 Warmstart Error Types (pkg/warmstart/error.go)

**File:** `pkg/warmstart/error.go`

#### Error (Warmstart-Specific Error Type)

**Lines:** 47-62

**Purpose:** Error type for tarball extraction and validation failures

**Structure:**
```go
type Error struct {
    Kind       ErrorKind  // Category of error (Truncated, MissingMember, CorruptPack, IO, Other)
    Context    string     // Human-readable details about what went wrong
    MemberName string     // Tarball member name (if applicable)
    Offset     int64      // Byte offset in the tarball (if applicable)
    Underlying error      // Original error (if applicable)
}
```

**Related Types:**
- `ErrorKind` (enum): Truncated, MissingMember, CorruptPack, IO, Other

**Constructors:** See Section 2.3

---

### 1.4 Ingest Log Error Types (pkg/ingestlog/logger.go)

#### ErrorContext

**File:** `pkg/ingestlog/logger.go`  
**Lines:** 28-35

**Purpose:** Error context structure for ingest endpoint logging

**Structure:**
```go
type ErrorContext struct {
    Type        string `json:"type"`                  // Type of error
    Message     string `json:"message"`               // Human-readable error message
    StackTrace  string `json:"stack_trace,omitempty"` // Stack trace captured at error time
}
```

**Classification Logic:**
The `classifyError()` function in `pkg/ingestlog/logger.go` (line 734) checks for parse error keywords:
- "invalid JSON"
- "cannot unmarshal"
- "invalid character"
- "unmarshal"
- "parse error"

---

## 2. Constructor Signatures

### 2.1 Core Error Constructors (pkg/errors/types.go)

#### NewError

**Line:** 97-113  
**Status:** ✅ Has commit SHA via `WithCommitSHAOption()`

**Signature:**
```go
func NewError(
    typ ErrorCategory,
    severity SeverityLevel,
    message string,
    code string,
    component string,
    operation string,
    opts ...ErrorContextOption
) *StructuredError
```

**Parameters:**
- `typ` (ErrorCategory): Categorized error type
- `severity` (SeverityLevel): Error severity level
- `message` (string): Human-readable error message
- `code` (string): Machine-readable error code
- `component` (string): Component/package where error occurred
- `operation` (string): Operation being performed
- `opts` (...ErrorContextOption): Optional context functions (variadic)

**Optional Context Functions:**
- `WithCommitSHAOption(commitSHA string)` - Sets commit SHA
- `WithPositionOption(position int64)` - Sets position/offset
- `WithEmailOption(email string)` - Sets email
- `WithTraceIDOption(traceID string)` - Sets trace ID
- `WithRecordKeyOption(recordKey string)` - Sets record key

**Return:** `*StructuredError`

---

#### WrapError

**Line:** 177-195  
**Status:** N/A (wrapping function)

**Signature:**
```go
func WrapError(
    cause error,
    base StructuredError
) *StructuredError
```

**Parameters:**
- `cause` (error): Underlying error to wrap
- `base` (StructuredError): Base structured error with context

**Return:** `*StructuredError`

**Behavior:**
- Sets `Cause` field
- Sets `Timestamp` to current UTC time
- Captures stack trace at caller (depth=2)
- Infers Type and Severity from cause if not set
- Captures caller information (file, line, package)

---

#### ClassifyError

**Line:** 540-598  
**Status:** N/A (classification function)

**Signature:**
```go
func ClassifyError(
    err error,
    opts ClassifyOptions
) *StructuredError
```

**Parameters:**
- `err` (error): Error to classify (can be nil)
- `opts` (ClassifyOptions): Classification options

**ClassifyOptions Structure:**
```go
type ClassifyOptions struct {
    Component  string
    Operation  string
    StatusCode int
    Context    ErrorContext
}
```

**Return:** `*StructuredError` (nil if err is nil)

**Behavior:**
- Classifies error type using `classifyError()`
- Infers severity level
- Generates error code
- Sets retry policy based on HTTP status code
- Captures stack trace and caller information
- Adds recovery suggestion

---

### 2.2 Deprecated Centralized Helpers (pkg/errors/helpers.go)

#### ParseErrorf (Deprecated)

**Line:** 76  
**Status:** ⚠️ Deprecated - calls `ParseErrorfWithCommit("")`  
**Call Sites:** 0

**Recommendation:** Use `ParseErrorfWithCommit` instead

---

#### JSONParseError (Deprecated)

**Line:** 87  
**Status:** ⚠️ Deprecated - calls `JSONParseErrorWithCommit("")`  
**Call Sites:** 0

**Recommendation:** Use `JSONParseErrorWithCommit` instead

---

### 2.3 Warmstart Error Constructors (pkg/warmstart/error.go)

#### NewIOError

**Line:** 140-144  
**Status:** ❌ Missing commit SHA parameter  
**Call Sites:** 0 (defined but not used in extract.go)

**Signature:**
```go
func NewIOError(
    context string,
    err error
) *Error
```

**Parameters:**
- `context` (string): Human-readable context about the I/O operation
- `err` (error): Underlying I/O error

**Return:** `*Error`

**Set Fields:**
- Kind: IO
- Context: context parameter
- Underlying: err parameter

---

#### NewTruncatedError

**Line:** 149-153  
**Status:** ❌ Missing commit SHA parameter  
**Call Sites:** 0 (defined but not used in extract.go)

**Signature:**
```go
func NewTruncatedError(
    context string,
    offset int64
) *Error
```

**Parameters:**
- `context` (string): Human-readable context about truncation
- `offset` (int64): Byte offset in the tarball where truncation occurred

**Return:** `*Error`

**Set Fields:**
- Kind: Truncated
- Context: context parameter
- Offset: offset parameter

---

#### NewTruncatedMemberError

**Line:** 158-165  
**Status:** ❌ Missing commit SHA parameter  
**Call Sites:** 3 (see Section 3)

**Signature:**
```go
func NewTruncatedMemberError(
    memberName string,
    context string,
    offset int64
) *Error
```

**Parameters:**
- `memberName` (string): Tarball member name that was truncated
- `context` (string): Human-readable context about truncation
- `offset` (int64): Byte offset in the tarball where truncation occurred

**Return:** `*Error`

**Set Fields:**
- Kind: Truncated
- MemberName: memberName parameter
- Context: context parameter
- Offset: offset parameter

---

#### NewMissingMemberError

**Line:** 168-173  
**Status:** ❌ Missing commit SHA parameter  
**Call Sites:** 1 (see Section 3)

**Signature:**
```go
func NewMissingMemberError(
    memberName string
) *Error
```

**Parameters:**
- `memberName` (string): Name of the missing tarball member

**Return:** `*Error`

**Set Fields:**
- Kind: MissingMember
- MemberName: memberName parameter

---

#### NewMissingMemberErrorWithContext

**Line:** 177-183  
**Status:** ❌ Missing commit SHA parameter  
**Call Sites:** 2 (see Section 3)

**Signature:**
```go
func NewMissingMemberErrorWithContext(
    memberName string,
    context string
) *Error
```

**Parameters:**
- `memberName` (string): Name of the missing tarball member
- `context` (string): Human-readable details (e.g., list of missing files)

**Return:** `*Error`

**Set Fields:**
- Kind: MissingMember
- MemberName: memberName parameter
- Context: context parameter

---

#### NewCorruptPackError

**Line:** 186-192  
**Status:** ❌ Missing commit SHA parameter  
**Call Sites:** 2 (see Section 3)

**Signature:**
```go
func NewCorruptPackError(
    memberName string,
    context string
) *Error
```

**Parameters:**
- `memberName` (string): Tarball member name with corrupt pack data
- `context` (string): Human-readable details about corruption

**Return:** `*Error`

**Set Fields:**
- Kind: CorruptPack
- MemberName: memberName parameter
- Context: context parameter

---

## 3. Call Sites Catalog

### Summary Statistics

| Error Type | Call Sites | Files |
|------------|-----------|-------|
| `warmstart.Error` (via constructors) | 9 | pkg/warmstart/extract.go |
| `CorruptionError` (deprecated) | 2 | pkg/warmstart/extract.go |
| `NotAGitRepoError` (deprecated) | 1 | pkg/warmstart/extract.go |
| **TOTAL** | **12** | **1** |

**File:** `pkg/warmstart/extract.go` (all call sites in this file)

---

### 3.1 NewTruncatedMemberError - 3 Call Sites

**File:** `pkg/warmstart/extract.go`

#### Site 1: Line 122 - ParseTarball()

```go
if err == io.ErrUnexpectedEOF || errors.Is(err, io.ErrUnexpectedEOF) {
    return nil, NewTruncatedMemberError(hdr.Name, "ended prematurely", 0)
}
```

**Context:** Tarball extraction fails with unexpected EOF during file read  
**Trigger:** Corrupted or truncated tarball member  
**Priority:** HIGH - Should include commit SHA

---

#### Site 2: Line 129 - ParseTarball()

```go
if written != hdr.Size {
    return nil, NewTruncatedMemberError(hdr.Name, 
        fmt.Sprintf("expected %d bytes, got %d", hdr.Size, written), 0)
}
```

**Context:** Byte count mismatch after reading tarball member  
**Trigger:** Truncated file (fewer bytes than header claimed)  
**Priority:** HIGH - Should include commit SHA

---

#### Site 3: Line 163 - ParseTarball()

```go
if ext == ".pack" && len(data) < 12 {
    return nil, NewTruncatedMemberError(hdr.Name, 
        fmt.Sprintf("pack file too small: %d bytes (minimum 12 bytes for header)", len(data)), 0)
}
```

**Context:** Pack file header validation (minimum 12 bytes: "PACK" + version + object count)  
**Trigger:** Pack file is too small to contain valid header  
**Priority:** HIGH - Should include commit SHA

---

### 3.2 NewMissingMemberError - 1 Call Site

**File:** `pkg/warmstart/extract.go`

#### Site 1: Line 206 - ParseTarball()

```go
if !foundPack {
    return nil, NewMissingMemberError(".pack")
}
```

**Context:** Validation that at least one .pack file exists in tarball  
**Trigger:** Tarball contains zero .pack files (invalid warm-start snapshot)  
**Priority:** MEDIUM - Should include commit SHA

---

### 3.3 NewMissingMemberErrorWithContext - 2 Call Sites

**File:** `pkg/warmstart/extract.go`

#### Site 1: Line 223 - ParseTarball()

```go
missingIdxFiles := CollectMissingIdxFiles(snapshot.PackFiles)
if len(missingIdxFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".idx", 
        fmt.Sprintf("missing .idx files: %s", strings.Join(missingIdxFiles, ", ")))
}
```

**Context:** Validation that each .pack file has corresponding .idx file  
**Trigger:** Missing companion .idx files required for pack index  
**Priority:** MEDIUM - Should include commit SHA

---

#### Site 2: Line 231 - ParseTarball()

```go
missingRefFiles := CollectMissingRefFiles(snapshot.PackFiles)
if len(missingIdxFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ref", 
        fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
}
```

**Context:** Validation that each .pack file has corresponding .ref file  
**Trigger:** Missing companion .ref files required for promisor pack reverse index  
**Priority:** MEDIUM - Should include commit SHA

---

### 3.4 NewCorruptPackError - 2 Call Sites

**File:** `pkg/warmstart/extract.go`

#### Site 1: Line 671 - VerifyGitFsck()

```go
if strings.Contains(outputStr, "corrupt") || strings.Contains(outputStr, "bad") || 
   strings.Contains(outputStr, "missing") {
    return NewCorruptPackError("", 
        fmt.Sprintf("git fsck detected corruption: %s", outputStr))
}
```

**Context:** Git integrity check (`git fsck --no-full --no-progress`)  
**Trigger:** Git fsck detects pack file corruption or bad objects  
**Priority:** CRITICAL - Must include commit SHA

---

#### Site 2: Line 714 - VerifyGitLog()

```go
if strings.Contains(outputStr, "corrupt") || strings.Contains(outputStr, "bad") || 
   strings.Contains(outputStr, "object") {
    return NewCorruptPackError("", 
        fmt.Sprintf("git log detected corruption: %s", outputStr))
}
```

**Context:** Git commit history verification (`git log --oneline -n 1`)  
**Trigger:** Git log cannot read commit history due to corruption  
**Priority:** CRITICAL - Must include commit SHA

---

### 3.5 Deprecated Error Types - 3 Call Sites

**File:** `pkg/warmstart/extract.go`

#### CorruptionError (deprecated) - 2 sites

**Line 142 - ParseTarball()**
```go
if refParts == "" {
    return nil, &CorruptionError {
        Context: "empty ref data in ref file",
    }
}
```
**Context:** Legacy ref file format parsing (empty content)  
**Trigger:** Ref file contains no data (corrupted legacy format)

---

**Line 148 - ParseTarball()**
```go
parts := strings.Fields(refParts)
if len(parts) != 2 {
    return nil, &CorruptionError {
        Context: fmt.Sprintf("invalid ref format in ref file: expected 'refpath SHA', got '%s'", refParts),
    }
}
```
**Context:** Legacy ref file format validation (incorrect structure)  
**Trigger:** Ref file doesn't match expected "refpath SHA" format

---

#### NotAGitRepoError (deprecated) - 1 site

**Line 265 - Materialize()**
```go
if _, err := os.Stat(headPath); err != nil {
    return &NotAGitRepoError{
        Path:   gitDir,
        Reason: fmt.Sprintf("HEAD not found at %s", headPath),
    }
}
```
**Context:** Git repository validation before materialization  
**Trigger:** Target directory is missing HEAD file (not a valid git directory)

---

## 4. Gap Analysis: Commit SHA Parameter

### 4.1 Constructors WITH Commit SHA Support

| Constructor | Package | SHA Support | Call Sites | Status |
|-------------|---------|-------------|------------|--------|
| `ParseErrorfWithCommit` | pkg/errors/helpers.go | ✅ Full parameter | 0 | **Unused** |
| `JSONParseErrorWithCommit` | pkg/errors/helpers.go | ✅ Full parameter | 0 | **Unused** |
| `NewError` | pkg/errors/types.go | ✅ Via `WithCommitSHAOption()` | N/A | Available |

**Finding:** Centralized parse error helpers exist with SHA support but have **zero call sites** in production code.

---

### 4.2 Constructors WITHOUT Commit SHA Support

| Constructor | Package | Call Sites | Priority | Update Needed |
|-------------|---------|------------|----------|--------------|
| `NewCorruptPackError` | pkg/warmstart/error.go | 2 | **CRITICAL** | SHA parameter |
| `NewTruncatedMemberError` | pkg/warmstart/error.go | 3 | **HIGH** | SHA parameter |
| `NewMissingMemberErrorWithContext` | pkg/warmstart/error.go | 2 | **MEDIUM** | SHA parameter |
| `NewMissingMemberError` | pkg/warmstart/error.go | 1 | **MEDIUM** | SHA parameter |
| `NewIOError` | pkg/warmstart/error.go | 0 | **LOW** | SHA parameter |
| `NewTruncatedError` | pkg/warmstart/error.go | 0 | **LOW** | SHA parameter |

**Total constructors needing update:** 6  
**Total call sites needing update:** 8

---

### 4.3 Call Sites Requiring SHA - Priority Breakdown

#### CRITICAL Priority (2 sites)

**Constructor:** `NewCorruptPackError`  
**File:** `pkg/warmstart/extract.go`

| Line | Function | Context |
|------|----------|---------|
| 671 | VerifyGitFsck() | Git fsck detected corruption |
| 714 | VerifyGitLog() | Git log detected corruption |

**Impact:** CRITICAL - Corruption errors occur during integrity verification and MUST include commit SHA for debugging

---

#### HIGH Priority (3 sites)

**Constructor:** `NewTruncatedMemberError`  
**File:** `pkg/warmstart/extract.go`

| Line | Function | Context |
|------|----------|---------|
| 122 | ParseTarball() | Unexpected EOF during tar read |
| 129 | ParseTarball() | Byte count mismatch |
| 163 | ParseTarball() | Pack file too small |

**Impact:** HIGH - Truncation errors should include commit SHA for traceability

---

#### MEDIUM Priority (3 sites)

**Constructor:** `NewMissingMemberError*`  
**File:** `pkg/warmstart/extract.go`

| Line | Function | Constructor | Context |
|------|----------|-------------|---------|
| 206 | ParseTarball() | `NewMissingMemberError` | No .pack files found |
| 223 | ParseTarball() | `NewMissingMemberErrorWithContext` | Missing .idx files |
| 231 | ParseTarball() | `NewMissingMemberErrorWithContext` | Missing .ref files |

**Impact:** MEDIUM - Validation errors benefit from SHA context

---

### 4.4 JSON Parse Error Opportunity

**File:** `pkg/warmstart/extract.go`  
**Line:** 235

**Current Code:**
```go
if err := json.Unmarshal(configData, &snapshot.Config); err != nil {
    return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
}
```

**Issue:** Uses standard library `json.Unmarshal` error wrapping, not centralized helper

**Recommendation:** Consider using `JSONParseErrorWithCommit` here for structured JSON parse errors with SHA support

---

### 4.5 Deprecated Error Types - Migration Path

| Deprecated Type | Call Sites | Lines | Migration Priority |
|-----------------|-----------|-------|-------------------|
| `CorruptionError` | 2 | 142, 148 | **HIGH** - Use `NewCorruptPackError` with SHA |
| `NotAGitRepoError` | 1 | 265 | **MEDIUM** - Use `NewIOError` with SHA or create dedicated constructor |

**Impact:** These deprecated types also lack SHA support and should be migrated

---

### 4.6 Implementation Priority Phases

#### Phase 1: Critical Corruption Errors (CRITICAL)
1. Add SHA parameter to `NewCorruptPackError`
2. Update 2 call sites in VerifyGitFsck() and VerifyGitLog()

#### Phase 2: Truncation Errors (HIGH)
3. Add SHA parameter to `NewTruncatedMemberError`
4. Update 3 call sites in ParseTarball()

#### Phase 3: Missing Member Errors (MEDIUM)
5. Add SHA parameter to `NewMissingMemberError` and `NewMissingMemberErrorWithContext`
6. Update 3 call sites in ParseTarball()

#### Phase 4: Unused Constructors (LOW)
7. Add SHA parameter to `NewIOError` and `NewTruncatedError` (preventative)

#### Phase 5: Deprecated Type Migration (HIGH)
8. Migrate `CorruptionError` sites to `NewCorruptPackError` with SHA
9. Migrate `NotAGitRepoError` site to appropriate constructor with SHA

---

### 4.7 Recommended Signature Pattern

Based on the existing `ParseErrorfWithCommit` and `JSONParseErrorWithCommit` patterns, the recommended signature for updated warmstart constructors is:

**Option 1: Direct Parameter (matches existing pattern)**
```go
func NewTruncatedMemberError(
    memberName string,
    context string,
    offset int64,
    commitSHA string,  // ← NEW PARAMETER (last position)
) *Error
```

**Option 2: Functional Options (more flexible, breaking change)**
```go
func NewTruncatedMemberError(
    memberName string,
    context string,
    offset int64,
    opts ...ErrorOption,  // ← FUNCTIONAL OPTIONS
) *Error

// Usage:
NewTruncatedMemberError(hdr.Name, "ended prematurely", 0, WithCommitSHA(sha))
```

**Recommendation:** Option 1 (direct parameter) for consistency with existing parse error helpers

---

## 5. Key Findings

### 5.1 Error Usage Pattern

- **Warmstart package** uses its own local error types (`warmstart.Error`)
- **Centralized error helpers** from `pkg/errors/` are defined but unused
- **No direct usage** of `ParseErrorfWithCommit`, `JSONParseErrorWithCommit`, or related helpers in production code
- **All parse error instantiation** is confined to warmstart package's tarball extraction logic

---

### 5.2 Parse Error Detection

- Parse errors are **detected by keyword matching** in error messages
- Both `classifyError()` functions check for: "invalid JSON", "cannot unmarshal", "invalid character", "unmarshal", "parse error"
- No **explicit parse error instantiation**—only error classification after the fact
- JSON unmarshal errors use standard library wrapping (line 235)

---

### 5.3 Deprecated Error Types Still Active

- `CorruptionError`: 2 call sites (lines 142, 148)
- `NotAGitRepoError`: 1 call site (line 265)
- These should be migrated to `warmstart.Error` with appropriate `ErrorKind`
- Migration priority: HIGH for CorruptionError, MEDIUM for NotAGitRepoError

---

### 5.4 Warmstart Error Distribution

| ErrorKind | Call Sites | Constructor |
|-----------|-----------|-------------|
| Truncated | 3 | `NewTruncatedMemberError` |
| MissingMember | 3 | `NewMissingMemberError*` |
| CorruptPack | 2 | `NewCorruptPackError` |
| IO | 0 | `NewIOError` (not used) |
| **Total** | **8** | - |

---

### 5.5 Disconnected Pattern

The commitgraph codebase has a **disconnected error handling pattern**:

1. **Warmstart package** uses local `warmstart.Error` types with 9 call sites
2. **Centralized error helpers** exist but are completely unused (0 call sites)
3. **Parse error detection** happens via keyword matching in classification functions
4. **Deprecated error types** (`CorruptionError`, `NotAGitRepoError`) are still actively used

---

## 6. Recommendations

### 6.1 Immediate Actions (High Priority)

1. **Add commit SHA parameter to critical constructors:**
   - `NewCorruptPackError` (CRITICAL - 2 sites)
   - `NewTruncatedMemberError` (HIGH - 3 sites)

2. **Migrate deprecated error types:**
   - Replace `CorruptionError` with `NewCorruptPackError` (2 sites)
   - Consider migrating `NotAGitRepoError` (1 site)

3. **Consider using centralized helpers:**
   - Evaluate whether warmstart errors should migrate to `StructuredError` pattern
   - Or, continue using `warmstart.Error` but add SHA support consistently

---

### 6.2 Architectural Considerations

1. **Standardize on one error pattern:**
   - Either use centralized `StructuredError` with `ParseErrorfWithCommit`
   - Or add SHA support to all `warmstart.Error` constructors
   - Current mix creates inconsistency

2. **Make parse errors explicit:**
   - Replace keyword-based classification with explicit error types
   - Use `JSONParseErrorWithCommit` for JSON parsing failures
   - Add structured context at error creation time

3. **Improve error traceability:**
   - All parsing errors should include commit SHA
   - Add position/offset context where applicable
   - Include tarball member name in all warmstart errors

---

### 6.3 Future Improvements

1. **Add error context preservation:**
   - Ensure commit SHA flows through all error transformations
   - Add structured context for async operations
   - Consider adding trace ID support

2. **Standardize error serialization:**
   - Use consistent error format across logging
   - Include all relevant context in serialized errors
   - Ensure error types are machine-readable

3. **Improve error testing:**
   - Add tests for error constructor signatures
   - Verify SHA parameter propagation
   - Test error classification logic

---

## 7. Conclusion

This catalog provides a comprehensive view of parsing error handling in the commitgraph codebase. The key takeaways are:

- **Disconnected error patterns:** Warmstart uses local errors, centralized helpers exist but unused
- **Missing commit SHA context:** 6 constructors and 8 call sites need SHA parameter added
- **Deprecated types still active:** 3 deprecated error types with 3 call sites require migration
- **Critical errors lack SHA:** Corruption and truncation errors must include commit SHA for debugging

**Implementation priority:** Start with `NewCorruptPackError` (CRITICAL), then `NewTruncatedMemberError` (HIGH), followed by missing member errors (MEDIUM).

---

## Appendix: Files Analyzed

| File | Purpose | Lines Analyzed |
|------|---------|----------------|
| pkg/errors/types.go | Core error types and classification | 63-134, 453, 540-598 |
| pkg/errors/helpers.go | Parse error helper constructors | 48-93 |
| pkg/warmstart/error.go | Warmstart-specific error types | 47-83, 140-192 |
| pkg/warmstart/extract.go | Tarball extraction and validation | 122-265, 671, 714 |
| pkg/ingestlog/logger.go | Ingest endpoint error logging | 734, 800, 872 |
| pkg/ingestlog/error_serializer.go | Error serialization utilities | All |
| pkg/identity/ingest.go | Identity ingestion (referenced) | - |

---

**Document Version:** 2.0  
**Last Updated:** 2026-08-07  
**Previous Version:** 2026-08-06 (inaccurate - claimed constructors had SHA support when they didn't)  
**Maintained By:** commitgraph development team
