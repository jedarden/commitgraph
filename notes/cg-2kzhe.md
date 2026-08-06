# .idx File Validation Implementation Status

## Task: cg-2kzhe - Add idx file member existence check

**Status: ALREADY IMPLEMENTED ✅**

## Implementation Location

The .idx file validation is fully implemented in `pkg/warmstart/extract.go` (lines 208-243).

## Implementation Details

### Code Section: `ParseTarball()` function

1. **Collect pack base names** (lines 208-216):
   - Iterates over pack files in the snapshot
   - Extracts base name (without `.pack` extension) for each pack file
   - Stores base names in `packBaseNames` slice

2. **Check for corresponding .idx files** (lines 218-232):
   - For each pack base name, constructs expected `.idx` filename
   - Searches through snapshot pack files to find matching .idx file
   - Tracks missing .idx files in `missingIdxFiles` slice

3. **Return error with missing files** (lines 234-243):
   - If any .idx files are missing, returns comprehensive error
   - Error message includes list of all missing .idx files (supports single and multiple)

## Acceptance Criteria Verification

✅ **Function iterates over pack files**
- Lines 208-216: Loop through `snapshot.PackFiles` to collect pack base names

✅ **For each pack, constructs expected .idx filename**
- Line 221: `idxName := baseName + ".idx"`

✅ **Returns list of missing .idx files (if any)**
- Lines 234-243: Returns `NewMissingMemberError()` with comprehensive list of missing .idx files
- Supports both single file (`"objects/pack/pack-123.idx"`) and multiple files (`"[objects/pack/pack-abc.idx objects/pack/pack-def.idx]"`)

✅ **Test case with missing .idx demonstrates detection**
- `TestParseTarball_MissingIdxFileMember` (line 1622): Tests single missing .idx file
- `TestParseTarball_MultiplePackFilesMissingIdxForOne` (line 1809): Tests one of multiple packs missing .idx
- `TestParseTarball_MultipleMissingIdxFiles` (line 1853): Tests multiple missing .idx files

## Test Results

All .idx validation tests pass:

```
=== RUN   TestParseTarball_MissingIdxFileMember
    Successfully detected missing .idx file: warmstart: missing required member (member=.idx file(s) missing: objects/pack/pack-123.idx)
--- PASS: TestParseTarball_MissingIdxFileMember (0.00s)

=== RUN   TestParseTarball_MultiplePackFilesMissingIdxForOne
    Successfully detected missing .idx file for one of multiple pack files: warmstart: missing required member (member=.idx file(s) missing: objects/pack/pack-def.idx)
--- PASS: TestParseTarball_MultiplePackFilesMissingIdxForOne (0.00s)

=== RUN   TestParseTarball_MultipleMissingIdxFiles
    Successfully detected multiple missing .idx files: warmstart: missing required member (member=.idx file(s) missing: [objects/pack/pack-abc.idx objects/pack/pack-def.idx])
--- PASS: TestParseTarball_MultipleMissingIdxFiles (0.00s)
```

## Conclusion

The .idx file member existence check is fully implemented and all acceptance criteria are met. The implementation:

1. Uses the existing pack file list as reference
2. For each .pack file, checks if corresponding .idx exists  
3. Identifies missing .idx files by name
4. Returns comprehensive error with all missing files listed

No additional work is required for this task.
