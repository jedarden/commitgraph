# Test Coverage for Missing .idx and .ref Files

## Task Summary
Add comprehensive tests for .idx and .ref file validation in warmstart tarball parsing.

## Status: COMPLETE ✅

All required test cases already exist in `pkg/warmstart/extract_test.go` and pass successfully:

### Required Tests (All Present and Passing)

1. **TestParseTarball_MissingIdxFileMember** (line 1613)
   - Tests tarball with .pack but missing .idx file
   - ✅ PASS: Verifies MissingMember error with member name ".idx"

2. **TestParseTarball_MissingRefFileMember** (line 1655)
   - Tests tarball with .pack but missing .ref file
   - ✅ PASS: Verifies MissingMember error with member name ".ref"

3. **TestParseTarball_CompletePackFileSet** (line 1699)
   - Tests tarball with complete pack file set (.pack, .idx, .ref)
   - ✅ PASS: Validates all pack files captured correctly

4. **TestParseTarball_MultiplePackFilesMissingIdxForOne** (line 1796)
   - Tests multiple pack files with one missing .idx
   - ✅ PASS: Verifies MissingMember error for the missing file

## Implementation Details

The validation logic in `pkg/warmstart/extract.go` (lines 208-246):

1. Collects base names of all .pack files
2. Validates corresponding .idx files exist for each .pack file (line 218-231)
3. Validates corresponding .ref files exist for each .pack file (line 233-246)
4. Returns `NewMissingMemberError(".idx")` or `NewMissingMemberError(".ref")` when validation fails

## Test Results
```
=== RUN   TestParseTarball_MissingIdxFileMember
    extract_test.go:1652: Successfully detected missing .idx file: warmstart: missing required member (member=.idx)
--- PASS: TestParseTarball_MissingIdxFileMember (0.00s)

=== RUN   TestParseTarball_MissingRefFileMember
    extract_test.go:1696: Successfully detected missing .ref file: warmstart: missing required member (member=.ref)
--- PASS: TestParseTarball_MissingRefFileMember (0.00s)

=== RUN   TestParseTarball_CompletePackFileSet
    extract_test.go:1758: Successfully validated complete pack file set
--- PASS: TestParseTarball_CompletePackFileSet (0.00s)

=== RUN   TestParseTarball_MultiplePackFilesMissingIdxForOne
    extract_test.go:1836: Successfully detected missing .idx file for one of multiple pack files: warmstart: missing required member (member=.idx)
--- PASS: TestParseTarball_MultiplePackFilesMissingIdxForOne (0.00s)

PASS
ok      github.com/jedarden/commitgraph/pkg/warmstart    0.004s
```

## Acceptance Criteria Status
- [x] TestParseTarball_MissingIdxFileMember test exists and passes
- [x] TestParseTarball_MissingRefFileMember test exists and passes
- [x] TestParseTarball_CompletePackFileSet test exists and passes
- [x] TestParseTarball_MultiplePackFilesMissingIdxForOne test exists and passes
- [x] All tests verify MissingMember error is raised with correct member name

## Conclusion
All test coverage was already implemented and working correctly. No changes were needed.
