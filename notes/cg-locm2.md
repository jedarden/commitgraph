# Truncated Tarball Member Detection Verification

## Task: cg-locm2

**Date:** 2026-08-06  
**Component:** `pkg/warmstart/extract.go`

## Acceptance Criteria Verification

### ✅ 1. io.ErrUnexpectedEOF Detection
**Location:** `extract.go:118-121`
```go
if err == io.ErrUnexpectedEOF || errors.Is(err, io.ErrUnexpectedEOF) {
    return nil, NewTruncatedMemberError(hdr.Name, "ended prematurely", 0)
}
```
- **Confirmed:** Detection exists for unexpected EOF during tarball member read
- **Dual check:** Uses both direct equality and `errors.Is()` for compatibility

### ✅ 2. Size Mismatch Detection
**Location:** `extract.go:126-128`
```go
if written != hdr.Size {
    return nil, NewTruncatedMemberError(hdr.Name, fmt.Sprintf("expected %d bytes, got %d", hdr.Size, written), 0)
}
```
- **Confirmed:** Validates bytes written matches header size
- **Context:** Includes expected vs actual byte count in error message

### ✅ 3. Pack File Minimum Size Validation
**Location:** `extract.go:160-162`
```go
if ext == ".pack" && len(data) < 12 {
    return nil, NewTruncatedMemberError(hdr.Name, fmt.Sprintf("pack file too small: %d bytes (minimum 12 bytes for header)", len(data)), 0)
}
```
- **Confirmed:** Enforces 12-byte minimum for pack file header
- **Minimum:** "PACK" (4) + version (4) + object count (4) = 12 bytes

### ✅ 4. Error Kind = Truncated
**Location:** `error.go:158-165`
```go
func NewTruncatedMemberError(memberName string, context string, offset int64) *Error {
    return &Error{
        Kind:       Truncated,  // ✅ Sets Kind to Truncated
        MemberName: memberName,
        Context:    context,
        Offset:     offset,
    }
}
```
- **Confirmed:** All three detection paths use `NewTruncatedMemberError`
- **Verified:** Function sets `Kind: Truncated` in returned Error struct

### ✅ 5. MemberName Field Population
All three detection calls pass the member name:
- Line 120: `NewTruncatedMemberError(hdr.Name, ...)`
- Line 127: `NewTruncatedMemberError(hdr.Name, ...)`
- Line 161: `NewTruncatedMemberError(hdr.Name, ...)`
- **Confirmed:** `hdr.Name` (the tarball member path) is included in all error paths

## Additional Findings

### Test Coverage
The implementation has comprehensive test coverage in `extract_test.go`:
- `TestParseTarball_UnexpectedEOF` - Tests io.ErrUnexpectedEOF detection
- `TestParseTarball_SizeMismatchDetection` - Tests size mismatch detection
- `TestParseTarball_PackFileHeaderTooSmall` - Tests pack file minimum size validation
- `TestParseTarball_TruncatedErrorHasMemberName` - Validates member name inclusion

### Error Structure
The `Error` struct (lines 46-62 in `error.go`) includes:
- `Kind ErrorKind` - Category of error
- `MemberName string` - Tarball member path
- `Context string` - Human-readable details
- `Offset int64` - Byte offset in tarball
- `Underlying error` - Original error for unwrapping

## Conclusion

✅ **All acceptance criteria met.** The truncated tarball member detection is properly implemented with:
1. Three distinct detection paths (unexpected EOF, size mismatch, minimum size)
2. Consistent error construction with `Kind=Truncated`
3. Member name inclusion in all errors
4. Comprehensive test coverage
