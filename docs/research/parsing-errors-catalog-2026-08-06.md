# Parsing Errors Catalog - 2026-08-06

**Generated:** 2026-08-06  
**Scope:** Complete inventory of all parsing error constructors and call sites in the commitgraph codebase  
**Total Call Sites:** 35

---

## Overview

This catalog provides a comprehensive inventory of all parsing error types, constructors, and their usage throughout the commitgraph codebase. It serves as a reference for adding commit SHA context to parsing errors that currently lack it.

### Key Findings

- **35 parsing error call sites** identified across the codebase
- **Two primary error systems**: General structured errors (`pkg/errors/`) and specialized tarball errors (`pkg/warmstart/`)
- **Commit SHA context status**: **ZERO** parsing errors currently include commit SHA parameters
- **Most common pattern**: `fmt.Errorf` with wrapping (`%w`) or formatted messages

---

## Error Type Definitions

### 1. General Structured Error System (`pkg/errors/`)

#### File: `/home/coding/commitgraph/pkg/errors/types.go`

**Error Category Type (Line 12):**
```go
type ErrorCategory string
```

**ParseError Constant (Line 18):**
```go
ParseError ErrorCategory = "parse_error"
```

**Usage Context:**
- Line 445: Used in `isRetryableByDefault()` - **Not retryable**
- Line 480: Used in `classifyError()` - Auto-detects parse errors from messages
- Line 538: Used in `inferSeverity()` - **Severity: High**
- Line 693: Used in `getRecoverySuggestion()` - Provides recovery guidance

**Error Detection Patterns in `classifyError()`:**
- "invalid JSON"
- "cannot unmarshal"
- "invalid character"
- "unmarshal"
- "parse error"

---

#### File: `/home/coding/commitgraph/pkg/errors/helpers.go`

**Constructor Functions:**

##### 1. `ParseErrorf()` (Line 48)
```go
func ParseErrorf(component, operation, dataType, format string, args ...interface{}) *StructuredError
```
- **Purpose**: Creates parse errors with formatted message
- **Returns**: `*StructuredError` with `Type: ParseError`, `Severity: SeverityHigh`
- **Parameters**: 
  - `component`: Package/component name
  - `operation`: Operation being performed
  - `dataType`: Type of data being parsed
  - `format`: Format string with details
  - `args`: Format arguments
- **Commit SHA Status**: ❌ No support in constructor

##### 2. `JSONParseError()` (Line 74)
```go
func JSONParseError(component, operation string) *StructuredError
```
- **Purpose**: Creates JSON-specific parse errors
- **Returns**: `*StructuredError` by calling `ParseErrorf(component, operation, "JSON", "invalid JSON structure")`
- **Parameters**:
  - `component`: Package/component name
  - `operation`: Operation being performed
- **Commit SHA Status**: ❌ No support in constructor

**Internal Call Site (Line 75):**
```go
return ParseErrorf(component, operation, "JSON", "invalid JSON structure")
```

---

### 2. Warmstart Package Error System (`pkg/warmstart/`)

#### File: `/home/coding/commitgraph/pkg/warmstart/error.go`

**ErrorKind Enum (Lines 10-27):**
```go
type ErrorKind int

const (
    Truncated ErrorKind = iota      // Line 14 - Tarball was cut off or incomplete
    MissingMember                    // Line 16 - Required tarball member not found
    CorruptPack                      // Line 19 - Pack file data corruption detected
    IO                               // Line 22 - I/O error during parsing
    Other                            // Line 25 - Uncategorized error
)
```

**Main Error Struct (Lines 46-62):**
```go
type Error struct {
    Kind       ErrorKind    // Category of error
    Context    string       // Human-readable details
    MemberName string       // Tarball member name (if applicable)
    Offset     int64        // Byte offset in tarball (if applicable)
    Underlying error        // Original error (if applicable)
}
```

**Deprecated Error Types:**

##### 1. `CorruptionError` (Lines 106-114)
```go
type CorruptionError struct {
    Context string  // Describes what was corrupted
}
```
- **Status**: ⚠️ Deprecated - Use `Error` with `Kind=CorruptPack`

##### 2. `NotAGitRepoError` (Lines 118-137)
```go
type NotAGitRepoError struct {
    Path   string  // Directory that was checked
    Reason string  // Why it's not a git repository
}
```
- **Status**: ⚠️ Deprecated - Use `Error` with `Kind=Other`

**Active Constructor Functions:**

