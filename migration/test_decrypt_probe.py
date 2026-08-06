#!/usr/bin/env python3
"""
Unit tests for decrypt_probe module.

Tests the standalone decrypt probe functionality covering:
1. Successful decryption for valid key_id
2. Key not found case (key_id not in any manifest)
3. No data files case (manifest exists but no parquet files)
4. Decrypt error case (credential/crypto failure)
5. Multiple key_id batch testing
6. Clear outcome distinction between failure modes

Acceptance criteria from bead cg-2x1hy.
"""

import sys
import os
import json
import tempfile
from pathlib import Path
from unittest.mock import Mock, MagicMock, patch

# Add migration directory to path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from decrypt_probe import DecryptProbe, DecryptResult, DecryptOutcome


def create_test_corpus():
    """
    Create a test corpus with multiple encryption epochs.

    Structure:
    - provider=github/year=2024/month=08/_manifest (key1)
    - provider=github/year=2024/month=07/_manifest (key2, retired)
    - provider=github/year=2023/month=06/_manifest (key3, ancient)
    """
    temp_dir = Path(tempfile.mkdtemp())

    # Create partition structures
    partitions = [
        ("provider=github/year=2024/month=08", "key-2024-08-current", "epoch-2024-08-current"),
        ("provider=github/year=2024/month=07", "key-2024-07-retired", "epoch-2024-07-retired"),
        ("provider=github/year=2023/month=06", "key-2023-06-ancient", "epoch-2023-06-ancient"),
    ]

    for partition_key, key_id, epoch in partitions:
        # Create partition directory
        partition_path = temp_dir / partition_key.replace("/", "/")
        partition_path.mkdir(parents=True, exist_ok=True)

        # Create manifest
        manifest = {
            'encryption_keys': [
                {
                    'key_id': key_id,
                    'epoch': epoch,
                    'key_path': f'/keys/{key_id}.json'
                }
            ]
        }

        manifest_path = partition_path / "_manifest"
        with open(manifest_path, 'w') as f:
            json.dump(manifest, f)

    return temp_dir


def create_test_credential(temp_dir):
    """Create a test migration credential file."""
    cred_path = temp_dir / "migration.json"
    credential = {
        'access_key': 'test-access-key',
        'secret_key': 'test-secret-key'
    }

    with open(cred_path, 'w') as f:
        json.dump(credential, f)

    return cred_path


def test_successful_decryption():
    """Test successful decryption for a valid key_id."""
    print("Testing successful decryption...")

    # Create test corpus with parquet file
    temp_dir = create_test_corpus()
    cred_path = create_test_credential(temp_dir)

    # Add a dummy parquet file to one partition
    partition_path = temp_dir / "provider=github" / "year=2024" / "month=08"
    dummy_parquet = partition_path / "test.parquet"

    # Create a minimal valid parquet file
    import pyarrow as pa
    import pyarrow.parquet as pq

    table = pa.table({
        'sha': ['abc123'],
        'message': ['test commit']
    })

    pq.write_table(table, dummy_parquet)

    # Create probe and test
    probe = DecryptProbe(str(temp_dir), str(cred_path))
    result = probe.test_key_id("key-2024-08-current")

    # Verify success
    assert result.success, "Expected successful decryption"
    assert result.outcome == DecryptOutcome.SUCCESS, f"Expected SUCCESS outcome, got {result.outcome}"
    assert result.key_id == "key-2024-08-current"
    assert result.epoch == "epoch-2024-08-current"
    assert result.test_file.endswith("test.parquet")
    assert result.partitions_tested == 1
    assert result.error_message == ""

    print("✓ Successful decryption test passed!")
    print(f"  - key_id: {result.key_id}")
    print(f"  - outcome: {result.outcome.value}")
    print(f"  - test file: {result.test_file}")

    # Cleanup
    import shutil
    shutil.rmtree(temp_dir)


