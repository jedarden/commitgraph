# Typed Error Types for Tarball Operations (cg-317vk)

## Implementation Summary

The typed error types for tarball operations have been successfully implemented in `pkg/warmstart/error.go` and committed as 052dc30.

## Acceptance Criteria Verification

- ✅ **Error enum defined in appropriate module**: `ErrorKind` enum in `pkg/warmstart/error.go`
- ✅ **ErrorKind enum covers all required corruption modes**: Truncated, MissingMember, CorruptPack, IO, Other
- ✅ **Error implements error interface**: `Error()` method provides human-readable messages, `Unwrap()` supports errors.Is/As
- ✅ **Error variants carry relevant context**: MemberName, Offset, Context, Underlying fields
- ✅ **Error can be constructed from underlying io::Error with context**: `NewIOError()` constructor

## Implementation Details

### ErrorKind Variants
- `Truncated`: Tarball was cut off or incomplete
- `MissingMember`: Required tarball member was not found
- `CorruptPack`: Pack file data corruption detected
- `IO`: Underlying input/output error occurred
- `Other`: Uncategorized error

### Error Struct
```go
type Error struct {
    Kind       ErrorKind
    Context    string
    MemberName string
    Offset     int64
    Underlying error
}
```

### Constructors
- `NewIOError(context string, err error) *Error`
- `NewTruncatedError(context string, offset int64) *Error`
- `NewMissingMemberError(memberName string) *Error`
- `NewCorruptPackError(memberName string, context string) *Error`

### Example Error Messages
- `warmstart: truncated tarball: unexpected EOF`
- `warmstart: missing required member (member=config.json)`
- `warmstart: corrupt pack data (member=objects/pack/pack-123.pack) - SHA256 checksum mismatch`
- `warmstart: I/O error: disk full - failed to write pack file`

## Test Coverage

All tests pass (617 lines added in error_test.go):
- ErrorKind string representation
- Error message formatting with various field combinations
- Unwrap() for errors.Is/As compatibility
- Constructor function validation
- Permission error detection helpers
- Comprehensive formatting examples

## Backward Compatibility

Legacy error types maintained:
- `CorruptionError` (deprecated, use Error with CorruptPack/Truncated)
- `NotAGitRepoError` (deprecated, use Error with Other)

## Status

**COMPLETED** - Implementation committed and all tests passing.
