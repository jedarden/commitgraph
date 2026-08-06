# cg-1onrj: Loose Ref File Writing from Tarball

## Summary
Implemented loose ref file writing from tarball at the ref's original path (NOT packed-refs format).

## Changes Made

### pkg/warmstart/extract.go
- Modified `ParseTarball` to extract refs at their original paths from the tarball:
  - New format: refs stored at original paths (e.g., `refs/heads/main` containing just the SHA)
  - Legacy format: still supported (file named `ref` containing "refs/heads/main SHA")
  - Handles direct refs (under `refs/heads/`, `refs/tags/`) and symbolic refs (e.g., `HEAD`)
  - Symbolic refs detected by `ref:` prefix and stored without trailing newline
- Modified `Materialize` to handle both direct and symbolic refs correctly:
  - Direct refs: written with newline (e.g., `abc123\n`)
  - Symbolic refs: written without newline (e.g., `ref: refs/heads/main`)
  - Still NEVER writes to `packed-refs` file

### pkg/warmstart/extract_test.go
Added comprehensive tests:
- `TestParseTarball_RefAtOriginalPath` - Verifies parsing refs at original paths
- `TestParseTarball_SymbolicRef` - Verifies symbolic ref parsing (e.g., `HEAD`)
- `TestParseTarball_LegacyRefFormat` - Verifies backward compatibility with legacy format
- `TestMaterialize_RefAtOriginalPath` - Verifies materialization creates correct loose ref file
- `TestMaterialize_SymbolicRef` - Verifies symbolic ref materialization

## Acceptance Criteria Met
- ✅ Ref file is written at the correct loose ref path (e.g., .git/refs/heads/main)
- ✅ Ref content matches exactly what was in the tarball
- ✅ No packed-refs file is created or modified
- ✅ Symbolic refs are handled correctly if present in tarball
- ✅ Unit tests verify the ref file exists at the expected loose path and contains the correct SHA

## Test Results
All 18 warmstart tests pass, including:
- 13 existing tests (unchanged behavior)
- 5 new tests (new ref-at-original-path functionality)

## Backward Compatibility
The legacy tarball format (with `ref` file containing "refpath SHA") is fully supported alongside the new format (refs at their original paths).
