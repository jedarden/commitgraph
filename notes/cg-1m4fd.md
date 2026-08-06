# Task cg-1m4fd: Add missing .ref file list to ParseTarball error

## Status: Already Implemented

This task requested updating the ParseTarball error handling to include the full list of missing .ref files. Upon investigation, this feature was **already implemented** in a previous commit.

## Implementation Details

**Commit:** `61da8549faebc8c09191ab29df41c6cfbf9d75f1`  
**Date:** Thu Aug 6 07:29:30 2026 -0400  
**Original Bead:** cg-62hsv (Improve ref validation error handling)  
**Title:** `feat(cg-62hsv): improve ref validation error handling`

## Current Implementation (extract.go:233-237)

```go
// Validate that corresponding .ref files exist for each .pack file
missingRefFiles := CollectMissingRefFiles(snapshot.PackFiles)
if len(missingRefFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ref", fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
}
```

## Error Message Format

The error message includes:
- **Kind:** `MissingMember` (via `NewMissingMemberErrorWithContext`)
- **Member Name:** `.ref`
- **Context:** Complete list of missing .ref file paths

### Example Error Messages

**Single missing .ref file:**
```
warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-123.ref
```

**Multiple missing .ref files:**
```
warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref
```

## Test Coverage

**Test:** `TestParseTarball_MultipleMissingRefFiles` (extract_test.go:1867-1917)

This test verifies:
1. Error kind is `MissingMember`
2. Member name is `.ref`
3. Error context lists all missing .ref files:
   - `objects/pack/pack-abc.ref`
   - `objects/pack/pack-def.ref`

## Acceptance Criteria Status

✅ **All criteria met:**
- [x] Error message includes complete list of missing .ref file paths
- [x] Error context clearly indicates which files are missing
- [x] Error remains actionable for debugging
- [x] Change is isolated to .ref validation only

## Related Documentation

See `notes/cg-69cc0.md` for complete documentation of MissingMember error patterns for .pack/.idx/.ref validation.

## Conclusion

The requested feature is fully implemented and tested. The code uses `NewMissingMemberErrorWithContext` to provide detailed error context that lists all missing .ref files, making debugging clear and actionable.
