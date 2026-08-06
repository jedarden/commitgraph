# Ref Validation Test Structure Analysis

## Overview
This document analyzes the existing test structure for `.ref` file validation in the warmstart package, specifically focusing on the `CollectMissingRefFiles` helper function and related test patterns.

## Test Structure in extract_test.go

### Test Setup Patterns

#### 1. TarballMember Structure
Tests use the `TarballMember` struct to define tarball contents:
```go
type TarballMember struct {
    Name string
    Data []byte
}
```

#### 2. Helper Functions for Test Creation

**`createTestTarball(t, members)`** (line 14)
- Creates a tarball from a slice of TarballMember
- Uses `archive/tar` to write headers and data
- Returns `[]byte` containing the tarball data

**`makeMockTarballWithPack(t, packContent, packName)`** (line 61)
- Creates complete valid tarballs with custom pack content
- Automatically includes config.json, ref, .idx, and .ref files
- Derives .idx and .ref filenames from pack name
- Useful for testing undersized or corrupted pack files

### Test Patterns

#### Pattern 1: Single Missing File Detection
```go
members := []TarballMember{
    {Name: "objects/pack/pack-123.pack", Data: []byte("PACK123456789")},
    {Name: "objects/pack/pack-123.idx", Data: idxData},
    // Missing pack-123.ref
    {Name: "config.json", Data: configData},
    {Name: "ref", Data: refData},
}
```

#### Pattern 2: Multiple Missing Files
Tests verify that ALL missing .ref files are reported:
- `TestMissingRefErrorMessage_MultipleMissing` (line 66)
- `TestParseTarball_MultipleMissingRefFiles` (line 1914)

#### Pattern 3: Edge Cases
- Empty list (no missing files)
- Duplicate pack file names
- Pack files without `objects/pack/` prefix
- Mixed complete and incomplete sets

### Test Assertion Patterns

#### Error Type Verification
```go
var missingErr *Error
if !errors.As(err, &missingErr) {
    t.Fatalf("expected *Error type, got %T: %v", err, err)
}
```

#### Error Message Validation
```go
if !strings.Contains(errMsg, "objects/pack/pack-123.ref") {
    t.Errorf("error message should contain missing file path")
}
if !strings.Contains(errMsg, ".ref") {
    t.Errorf("error message should mention '.ref'")
}
if !strings.Contains(errMsg, "missing") {
    t.Errorf("error message should mention 'missing'")
}
```

#### Context Field Verification
```go
if missingErr.Context == "" {
    t.Error("Context field should not be empty when .ref files are missing")
}
for _, expectedFile := range expectedFiles {
    if !strings.Contains(missingErr.Context, expectedFile) {
        t.Errorf("Context should contain '%s'", expectedFile)
    }
}
```

## CollectMissingRefFiles Implementation

### Function Signature (extract.go line 484)
```go
func CollectMissingRefFiles(members []TarballMember) []string
```

### Algorithm
1. Initialize empty slice for missing files
2. Iterate through all `TarballMember` entries
3. Filter for only `.pack` files (check `strings.HasSuffix(member.Name, ".pack")`)
4. For each `.pack` file:
   - Call `RefFileExistsInTarball(member.Name, members)`
   - If returns `false`, append expected .ref filename to missing list
5. Return list of missing .ref filenames

### Helper Functions Used

**`RefFilenameFromPackFilename(packFilename string) string`** (line 438)
```go
return strings.TrimSuffix(packFilename, ".pack") + ".ref"
```
- Strips `.pack` extension
- Appends `.ref`
- Handles edge cases: no extension, multiple dots, double extensions

**`RefFileExistsInTarball(packFilename string, members []TarballMember) bool`** (line 457)
```go
expectedRefName := RefFilenameFromPackFilename(packFilename)
for _, member := range members {
    if member.Name == expectedRefName {
        return true
    }
}
return false
```
- Constructs expected .ref filename
- Linear search through members for exact name match
- Returns true if found, false otherwise

## Test Coverage for CollectMissingRefFiles

### Test Cases (line 2160-2301)

1. **All ref files present** - Returns empty slice
2. **One ref file missing** - Returns single missing filename
3. **Multiple ref files missing** - Returns all missing filenames in order
4. **No pack files** - Returns empty slice
5. **Only pack files, no refs** - Returns all corresponding .ref files
6. **Pack files without objects/pack prefix** - Handles relative paths
7. **Mixed pack and other files** - Filters non-pack files correctly
8. **Empty member list** - Returns empty slice
9. **Single pack with ref** - Returns empty slice
10. **Single pack without ref** - Returns single missing file
11. **Preserves order** - Missing files reported in pack file order

