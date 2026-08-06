# Bead cg-4cb87: Verify idx/ref File Validation Implementation

## Summary
Verified that idx and ref file member validation is fully implemented and tested.

## Implementation Location
**File:** `pkg/warmstart/extract.go`  
**Function:** `ParseTarball()` lines 196-242

## Validation Flow
1. **Pack file check** (lines 196-206): Ensures at least one .pack file exists
2. **Idx file validation** (lines 229-234): Calls `CollectMissingIdxFiles()` and returns `MissingMember` error if any .idx files missing
3. **Ref file validation** (lines 236-242): Calls `CollectMissingRefFiles()` and returns `MissingMember` error if any .ref files missing

## Helper Functions
- `CollectMissingIdxFiles(members []TarballMember) []string` - line 589
- `CollectMissingRefFiles(members []TarballMember) []string` - line 517
- `IdxFileExistsInTarball(packFilename string, members []TarballMember) bool` - line 562
- `RefFileExistsInTarball(packFilename string, members []TarballMember) bool` - line 490
- `IdxFilenameFromPackFilename(packFilename string) string` - line 543
- `RefFilenameFromPackFilename(packFilename string) string` - line 471

## Error Handling
Both validation functions return `NewMissingMemberErrorWithContext()`:
- For .idx files: `NewMissingMemberErrorWithContext(".idx", "missing .idx files: ...")`
- For .ref files: `NewMissingMemberErrorWithContext(".ref", "missing .ref files: ...")`

## Test Coverage
All tests passing:
- `TestParseTarball_MissingIdxFileMember` - Single missing .idx file
- `TestParseTarball_MissingRefFileMember` - Single missing .ref file
- `TestParseTarball_MultiplePackFilesMissingIdxForOne` - Multiple .pack files, one missing .idx
- `TestParseTarball_MultipleMissingRefFiles` - Multiple missing .ref files
- `TestCollectMissingRefFiles` - Comprehensive .ref validation tests
- `TestCollectMissingRefFiles_*` - Edge cases and scenarios
- Plus similar coverage for .idx files

## Acceptance Criteria Status
- [x] idx file validation checks for .idx extension
- [x] ref file validation checks for .ref extension
- [x] MissingMember error raised with missing member name
- [x] Validation integrated after pack file check

## Conclusion
The implementation is complete, tested, and working correctly. All acceptance criteria have been met.