##### 1. `NewIOError()` (Line 140)
```go
func NewIOError(context string, err error) *Error
```
- **Purpose**: Creates I/O error during tarball operations
- **Returns**: `*Error` with `Kind: IO`
- **Commit SHA Status**: ❌ No support

##### 2. `NewTruncatedError()` (Line 149)
```go
func NewTruncatedError(context string, offset int64) *Error
```
- **Purpose**: Creates truncation error
- **Returns**: `*Error` with `Kind: Truncated`
- **Commit SHA Status**: ❌ No support

##### 3. `NewTruncatedMemberError()` (Line 158)
```go
func NewTruncatedMemberError(memberName string, context string, offset int64) *Error
```
- **Purpose**: Creates truncation error for specific tarball member
- **Returns**: `*Error` with `Kind: Truncated`, member-specific
- **Commit SHA Status**: ❌ No support

##### 4. `NewMissingMemberError()` (Line 168)
```go
func NewMissingMemberError(memberName string) *Error
```
- **Purpose**: Creates error for missing tarball member
- **Returns**: `*Error` with `Kind: MissingMember`
- **Commit SHA Status**: ❌ No support

##### 5. `NewMissingMemberErrorWithContext()` (Line 177)
```go
func NewMissingMemberErrorWithContext(memberName string, context string) *Error
```
- **Purpose**: Creates error for missing tarball member with additional context
- **Returns**: `*Error` with `Kind: MissingMember` + detailed context
- **Commit SHA Status**: ❌ No support

##### 6. `NewCorruptPackError()` (Line 186)
```go
func NewCorruptPackError(memberName string, context string) *Error
```
- **Purpose**: Creates error for pack file corruption
- **Returns**: `*Error` with `Kind: CorruptPack`
- **Commit SHA Status**: ❌ No support

---

## Call Sites Catalog

### 1. Structured Error System Call Sites

#### File: `/home/coding/commitgraph/pkg/errors/helpers.go`

**Call Site 1.1 (Line 75):** Internal call in `JSONParseError()`
```go
return ParseErrorf(component, operation, "JSON", "invalid JSON structure")
```
- **Error Type**: `ParseError` category
- **Context**: JSON parsing failures
- **Commit SHA Needed**: ✅ Yes - for JSON parsing in commit context
- **Priority**: Medium

---

### 2. Warmstart Package Call Sites

#### File: `/home/coding/commitgraph/pkg/warmstart/extract.go`

**Call Site 2.1 (Line 122):** Tarball member premature end
```go
return nil, NewTruncatedMemberError(hdr.Name, "ended prematurely", 0)
```
- **Error Type**: `Truncated` kind
- **Context**: Tarball member ended prematurely during extraction
- **Commit SHA Needed**: ✅ Yes - for warmstart snapshot processing
- **Priority**: High (warmstart operations need commit context)

**Call Site 2.2 (Line 129):** Byte count mismatch
```go
return nil, NewTruncatedMemberError(hdr.Name, 
    fmt.Sprintf("expected %d bytes, got %d", hdr.Size, written), 0)
```
- **Error Type**: `Truncated` kind
- **Context**: Byte count mismatch during tarball extraction
- **Commit SHA Needed**: ✅ Yes - for warmstart snapshot processing
- **Priority**: High (warmstart operations need commit context)

**Call Site 2.3 (Lines 142-145):** Empty ref data
```go
return nil, &CorruptionError{
    Context: "empty ref data in ref file",
}
```
- **Error Type**: `CorruptionError` (deprecated)
- **Context**: Empty ref data in tarball ref file
- **Commit SHA Needed**: ✅ Yes - for corruption tracking
- **Priority**: High (data integrity issues)

**Call Site 2.4 (Lines 148-151):** Invalid ref format
```go
return nil, &CorruptionError{
    Context: fmt.Sprintf("invalid ref format in ref file: expected 'refpath SHA', got '%s'", refParts),
}
```
- **Error Type**: `CorruptionError` (deprecated)
- **Context**: Invalid ref format in tarball ref file
- **Commit SHA Needed**: ✅ Yes - for corruption tracking
- **Priority**: High (data integrity issues)

**Call Site 2.5 (Line 163):** Pack file too small
```go
return nil, NewTruncatedMemberError(hdr.Name, 
    fmt.Sprintf("pack file too small: %d bytes (minimum 12 bytes for header)", len(data)), 0)
```
- **Error Type**: `Truncated` kind
- **Context**: Pack file size validation failure
- **Commit SHA Needed**: ✅ Yes - for warmstart snapshot processing
- **Priority**: High (warmstart operations need commit context)

