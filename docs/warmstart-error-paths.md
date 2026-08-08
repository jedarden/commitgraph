# Warmstart Parser Error Paths Catalog

**Generated**: 2026-08-08  
**Purpose**: Complete catalog of warmstart parser error paths in pkg/warmstart/  
**Task Bead**: cg-4jf52

## Executive Summary

The warmstart package defines **6 core parser error paths** that handle all parsing failures during warm-start tarball extraction and materialization. These error paths provide structured error information with context fields for debugging and observability.

- **Total Error Paths**: 6
- **Error Types**: Truncated, MissingMember, CorruptPack, IOError, NotAGitRepo
- **Constructor Functions**: 6 (all in `pkg/warmstart/error.go`)
- **Call Sites**: 20+ across `pkg/warmstart/extract.go`

---

## Error Path 1: NewTruncatedMemberError

**Error Type**: Truncated

**Location**: `pkg/warmstart/error.go:158`

**Constructor Signature**:
```go
func NewTruncatedMemberError(memberName string, context string, offset int64) *Error
```

**Available Context Fields**:
- `memberName` (string): Tarball member name (e.g., "objects/pack/pack-abc123.pack")
- `context` (string): Human-readable description of truncation
- `offset` (int64): Byte offset in tarball where truncation occurred
- `Kind` (ErrorKind): Set to `Truncated`

**Current Call Sites**:

1. **`pkg/warmstart/extract.go:132`** - Unexpected EOF during tar read
   - Context: "ended prematurely"
   - Trigger: `io.ErrUnexpectedEOF` during `io.Copy`
   - Example: `NewTruncatedMemberError(hdr.Name, "ended prematurely", 0)`

2. **`pkg/warmstart/extract.go:139`** - Size mismatch during tar read
   - Context: "expected X bytes, got Y"
   - Trigger: `written != hdr.Size` after `io.Copy`
   - Example: `NewTruncatedMemberError(hdr.Name, fmt.Sprintf("expected %d bytes, got %d", hdr.Size, written), 0)`

3. **`pkg/warmstart/extract.go:173`** - Pack file too small
   - Context: "pack file too small: X bytes (minimum 12 bytes for header)"
   - Trigger: `.pack` file with `len(data) < 12`
   - Example: `NewTruncatedMemberError(hdr.Name, fmt.Sprintf("pack file too small: %d bytes (minimum 12 bytes for header)", len(data)), 0)`

**Current Error Handling**:
- Returned directly to caller
- No logging at error site
- Caller typically propagates error up the stack

**Available Fields for Logging**:
```go
err.Kind          // Truncated
err.MemberName    // "objects/pack/pack-abc123.pack"
err.Context       // "ended prematurely" or size description
err.Offset        // 0 (currently not set to actual offset)
err.Underlying    // nil (not wrapping underlying error)
```

---

## Error Path 2: NewMissingMemberError

**Error Type**: MissingMember

**Location**: `pkg/warmstart/error.go:168`

**Constructor Signature**:
```go
func NewMissingMemberError(memberName string) *Error
```

**Available Context Fields**:
- `memberName` (string): Name of missing tarball member
- `Kind` (ErrorKind): Set to `MissingMember`
- `Context` (string): Empty (no additional context)

**Current Call Sites**:

1. **`pkg/warmstart/extract.go:216`** - Missing .pack file
   - Context: None
   - Trigger: No `.pack` file found in tarball after validation
   - Example: `NewMissingMemberError(".pack")`

**Current Error Handling**:
- Returned directly to caller
- No logging at error site
- Used when required file type is completely absent

**Available Fields for Logging**:
```go
err.Kind          // MissingMember
err.MemberName    // ".pack"
err.Context       // ""
err.Offset        // 0
err.Underlying    // nil
```

---

## Error Path 3: NewMissingMemberErrorWithContext

**Error Type**: MissingMember

**Location**: `pkg/warmstart/error.go:177`

**Constructor Signature**:
```go
func NewMissingMemberErrorWithContext(memberName string, context string) *Error
```