### Key Test Observations

- **Order preservation**: Missing files maintain the order they appear in the member list
- **Case sensitivity**: Function expects exact case match
- **Path handling**: Works with both full paths (`objects/pack/...`) and relative paths
- **Filtering**: Only `.pack` files are checked; other extensions ignored

## Expected Validation Behavior

### ParseTarball Integration (extract.go line 232-236)

```go
missingRefFiles := CollectMissingRefFiles(snapshot.PackFiles)
if len(missingRefFiles) > 0 {
    return nil, NewMissingMemberErrorWithContext(".ref", 
        fmt.Sprintf("missing .ref files: %s", strings.Join(missingRefFiles, ", ")))
}
```

### Error Structure

When .ref files are missing:
- **Error Type**: `*Error` 
- **Error Kind**: `MissingMember`
- **Member Name**: ".ref"
- **Context**: Comma-separated list of all missing .ref files

### Validation Rules

1. **Requirement**: Every `.pack` file MUST have a corresponding `.ref` file
2. **Naming**: `.ref` filename derived from `.pack` filename (strip `.pack`, append `.ref`)
3. **Validation Timing**: After validating .idx files, before parsing config
4. **Failure Mode**: Returns error immediately if ANY .ref files are missing
5. **Success Condition**: Empty missing list (all .ref files present)

## Additional Helper: ValidateRefFiles

### Purpose
Similar to `CollectMissingRefFiles` but operates on filesystem paths instead of tarball members.

### Signature (extract.go line 523)
```go
func ValidateRefFiles(packFiles []string) []string
```

### Algorithm
1. Iterate through pack file paths
2. Construct expected .ref filename using `RefFilenameFromPackFilename`
3. Check filesystem with `os.Stat(refFile)`
4. If `os.IsNotExist(err)`, append to missing list
5. Other errors (permissions, etc.) are silently ignored
6. Return list of missing .ref file paths

### Difference from CollectMissingRefFiles
- `CollectMissingRefFiles`: Works with in-memory `[]TarballMember` (tarball parsing)
- `ValidateRefFiles`: Works with filesystem paths (post-materialization validation)

## Test Coverage Gaps

### Current Coverage
- ✅ Single/multiple missing files
- ✅ Empty lists and edge cases
- ✅ Order preservation
- ✅ Error message formatting
- ✅ Context field population

### Potential Additions
- Tests for `.promisor` and `.rev` file validation (similar pattern to .ref)
- Integration tests covering full ParseTarball → Materialize workflow
- Performance tests with large numbers of pack files
- Concurrent access patterns

## Usage Patterns

### Creating a Missing Ref Test
```go
func TestMyMissingRefScenario(t *testing.T) {
    configData := []byte(`{
        "core.repositoryformatversion": "1",
        "remote.origin.promisor": "true",
        "remote.origin.partialclonefilter": "blob:none"
    }`)
    refData := []byte("refs/heads/main abc123")

    members := []TarballMember{
        {Name: "objects/pack/pack-123.pack", Data: []byte("PACK123456789")},
        {Name: "objects/pack/pack-123.idx", Data: []byte("idx data")},
        // Missing: pack-123.ref
        {Name: "config.json", Data: configData},
        {Name: "ref", Data: refData},
    }

    tarball := createTestTarball(t, members)
    _, err := ParseTarball(tarball)

    // Assertions...
}
```

### Testing CollectMissingRefFiles Directly
```go
func TestCollectMissingRefFiles_Direct(t *testing.T) {
    members := []TarballMember{
        {Name: "objects/pack/pack-abc.pack", Data: []byte("data")},
        {Name: "objects/pack/pack-def.pack", Data: []byte("data")},
        {Name: "objects/pack/pack-abc.ref", Data: []byte("data")},
        // pack-def.ref missing
    }

    result := CollectMissingRefFiles(members)
    
    expected := []string{"objects/pack/pack-def.ref"}
    // Compare result to expected...
}
```

## Summary

The test structure is well-established with clear patterns for:
1. **Test Setup**: Use `TarballMember` slices and helper functions
2. **Validation**: Check error types, messages, and context fields
3. **Coverage**: Comprehensive edge cases and scenarios
4. **Helper Functions**: Modular design with clear responsibilities

The `CollectMissingRefFiles` function is a focused utility that:
- Operates on tarball members (not filesystem)
- Returns list of missing .ref filenames
- Preserves order from input
- Handles edge cases (empty lists, no pack files, etc.)

This structure can be extended to test validation of `.promisor` and `.rev` files using similar patterns.