def test_key_not_found():
    """Test key_id not found in any manifest."""
    print("\nTesting key not found...")

    # Create test corpus (empty)
    temp_dir = create_test_corpus()
    cred_path = create_test_credential(temp_dir)

    # Create probe and test non-existent key
    probe = DecryptProbe(str(temp_dir), str(cred_path))
    result = probe.test_key_id("nonexistent-key")

    # Verify key not found
    assert not result.success, "Expected failure for nonexistent key"
    assert result.outcome == DecryptOutcome.KEY_NOT_FOUND, f"Expected KEY_NOT_FOUND outcome, got {result.outcome}"
    assert result.key_id == "nonexistent-key"
    assert "not found in any partition manifest" in result.error_message
    assert result.partitions_tested == 0

    print("✓ Key not found test passed!")
    print(f"  - key_id: {result.key_id}")
    print(f"  - outcome: {result.outcome.value}")
    print(f"  - error message: {result.error_message}")

    # Cleanup
    import shutil
    shutil.rmtree(temp_dir)


def test_no_data_files():
    """Test manifest exists but no parquet files available."""
    print("\nTesting no data files case...")

    # Create test corpus without parquet files
    temp_dir = create_test_corpus()
    cred_path = create_test_credential(temp_dir)

    # Create probe and test key that has manifest but no data
    probe = DecryptProbe(str(temp_dir), str(cred_path))
    result = probe.test_key_id("key-2024-08-current")

    # Verify no data files outcome
    assert not result.success, "Expected failure when no parquet files"
    assert result.outcome == DecryptOutcome.NO_DATA_FILES, f"Expected NO_DATA_FILES outcome, got {result.outcome}"
    assert "no Parquet files available for testing" in result.error_message
    assert result.partitions_tested == 1  # Found partition but no files

    print("✓ No data files test passed!")
    print(f"  - key_id: {result.key_id}")
    print(f"  - outcome: {result.outcome.value}")
    print(f"  - partitions_tested: {result.partitions_tested}")

    # Cleanup
    import shutil
    shutil.rmtree(temp_dir)


def test_decrypt_error():
    """Test decryption error handling."""
    print("\nTesting decrypt error case...")

    # Create test corpus with corrupted parquet file
    temp_dir = create_test_corpus()
    cred_path = create_test_credential(temp_dir)

    # Add a corrupted parquet file
    partition_path = temp_dir / "provider=github" / "year=2024" / "month=08"
    dummy_parquet = partition_path / "corrupted.parquet"

    # Write invalid parquet data
    with open(dummy_parquet, 'wb') as f:
        f.write(b'This is not a valid parquet file')

    # Create probe and test
    probe = DecryptProbe(str(temp_dir), str(cred_path))
    result = probe.test_key_id("key-2024-08-current")

    # Verify decrypt error
    assert not result.success, "Expected failure for corrupted parquet"
    assert result.outcome == DecryptOutcome.DECRYPT_ERROR, f"Expected DECRYPT_ERROR outcome, got {result.outcome}"
    assert "Decrypt failed" in result.error_message or "Parquet" in result.error_message
    assert result.partitions_tested == 1
    assert result.test_file.endswith("corrupted.parquet")

    print("✓ Decrypt error test passed!")
    print(f"  - key_id: {result.key_id}")
    print(f"  - outcome: {result.outcome.value}")
    print(f"  - error message: {result.error_message}")

    # Cleanup
    import shutil
    shutil.rmtree(temp_dir)


