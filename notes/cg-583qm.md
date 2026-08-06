# cg-583qm: Git Config Value Setting for Warm-Start

## Summary
Verified that the warm-start extraction properly sets all three required git config values before any fetch operation.

## Implementation Analysis

### Existing Code (pkg/warmstart/extract.go)

The `writeGitConfig()` function (lines 249-268) already implements setting all three required config values:

1. **core.repositoryformatversion** - Set via `runGitConfig()` at line 253-255
2. **remote.origin.promisor** - Set via `runGitConfig()` at line 258-260
3. **remote.origin.partialclonefilter** - Set via `runGitConfig()` at line 263-265

The implementation uses direct config file manipulation via `setGitConfigValue()` (lines 278-348), which:
- Parses and rewrites the git config file
- Handles both simple keys (e.g., `core.repositoryformatversion`) and nested keys (e.g., `remote.origin.promisor`)
- Creates sections if they don't exist
- Updates existing values or adds new ones

### Timing Verification

The config values are set in the `Materialize()` function at line 242-244, which occurs **before** the function returns. This ensures the config is in place before any subsequent git fetch operation.

## Changes Made

### Added Error Handling Tests

Added two new test functions to `pkg/warmstart/extract_test.go`:

1. **TestSetGitConfigValue_ReadOnlyConfigFile** - Verifies error handling when the config file is read-only
2. **TestSetGitConfigValue_MissingConfigDirectory** - Verifies error handling when the git directory is not accessible

These tests ensure that errors during config file operations are properly propagated.

## Acceptance Criteria Status

- [x] core.repositoryformatversion is set correctly via git config
- [x] remote.origin.promisor is set to true
- [x] remote.origin.partialclonefilter is set to the specified value
- [x] Config values are set before the extraction function returns (before any fetch)
- [x] Unit test verifies all three config values exist and have correct values (TestMaterialize_GitConfigValuesSet)
- [x] Error handling covers cases where git config commands fail

## Testing

All warmstart tests pass:
```
go test -v ./pkg/warmstart/...
PASS
ok  github.com/jedarden/commitgraph/pkg/warmstart 0.029s
```

## Notes

The implementation uses direct config file manipulation rather than `git config` commands. This is intentional as noted in the code comment (line 272-273):
> We'll directly manipulate the config file since we can't rely on git being available and we need this to work even in environments without git

This approach is more robust for the use case where warm-start snapshots are materialized in environments that may not have git installed.
