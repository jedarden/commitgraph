# Pack File Handling Exploration - cg-4h4pf

## Task
Survey the codebase to understand how pack files are currently processed and where ref file validation should be added.

## Findings

### 1. Pack File Processing Location

**Primary file:** `pkg/warmstart/extract.go`

**Key function:** `ParseTarball(data []byte) (*WarmStartSnapshot, error)` (lines 92-265)

This function is the entry point for all tarball processing. It:
- Reads tarball data byte-by-byte using `archive/tar`
- Validates tarball structure and member sizes
- Extracts pack files, refs, and config
- Validates that required components are present

### 2. Pack File Enumeration

**Location:** `ParseTarball` function, lines 154-169

Pack files are identified by:
```go
if strings.HasPrefix(hdr.Name, "objects/pack/") {
    ext := filepath.Ext(hdr.Name)
    base := filepath.Base(hdr.Name)
    if packExtensions[ext] || strings.HasSuffix(base, ".promisor") || strings.HasSuffix(base, ".rev") {
        // Pack file found
    }
}
```

**Supported extensions** (lines 98-104):
- `.pack` - Pack data file (required)
- `.idx` - Pack index file (required)
- `.ref` - Pack reference file (required)
- `.promisor` - Promisor file (optional)
- `.rev` - Reverse index file (optional)

### 3. Data Structures

**WarmStartSnapshot struct** (lines 76-89):
```go
type WarmStartSnapshot struct {
    PackFiles []TarballMember  // All pack-related files
    RefPath   string           // Ref path (e.g., "refs/heads/main")
    RefSHA    string           // SHA the ref points to
    Config    Config           // Git configuration
}
```

**TarballMember struct** (lines 70-74):
```go
type TarballMember struct {
    Name string
    Data []byte
}
```

### 4. File Naming Convention (.pack → .ref)

**Helper function:** `RefFilenameFromPackFilename` (lines 448-457)

**Convention:** Strip `.pack` extension, append `.ref`

Examples:
- `pack-abc123.pack` → `pack-abc123.ref`
- `objects/pack/pack-xyz.pack` → `objects/pack/pack-xyz.ref`
- `pack-test.123.pack` → `pack-test.123.ref`

**Edge cases handled:**
- No `.pack` extension: appends `.ref` to the input as-is
- Multiple dots: only the final `.pack` extension is stripped
- Double extensions: `pack-abc123.pack.promisor` → `pack-abc123.pack.promisor.ref`

### 5. Current Validation Implementation

**Location:** `ParseTarball` function, lines 207-254

#### 5.1 Pack File Presence Validation (lines 196-206)
- Checks that at least one `.pack` file exists
- Returns `NewMissingMemberError(".pack")` if missing

#### 5.2 Corresponding .idx File Validation (lines 218-231)
- Collects base names of all `.pack` files (line 209-216)
- For each base name, checks if corresponding `.idx` file exists
- Returns `NewMissingMemberError(".idx")` if any missing

#### 5.3 Corresponding .ref File Validation (lines 233-254)
- Collects all missing `.ref` files in a list
- Returns `&Error{Kind: MissingMember, MemberName: ".ref", Context: "missing .ref files: ..."}`
- **Lists ALL missing .ref files in the error context** for batch reporting

**Example error context:**
```
missing .ref files: objects/pack/pack-abc.ref, objects/pack/pack-def.ref
```

### 6. Helper Functions for .ref Validation

**RefFileExistsInTarball** (lines 459-482):
- Checks if a specific `.ref` file exists in the tarball members
- Returns `true` if found, `false` otherwise
- Used by validation logic

**CollectMissingRefFiles** (lines 484-518):
- Iterates over all `.pack` files in members
- Checks if corresponding `.ref` exists for each
- Returns list of missing `.ref` file names (empty if all present)

### 7. Validation Patterns to Follow

**Error structure:** Use the structured `Error` type from `pkg/warmstart/error.go`

**Error kinds:**
- `MissingMember` - Required file is missing
- `Truncated` - File is incomplete or undersized
- `CorruptPack` - Pack data is corrupted
- `IO` - Input/output errors

