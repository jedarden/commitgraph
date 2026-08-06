# .ref Validation Error Handling - Task cg-62hsv

## Task Status: ✅ COMPLETE

The .ref validation error handling already implements all required functionality and meets all acceptance criteria.

## Verification Results

### ✅ Acceptance Criteria 1: Error messages include full list of missing .ref files
**Location:** `pkg/warmstart/extract.go:234-237`

**Implementation:**
```go
missingRefFiles := CollectMissingRefFiles(snapshot.PackFiles)
if len(missingRefFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ref", fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
}
```

**Example Error Output:**
```
warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref, objects/pack/pack-ghi.ref
```

### ✅ Acceptance Criteria 2: Error format matches other MissingMember errors
The implementation uses `NewMissingMemberErrorWithContext()`, which is the standard pattern for MissingMember errors that require additional context. This ensures consistency with other error handling in the codebase.

**Standard Format:** `warmstart: missing required member (member=<name>) - <context>`

### ✅ Acceptance Criteria 3: Error context is clear for debugging
**Features:**
- Full file paths included (e.g., "objects/pack/pack-123.ref")
- Complete list of all missing files when multiple are missing
- Context field properly populated for programmatic access
- Structured format allows parsing by automated tools

### ✅ Acceptance Criteria 4: All error handling tests pass
**Test Coverage:** All 7 .ref-specific tests pass:
- `TestMissingRefErrorMessage_SingleMissing` ✓
- `TestMissingRefErrorMessage_MultipleMissing` ✓
- `TestMissingRefErrorMessage_CompleteList` ✓
- `TestMissingRefErrorMessage_EdgeCaseEmptyList` ✓
- `TestMissingRefErrorMessage_EdgeCaseDuplicates` ✓
- `TestMissingRefErrorMessage_ContextField` ✓
- `TestMissingRefErrorMessage_FormattedProperly` ✓

## Implementation Details

The error handling consists of two main components:

1. **`CollectMissingRefFiles()` helper** (`extract.go:484-501`)
   - Iterates through pack files
   - Checks for corresponding .ref files
   - Returns list of missing .ref file paths

2. **Error creation** (`extract.go:234-237`)
   - Collects missing files
   - Creates MissingMember error with context
   - Includes complete list of missing files in error message

## Comparison with Other Validation Errors

| Validation | Error Type | Context Provided |
|------------|------------|------------------|
| .pack files | `NewMissingMemberError(".pack")` | No specific file list |
| .idx files | `NewMissingMemberError(".idx")` | No specific file list |
| .ref files | `NewMissingMemberErrorWithContext(".ref", ...)` | **Complete list of missing files** |

The .ref validation actually provides **more** debugging context than .pack and .idx validation, which is appropriate since multiple .ref files can be missing simultaneously.

## Conclusion

No code changes were required. The existing .ref validation error handling implementation already satisfies all acceptance criteria and provides excellent debugging information through comprehensive error messages and context.
