# cg-2jpd4: Enhanced .ref File Validation

## Task
Add validation to check for .ref file existence corresponding to each pack file in tarballs.

## Status
✅ **COMPLETED** - .ref validation enhanced to report all missing files

## What Was Done

### 1. Enhanced Validation Logic (pkg/warmstart/extract.go)
Modified the .ref file validation to collect and report **all missing .ref files at once**, rather than failing on the first one:

**Before:**
```go
// Validate that corresponding .ref files exist for each .pack file
for _, baseName := range packBaseNames {
    refName := baseName + ".ref"
    foundRef := false
    for _, pf := range snapshot.PackFiles {
        if pf.Name == refName {
            foundRef = true
            break
        }
    }
    if !foundRef {
        return nil, NewMissingMemberError(".ref")
    }
}
```

**After:**
```go
// Validate that corresponding .ref files exist for each .pack file
var missingRefFiles []string
for _, baseName := range packBaseNames {
    refName := baseName + ".ref"
    foundRef := false
    for _, pf := range snapshot.PackFiles {
        if pf.Name == refName {
            foundRef = true
            break
        }
    }
    if !foundRef {
        missingRefFiles = append(missingRefFiles, refName)
    }
}
if len(missingRefFiles) > 0 {
    return nil, &Error{
        Kind:       MissingMember,
        MemberName: ".ref",
        Context:    fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")),
    }
}
```

### 2. Updated Legacy Tests
Fixed two tests that were written before .ref validation was implemented:
- `TestParseTarball_Valid` - Added .ref file to test tarball
- `TestParseTarball_WithPromisorAndRev` - Added .ref file to test tarball

## Acceptance Criteria Met
- ✅ Function iterates over pack files
- ✅ For each pack, constructs expected .ref filename  
- ✅ Returns list of missing .ref files (if any)
- ✅ Test case with missing .ref demonstrates detection (both single and multiple)

## Example Error Output
When multiple .ref files are missing:
```
warmstart: missing required member (member=.ref) - missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref
```

## Tests Status
All .ref-specific validation tests pass:
- ✅ `TestParseTarball_MissingRefFileMember` - Single missing .ref file detection
- ✅ `TestParseTarball_MultipleMissingRefFiles` - Multiple missing .ref files detection
- ✅ `TestParseTarball_CompletePackFileSet` - Validation succeeds when all files present