**Error creation pattern:**
```go
return NewMissingMemberError(".extension")
return NewTruncatedMemberError(filename, context, offset)
```

**Validation pattern:**
1. Collect base names of all files of one type (e.g., `.pack`)
2. Iterate through each base name
3. Check if corresponding file exists (e.g., `.idx`, `.ref`)
4. Collect all missing files in a list
5. Return comprehensive error listing all missing files

### 8. Test Coverage

**File:** `pkg/warmstart/extract_test.go`

**Key tests:**
- `TestParseTarball_MissingIdxFileMember` (line 1641) - Tests `.idx` validation
- `TestParseTarball_MissingRefFileMember` (line 1683) - Tests `.ref` validation
- `TestParseTarball_CompletePackFileSet` (line 1727) - Tests success case
- `TestParseTarball_MultiplePackFilesWithCompleteSets` (line 1789) - Tests multiple packs
- `TestParseTarball_MultiplePackFilesMissingIdxForOne` (line 1824) - Tests partial validation
- `TestParseTarball_MultipleMissingRefFiles` (line 1867) - Tests batch missing file reporting

**Test pattern:**
```go
members := []TarballMember{
    {Name: "objects/pack/pack-123.pack", Data: ...},
    // Missing: pack-123.idx
    {Name: "config.json", Data: ...},
}
_, err := ParseTarball(tarball)
var missingErr *Error
if !errors.As(err, &missingErr) {
    t.Fatalf("expected *Error type, got %T: %v", err, err)
}
if missingErr.MemberName != ".idx" {
    t.Errorf("expected member name '.idx', got %s", missingErr.MemberName)
}
```

### 9. Existing Validation Injection Points

**Where to add new validation:**

1. **After pack file collection** (after line 169)
   - After all pack files are collected in `snapshot.PackFiles`
   - Before config validation

2. **Within existing validation block** (lines 207-254)
   - Follow the pattern for `.idx` and `.ref` validation
   - Add new validation for additional file types if needed

3. **Custom validation section**
   - Could add new validation logic after line 254 (ref validation)
   - Before config parsing (line 256)

### 10. Pack File Size Validation

**Location:** Lines 161-163

**Current implementation:**
```go
if ext == ".pack" && len(data) < 12 {
    return nil, NewTruncatedMemberError(hdr.Name, 
        fmt.Sprintf("pack file too small: %d bytes (minimum 12 bytes for header)", len(data)), 0)
}
```

**Minimum size:** 12 bytes
- "PACK" magic number (4 bytes) + version (4 bytes) + object count (4 bytes)

**Test coverage:**
- `TestParseTarball_PackFileHeaderTooSmall` (line 1280)
- `TestParseTarball_TruncatedPackFileWith11BytePackUsingHelper` (line 1589)
- `TestMakeMockTarballWithPack_UndersizedPack` (line 1464)

## Summary

The codebase has a well-established pattern for pack file validation in `pkg/warmstart/extract.go`. The validation occurs in the `ParseTarball` function and follows this pattern:

1. **Enumerate** all pack files with extensions matching known patterns
2. **Validate** required files exist (`.pack`, `.idx`, `.ref`)
3. **Check** correspondence between file types (e.g., each `.pack` has a `.idx` and `.ref`)
4. **Report** all missing files in a single error for batch reporting

The naming convention is simple: strip `.pack` and add `.ref`. This is implemented in `RefFilenameFromPackFilename`.

Any new validation should follow the existing patterns using the `Error` type from `error.go` and should be added in the validation section (lines 207-254) of `ParseTarball`.

## Recommendations

- **Follow existing error structure:** Use `Error` type with appropriate `Kind`
- **Batch report missing files:** Collect all missing files before returning error
- **Add test coverage:** Create tests for new validation following existing patterns
- **Use helper functions:** Leverage `RefFileExistsInTarball` and `CollectMissingRefFiles` patterns
- **Document in error context:** Include specific file names and counts in error messages
