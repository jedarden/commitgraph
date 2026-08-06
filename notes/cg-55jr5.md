# Tests for Missing .ref File Error Messages (Bead cg-55jr5)

## Summary
Implemented comprehensive tests that verify the complete list of missing .ref files is included in error messages.

## Implementation Details

### New Test File
Created `pkg/warmstart/missing_ref_error_test.go` with 7 test cases:

1. **TestMissingRefErrorMessage_SingleMissing**
   - Verifies error message contains the single missing .ref file name
   - Checks that error mentions ".ref" and "missing"
   - Example error: `warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-123.ref`

2. **TestMissingRefErrorMessage_MultipleMissing**
   - Verifies error message contains all 3 missing .ref files
   - Tests pack-abc.ref, pack-def.ref, and pack-ghi.ref are all listed
   - Example error: `warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref, objects/pack/pack-ghi.ref`

3. **TestMissingRefErrorMessage_CompleteList**
   - Verifies error message contains all 5 missing .ref files
   - Tests with pack-001 through pack-005
   - Validates complete list coverage

4. **TestMissingRefErrorMessage_EdgeCaseEmptyList**
   - Tests behavior when no .ref files are missing
   - Verifies no error is raised when all .ref files are present
   - Edge case: empty missing list

5. **TestMissingRefErrorMessage_EdgeCaseDuplicates**
   - Tests behavior when same pack name appears multiple times in tarball
   - Verifies error message handles duplicate entries correctly
   - Example: pack-123.ref may appear twice in error message

6. **TestMissingRefErrorMessage_ContextField**
   - Verifies the Context field contains the complete list
   - Tests with 3 missing files: pack-alpha, pack-beta, pack-gamma
   - Validates Context field is properly populated

7. **TestMissingRefErrorMessage_FormattedProperly**
   - Verifies error message structure includes expected components
   - Checks for "warmstart", "missing required member", "member=.ref"
   - Validates file path is present in error message

## Test Results
All 7 tests pass successfully:
```
=== RUN   TestMissingRefErrorMessage_SingleMissing
--- PASS: TestMissingRefErrorMessage_SingleMissing (0.00s)
=== RUN   TestMissingRefErrorMessage_MultipleMissing
--- PASS: TestMissingRefErrorMessage_MultipleMissing (0.00s)
=== RUN   TestMissingRefErrorMessage_CompleteList
--- PASS: TestMissingRefErrorMessage_CompleteList (0.00s)
=== RUN   TestMissingRefErrorMessage_EdgeCaseEmptyList
--- PASS: TestMissingRefErrorMessage_EdgeCaseEmptyList (0.00s)
=== RUN   TestMissingRefErrorMessage_EdgeCaseDuplicates
--- PASS: TestMissingRefErrorMessage_EdgeCaseDuplicates (0.00s)
=== RUN   TestMissingRefErrorMessage_ContextField
--- PASS: TestMissingRefErrorMessage_ContextField (0.00s)
=== RUN   TestMissingRefErrorMessage_FormattedProperly
--- PASS: TestMissingRefErrorMessage_FormattedProperly (0.00s)
PASS
```

## Acceptance Criteria Met
- [x] Test passes for single missing .ref file error message
- [x] Test passes for multiple missing .ref files
- [x] Error message includes complete list of missing files
- [x] Edge cases are covered (empty, duplicates)

## Files Modified
- Created: `pkg/warmstart/missing_ref_error_test.go` (486 lines, 7 test functions)

## Testing Command
```bash
go test -v ./pkg/warmstart -run "TestMissingRefErrorMessage"
```