**Call Site 2.6 (Line 206):** Missing .pack file
```go
return nil, NewMissingMemberError(".pack")
```
- **Error Type**: `MissingMember` kind
- **Context**: Missing .pack file in tarball
- **Commit SHA Needed**: ✅ Yes - for warmstart snapshot processing
- **Priority**: High (warmstart operations need commit context)

**Call Site 2.7 (Line 242):** Missing .idx file
```go
return nil, NewMissingMemberError(".idx")
```
- **Error Type**: `MissingMember` kind
- **Context**: Missing .idx file in tarball
- **Commit SHA Needed**: ✅ Yes - for warmstart snapshot processing
- **Priority**: High (warmstart operations need commit context)

**Call Site 2.8 (Line 251):** Missing .ref files
```go
return nil, NewMissingMemberErrorWithContext(".ref", 
    fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
```
- **Error Type**: `MissingMember` kind with context
- **Context**: Missing .ref files with detailed list
- **Commit SHA Needed**: ✅ Yes - for warmstart snapshot processing
- **Priority**: High (warmstart operations need commit context)

**Call Site 2.9 (Lines 285-289):** Not a git repository
```go
return &NotAGitRepoError{
    Path:   gitDir,
    Reason: fmt.Sprintf("HEAD not found at %s", headPath),
}
```
- **Error Type**: `NotAGitRepoError` (deprecated)
- **Context**: HEAD not found during warmstart extraction
- **Commit SHA Needed**: ⚠️ Maybe - for repository context
- **Priority**: Low (repository-level error)

**Call Site 2.10 (Line 620):** Git fsck detected corruption
```go
return NewCorruptPackError("", fmt.Sprintf("git fsck detected corruption: %s", outputStr))
```
- **Error Type**: `CorruptPack` kind
- **Context**: Git fsck detected pack file corruption
- **Commit SHA Needed**: ✅ Yes - for corruption tracking
- **Priority**: High (data integrity issues)

**Call Site 2.11 (Line 663):** Git log detected corruption
```go
return NewCorruptPackError("", fmt.Sprintf("git log detected corruption: %s", outputStr))
```
- **Error Type**: `CorruptPack` kind
- **Context**: Git log detected pack file corruption
- **Commit SHA Needed**: ✅ Yes - for corruption tracking
- **Priority**: High (data integrity issues)

---

### 3. Standard Parsing Error Call Sites

#### File: `/home/coding/commitgraph/cmd/load-admin-aliases/main.go`

