# MissingMember Error Patterns for .pack/.idx/.ref Validation

## Overview
This document describes the standard error format and patterns used for MissingMember errors when validating pack file members (.pack, .idx, .ref) in warm-start tarballs.

## Error Structure

### Type Definition
```go
type Error struct {
    Kind       ErrorKind   // MissingMember for these errors
    Context    string      // Human-readable details about what went wrong
    MemberName string      // The tarball member name (e.g., ".pack", ".idx", ".ref")
    Offset     int64       // Byte offset (not used for MissingMember)
    Underlying error       // Wrapped error (not used for MissingMember)
}
```

### Constructors
```go
// Basic MissingMember error (for .pack and .idx)
func NewMissingMemberError(memberName string) *Error

// MissingMember error with additional context (for .ref with file list)
func NewMissingMemberErrorWithContext(memberName string, context string) *Error
```

## Error Message Format

### Template
```
warmstart: {kind} (member={name}) - {context}
```

### Component Breakdown
- **Prefix**: `"warmstart: "` - fixed prefix
- **Kind**: `"missing required member"` - from ErrorKind.String()
- **Member**: `(member={name})` - parenthesized member name
- **Context**: `- {context}` - dash-separated details (optional)

## Usage Patterns by File Type

### .pack Files
**Constructor**: `NewMissingMemberError(".pack")`
**Error Message**: `"warmstart: missing required member (member=.pack)"`

**Example Usage** (extract.go:205):
```go
if !foundPack {
    return nil, NewMissingMemberError(".pack")
}
```

**Pattern**: Simple error without context - the member name ".pack" is sufficient to identify the missing file type.

### .idx Files  
**Constructor**: `NewMissingMemberError(".idx")`
**Error Message**: `"warmstart: missing required member (member=.idx)"`

**Example Usage** (extract.go:229):
```go
if !foundIdx {
    return nil, NewMissingMemberError(".idx")
}
```

**Pattern**: Simple error without context - the member name ".idx" is sufficient to identify the missing file type.

### .ref Files
**Constructor**: `NewMissingMemberErrorWithContext(".ref", context)`
**Error Message**: `"warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref"`

**Example Usage** (extract.go:236):
```go
missingRefFiles := CollectMissingRefFiles(snapshot.PackFiles)
if len(missingRefFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ref", 
        fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
}
```

**Pattern**: Error WITH context - lists all missing .ref files by full path, separated by commas.

**Context Format**: `"missing .ref files: {path1}, {path2}, ..."`
- Example: `"missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref"`

## Validation Logic

### .pack Validation (extract.go:196-206)
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

### .idx Validation (extract.go:208-231)
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

### .ref Validation (extract.go:233-237)
```go
// Validate that corresponding .ref files exist for each .pack file
missingRefFiles := CollectMissingRefFiles(snapshot.PackFiles)
if len(missingRefFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ref", 
        fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
}
```

## Test Coverage Examples

### .pack Missing Test (extract_test.go:192-239)
```go
func TestParseTarball_MissingPackFileMember(t *testing.T) {
    // Tarball has .idx and .promisor but NO .pack file
    // ...
    
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
}
```

### .idx Missing Test (extract_test.go:1641-1681)
```go
func TestParseTarball_MissingIdxFileMember(t *testing.T) {
    // Tarball has .pack and .promisor but NO .idx file
    // ...
    
    if missingErr.Kind != MissingMember {
        t.Errorf("expected MissingMember error kind, got %v", missingErr.Kind)
    }
    
    if missingErr.MemberName != ".idx" {
        t.Errorf("expected member name '.idx', got %s", missingErr.MemberName)
    }
}
```

### .ref Missing Test (extract_test.go:1867-1917)
```go
func TestParseTarball_MultipleMissingRefFiles(t *testing.T) {
    // Tarball has .pack and .idx but NO .ref files
    // ...
    
    if missingErr.Kind != MissingMember {
        t.Errorf("expected MissingMember error kind, got %v", missingErr.Kind)
    }
    
    if missingErr.MemberName != ".ref" {
        t.Errorf("expected member name '.ref', got %s", missingErr.MemberName)
    }
    
    // Verify error context lists both missing files
    if !strings.Contains(missingErr.Context, "objects/pack/pack-abc.ref") {
        t.Errorf("error context should list missing pack-abc.ref, got: %s", missingErr.Context)
    }
    if !strings.Contains(missingErr.Context, "objects/pack/pack-def.ref") {
        t.Errorf("error context should list missing pack-def.ref, got: %s", missingErr.Context)
    }
}
```

## Summary Table

| File Type | Constructor | Context Format | Example Message |
|-----------|-------------|----------------|-----------------|
| `.pack` | `NewMissingMemberError(".pack")` | None | `warmstart: missing required member (member=.pack)` |
| `.idx` | `NewMissingMemberError(".idx")` | None | `warmstart: missing required member (member=.idx)` |
| `.ref` | `NewMissingMemberErrorWithContext(".ref", ...)` | `"missing .ref files: {paths}"` | `warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref` |

## Key Design Decisions

1. **.pack and .idx use simple errors**: Since these are binary-critical files, any absence is a fatal error and the file type alone is sufficient context.

2. **.ref uses detailed context**: When multiple .ref files are missing, listing them all helps debugging which pack files are affected.

3. **Consistent member naming**: All member names use the extension with a leading dot (`.pack`, `.idx`, `.ref`) for clarity.

4. **Full paths in context**: Missing .ref files are listed with their full tarball paths (`objects/pack/pack-abc.ref`) to uniquely identify them.

5. **Comma-separated lists**: Multiple missing files in context are joined with `", "` for readability.
