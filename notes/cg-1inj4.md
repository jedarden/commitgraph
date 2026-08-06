# Task cg-1inj4: MissingMember Error Implementation

## Status: Already Complete ✓

The MissingMember error functionality is **already fully implemented** in the codebase.

## Implementation Summary

The MissingMember error handling is implemented in `pkg/warmstart/error.go`:

1. **ErrorKind Variant**: `MissingMember` is defined as an ErrorKind constant (line 17)
2. **Error Structure**: The `Error` struct includes a `MemberName string` field (line 55)
3. **Error Message**: The `Error()` method (lines 65-97) includes the member name in the output
4. **Constructor Function**: `NewMissingMemberError(memberName string)` creates errors with Kind=MissingMember (lines 167-173)
5. **Display String**: The ErrorKind.String() method returns "missing required member" for MissingMember (line 34)

## Verification

All acceptance criteria are met:
- ✅ **MissingMember variant exists**: Defined as ErrorKind constant
- ✅ **Error message includes missing member name**: Format is `"warmstart: missing required member (member=<name>)"`
- ✅ **Code compiles**: `go build ./pkg/warmstart/...` succeeds
- ✅ **Tests pass**: All warmstart tests pass, including `TestNewMissingMemberError`

## Example Output

```go
err := NewMissingMemberError("config.json")
// Error message: "warmstart: missing required member (member=config.json)"
```

The implementation is production-ready and fully tested.