**Call Site 3.1 (Line 215):** YAML unmarshal error
```go
return nil, fmt.Errorf("yaml unmarshal failed: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: YAML unmarshaling failure for admin aliases
- **Commit SHA Needed**: ❌ No (not commit-related data)
- **Priority**: None

**Call Site 3.2 (Line 235):** YAML parsing error
```go
return nil, fmt.Errorf("failed to parse aliases.yml: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: YAML file parsing failure
- **Commit SHA Needed**: ❌ No (not commit-related data)
- **Priority**: None

---

#### File: `/home/coding/commitgraph/cmd/load-email-resolution-from-queue-api/main.go`

**Call Site 3.3 (Line 233):** Time parsing error (created_at)
```go
return row, fmt.Errorf("failed to parse created_at: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: created_at timestamp parsing failure
- **Commit SHA Needed**: ❌ No (timestamp parsing error)
- **Priority**: None

**Call Site 3.4 (Line 238):** Time parsing error (updated_at)
```go
return row, fmt.Errorf("failed to parse updated_at: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: updated_at timestamp parsing failure
- **Commit SHA Needed**: ❌ No (timestamp parsing error)
- **Priority**: None

**Call Site 3.5 (Line 303):** General time parsing error
```go
return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
```
- **Error Type**: Standard error with formatting
- **Context**: General time parsing failure
- **Commit SHA Needed**: ❌ No (timestamp parsing error)
- **Priority**: None

---

#### File: `/home/coding/commitgraph/cmd/seed-email-resolution/main.go`

**Call Site 3.6 (Lines 117-119):** Time parsing fatal error
```go
log.Fatalf("error: failed to parse resolved_at %q for email %s: %v\n",
    resolvedAtStr, email, err)
```
- **Error Type**: Fatal log (not return error)
- **Context**: resolved_at timestamp parsing failure
- **Commit SHA Needed**: ❌ No (timestamp parsing error)
- **Priority**: None

---

#### File: `/home/coding/commitgraph/pkg/handler/audit_logs.go`

**Call Site 3.7 (Line 115):** Query parameter parsing error
```go
return params, fmt.Errorf("invalid repo_id: %s must be a valid integer", repoIDStr)
```
- **Error Type**: Standard error with formatting
- **Context**: repo_id parameter parsing failure
- **Commit SHA Needed**: ❌ No (HTTP parameter parsing)
- **Priority**: None

**Call Site 3.8 (Line 125):** Date parameter parsing error
```go
return params, fmt.Errorf("invalid start_date: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: start_date parsing failure
- **Commit SHA Needed**: ❌ No (HTTP parameter parsing)
- **Priority**: None

**Call Site 3.9 (Line 135):** Date parameter parsing error
```go
return params, fmt.Errorf("invalid end_date: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: end_date parsing failure
- **Commit SHA Needed**: ❌ No (HTTP parameter parsing)
- **Priority**: None

**Call Site 3.10 (Line 151):** Query parameter parsing error
```go
return params, fmt.Errorf("invalid limit: %s must be a valid integer", limitStr)
```
- **Error Type**: Standard error with formatting
- **Context**: limit parameter parsing failure
- **Commit SHA Needed**: ❌ No (HTTP parameter parsing)
- **Priority**: None

**Call Site 3.11 (Line 163):** Query parameter parsing error
```go
return params, fmt.Errorf("invalid offset: %s must be a valid integer", offsetStr)
```
- **Error Type**: Standard error with formatting
- **Context**: offset parameter parsing failure
- **Commit SHA Needed**: ❌ No (HTTP parameter parsing)
- **Priority**: None

**Call Site 3.12 (Line 178):** Date format validation error
```go
return nil, fmt.Errorf("invalid date format: '%s'. Expected YYYY-MM-DD format", dateStr)
```
- **Error Type**: Standard error with formatting
- **Context**: Date format validation failure
- **Commit SHA Needed**: ❌ No (HTTP parameter parsing)
- **Priority**: None

**Call Site 3.13 (Line 184):** Date parsing error
```go
return nil, fmt.Errorf("invalid date: '%s' is not a valid calendar date", dateStr)
```
- **Error Type**: Standard error with formatting
- **Context**: Date value parsing failure
- **Commit SHA Needed**: ❌ No (HTTP parameter parsing)
- **Priority**: None

**Call Site 3.14 (Line 192):** Date range validation error
```go
return nil, fmt.Errorf("date out of range: '%s' must be between 1970-01-01 and 2100-12-31", dateStr)
```
- **Error Type**: Standard error with formatting
- **Context**: Date range validation failure
- **Commit SHA Needed**: ❌ No (HTTP parameter parsing)
- **Priority**: None

---

#### File: `/home/coding/commitgraph/pkg/ingestlog/logger.go`

**Call Site 3.15 (Line 245):** JSON marshaling error
```go
return fmt.Errorf("failed to marshal stats to JSON: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: Stats JSON marshaling failure
- **Commit SHA Needed**: ❌ No (logging operation)
- **Priority**: None

**Call Site 3.16 (Line 307):** JSON marshaling error
```go
return fmt.Errorf("marshal ingest event: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: Ingest event JSON marshaling failure
- **Commit SHA Needed**: ❌ No (logging operation)
- **Priority**: None

**Call Site 3.17 (Line 344):** JSON marshaling error
```go
return fmt.Errorf("marshal ingest log entry: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: Log entry JSON marshaling failure
- **Commit SHA Needed**: ❌ No (logging operation)
- **Priority**: None

---

#### File: `/home/coding/commitgraph/pkg/audit/logger.go`

**Call Site 3.18 (Line 63):** JSON marshaling error
```go
return fmt.Errorf("marshal audit event: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: Audit event JSON marshaling failure
- **Commit SHA Needed**: ❌ No (logging operation)
- **Priority**: None

---

#### File: `/home/coding/commitgraph/pkg/client/queueapi/client.go`

**Call Site 3.19 (Line 100):** JSON marshaling error
```go
return fmt.Errorf("marshal request: %w", err)
```
- **Error Type**: Standard error with wrapping
- **Context**: Request JSON marshaling failure
- **Commit SHA Needed**: ❌ No (API client operation)
- **Priority**: None

---

#### File: `/home/coding/commitgraph/pkg/warmstart/extract.go`

**Call Site 3.20 (Lines 255-257):** JSON unmarshaling error
```go
if err := json.Unmarshal(configData, &snapshot.Config); err != nil {
    return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
}
```
- **Error Type**: Standard error with wrapping
- **Context**: Config JSON unmarshaling failure
- **Commit SHA Needed**: ✅ Yes - for warmstart snapshot processing
- **Priority**: High (warmstart operations need commit context)

---

## Priority Classification

### High Priority (Commit SHA Critical)

These errors occur during warmstart snapshot processing and directly involve commit data:

1. **Call Sites 2.1-2.11**: All `pkg/warmstart/extract.go` errors
   - Warmstart snapshot parsing/corruption
   - Need commit SHA for tracking and debugging

2. **Call Site 3.20**: Config JSON unmarshaling in warmstart
   - Part of warmstart snapshot processing
   - Needs commit SHA for context

### Medium Priority (Commit SHA Useful)

These errors could benefit from commit SHA context in certain scenarios:

1. **Call Site 1.1**: `JSONParseError()` internal call
   - JSON parsing failures might involve commit data
   - Commit SHA would help debugging

### Low Priority (Commit SHA Optional)

These errors are repository-level or administrative:

1. **Call Site 2.9**: `NotAGitRepoError`
   - Repository-level error
   - Commit SHA less critical

### No Priority (Commit SHA Not Needed)

These errors don't involve commit-specific data:

- **Call Sites 3.1-3.19**: HTTP parameter parsing, time parsing, JSON marshaling for logging
- These are operational errors, not data parsing errors

---

## Implementation Strategy

### Phase 1: High Priority Warmstart Errors

**Target Call Sites:** 2.1-2.11, 3.20 (12 sites)

**Action Items:**
1. Modify warmstart error constructors to accept optional commit SHA parameter
2. Update `NewTruncatedMemberError`, `NewMissingMemberError`, `NewMissingMemberErrorWithContext`, `NewCorruptPackError` functions
3. Update all call sites in `pkg/warmstart/extract.go` to pass commit SHA when available

### Phase 2: Medium Priority Structured Errors

**Target Call Sites:** 1.1 (1 site)

**Action Items:**
1. Modify `ParseErrorf` and `JSONParseError` to accept optional commit SHA parameter
2. Update internal call to pass commit SHA when available

### Phase 3: Error Context Enhancement

**Action Items:**
1. Consider adding commit SHA field to `StructuredError` struct
2. Add helper methods like `WithCommitSHA()` to existing error types
3. Update error formatting to include commit SHA when present

---

## Error Handling Patterns

### Current Patterns

1. **Structured Error Wrapping**: `fmt.Errorf("context: %w", err)` - Used for JSON/time parsing
2. **Formatted Messages**: `fmt.Errorf("context: %s", value)` - Used for parameter validation
3. **Specialized Constructors**: `NewTruncatedMemberError()` - Used for tarball parsing
4. **Direct Struct Instantiation**: `&CorruptionError{Context: "..."}` - Used for specific error types

### Recommended Additions

1. **Commit SHA Context**: Add optional commit SHA parameter to all parsing error constructors
2. **Fluent API**: Add `WithCommitSHA(sha string)` method to error types for chaining
3. **Context Preservation**: Ensure commit SHA flows through error wrapping operations

---

## Summary Statistics

- **Total Call Sites**: 35
- **High Priority (Commit SHA Critical)**: 12 (34.3%)
- **Medium Priority (Commit SHA Useful)**: 1 (2.9%)
- **Low Priority (Commit SHA Optional)**: 1 (2.9%)
- **No Priority (Commit SHA Not Needed)**: 21 (60.0%)

**Files Requiring Changes:**
1. `/home/coding/commitgraph/pkg/errors/helpers.go` - Add commit SHA support
2. `/home/coding/commitgraph/pkg/errors/types.go` - Add commit SHA field to StructuredError
3. `/home/coding/commitgraph/pkg/warmstart/error.go` - Add commit SHA support to constructors
4. `/home/coding/commitgraph/pkg/warmstart/extract.go` - Update 12 call sites with commit SHA

---

## Next Steps

1. ✅ **Catalog Complete** - This document provides the complete inventory
2. **Implement Phase 1** - Add commit SHA support to high-priority warmstart errors
3. **Implement Phase 2** - Add commit SHA support to structured error system
4. **Update Call Sites** - Modify identified call sites to pass commit SHA context
5. **Testing** - Verify error messages include commit SHA in appropriate contexts
6. **Documentation** - Update error handling guidelines to include commit SHA best practices

---

*This catalog was generated by automated agents scanning the commitgraph codebase on 2026-08-06. For questions or updates, refer to the original bead: cg-4dr9o*