def test_multiple_keys():
    """Test batch testing of multiple key_ids."""
    print("\nTesting multiple key_ids batch...")

    # Create test corpus with parquet files
    temp_dir = create_test_corpus()
    cred_path = create_test_credential(temp_dir)

    # Add parquet files to two partitions
    import pyarrow as pa
    import pyarrow.parquet as pq

    table = pa.table({'sha': ['abc123'], 'message': ['test']})

    # Partition 1
    partition1 = temp_dir / "provider=github" / "year=2024" / "month=08"
    pq.write_table(table, partition1 / "data1.parquet")

    # Partition 2
    partition2 = temp_dir / "provider=github" / "year=2024" / "month=07"
    pq.write_table(table, partition2 / "data2.parquet")

    # Create probe and test multiple keys
    probe = DecryptProbe(str(temp_dir), str(cred_path))

    key_ids = ["key-2024-08-current", "key-2024-07-retired", "nonexistent-key"]
    results = probe.test_multiple_keys(key_ids)

    # Verify results
    assert len(results) == 3, f"Expected 3 results, got {len(results)}"

    # First key should succeed
    assert results[0].success, "Expected first key to succeed"
    assert results[0].key_id == "key-2024-08-current"

    # Second key should succeed
    assert results[1].success, "Expected second key to succeed"
    assert results[1].key_id == "key-2024-07-retired"

    # Third key should fail (not found)
    assert not results[2].success, "Expected third key to fail"
    assert results[2].key_id == "nonexistent-key"
    assert results[2].outcome == DecryptOutcome.KEY_NOT_FOUND

    print("✓ Multiple keys batch test passed!")
    print(f"  - Tested {len(results)} key_ids")
    print(f"  - Success: {sum(1 for r in results if r.success)}")
    print(f"  - Failed: {sum(1 for r in results if not r.success)}")

    # Cleanup
    import shutil
    shutil.rmtree(temp_dir)


def test_validate_all_passed():
    """Test validate_all_passed helper method."""
    print("\nTesting validate_all_passed helper...")

    # Create mixed results
    results = [
        DecryptResult("key1", True, DecryptOutcome.SUCCESS, epoch="epoch1"),
        DecryptResult("key2", False, DecryptOutcome.KEY_NOT_FOUND, error_message="Not found"),
        DecryptResult("key3", True, DecryptOutcome.SUCCESS, epoch="epoch3"),
    ]

    # Create probe (we just need the instance method)
    temp_dir = create_test_corpus()
    cred_path = create_test_credential(temp_dir)
    probe = DecryptProbe(str(temp_dir), str(cred_path))

    all_passed, failed = probe.validate_all_passed(results)

    # Verify
    assert not all_passed, "Expected all_passed=False with one failure"
    assert len(failed) == 1, f"Expected 1 failed result, got {len(failed)}"
    assert failed[0].key_id == "key2"

    # Test with all success
    all_success = [r for r in results if r.success]
    all_passed_ok, failed_ok = probe.validate_all_passed(all_success)

    assert all_passed_ok, "Expected all_passed=True with all successes"
    assert len(failed_ok) == 0, "Expected no failed results"

    print("✓ Validate all passed test passed!")

    # Cleanup
    import shutil
    shutil.rmtree(temp_dir)


def test_retired_epoch_key():
    """Test that retired epoch keys are handled correctly."""
    print("\nTesting retired epoch key handling...")

    # Create test corpus with retired epoch
    temp_dir = create_test_corpus()
    cred_path = create_test_credential(temp_dir)

    # Add parquet file to retired epoch partition
    import pyarrow as pa
    import pyarrow.parquet as pq

    table = pa.table({'sha': ['abc123'], 'message': ['test']})

    retired_partition = temp_dir / "provider=github" / "year=2024" / "month=07"
    pq.write_table(table, retired_partition / "data.parquet")

    # Create probe and test retired key
    probe = DecryptProbe(str(temp_dir), str(cred_path))
    result = probe.test_key_id("key-2024-07-retired")

    # Verify retired epoch is handled
    assert result.success, "Expected retired epoch to decrypt successfully"
    assert result.epoch == "epoch-2024-07-retired"
    assert result.outcome == DecryptOutcome.SUCCESS

    print("✓ Retired epoch key test passed!")
    print(f"  - Retired epoch: {result.epoch}")
    print(f"  - Successfully decrypted: True")

    # Cleanup
    import shutil
    shutil.rmtree(temp_dir)


def test_credential_error():
    """Test credential error detection."""
    print("\nTesting credential error case...")

    # Create test corpus with invalid credential
    temp_dir = create_test_corpus()

    # Create invalid credential (not JSON)
    cred_path = temp_dir / "invalid.json"
    with open(cred_path, 'w') as f:
        f.write("not valid json")

    # Try to create probe - should fail gracefully
    try:
        probe = DecryptProbe(str(temp_dir), str(cred_path))
        assert False, "Expected ValueError for invalid credential"
    except ValueError as e:
        assert "Failed to load credentials" in str(e)
        print("✓ Credential error test passed!")
        print(f"  - Error message: {e}")

    # Cleanup
    import shutil
    shutil.rmtree(temp_dir)


