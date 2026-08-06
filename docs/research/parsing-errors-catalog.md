# Parsing Errors Catalog

## Overview

This document catalogs all parsing error type definitions, constructors, and their call sites in the commitgraph codebase. It identifies which errors currently support commit SHA tracking and which need to be updated.

**Generated:** 2026-08-06
**Scope:** Entire codebase (`pkg/`, `cmd/`, and related packages)

---

## 1. Error Type Definitions

### 1.1 Primary Error Package (`pkg/errors/`)

#### StructuredError Type
**File:** `pkg/errors/types.go` (Lines 112-143)

```go
type StructuredError struct {
    Type      ErrorCategory
    Severity  SeverityLevel
    Message   string
    Code      string
    Component string
    Operation string
    Context   ErrorContext
    CommitSHA string         // ← Commit SHA tracking field
    Position  int64          // Position/offset in data stream
    Email     string
    TraceID   string
    RecordKey string
    Cause     error
    StackTrace string
    Timestamp time.Time
    Retryable bool
    RetryPolicy RetryPolicy
    Recovery  RecoverySuggestion
    Metadata  map[string]interface{}
}
```

**Commit SHA Support:** ✅ YES - StructuredError has `CommitSHA string` field

---

### 1.2 Warmstart Package (`pkg/warmstart/`)

#### Error Type
**File:** `pkg/warmstart/error.go` (Lines 46-65)

```go
type Error struct {
    Kind       ErrorKind
    Context    string
    MemberName string
    Offset     int64
    CommitSHA  string         // ← Commit SHA tracking field
    Underlying error
}
```

**Commit SHA Support:** ✅ YES - Warmstart Error has `CommitSHA string` field

#### ErrorKind Constants
**File:** `pkg/warmstart/error.go` (Lines 9-27)

```go
type ErrorKind int
const (
    Truncated ErrorKind = iota  // Tarball was cut off or incomplete
    MissingMember               // Required tarball member not found
    CorruptPack                 // Pack file data corruption detected
    IO                          // I/O error
    Other                       // Uncategorized error
)
```

---

## 2. Error Constructor Catalog

### 2.1 Structured Error Constructors (`pkg/errors/helpers.go`)

| Constructor | Signature | Commit SHA Param | File Location | Status |
|-------------|-----------|------------------|---------------|---------|
| `ParseErrorf` | `func ParseErrorf(component, operation, dataType, format string, args ...interface{}) *StructuredError` | ❌ NO | Line 48 | ⚠️ **NEEDS UPDATE** |
| `ParseErrorfWithCommit` | `func ParseErrorfWithCommit(component, operation, dataType, commitSHA, format string, args ...interface{}) *StructuredError` | ✅ YES | Line 53 | ✅ Complete |
| `JSONParseError` | `func JSONParseError(component, operation string) *StructuredError` | ❌ NO | Line 80 | ⚠️ **NEEDS UPDATE** |
| `JSONParseErrorWithCommit` | `func JSONParseErrorWithCommit(component, operation, commitSHA string) *StructuredError` | ✅ YES | Line 85 | ✅ Complete |

**Implementation Pattern:**
- `ParseErrorf()` calls `ParseErrorfWithCommit()` with empty `commitSHA` ("")
- `JSONParseError()` calls `JSONParseErrorWithCommit()` with empty `commitSHA` ("")

---

### 2.2 Warmstart Error Constructors (`pkg/warmstart/error.go`)

| Constructor | Signature | Commit SHA Param | File Location | Status |
|-------------|-----------|------------------|---------------|---------|
| `NewIOError` | `func NewIOError(context string, err error, commitSHA string) *Error` | ✅ YES | Line 151 | ✅ Complete |
| `NewTruncatedError` | `func NewTruncatedError(context string, offset int64, commitSHA string) *Error` | ✅ YES | Line 161 | ✅ Complete |
| `NewTruncatedMemberError` | `func NewTruncatedMemberError(memberName string, context string, offset int64, commitSHA string) *Error` | ✅ YES | Line 171 | ✅ Complete |
| `NewMissingMemberError` | `func NewMissingMemberError(memberName string, commitSHA string) *Error` | ✅ YES | Line 182 | ✅ Complete |
| `NewMissingMemberErrorWithContext` | `func NewMissingMemberErrorWithContext(memberName string, context string, commitSHA string) *Error` | ✅ YES | Line 192 | ✅ Complete |
| `NewCorruptPackError` | `func NewCorruptPackError(memberName string, context string, commitSHA string) *Error` | ✅ YES | Line 202 | ✅ Complete |

