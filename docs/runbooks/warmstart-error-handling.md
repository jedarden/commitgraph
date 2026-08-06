# Warmstart Error Handling

This document describes the typed error handling for tarball corruption in the warmstart package, including error types, when they occur, and how to handle them.

## Overview

The warmstart package provides comprehensive error detection and typed errors for all corruption and failure modes that can occur during tarball extraction and materialization. All errors implement the standard `error` interface and can be inspected using `errors.As()` to determine the specific error type and recovery strategy.

## Error Types

The package defines an `Error` struct with an `ErrorKind` field that categorizes the type of failure:

### Truncated (Truncated ErrorKind)

**Description:** The tarball was cut off or incomplete.

**When it occurs:**
- Tarball ends prematurely (unexpected EOF during reading)
- Individual tarball member is truncated (header claims more bytes than available)
- Pack file is too small (< 12 bytes minimum header size)

**Example error message:**
```
warmstart: truncated tarball (member=objects/pack/pack-abc.pack) - expected 100 bytes, got 50
warmstart: truncated tarball (member=objects/pack/pack-xyz.pack) - pack file too small: 4 bytes (minimum 12 bytes for header)
```

**Handling strategy:** Fall back to cold clone. The artifact is corrupt and unusable.

---

### MissingMember (MissingMember ErrorKind)

**Description:** A required tarball member was not found.

**When it occurs:**
- Missing required `.pack` file (no pack files at all)
- Missing corresponding `.idx` file for a `.pack` file
- Missing corresponding `.ref` file for a `.pack` file

**Example error messages:**
```
warmstart: missing required member (member=.pack)
warmstart: missing required member (member=.idx)
warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref
```

**Handling strategy:** Fall back to cold clone. The artifact is incomplete.

---

### CorruptPack (CorruptPack ErrorKind)

**Description:** Pack file data corruption was detected.

**When it occurs:**
- Pack file checksum validation fails (future implementation)
- Pack file header is invalid

**Example error message:**
```
warmstart: corrupt pack data (member=objects/pack/pack-corrupt.pack) - checksum validation failed
```

**Handling strategy:** Fall back to cold clone. Pack data is corrupted.

---

### IO (IO ErrorKind)

**Description:** An underlying input/output error occurred.

**When it occurs:**
- Network errors during artifact fetch
- Temporary I/O failures
- Permission errors (local infrastructure issue)
- Disk space errors (disk full)

**Example error messages:**
```
warmstart: I/O error: connection reset by peer
warmstart: I/O error (member=config.json): permission denied
```

**Handling strategy:** 
- For permission/disk errors: Fail immediately (cold clone will also fail)
- For network/other I/O errors: Fall back to cold clone

---

### Other (Other ErrorKind)

**Description:** An uncategorized error occurred.

**When it occurs:**
- NotAGitRepo error (target directory is not a git repository)
- Other unexpected errors

**Example error message:**
```
warmstart: other error - not a git repository at /path/to/dir: HEAD not found
```

**Handling strategy:** 
- For NotAGitRepo: Fail immediately (invalid target directory)
- For other unknown errors: Fall back to cold clone for robustness

---

## Error Inspection

Use `errors.As()` to inspect the error type and fields:

```go
var werr *warmstart.Error
if errors.As(err, &werr) {
    switch werr.Kind {
    case warmstart.Truncated:
        log.Printf("Truncated error: member=%s, context=%s", werr.MemberName, werr.Context)
        // Fall back to cold clone
    case warmstart.MissingMember:
        log.Printf("Missing member: %s", werr.MemberName)
        // Fall back to cold clone
    case warmstart.CorruptPack:
        log.Printf("Corrupt pack: %s", werr.MemberName)
        // Fall back to cold clone
    case warmstart.IO:
        // Check if it's a permission error (don't fall back)
        if warmstart.IsOsPermissionError(err) {
            return fmt.Errorf("permission error, cannot fall back: %w", err)
        }
        // Check if it's a disk space error (don't fall back)
        if isDiskSpaceError(werr.Underlying) {
            return fmt.Errorf("disk space error, cannot fall back: %w", err)
        }
        // Other I/O errors - fall back
        log.Printf("I/O error, falling back: %v", err)
    case warmstart.Other:
        // Check for NotAGitRepo
        if errors.Is(err, warmstart.ErrNotAGitRepo) {
            return fmt.Errorf("invalid target directory: %w", err)
        }
        // Unknown errors - fall back for robustness
        log.Printf("Unknown error, falling back: %v", err)
    }
}
```