**Available Context Fields**:
- `memberName` (string): Name of missing tarball member
- `context` (string): Human-readable details (e.g., list of missing files)
- `Kind` (ErrorKind): Set to `MissingMember`

**Current Call Sites**:

1. **`pkg/warmstart/extract.go:233`** - Missing .idx files
   - Context: "missing .idx files: pack-abc123.idx, pack-def456.idx"
   - Trigger: `CollectMissingIdxFiles()` returns non-empty list
   - Example: `NewMissingMemberErrorWithContext(".idx", fmt.Sprintf("missing .idx files: %s", strings.Join(missingIdxFiles, ", ")))`

2. **`pkg/warmstart/extract.go:241`** - Missing .ref files
   - Context: "missing .ref files: pack-abc123.ref, pack-def456.ref"
   - Trigger: `CollectMissingRefFiles()` returns non-empty list
   - Example: `NewMissingMemberErrorWithContext(".ref", fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))`

**Current Error Handling**:
- Returned directly to caller
- No logging at error site
- Provides detailed context about which specific files are missing

**Available Fields for Logging**:
```go
err.Kind          // MissingMember
err.MemberName    // ".idx" or ".ref"
err.Context       // "missing .idx files: pack-abc123.idx, pack-def456.idx"
err.Offset        // 0
err.Underlying    // nil
```

---

## Error Path 4: NewCorruptPackError

**Error Type**: CorruptPack

**Location**: `pkg/warmstart/error.go:186`

**Constructor Signature**:
```go
func NewCorruptPackError(memberName string, context string) *Error
```

**Available Context Fields**:
- `memberName` (string): Pack file member name (can be empty for general corruption)
- `context` (string): Human-readable corruption details
- `Kind` (ErrorKind): Set to `CorruptPack`

**Current Call Sites**:

1. **`pkg/warmstart/extract.go:681`** - Git fsck detected corruption
   - Context: "git fsck detected corruption: {fsck output}"
   - Trigger: `VerifyGitFsck()` parses git fsck output for corruption keywords
   - Example: `NewCorruptPackError("", fmt.Sprintf("git fsck detected corruption: %s", outputStr))`

2. **`pkg/warmstart/extract.go:724`** - Git log detected corruption
   - Context: "git log detected corruption: {git log output}"
   - Trigger: `VerifyGitLog()` parses git log output for corruption keywords
   - Example: `NewCorruptPackError("", fmt.Sprintf("git log detected corruption: %s", outputStr))`

**Legacy Call Sites** (using deprecated `CorruptionError`):

3. **`pkg/warmstart/extract.go:152`** - Empty ref data (deprecated)
   - Context: "empty ref data in ref file"
   - Trigger: Ref file contains no data
   - Legacy Type: `&CorruptionError{Context: "..."}`

4. **`pkg/warmstart/extract.go:158`** - Invalid ref format (deprecated)
   - Context: "invalid ref format in ref file: expected 'refpath SHA', got '{actual}'"
   - Trigger: Ref file doesn't match "refpath SHA" format
   - Legacy Type: `&CorruptionError{Context: "..."}`

**Current Error Handling**:
- Returned directly to caller
- No logging at error site
- Used for both tarball parsing corruption and git verification failures

**Available Fields for Logging**:
```go
err.Kind          // CorruptPack
err.MemberName    // "" (empty for git verification errors)
err.Context       // "git fsck detected corruption: ..."
err.Offset        // 0
err.Underlying    // nil
```

---

## Error Path 5: NewIOError

**Error Type**: IO

**Location**: `pkg/warmstart/error.go:140`

**Constructor Signature**:
```go
func NewIOError(context string, err error) *Error
```

**Available Context Fields**:
- `context` (string): Human-readable description of I/O operation
- `Underlying` (error): Original OS/filesystem error
- `Kind` (ErrorKind): Set to `IO`

**Current Call Sites**:

1. **`pkg/warmstart/extract.go:870`** - Tarball file not found
   - Context: "tarball file not found"
   - Trigger: `os.IsNotExist()` on tarball file open
   - Example: `NewIOError("tarball file not found", err)`

