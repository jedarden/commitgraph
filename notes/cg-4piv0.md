# cg-4piv0: Truncated File Detection Implementation Summary

## Task
Implement truncated file detection during tarball extraction using the typed errors.

## Implementation Status: ✅ COMPLETE

All truncated file detection mechanisms are implemented and tested.

### Detection Points (pkg/warmstart/extract.go)

1. **Unexpected EOF Detection** (lines 118-121)
   ```go
   if err == io.ErrUnexpectedEOF || errors.Is(err, io.ErrUnexpectedEOF) {
       return nil, NewTruncatedMemberError(hdr.Name, "ended prematurely", 0)
   }
   ```
   - Catches when tarball member data ends unexpectedly
   - Raises `Error{Kind: Truncated, MemberName: hdr.Name}`

2. **Size Mismatch Detection** (lines 126-128)
   ```go
   if written != hdr.Size {
       return nil, NewTruncatedMemberError(hdr.Name,
           fmt.Sprintf("expected %d bytes, got %d", hdr.Size, written), 0)
   }
   ```
   - Validates bytes read matches tar header size
   - Detects truncated members where header claims more data than present

3. **Pack File Header Size Validation** (lines 160-162)
   ```go
   if ext == ".pack" && len(data) < 12 {
       return nil, NewTruncatedMemberError(hdr.Name,
           fmt.Sprintf("pack file too small: %d bytes (minimum 12 bytes for header)", len(data)), 0)
   }
   ```
   - Ensures pack files have minimum 12-byte header ("PACK" + version + object count)
   - Detects severely undersized pack files

### Error Type (pkg/warmstart/error.go)

- `ErrorKind.Truncated` - Typed error kind for truncated files
- `NewTruncatedMemberError(memberName, context, offset)` - Constructor with full context
- Error messages include: member name, offset, context, and underlying errors

### Test Coverage (pkg/warmstart/extract_test.go)

All tests pass:
- ✅ `TestParseTarball_TruncatedTarball` - Basic truncation detection
- ✅ `TestParseTarball_TruncatedMember` - Member-level truncation
- ✅ `TestParseTarball_TruncatedPackFileExactly11Bytes` - Boundary condition (11 < 12)
- ✅ `TestParseTarball_UnexpectedEOF` - Unexpected EOF detection
- ✅ `TestParseTarball_PackFileHeaderTooSmall` - Pack file size validation
- ✅ `TestMakeMockTarballWithPack_UndersizedPack` - Multiple undersized scenarios (0, 4, 11 bytes)
- ✅ `TestTruncatedError_HasMemberName` - Error includes member context

### Test Results
```bash
$ go test ./pkg/warmstart/... -v -run "Truncated"
=== RUN   TestNewTruncatedError
--- PASS: TestNewTruncatedError (0.00s)
=== RUN   TestParseTarball_TruncatedTarball
--- PASS: TestParseTarball_TruncatedTarball (0.00s)
=== RUN   TestParseTarball_TruncatedMember
--- PASS: TestParseTarball_TruncatedMember (0.00s)
=== RUN   TestParseTarball_TruncatedPackFileExactly11Bytes
--- PASS: TestParseTarball_TruncatedPackFileExactly11Bytes (0.00s)
=== RUN   TestTruncatedError_HasMemberName
--- PASS: TestTruncatedError_HasMemberName (0.00s)
=== RUN   TestParseTarball_TruncatedErrorHasMemberName
--- PASS: TestParseTarball_TruncatedErrorHasMemberName (0.00s)
=== RUN   TestParseTarball_TruncatedPackFileWith11BytePackUsingHelper
--- PASS: TestParseTarball_TruncatedPackFileWith11BytePackUsingHelper (0.00s)
PASS
```

### Error Message Examples

```
warmstart: truncated tarball (member=objects/pack/pack-test.pack) - pack file too small: 11 bytes (minimum 12 bytes for header)
warmstart: truncated tarball (member=objects/pack/pack-123.pack) - ended prematurely
warmstart: truncated tarball (member=config.json) - expected 1024 bytes, got 512
```

### Integration Note

The detection is fully implemented in `pkg/warmstart/ParseTarball()`. 
The caller (referenced as "cw-clone-fallback" in requirements) is a future integration point.
See bead `cg-hnsp`: "Wrap warm-start attempt with full-clone fallback that never hard-fails"

### Acceptance Criteria Status

- [x] Truncated detection added to extraction code
- [x] Truncated error raised with member name/path in context
- [x] Unit test with mock truncated tarball raises correct error
- [n/a] Error propagates to caller (cw-clone-fallback) - Pending integration (separate bead: cg-hnsp)

## Conclusion

Truncated file detection is fully implemented with comprehensive test coverage.
The code correctly detects:
- Unexpected EOF during member extraction
- Size mismatches between tar header and actual data
- Undersized pack files (< 12 byte header)

All errors include member name and context for debugging.
Ready for integration into clone-worker warm-start fallback flow.