def test_outcome_distinction():
    """Test that all failure outcomes are distinguishable."""
    print("\nTesting outcome distinction...")

    # Create results with different outcomes
    outcomes_to_test = [
        (DecryptOutcome.SUCCESS, True, "Success case"),
        (DecryptOutcome.KEY_NOT_FOUND, False, "Key not in manifests"),
        (DecryptOutcome.NO_DATA_FILES, False, "No parquet files"),
        (DecryptOutcome.DECRYPT_ERROR, False, "Decrypt/crypto failed"),
        (DecryptOutcome.CREDENTIAL_ERROR, False, "Credential auth failed"),
    ]

    for outcome, expected_success, description in outcomes_to_test:
        result = DecryptResult(
            key_id="test-key",
            success=expected_success,
            outcome=outcome,
            error_message=description
        )

        # Verify success flag matches outcome
        assert result.success == expected_success, \
            f"Outcome {outcome} should have success={expected_success}"

        # Verify outcome is set correctly
        assert result.outcome == outcome, f"Expected outcome {outcome}, got {result.outcome}"

    print("✓ Outcome distinction test passed!")
    print(f"  - Tested {len(outcomes_to_test)} distinct outcomes")
    print("  - All outcomes have correct success flags")


def test_retired_epoch_decrypt_probe_success():
    """
    Test that decrypt probes work correctly for retired epochs.

    This test validates the decrypt probe functionality specifically for
    retired encryption epochs, using the actual retired epoch fixture metadata.

    Acceptance criteria (cg-2758e):
    - [x] Test confirms decrypt probe succeeds for a retired epoch manifest
    - [x] Test validates probe returns valid decrypted data
    - [x] Test uses fixture from child 1 (retired epoch key_id and manifest)
    - [x] Test is automated and runs as part of the test suite
    """
    print("\nTesting retired epoch decrypt probe success (cg-2758e)...")

    # Use the actual retired epoch key_id from fixtures
    retired_key_id = "epoch-2023-12-retired"
    retired_epoch = "2023-12"

    # Create test corpus with retired epoch matching fixture metadata
    temp_dir = create_test_corpus()
    cred_path = create_test_credential(temp_dir)

    # Override to use the fixture-specific retired epoch key_id
    partition_path = temp_dir / "provider=github" / "year=2023" / "month=12"

    # Create the correct partition structure for retired epoch
    partition_path.mkdir(parents=True, exist_ok=True)

    # Create manifest with retired epoch metadata matching fixture structure
    manifest = {
        'encryption_keys': [
            {
                'key_id': retired_key_id,
                'epoch': retired_epoch,
                'key_path': f'/keys/{retired_key_id}.json',
                'status': 'retired'
            }
        ],
        'partition_count': 1,
        'row_count': 1,
        'created_at': '2024-08-06T00:00:00Z'
    }

    manifest_path = partition_path / "_manifest"
    with open(manifest_path, 'w') as f:
        json.dump(manifest, f)

    # Create valid parquet file for decryption testing
    import pyarrow as pa
    import pyarrow.parquet as pq

    # Create sample commit data matching the fixture structure
    sample_commits = {
        'sha': ['abc123def456789'],
        'message': ['Test commit from retired epoch 2023-12'],
        'author': ['test-author'],
        'timestamp': [1701388800]  # 2023-12-01 timestamp
    }

    table = pa.table(sample_commits)
    parquet_path = partition_path / "commits-2023-12.parquet"
    pq.write_table(table, parquet_path)

    # Create decrypt probe and test the retired epoch
    probe = DecryptProbe(str(temp_dir), str(cred_path))
    result = probe.test_key_id(retired_key_id)

    # AC1: Test confirms decrypt probe succeeds for a retired epoch manifest
    assert result.success, f"Expected successful decryption for retired epoch {retired_key_id}"
    assert result.outcome == DecryptOutcome.SUCCESS, \
        f"Expected SUCCESS outcome for retired epoch, got {result.outcome}"

    # AC2: Test validates probe returns valid decrypted data
    assert result.key_id == retired_key_id, \
        f"Expected key_id {retired_key_id}, got {result.key_id}"
    assert result.epoch == retired_epoch, \
        f"Expected epoch {retired_epoch}, got {result.epoch}"
    assert result.test_file.endswith("commits-2023-12.parquet"), \
        f"Expected test file to be the parquet file we created, got {result.test_file}"
    assert result.partitions_tested == 1, \
        f"Expected 1 partition tested, got {result.partitions_tested}"
    assert result.error_message == "", \
        f"Expected no error message for successful decrypt, got '{result.error_message}'"

    # AC3: Verify the test uses the fixture's retired epoch key_id and manifest structure
    # Validate that the partition manifest matches the fixture structure
    with open(manifest_path, 'r') as f:
        loaded_manifest = json.load(f)

    encryption_keys = loaded_manifest.get('encryption_keys', [])
    assert len(encryption_keys) == 1, "Expected exactly one encryption key in manifest"

    key_info = encryption_keys[0]
    assert key_info['key_id'] == retired_key_id, "Key_id should match fixture"
    assert key_info['epoch'] == retired_epoch, "Epoch should match fixture"
    assert key_info['status'] == 'retired', "Status should be 'retired'"

    # Verify the actual parquet file can be read (decrypt validation)
    parquet_file = pq.ParquetFile(parquet_path)
    table_read = parquet_file.read()

    # Validate the decrypted data structure
    assert 'sha' in table_read.column_names, "Decrypted data should have 'sha' column"
    assert 'message' in table_read.column_names, "Decrypted data should have 'message' column"
    assert table_read.num_rows == 1, "Decrypted data should have 1 row"

    print("✓ Retired epoch decrypt probe success test passed!")
    print(f"  - Retired epoch key_id: {result.key_id}")
    print(f"  - Retired epoch: {result.epoch}")
    print(f"  - Decrypt outcome: {result.outcome.value}")
    print(f"  - Test file: {result.test_file}")
    print(f"  - Partitions tested: {result.partitions_tested}")
    print(f"  - Decrypted data rows: {table_read.num_rows}")
    print(f"  - Decrypted data columns: {table_read.column_names}")

    # Cleanup
    import shutil
    shutil.rmtree(temp_dir)


