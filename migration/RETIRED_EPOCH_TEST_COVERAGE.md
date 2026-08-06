# Retired Epoch Test Coverage

## Overview

This document describes the test coverage added for retired epoch manifest validation in the corpus migration preflight system.

## Background

The critical requirement being tested: "scoping to only the current epoch would silently skip older partitions still sitting on retired epochs."

A **retired epoch** is an encryption key that is no longer the current/active key, but may still be used by existing corpus partitions. The preflight system MUST discover and validate ALL epochs, not just the current one.

## Test File

**File:** `/home/coding/commitgraph/migration/test_retired_epoch_preflight.py`

This comprehensive test suite validates end-to-end handling of retired encryption epochs during migration preflight checks.

## Acceptance Criteria Coverage

### ✅ AC1: Test fixture includes at least one manifest referencing a retired epoch key_id

**Test:** `test_retired_epoch_manifest_included_in_fixture()`

- Creates corpus with 3 epochs: 1 current, 2 retired
- Validates fixture metadata includes retired epochs
- Verifies retired epoch count > 0

### ✅ AC2: Test confirms preflight enumerates the retired key_id (does not skip it)

**Tests:** 
- `test_preflight_enumerates_retired_key_id_not_skipped()`
- `test_multiple_retired_epochs_aggregation()`

**Validations:**
- Preflight discovers ALL key_ids including retired ones
- Retired epochs are NOT silently skipped
- Partition aggregation works for retired epochs
- Multiple retired epochs are all discovered

**Critical Assertion:**
```python
# Must discover both current AND retired epochs
self.assertEqual(len(keys_by_id), 3, "Must discover all 3 distinct keys (current + 2 retired)")
self.assertIn("epoch-ancient-2020", keys_by_id, "Retired epoch 2020 must NOT be skipped")
```

### ✅ AC3: Test confirms decrypt probe succeeds for the retired epoch (probes work)

**Tests:**
- `test_preflight_runs_decrypt_probe_for_retired_epoch()`
- `test_decrypt_probe_standalone_for_retired_epoch()`

**Validations:**
- Decrypt probe is attempted for retired epochs
- Result structure includes retired epoch metadata
- Standalone `DecryptProbe.test_key_id()` works with retired key_ids
- Proper error reporting when Parquet files are missing

### ✅ AC4: Test confirms migration would start if all epochs pass

**Test:** `test_migration_would_start_if_all_epochs_pass()`

**Validations:**
- Preflight runs complete validation for all epochs
- All epochs receive validation results (current + retired)
- Preflight completes without crashing on retired epochs
- Clear reporting of pass/fail status for each epoch

### ✅ AC5: Test confirms migration aborts if a retired epoch fails decrypt

**Test:** `test_migration_aborts_if_retired_epoch_fails_decrypt()`

**Validations:**
- Migration aborts if ANY epoch fails (including retired)
- Failed epochs are properly detected and reported
- Clear error messages for failed decryption
- Migration doesn't proceed with partial success

## Additional Edge Cases Covered

### TestRetiredEpochPreflight Class

1. **`test_preflight_logs_retired_epoch_discovery()`**
   - Verifies logging includes retired epoch information
   - Validates metadata completeness for all epochs

2. **`test_preflight_with_empty_retired_partition()`**
   - Tests retired epoch with no Parquet files
   - Validates proper error handling

3. **`test_multiple_retired_epochs_aggregation()`**
   - Tests corpus with 4 retired epochs + 1 current
   - Validates aggregation across many retired keys

### TestRetiredEpochEdgeCases Class

1. **`test_all_retired_no_current_epoch()`**
   - Tests corpus with only retired epochs (no current)
   - Validates system handles this edge case

2. **`test_same_key_id_multiple_partitions_retired()`**
   - Tests same retired key across multiple partitions
   - Validates partition aggregation

3. **`test_mixed_current_and_retired_same_year()`**
   - Tests multiple epochs within same calendar year
   - Validates epoch distinction beyond year boundaries

## Running the Tests

```bash
cd /home/coding/commitgraph/migration
python3 test_retired_epoch_preflight.py
```

**Note:** Tests require `pyarrow` to be installed. The tests use the actual preflight system modules (`preflight_check_epochs.py`, `decrypt_probe.py`) which depend on pyarrow for Parquet file handling.

## Test Dependencies

- `unittest` (Python standard library)
- `tempfile` (Python standard library)  
- `pathlib` (Python standard library)
- `json` (Python standard library)
- `datetime` (Python standard library)
- `dataclasses` (Python standard library)
- `pyarrow` (external dependency - for Parquet operations)
- Local modules:
  - `preflight_check_epochs.py`
  - `decrypt_probe.py`

## What Makes These Tests Comprehensive

1. **End-to-end validation:** Tests cover the complete flow from discovery → validation → reporting
2. **Retired epoch focus:** Specifically targets the critical requirement of not skipping old epochs
3. **Multiple scenarios:** Current, retired, ancient, empty partitions, aggregations
4. **Both APIs tested:** 
   - Integrated `EpochPreflightChecker` (main preflight system)
   - Standalone `DecryptProbe` (individual key testing)
5. **Edge cases:** All retired, same key multiple partitions, same year different epochs
6. **Failure modes:** Tests both success and failure paths

## Integration with Existing Tests

These new tests complement the existing test suite:

- **Existing:** `test_preflight_epochs.py` - Basic preflight functionality
- **New:** `test_retired_epoch_preflight.py` - Comprehensive retired epoch coverage

The new tests can be run standalone or integrated into the main test suite.

## Validation of Critical Requirement

The core requirement being validated:

> "scoping to only the current epoch would silently skip older partitions still sitting on retired epochs."

**Test evidence:**
- `test_preflight_enumerates_retired_key_id_not_skipped()` explicitly validates this
- Creates corpus with current + retired epochs
- Asserts that retired epochs are discovered, not skipped
- Verifies partition aggregation works for retired epochs

This test would fail if the preflight system silently skipped retired epochs, making it a strong validation of the critical requirement.