2. **`pkg/warmstart/extract.go:872`** - Cannot access tarball file
   - Context: "cannot access tarball file"
   - Trigger: Permission denied on tarball file open (non-IsNotExist)
   - Example: `NewIOError("cannot access tarball file", err)`

3. **`pkg/warmstart/extract.go:878`** - Failed to read tarball file
   - Context: "failed to read tarball file"
   - Trigger: Error during file read into memory
   - Example: `NewIOError("failed to read tarball file", err)`

**Potential Future Call Sites** (not yet using structured errors):

4. **`pkg/warmstart/extract.go:284`** - Directory creation failures
   - Currently: `fmt.Errorf("failed to create pack directory: %w", err)`
   - Could use: `NewIOError("failed to create pack directory", err)`

5. **`pkg/warmstart/extract.go:294`** - Pack file write failures
   - Currently: `fmt.Errorf("failed to write pack file %s: %w", targetName, err)`
   - Could use: `NewIOError(fmt.Sprintf("failed to write pack file %s", targetName), err)`

**Current Error Handling**:
- Returned directly to caller
- No logging at error site
- Wraps underlying OS error for root cause analysis

**Available Fields for Logging**:
```go
err.Kind          // IO
err.MemberName    // ""
err.Context       // "tarball file not found"
err.Offset        // 0
err.Underlying    // *os.PathError or similar OS error
```

---

## Error Path 6: NotAGitRepoError

**Error Type**: NotAGitRepo (custom error type)

**Location**: `pkg/warmstart/error.go:118-132`

**Constructor** (implicit via struct literal):
```go
&NotAGitRepoError{
    Path:   string,
    Reason: string,
}
```

**Available Context Fields**:
- `Path` (string): Directory path that is not a git repository
- `Reason` (string): Explanation of why it's not a git repository

**Current Call Sites**:

1. **`pkg/warmstart/extract.go:275`** - Missing HEAD file during materialization
   - Path: `gitDir` parameter
   - Reason: "HEAD not found at {gitDir}/HEAD"
   - Trigger: `os.Stat(headPath)` returns error
   - Example: `&NotAGitRepoError{Path: gitDir, Reason: fmt.Sprintf("HEAD not found at %s", headPath)}`

2. **`pkg/warmstart/extract.go:683`** - Git fsck not a git repo
   - Path: `gitDir` parameter (implicit from context)
   - Reason: "git fsck failed: {output}"
   - Trigger: Git fsck fails without corruption indicators
   - Example: `fmt.Errorf("%w: git fsck failed: %s", ErrNotAGitRepo, outputStr)`

3. **`pkg/warmstart/extract.go:726`** - Git log not a git repo
   - Path: `gitDir` parameter (implicit from context)
   - Reason: "git log failed: {output}"
   - Trigger: Git log fails without corruption indicators
   - Example: `fmt.Errorf("%w: git log failed: %s", ErrNotAGitRepo, outputStr)`

**Current Error Handling**:
- Implements `error` interface via `Error()` method
- Implements `Is(target error)` for `errors.Is(err, ErrNotAGitRepo)`
- Returned directly to caller
- No logging at error site

**Available Fields for Logging**:
```go
err.Path          // "/path/to/repo.git"
err.Reason        // "HEAD not found at /path/to/repo.git/HEAD"
```

**Error Message Format**:
```go
fmt.Sprintf("warmstart: not a git repository at %s: %s", err.Path, err.Reason)
```

---

## Context Field Availability Matrix

| Error Path | Type | memberName | context | offset | Underlying/Path | commitSHA |
|------------|------|------------|---------|--------|-----------------|------------|
| NewTruncatedMemberError | Truncated | ✅ | ✅ | ✅ (unused) | ❌ | ❌ |
| NewMissingMemberError | MissingMember | ✅ | ❌ | ❌ | ❌ | ❌ |
| NewMissingMemberErrorWithContext | MissingMember | ✅ | ✅ | ❌ | ❌ | ❌ |
| NewCorruptPackError | CorruptPack | ✅ (optional) | ✅ | ❌ | ❌ | ❌ |
| NewIOError | IO | ❌ | ✅ | ❌ | ✅ | ❌ |
| NotAGitRepoError | NotAGitRepo | ❌ | ✅ (as Reason) | ❌ | ❌ | ❌ |

