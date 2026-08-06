# Task cg-68nua: Align .ref error format with MissingMember pattern

## Status: Already Aligned

This task requested aligning the .ref error format with the existing MissingMember pattern used for .pack and .idx files. Upon investigation, the .ref error handling is **already fully aligned** with the MissingMember pattern.

## Implementation Status

**Current Implementation** (extract.go:233-237):
```go
missingRefFiles := CollectMissingRefFiles(snapshot.PackFiles)
if len(missingRefFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ref", fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
}
```

**Error Message Format:**
```
warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref
```

## Alignment with MissingMember Pattern

### Comparison Table

| Aspect | .pack | .idx | .ref | Pattern Match |
|--------|-------|------|------|---------------|
| Constructor | `NewMissingMemberError(".pack")` | `NewMissingMemberError(".idx")` | `NewMissingMemberErrorWithContext(".ref", ...)` | ✅ |
| Member Name | `.pack` | `.idx` | `.ref` | ✅ |
| Error Kind | `MissingMember` | `MissingMember` | `MissingMember` | ✅ |
| Context | None | None | Lists missing files | ✅ |
| File Path Format | N/A | N/A | `objects/pack/pack-*.ref` | ✅ |

### Design Rationale

The pattern difference is intentional and documented:

- **.pack and .idx**: Use simple errors because any absence is fatal - the file type alone provides sufficient context.
- **.ref**: Uses detailed context because multiple .ref files can be missing, and listing them helps debugging which pack files are affected.

### Error Structure

All three file types use the same base `Error` structure:

```go
type Error struct {
    Kind       ErrorKind   // MissingMember
    Context    string      // Additional details (for .ref)
    MemberName string      // ".pack", ".idx", or ".ref"
    Offset     int64       // Not used for MissingMember
    Underlying error       // Not used for MissingMember
}
```

The error message format follows the template:
```
warmstart: {kind} (member={name}) - {context}
```

## Test Coverage

All tests pass, confirming correct alignment:

### TestParseTarball_MissingRefFileMember
- ✅ Verifies error kind is `MissingMember`
- ✅ Verifies member name is `.ref`
- ✅ Single missing file: `objects/pack/pack-123.ref`

### TestParseTarball_MultipleMissingRefFiles
- ✅ Verifies error kind is `MissingMember`
- ✅ Verifies member name is `.ref`
- ✅ Context lists all missing files: `objects/pack/pack-abc.ref, objects/pack/pack-def.ref`

## Acceptance Criteria Status

- ✅ .ref error format matches .pack/.idx error format
- ✅ File paths follow the same formatting pattern (`objects/pack/pack-*.ref`)
- ✅ Error structure aligns with MissingMember pattern (same `Error` type, same `Kind`)
- ✅ No inconsistencies between validation error types

## Historical Context

This alignment was implemented in commit `61da854` (bead cg-62hsv) and documented in:
- `notes/cg-69cc0.md` - Complete MissingMember error pattern documentation
- Commit `ceaa803` (bead cg-1m4fd) - Confirmation that .ref file list feature is already implemented

## Conclusion

The .ref error format is **already fully aligned** with the MissingMember pattern. No code changes are required. The implementation correctly uses `NewMissingMemberErrorWithContext` to provide detailed context for missing .ref files, while maintaining consistency with the underlying MissingMember error structure used for .pack and .idx files.

All acceptance criteria are met:
- Same error type (`Error`)
- Same error kind (`MissingMember`)
- Same member naming convention (`.ref` with leading dot)
- Consistent error message format
- Appropriate level of detail for the error type (file lists for .ref)