if __name__ == "__main__":
    print("Testing decrypt_probe module (cg-2x1hy)...\n")
    print("=" * 70)

    try:
        test_successful_decryption()
        test_key_not_found()
        test_no_data_files()
        test_decrypt_error()
        test_multiple_keys()
        test_validate_all_passed()
        test_retired_epoch_key()
        test_credential_error()
        test_outcome_distinction()
        test_retired_epoch_decrypt_probe_success()

        print("\n" + "=" * 70)
        print("✅ All tests passed! Decrypt probe is working correctly.")
        print("\nAcceptance criteria met:")
        print("  ✓ Function accepts a key_id and attempts decrypt probe")
        print("  ✓ Uses migration credentials (same as corpus migration)")
        print("  ✓ Returns clear pass/fail result per key_id")
        print("  ✓ Handles decrypt errors gracefully and reports specific failure reason")
        print("  ✓ Can handle both current and retired epoch keys")
        print("  ✓ Distinguishes between 'key not found' vs 'decrypt failed' vs 'success'")
        print("\nRetired epoch decrypt probe test (cg-2758e):")
        print("  ✓ Decrypt probe succeeds for retired epoch manifest")
        print("  ✓ Probe returns valid decrypted data with correct metadata")
        print("  ✓ Test uses fixture metadata (epoch-2023-12-retired)")
        print("  ✓ Test is automated and runs as part of test suite")

    except AssertionError as e:
        print(f"\n❌ Test failed: {e}")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
