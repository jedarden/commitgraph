# MissingMember Error Patterns Documentation

## Overview

This document describes how `MissingMember` errors are currently formatted and used for `.pack` and `.idx` file validation in the warmstart package.

## Error Structure

The `MissingMember` error is a structured error type defined in `pkg/warmstart/error.go`:

```go
type Error struct {
    Kind       ErrorKind    // MissingMember for required tarball member errors
    Context    string       // Human-readable details about what went wrong (optional)
    MemberName string       // The tarball member name (e.g., ".pack", ".idx", ".ref")
    Offset     int64        // Byte offset in the tarball (not used for MissingMember)
    Underlying error        // Original error (not used for MissingMember)
}
```

## Error Kind

`MissingMember` is one of several error kinds:
- `Truncated` - Tarball was cut off or incomplete
- `MissingMember` - Required tarball member was not found
- `CorruptPack` - Pack file data corruption detected
- `IO` - Underlying input/output error occurred
- `Other` - Uncategorized error

## Constructor Functions

### NewMissingMemberError

Creates a basic `MissingMember` error with just the member name:

```go
func NewMissingMemberError(memberName string) *Error
```

**Usage examples from `extract.go`:**
```go
// Line 205: Missing .pack file
return nil, NewMissingMemberError(".pack")

// Line 229: Missing .idx file  
return nil, NewMissingMemberError(".idx")
```

### NewMissingMemberErrorWithContext

Creates a `MissingMember` error with additional context (used for listing multiple missing files):

```go
func NewMissingMemberErrorWithContext(memberName string, context string) *Error
```

**Usage example from `extract.go`:**
```go
// Line 236: Missing .ref files (with context listing all missing files)
missingRefFiles := CollectMissingRefFiles(snapshot.PackFiles)
if len(missingRefFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ref", 
        fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
}
```

## Error Message Format

### Error Method

The `Error()` method formats the message as follows:

```go
func (e *Error) Error() string {
    return fmt.Sprintf("warmstart: %s%s", e.Kind, details)
}
```

Where `details` is constructed from:
1. Member name: ` (member=<MemberName>)` - if `MemberName` is set
2. Context: ` - <Context>` - if `Context` is set (added after member name)

### Format Examples

#### Basic MissingMember Error (no context)
```go
err := NewMissingMemberError(".pack")
// Output: "warmstart: missing required member (member=.pack)"
```

#### MissingMember Error with Context
```go
err := NewMissingMemberErrorWithContext(".ref", 
    "missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref")
// Output: "warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref"
```

## Current Usage Patterns

### For .pack Files

**Location:** `pkg/warmstart/extract.go` line 205

```go
// Validate that at least one .pack file is present
foundPack := false
for _, pf := range snapshot.PackFiles {
    if strings.HasSuffix(pf.Name, ".pack") {
        foundPack = true
        break
    }
}
if !foundPack {
    return nil, NewMissingMemberError(".pack")
}
```

**Pattern:** Simple error, no context needed (only checks if ANY .pack file exists)

### For .idx Files

**Location:** `pkg/warmstart/extract.go` lines 218-231

```go
// Collect base names of all .pack files for corresponding file validation
var packBaseNames []string
for _, pf := range snapshot.PackFiles {
    if strings.HasSuffix(pf.Name, ".pack") {
        baseName := strings.TrimSuffix(pf.Name, ".pack")
        packBaseNames = append(packBaseNames, baseName)
    }
}

// Validate that corresponding .idx files exist for each .pack file
for _, baseName := range packBaseNames {
    idxName := baseName + ".idx"
    foundIdx := false
    for _, pf := range snapshot.PackFiles {
        if pf.Name == idxName {
            foundIdx = true
            break
        }
    }
    if !foundIdx {
        return nil, NewMissingMemberError(".idx")
    }
}
```

**Pattern:** Simple error, no context needed (fails on first missing .idx)

### For .ref Files

**Location:** `pkg/warmstart/extract.go` lines 233-237

```go
// Validate that corresponding .ref files exist for each .pack file
missingRefFiles := CollectMissingRefFiles(snapshot.PackFiles)
if len(missingRefFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ref", 
        fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
}
```

**Pattern:** Error with context listing ALL missing .ref files (uses `CollectMissingRefFiles()` helper)

## Test Coverage

The tests in `pkg/warmstart/extract_test.go` verify:

### Missing .pack File (TestParseTarball_MissingPackFileMember)

```go
var missingErr *Error
if !errors.As(err, &missingErr) {
    t.Fatalf("expected *Error type, got %T: %v", err, err)
}

if missingErr.Kind != MissingMember {
    t.Errorf("expected MissingMember error kind, got %v", missingErr.Kind)
}

if missingErr.MemberName != ".pack" {
    t.Errorf("expected member name '.pack', got %s", missingErr.MemberName)
}
```

### Missing .idx File (TestParseTarball_MissingIdxFileMember)

Same pattern as .pack, checking for `.idx` member name.

### Missing .ref File (TestParseTarball_MissingRefFileMember)

Same pattern, checking for `.ref` member name.

### Multiple Missing .ref Files (TestParseTarball_MultipleMissingRefFiles)

Additionally verifies the context field lists all missing files:

```go
// Verify error context lists both missing files
if !strings.Contains(missingErr.Context, "objects/pack/pack-abc.ref") {
    t.Errorf("error context should list missing pack-abc.ref, got: %s", missingErr.Context)
}
if !strings.Contains(missingErr.Context, "objects/pack/pack-def.ref") {
    t.Errorf("error context should list missing pack-def.ref, got: %s", missingErr.Context)
}
```

## Reference Example

For implementing similar validation for other file types, use this pattern:

```go
// Simple case (any file will do, no context needed)
if !foundFile {
    return nil, NewMissingMemberError(".ext")
}

// Context case (need to list all missing files)
missingFiles := collectMissingFiles()
if len(missingFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ext", 
        fmt.Sprintf("missing .ext files: %s", strings.Join(missingFiles, ", ")))
}
```

## Files Containing MissingMember Logic

- `pkg/warmstart/error.go` - Error type definition and constructors
- `pkg/warmstart/extract.go` - Validation logic using MissingMember errors
- `pkg/warmstart/error_test.go` - Unit tests for error construction
- `pkg/warmstart/extract_test.go` - Integration tests for validation