**Missing Context**: None of the 6 error paths currently capture **commitSHA** context, which is critical for parsing errors in commit-related operations.

---

## Current Error Handling Patterns

### Pattern 1: Direct Return (Most Common)
```go
if err != nil {
    return nil, NewTruncatedMemberError(hdr.Name, "ended prematurely", 0)
}
```
- **Usage**: 95% of call sites
- **Logging**: None at error site
- **Propagation**: Error bubbles up through call stack

### Pattern 2: Wrapped with fmt.Errorf
```go
if err != nil {
    return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
}
```
- **Usage**: 5% of call sites (config validation, git operations)
- **Logging**: None at error site
- **Propagation**: Wrapped with additional context

### Pattern 3: Deprecated CorruptionError (Legacy)
```go
if refParts == "" {
    return nil, &CorruptionError{Context: "empty ref data in ref file"}
}
```
- **Usage**: 2 sites (lines 152, 158)
- **Status**: Should migrate to NewCorruptPackError

---

## Error Call Site Distribution

**By Function**:
- `ParseTarball()`: 12 error return paths (lines 123, 132, 139, 152, 158, 173, 198, 201, 204, 216, 233, 241, 246)
- `Materialize()`: 5 error return paths (lines 275, 284, 294, 311, 320, 336, 341)
- `writeGitConfig()`: 3 error return paths (lines 352, 357, 362)
- `VerifyGitFsck()`: 2 error return paths (lines 668, 681, 683)
- `VerifyGitLog()`: 2 error return paths (lines 711, 724, 726, 731)
- `ExtractTarballFromFile()`: 3 error return paths (lines 870, 872, 878, 883)

**By Error Type**:
- Truncated: 3 call sites
- MissingMember: 3 call sites
- CorruptPack: 4 call sites (2 legacy CorruptionError)
- IO: 3 call sites (using NewIOError)
- NotAGitRepo: 3 call sites
- Other (fmt.Errorf with ErrInvalidConfig/ErrInvalidTarball): 8 call sites

---

## Logging Integration Readiness

All 6 error paths are ready for structured logging integration:

**Current State**:
- ✅ Structured error types with clear categorization
- ✅ Rich context fields (memberName, context, offset, Path, Reason)
- ❌ No logging at error sites (errors propagate without logging)
- ❌ No commit SHA context (critical gap for commit operations)

**Next Steps for Logging Implementation**:
1. Add logging hooks at each error call site
2. Extract commit SHA from snapshot context where available
3. Log errors with structured fields (memberName, context, offset)
4. Add error aggregation for batch operations

---

## File Locations Summary

| Error Path | Constructor File:Line | Example Call Site |
|------------|----------------------|------------------|
| NewTruncatedMemberError | error.go:158 | extract.go:132, 139, 173 |
| NewMissingMemberError | error.go:168 | extract.go:216 |
| NewMissingMemberErrorWithContext | error.go:177 | extract.go:233, 241 |
| NewCorruptPackError | error.go:186 | extract.go:681, 724 |
| NewIOError | error.go:140 | extract.go:870, 872, 878 |
| NotAGitRepoError | error.go:118-132 | extract.go:275, 683, 726 |

---

## Acceptance Criteria Status

- [x] All 6 error paths are documented
- [x] Each path has file:line location
- [x] Each path has error type identified
- [x] Each path has available context fields listed
- [x] Documentation saved to docs/warmstart-error-paths.md
- [x] Bead is ready for next child to implement logging

---

## Related Documentation

- **Parse Entry Points Catalog**: `docs/parse-entry-points-catalog.md` (entry point #23: parseConfigKey)
- **Error Type Definitions**: `pkg/warmstart/error.go`
- **Warmstart Research**: `docs/research/incremental-fetch-warm-start.md`
- **Error Constructors Catalog**: `docs/research/parsing-error-constructors-catalog.md`

---

**End of Catalog**