## Fallback Strategy

The recommended fallback pattern is implemented in `CloneWithFallback()`:

```go
func CloneWithFallback(
    repoURL string,
    gitDir string,
    fetchWarmstart func() ([]byte, error),
    coldClone func(url string, dir string) error,
) error {
    // Attempt warmstart
    tarballData, err := fetchWarmstart()
    if err != nil {
        log.Printf("Warmstart artifact not available, falling back to cold clone: %v", err)
        return coldClone(repoURL, gitDir)
    }

    // Parse warmstart tarball
    snapshot, err := ParseTarball(tarballData)
    if err != nil {
        fallback, fatalErr := ShouldFallbackToColdClone(err)
        if fatalErr != nil {
            return fmt.Errorf("warmstart fatal error, cannot fall back: %w", fatalErr)
        }
        if fallback {
            log.Printf("Warmstart extraction failed, falling back to cold clone: %v", err)
            return coldClone(repoURL, gitDir)
        }
        return fmt.Errorf("warmstart extraction error: %w", err)
    }

    // Materialize warmstart snapshot
    if err := Materialize(gitDir, snapshot); err != nil {
        fallback, fatalErr := ShouldFallbackToColdClone(err)
        if fatalErr != nil {
            return fmt.Errorf("warmstart materialization fatal error, cannot fall back: %w", fatalErr)
        }
        if fallback {
            log.Printf("Warmstart materialization failed, falling back to cold clone: %v", err)
            return coldClone(repoURL, gitDir)
        }
        return fmt.Errorf("warmstart materialization error: %w", err)
    }

    // Warmstart succeeded, perform incremental fetch
    log.Printf("Warmstart succeeded, performing incremental fetch")
    return performIncrementalFetch(repoURL, gitDir)
}
```

## Error Detection During Extraction

The `ParseTarball()` function performs comprehensive validation before returning:

1. **Truncated file detection:**
   - Checks for `io.ErrUnexpectedEOF` during member reading
   - Validates byte count matches header size
   - Validates pack file minimum size (12 bytes)

2. **Missing member detection:**
   - Requires at least one `.pack` file
   - Validates each `.pack` has a corresponding `.idx` file
   - Validates each `.pack` has a corresponding `.ref` file
   - Collects all missing `.ref` files and reports them in error context

3. **Corruption detection:**
   - Validates config.json is valid JSON
   - Validates all required config values are present
   - Validates ref file format (legacy and new formats)

## Fatal Errors (Do Not Fall Back)

Some errors indicate local infrastructure issues that will also affect cold clone:

1. **Permission errors:** Filesystem permissions prevent reading/writing
2. **Disk space errors:** Disk is full
3. **NotAGitRepo:** Target directory is not a valid git repository

These errors should fail the job immediately rather than attempting fallback.

## Testing

Comprehensive unit tests cover all error types:

- `TestCorruptionErrors_TruncatedTarball` - Truncated tarball detection
- `TestCorruptionErrors_TruncatedMember` - Truncated member detection
- `TestCorruptionErrors_UndersizedPackFile` - Pack file size validation
- `TestCorruptionErrors_MissingPackMember` - Missing .pack detection
- `TestCorruptionErrors_MissingIdxMember` - Missing .idx detection
- `TestCorruptionErrors_MissingRefMember` - Missing .ref detection
- `TestCorruptionErrors_MultipleMissingRefMembers` - Multiple missing .ref files
- `TestCorruptionErrors_ErrorContextValidation` - Error field validation
- `TestCorruptionErrors_ErrorMessageFormatting` - Error message formatting

Run tests with:
```bash
go test -v ./pkg/warmstart/... -run TestCorruption
```

## References

- Code: `pkg/warmstart/error.go` - Error type definitions
- Code: `pkg/warmstart/extract.go` - ParseTarball validation logic
- Code: `pkg/warmstart/fallback_example.go` - Fallback implementation
- Tests: `pkg/warmstart/corruption_errors_test.go` - Comprehensive error tests
