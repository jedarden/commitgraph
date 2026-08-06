# Task cg-2zu9s: Missing Member Detection Implementation

## Status: ✅ COMPLETE

The missing member detection feature is fully implemented and all tests pass.

## Implementation Summary

### 1. Error Type Definitions (`pkg/warmstart/error.go`)
- `MissingMember` error kind defined (line 17)
- `NewMissingMemberError(memberName string)` - creates basic missing member error
- `NewMissingMemberErrorWithContext(memberName, context string)` - creates error with additional context

### 2. Detection Logic (`pkg/warmstart/extract.go`)

#### .pack File Detection (lines 196-206)
```go
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

#### .idx File Detection (lines 229-243)
Validates that each .pack file has a corresponding .idx file:
```go
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

#### .ref File Detection (lines 245-251)
Uses `CollectMissingRefFiles()` to detect missing .ref files:
```go
missingRefFiles := CollectMissingRefFiles(snapshot.PackFiles)
if len(missingRefFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ref", fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
}
```

### 3. Test Coverage

All acceptance criteria tests exist and pass:

| Test | File | Coverage |
|------|------|----------|
| `TestParseTarball_MissingPackFileMember` | extract_test.go:198 | Tarball missing .pack file |
| `TestParseTarball_MissingIdxFileMember` | extract_test.go:1683 | Tarball missing .idx file |
| `TestParseTarball_MissingRefFileMember` | extract_test.go:1730 | Tarball missing .ref file |
| `TestParseTarball_MissingRefFile` | extract_test.go:2977 | Multiple .ref files missing |

### 4. Error Propagation

All errors properly propagate to caller via `ParseTarball()` return value:
- Type assertion works: `var missingErr *Error; errors.As(err, &missingErr)`
- Error fields are correctly populated: `Kind`, `MemberName`, `Context`

## Verification Results

```
✅ All warmstart tests pass (0.003s)
✅ MissingMember error raised with correct member names
✅ Error messages contain missing file information
✅ Errors propagate correctly to callers
```

## Acceptance Criteria Status

- [x] Missing member detection validates required files
- [x] MissingMember error raised with missing member name
- [x] Unit test with tarball missing .pack file raises correct error
- [x] Unit test with tarball missing .idx file raises correct error
- [x] Error propagates to caller
