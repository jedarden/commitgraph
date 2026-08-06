# Task cg-2fjg7: Comprehensive Ref Validation Tests

## Task Completed

Fixed failing tests in `pkg/warmstart/extract_test.go` to ensure comprehensive test coverage for .ref file validation.

## Changes Made

### Fixed Tests
1. **TestMakeMockTarballWithPack_UndersizedPack/valid-minimum-pack** (line 1571-1577)
   - Changed expectation from 1 to 3 pack files (.pack, .idx, .ref)
   - Updated logic to find .pack file in snapshot before content verification

2. **TestMakeMockTarboxWithPack_CustomPackName** (line 1600-1608)
   - Changed expectation from 1 to 3 pack files (.pack, .idx, .ref)
   - Updated logic to find .pack file by name verification

### Root Cause
The `makeMockTarballWithPack` helper creates complete warm-start tarballs with:
- .pack file
- .idx file (corresponding index)
- .ref file (corresponding ref file)
- config.json
- ref

Tests were expecting only 1 pack file but the helper correctly creates 3 pack-related files.

## Acceptance Criteria Status

✅ **TestParseTarball_MissingRefFile** - Added and passing (line 2507)
   - 3 test cases: single-pack-missing-ref, multiple-packs-multiple-refs-missing, mixed-scenario-some-refs-missing

✅ **TestParseTarball_AllRefFilesPresent** - Added and passing (line 2606)
   - 3 test cases: single-pack-complete-set, multiple-packs-all-complete-sets, pack-with-optional-files

✅ **TestCollectMissingRefFiles** - Comprehensive coverage (line 2160)
   - 11 test cases covering all edge cases (empty input, single/multiple missing, order preservation, etc.)

✅ **All tests in extract_test.go pass** - Verified with `go test ./pkg/warmstart/... -v`

✅ **Complete test coverage** for validation logic - All .ref validation scenarios covered

## Test Results

```
PASS
ok  	github.com/jedarden/commitgraph/pkg/warmstart	0.025s
```

All ref validation tests pass successfully, ensuring robust validation of .ref file existence in warm-start tarballs.
