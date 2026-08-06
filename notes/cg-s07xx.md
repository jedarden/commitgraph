# cg-s07xx: .idx File Member Validation Implementation

## Status: COMPLETE ✅

## Summary

The .idx file member validation logic is already fully implemented in `pkg/warmstart/extract.go` (lines 208-231). The implementation was added in commit 1e6d8fc (cg-t3lmx).

## Acceptance Criteria Verification

All acceptance criteria are met:

1. ✅ **Collect base names from all .pack files in tarball**
   - Implementation: Lines 208-216 in extract.go
   - Code: `var packBaseNames []string` + loop to extract base names

2. ✅ **For each .pack file base name, check if corresponding .idx file exists**
   - Implementation: Lines 218-231 in extract.go
   - Code: Loop over `packBaseNames`, check for `baseName + ".idx"`

3. ✅ **Return MissingMember error with ".idx" member name when missing**
   - Implementation: Line 229 in extract.go
   - Code: `return nil, NewMissingMemberError(".idx")`

4. ✅ **Validation runs after .pack file presence check**
   - Implementation: .pack check (lines 196-206) runs before .idx check (lines 218-231)
   - Order is correct

## Test Coverage

The implementation is verified by the following tests:
- `TestParseTarball_MissingIdxFileMember` - PASS ✅
- `TestParseTarball_MultiplePackFilesMissingIdxForOne` - PASS ✅
- `TestParseTarball_CompletePackFileSet` - PASS ✅
- `TestParseTarball_MultiplePackFilesWithCompleteSets` - PASS ✅

## Example Error Message

When a .pack file is missing its corresponding .idx file, the validation returns:
```
warmstart: missing required member (member=.idx)
```

## Implementation Details

The validation logic:
1. Collects all `.pack` file base names (e.g., "objects/pack/pack-123" from "objects/pack/pack-123.pack")
2. For each base name, checks if a corresponding `.idx` file exists
3. Returns `MissingMember` error if any `.idx` file is missing
4. Runs after the `.pack` file presence check to ensure proper error ordering

This ensures that all `.pack` files in the tarball have their required index files, maintaining the integrity of the Git pack file structure.
