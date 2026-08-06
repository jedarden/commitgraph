# .idx File Member Validation Implementation - cg-s07xx

## Task Verification

The task requested implementation of .idx file member validation logic with the following requirements:

### Requirements Status
- ✅ **Extract base pack names from .pack files found in tarball** - Implemented in `extract.go` lines 208-216
- ✅ **Check for presence of corresponding .idx files for each .pack file** - Implemented in `extract.go` lines 218-231  
- ✅ **Return MissingMember error with ".idx" member name if absent** - Implemented at line 229
- ✅ **Validation runs after .pack file presence check** - Correctly ordered after .pack validation (lines 196-206)

## Implementation Details

**File**: `pkg/warmstart/extract.go`

**Lines 208-216**: Collect base names from all .pack files
```go
// Collect base names of all .pack files for corresponding file validation
var packBaseNames []string
for _, pf := range snapshot.PackFiles {
    if strings.HasSuffix(pf.Name, ".pack") {
        // Extract base name without extension for corresponding file checks
        baseName := strings.TrimSuffix(pf.Name, ".pack")
        packBaseNames = append(packBaseNames, baseName)
    }
}
```

**Lines 218-231**: Validate .idx files exist for each .pack file
```go
// Validate that corresponding .idx files exist for each .pack file
for _, baseName := range packBaseNames {
    idxName := baseName + ".idx"
    foundIdx := false
    for _, pf := range snapshot.PackFiles {
        if pf.Name == idxName {
            foundIdx = true
            break
        }
    }
    if !foundIdx {
        return nil, NewMissingMemberError(".idx")
    }
}
```

## Test Coverage

**Test: `TestParseTarball_MissingIdxFileMember`** (extract_test.go:1614-1653)
- Tests tarball with .pack and .promisor files but NO .idx file
- Verifies MissingMember error is returned with MemberName = ".idx"
- ✅ PASS

**Test: `TestParseTarball_MultiplePackFilesMissingIdxForOne`** (extract_test.go:1797-1837)
- Tests multiple .pack files where one is missing its corresponding .idx file
- Verifies MissingMember error is returned with MemberName = ".idx"  
- ✅ PASS

## Execution Results

```bash
$ go test -v ./pkg/warmstart/... -run "TestParseTarball_MissingIdxFileMember|TestParseTarball_MultiplePackFilesMissingIdx"
=== RUN   TestParseTarball_MissingIdxFileMember
    extract_test.go:1652: Successfully detected missing .idx file: warmstart: missing required member (member=.idx)
--- PASS: TestParseTarball_MissingIdxFileMember (0.00s)
=== RUN   TestParseTarball_MultiplePackFilesMissingIdxForOne
    extract_test.go:1836: Successfully detected missing .idx file for one of multiple pack files: warmstart: missing required member (member=.idx)
--- PASS: TestParseTarball_MultiplePackFilesMissingIdxForOne (0.00s)
PASS
ok  	github.com/jedarden/commitgraph/pkg/warmstart	0.004s
```

## Conclusion

The .idx file member validation logic was already fully implemented and tested in the codebase. All acceptance criteria from the task are met, and all relevant tests pass successfully. No code changes were required.
