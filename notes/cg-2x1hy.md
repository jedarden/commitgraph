# cg-2x1hy: Epoch Decrypt Probe Implementation - COMPLETED

## Status: ✅ COMPLETE (2026-08-06)

Implementation completed with comprehensive unit tests. All acceptance criteria met.

## Acceptance Criteria - ALL MET

✅ **AC1**: Function accepts a key_id and attempts decrypt probe
- Implementation: `DecryptProbe.test_key_id(key_id)` in `migration/decrypt_probe.py`
- Standalone function that can be called with just a key_id

✅ **AC2**: Uses migration credentials (same as corpus migration will use)
- Implementation: `DecryptProbe._load_credentials()` loads same credential JSON format
- Compatible with `migrate_corpus.py` and `preflight_check_epochs.py`

✅ **AC3**: Returns clear pass/fail result per key_id
- Implementation: `DecryptResult` dataclass with `success` boolean
- Structured result with detailed metadata (epoch, test_file, partitions_tested)

✅ **AC4**: Handles decrypt errors gracefully and reports specific failure reason
- Implementation: `DecryptOutcome` enum with distinct failure modes:
  - `SUCCESS`: Decryption succeeded
  - `KEY_NOT_FOUND`: key_id not in any manifest
  - `NO_DATA_FILES`: manifest exists but no parquet files
  - `CREDENTIAL_ERROR`: credential access/parsing failure
  - `DECRYPT_ERROR`: crypto/parse failure
  - `PARTITION_MISSING`: partition path doesn't exist

✅ **AC5**: Can handle both current and retired epoch keys
- Implementation: Scans all manifests without epoch filtering
- Test suite includes retired epoch validation

✅ **AC6**: Distinguishes between "key not found" vs "decrypt failed" vs "success"
- Implementation: Clear `DecryptOutcome` enum for each failure mode
- `error_message` field provides detailed failure context

## Test Results

**Unit Tests** (9/9 passed):
- ✓ test_successful_decryption
- ✓ test_key_not_found
- ✓ test_no_data_files
- ✓ test_decrypt_error
- ✓ test_multiple_keys
- ✓ test_validate_all_passed
- ✓ test_retired_epoch_key
- ✓ test_credential_error
- ✓ test_outcome_distinction

## Usage

### As a library

```python
from decrypt_probe import DecryptProbe, DecryptResult

probe = DecryptProbe(corpus_root="/data/corpus", credential_path="/creds/migration.json")
result = probe.test_key_id("my-key-id")

if result.success:
    print(f"✓ {result.key_id} can decrypt (epoch: {result.epoch})")
else:
    print(f"✗ {result.key_id} failed: {result.error_message}")
```

### As a CLI tool

```bash
# Test specific key_ids
python3 migration/decrypt_probe.py /data/corpus /creds/migration.json key1 key2

# Test all discovered key_ids
python3 migration/decrypt_probe.py /data/corpus /creds/migration.json
```

### Batch testing

```python
key_ids = ["key-2024-08", "key-2024-07", "key-2023-06"]
results = probe.test_multiple_keys(key_ids)

all_passed, failed = probe.validate_all_passed(results)
if not all_passed:
    for result in failed:
        print(f"Failed: {result.key_id} - {result.error_message}")
```

## Design Highlights

1. **Standalone module** - Can be used independently without preflight checker
2. **Clear error distinction** - 6 distinct outcomes for precise failure reporting
3. **Credential reuse** - Same loading pattern as corpus migration
4. **Partition caching** - Caches partition lookups by key_id for efficiency
5. **Graceful degradation** - Tests each partition until one succeeds, handles missing files
6. **Structured output** - `DecryptResult.to_dict()` for JSON serialization

## Integration

The decrypt probe module can be integrated into:
- **Corpus migration** (`migrate_corpus.py`): Pre-migration validation
- **Preflight checker** (`preflight_check_epochs.py`): Can replace `_test_decrypt_one_key()`
- **CI/CD pipelines**: Standalone epoch health checks
- **Troubleshooting**: Debug specific decryption failures

## Files Added

- `migration/decrypt_probe.py` - Main decrypt probe implementation
- `migration/test_decrypt_probe.py` - Comprehensive unit tests

## Dependencies

- `pyarrow` - For reading Parquet files and testing actual decryption
- Standard library: `json`, `pathlib`, `dataclasses`, `enum`, `logging`

## Ready for Production

The decrypt probe module is production-ready and meets all acceptance criteria. It provides a focused, standalone interface for testing encryption epoch access with clear pass/fail reporting and specific failure reasons.