**Note:** ALL warmstart constructors already support commit SHA parameter.

---

### 2.3 Context Setter Methods (`pkg/errors/types.go`)

| Method | Signature | Purpose | File Location |
|--------|-----------|---------|---------------|
| `WithCommitSHA` | `func (e *StructuredError) WithCommitSHA(commitSHA string) *StructuredError` | Chainable method to add commit SHA to any error | Line 389 |
| `WithCommitSHAOption` | `func WithCommitSHAOption(commitSHA string) ErrorContextOption` | Functional option for NewError() | Line 225 |

---

## 3. Call Site Catalog

### 3.1 Warmstart Package Call Sites (`pkg/warmstart/extract.go`)

| Line | Constructor Used | Commit SHA Passed | Status | Notes |
|------|------------------|-------------------|---------|-------|
| 143 | `NewCorruptPackError("ref", "empty ref data in ref file", "")` | ❌ Empty string | ⚠️ **NEEDS UPDATE** | Should pass actual commit SHA |
| 152 | `NewCorruptPackError("ref", fmt.Sprintf("invalid ref format..."), possibleSHA)` | ✅ YES (variable `possibleSHA`) | ✅ Complete | |
| 212 | `NewMissingMemberError(".pack", currentCommitSHA)` | ✅ YES (variable `currentCommitSHA`) | ✅ Complete | |
| 248 | `NewMissingMemberError(".idx", currentCommitSHA)` | ✅ YES (variable `currentCommitSHA`) | ✅ Complete | |
| 257 | `NewMissingMemberErrorWithContext(".ref", fmt.Sprintf(...), currentCommitSHA)` | ✅ YES (variable `currentCommitSHA`) | ✅ Complete | |
| 262 | `NewCorruptPackError("config.json", fmt.Sprintf("failed to parse config.json: %v", err), currentCommitSHA)` | ✅ YES (variable `currentCommitSHA`) | ✅ Complete | |
| 265 | `NewCorruptPackError("config.json", fmt.Sprintf("config validation failed: %v", err), currentCommitSHA)` | ✅ YES (variable `currentCommitSHA`) | ✅ Complete | |
| 626 | `NewCorruptPackError("", fmt.Sprintf("git fsck detected corruption: %s", outputStr))` | ❌ Missing | ⚠️ **NEEDS UPDATE** | Function context: `VerifyGitFsck()` - no commit SHA available |
| 669 | `NewCorruptPackError("", fmt.Sprintf("git log detected corruption: %s", outputStr))` | ❌ Missing | ⚠️ **NEEDS UPDATE** | Function context: `VerifyGitLog()` - no commit SHA available |

**Warmstart Summary:**
- Total call sites: 9
- With commit SHA: 6 ✅
- Missing commit SHA: 3 ⚠️
- Special cases: Lines 626, 669 are in verification functions that operate on entire git repos, not specific commits

---

### 3.2 Handler Package Call Sites (`pkg/handler/audit_logs.go`)

**Note:** The handler package uses `fmt.Errorf()` instead of structured error constructors.

