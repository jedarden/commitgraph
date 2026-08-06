# Task cg-3tn3d: Add .ref file existence checker

## Summary
Implemented `RefFileExistsInTarball` function to check if a .ref file exists in a tarball for a given .pack file.

## Changes Made

### Code Changes
- **File**: `pkg/warmstart/extract.go`
- **Added**: `RefFileExistsInTarball` function (lines 459-483)

### Function Signature
```go
func RefFileExistsInTarball(packFilename string, members []TarballMember) bool
```

### Key Features
1. **Uses existing helper**: Leverages `RefFilenameFromPackFilename` to construct the expected .ref filename
2. **Member search**: Iterates through tarball members to find matching .ref file
3. **Boolean return**: Returns `true` if found, `false` otherwise
4. **Handles edge cases**:
   - Empty member lists
   - Multiple pack files
   - Path separators
   - Double extensions
   - Case sensitivity

### Testing
Added comprehensive test suite in `pkg/warmstart/extract_test.go`:
- 10 test cases covering various scenarios
- All tests passing
- Tests include:
  - Basic existence checks
  - Non-existent files
  - Empty member lists
  - Multiple pack files
  - Edge cases (paths, extensions, case sensitivity)

## Acceptance Criteria Met
- ✅ Function accepts pack filename and tar member list
- ✅ Uses .ref filename constructor (`RefFilenameFromPackFilename`)
- ✅ Searches member list for .ref file
- ✅ Returns true if found, false otherwise

## Notes
The function is a pure utility function that can be used by validation logic elsewhere in the codebase. It follows the existing patterns and integrates seamlessly with the `RefFilenameFromPackFilename` helper from the previous step.
