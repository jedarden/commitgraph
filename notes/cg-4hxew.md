# Bead cg-4hxew: Migration Start Guard Implementation

## Overview
Integrated the preflight tool into the corpus migration startup sequence as a non-bypassable guard.

## Changes Made

### 1. Modified `migration/migrate_corpus.py`

#### Added Startup Preflight Check (lines 733-810)
- **Location**: `__main__` block, before `CorpusMigrator` instantiation
- **Logic**: 
  1. Imports `EpochPreflightChecker` from `preflight_check_epochs.py`
  2. Runs `run_preflight()` which enumerates all key_ids and tests decryption
  3. If any key fails: aborts with `sys.exit(1)` and detailed error message
  4. Only proceeds to create `CorpusMigrator` if all epochs pass

#### Abort Message (lines 752-780)
When preflight fails, the migration aborts with clear output:
```
======================================================================
MIGRATION ABORTED: Preflight encryption validation failed
======================================================================

The following encryption epochs cannot be decrypted with
the provided migration credentials:

  [1] key_id='ep-2024-08'
      epoch='august-2024'
      error: Decryption failed: Invalid key
      test partition: provider=github/year=2024/month=08

TOTAL: 1 epoch(s) failed decryption test

This migration CANNOT proceed. Fix the credential access or restore
missing epoch keys before re-running the migration.

DO NOT bypass this check - doing so would silently skip all data
in the failed epochs, causing permanent data loss.
======================================================================
```

#### Error Handling
- `ImportError`: Catches missing `preflight_check_epochs.py` module
- General `Exception`: Catches any other preflight errors
- All error paths exit with `sys.exit(1)`

#### Removed Redundant Validation
- Removed duplicate validation from `run_migration()` method (was at lines 699-701)
- Preflight now happens once at startup, before any migration setup

## Acceptance Criteria Verification

| Criteria | Status | Evidence |
|----------|--------|----------|
| Migration startup calls preflight tool before beginning work | ✅ | Check is in `__main__` before `CorpusMigrator()` instantiation |
| Migration aborts immediately if preflight returns failure | ✅ | `if not all_passed: sys.exit(1)` at line 781 |
| Abort message is clear: lists which key_id(s) failed and why | ✅ | Detailed error output with key_id, epoch, error message, test partition |
| Migration only proceeds if all enumerated epochs pass decrypt probe | ✅ | Migrator only created after preflight passes |
| Preflight check is not bypassable (no --skip flag) | ✅ | No skip mechanism exists; check runs unconditionally |

## Testing Notes

The integration is defensive:
- Uses three separate exception handlers (ImportError, general Exception, validation failure)
- Each handler exits with `sys.exit(1)` and clear error messaging
- No bypass flags or conditional logic
- Runs before ANY migration work (before DB connection, before streaming)

## Why This Architecture

The preflight check is at the **outermost layer** (startup script) rather than inside the `CorpusMigrator` class because:

1. **Fail fast**: Detects credential issues before opening DB connections or setting up migration state
2. **Clear error boundaries**: Startup errors are distinct from runtime migration errors
3. **Non-bypassable**: No way to reach `CorpusMigrator` code without passing preflight
4. **Reusability**: The preflight tool remains standalone for manual validation

## Files Modified

- `migration/migrate_corpus.py`: Added startup guard (lines 733-810), removed duplicate validation from `run_migration()`