| Line | Function | Error Type | Commit SHA Support | Status |
|------|----------|------------|-------------------|---------|
| 115 | `parseQueryParams()` | `fmt.Errorf("invalid repo_id...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| 125 | `parseQueryParams()` | `fmt.Errorf("invalid start_date...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| 135 | `parseQueryParams()` | `fmt.Errorf("invalid end_date...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| 151 | `parseQueryParams()` | `fmt.Errorf("invalid limit...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| 163 | `parseQueryParams()` | `fmt.Errorf("invalid offset...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| 178 | `parseDate()` | `fmt.Errorf("invalid date format...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| 184 | `parseDate()` | `fmt.Errorf("invalid date...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| 202 | `Validate()` | `fmt.Errorf("invalid repo_id...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| 207 | `Validate()` | `fmt.Errorf("invalid limit...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| 212 | `Validate()` | `fmt.Errorf("invalid offset...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| 218 | `Validate()` | `fmt.Errorf("invalid event_type...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |

**Handler Summary:**
- Total call sites: 11
- All use `fmt.Errorf()` instead of structured error constructors
- All need migration to `errors.ParseErrorf()` or `errors.ValidationErrorf()`
- Context: HTTP request parameter parsing (no commit context available)

---

### 3.3 Identity Package Call Sites (`pkg/identity/`)

**Note:** The identity package uses `fmt.Errorf()` instead of structured error constructors.

| File | Line | Function | Error Type | Commit SHA Support | Status |
|------|------|----------|------------|-------------------|---------|
| ingest.go | 34 | `Validate()` | `fmt.Errorf("email cannot be empty")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| ingest.go | 37 | `Validate()` | `fmt.Errorf("login cannot be empty")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| ingest.go | 43 | `Validate()` | `fmt.Errorf("invalid source...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| ingest.go | 46 | `Validate()` | `fmt.Errorf("resolved_at cannot be zero")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| ingest.go | 142 | `IngestFromCSV()` | `fmt.Errorf("row %d: %w", idx, err)` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| snapshot.go | 80 | `TakeSnapshot()` | `fmt.Errorf("failed to query email_resolution: %w")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| snapshot.go | 95 | `TakeSnapshot()` | `fmt.Errorf("failed to scan row %d: %w")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| snapshot.go | 101 | `TakeSnapshot()` | `fmt.Errorf("invalid source %q...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| snapshot.go | 109 | `TakeSnapshot()` | `fmt.Errorf("failed to write to hash: %w")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| snapshot.go | 126 | `TakeSnapshot()` | `fmt.Errorf("error iterating rows: %w")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| snapshot.go | 153 | `Equal()` | `fmt.Errorf("first snapshot is nil")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| snapshot.go | 156 | `Equal()` | `fmt.Errorf("second snapshot is nil")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| snapshot.go | 161 | `Equal()` | `fmt.Errorf("row count differs...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| snapshot.go | 166 | `Equal()` | `fmt.Errorf("data hash differs...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |

**Identity Summary:**
- Total call sites: 14
- All use `fmt.Errorf()` instead of structured error constructors
- Context: CSV parsing and database operations (some may have commit context)

---

### 3.4 Command Package Call Sites (`cmd/`)

**Note:** Command packages use `fmt.Errorf()` instead of structured error constructors.

| File | Line | Function | Error Type | Commit SHA Support | Status |
|------|------|----------|------------|-------------------|---------|
| load-email-resolution-from-queue-api/main.go | 205 | `parseValuesString()` | `fmt.Errorf("expected 12 values, got %d")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| audit-logs/main.go | 201 | `parseDate()` | `fmt.Errorf("invalid date format...")` | ❌ NO | ⚠️ **NEEDS MIGRATION** |
| get-audit-logs/main.go | 216+ | `parseDate()` | (validation errors) | ❌ NO | ⚠️ **NEEDS MIGRATION** |

**Command Summary:**
- Total call sites: 3+
- All use `fmt.Errorf()` instead of structured error constructors
- Context: Data parsing and validation (no commit context)

---

## 4. Summary Statistics

### 4.1 Constructor Status

| Package | Total Constructors | With Commit SHA | Without Commit SHA | % Complete |
|---------|-------------------|-----------------|-------------------|------------|
| `pkg/errors` | 4 | 2 | 2 | 50% |
| `pkg/warmstart` | 6 | 6 | 0 | 100% |
| **TOTAL** | **10** | **8** | **2** | **80%** |

### 4.2 Call Site Status

| Package | Total Call Sites | With Commit SHA | Without Commit SHA | Using fmt.Errorf | % Complete |
|---------|-----------------|----------------|-------------------|------------------|------------|
| `pkg/warmstart` | 9 | 6 | 3 | 0 | 67% |
| `pkg/handler` | 11 | 0 | 11 | 11 | 0% |
| `pkg/identity` | 14 | 0 | 14 | 14 | 0% |
| `cmd/` | 3+ | 0 | 3+ | 3+ | 0% |
| **TOTAL** | **37+** | **6** | **31+** | **28+** | **16%** |

### 4.3 Issues Identified

#### High Priority
1. **pkg/errors/helpers.go**: `ParseErrorf()` and `JSONParseError()` need to be updated or replaced with commit SHA versions
2. **pkg/warmstart/extract.go line 143**: Empty commit SHA passed - needs actual commit context
3. **pkg/warmstart/extract.go lines 626, 669**: Verification functions lack commit SHA parameter

#### Medium Priority
4. **pkg/handler/**: All parsing errors use `fmt.Errorf()` - need migration to structured errors
5. **pkg/identity/**: All parsing/validation errors use `fmt.Errorf()` - need migration to structured errors
6. **cmd/***: All parsing errors use `fmt.Errorf()` - need migration to structured errors

---

## 5. Recommended Actions

### Phase 1: Update Core Constructors (High Impact)
1. Update all call sites of `ParseErrorf()` to use `ParseErrorfWithCommit()` with actual commit SHA
2. Update all call sites of `JSONParseError()` to use `JSONParseErrorWithCommit()` with actual commit SHA
3. Fix empty commit SHA in `pkg/warmstart/extract.go:143`

### Phase 2: Handle Verification Functions (Special Case)
4. Add optional commit SHA parameter to `VerifyGitFsck()` and `VerifyGitLog()` functions
5. Pass commit SHA when calling these functions in contexts where it's available

### Phase 3: Migrate Handler Package (Medium Impact)
6. Migrate `pkg/handler/audit_logs.go` parsing errors to use `errors.ParseErrorf()`
7. Note: Handler functions parse HTTP parameters - may not have commit context

### Phase 4: Migrate Identity Package (Lower Impact)
8. Migrate `pkg/identity/` parsing errors to use structured error constructors
9. Identify which contexts have commit SHA available

### Phase 5: Migrate Command Package (Lower Impact)
10. Migrate `cmd/` parsing errors to use structured error constructors

---

## 6. Context Notes

### 6.1 When Commit SHA is Available
- During warmstart materialization from git tarballs
- During commit graph processing
- When parsing commit-specific data

### 6.2 When Commit SHA is NOT Available
- HTTP request parameter parsing (handler package)
- Generic data validation (identity package)
- Verification functions that check entire repositories (warmstart)
- Command-line tool input parsing (cmd package)

### 6.3 Design Considerations
- Not all parsing errors need commit SHA context
- Commit SHA is most valuable for errors that prevent specific commits from being processed
- Generic validation errors (HTTP params, CLI input) don't require commit tracking
- Verification errors at repo level may not have specific commit context

---

## Appendix A: Search Methods Used

This catalog was created using:
1. **Agent-based exploration**: Comprehensive search for error type definitions
2. **Grep patterns**: 
   - `grep -rn "ParseErrorf|JSONParseError|New.*Error"`
   - `grep -rn "fmt\.Errorf.*invalid|fmt\.Errorf.*parse"`
   - `grep -rn "json\.Unmarshal|json\.Decode"`
3. **Manual code review**: Examined specific files for context

---

## Appendix B: Related Documentation

- **Error types**: `pkg/errors/types.go`
- **Error helpers**: `pkg/errors/helpers.go`
- **Warmstart errors**: `pkg/warmstart/error.go`
- **Existing error documentation**: See `pkg/` package documentation

---

**Document Version:** 1.0
**Last Updated:** 2026-08-06
**Status:** Initial catalog complete
**Next Review:** After implementation of recommended actions
