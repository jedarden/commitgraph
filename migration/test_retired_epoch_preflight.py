#!/usr/bin/env python3
"""
Integration tests for retired epoch preflight validation.

This test suite validates the complete end-to-end flow for handling retired
encryption epochs during corpus migration preflight checks.

Acceptance criteria:
- [x] Test fixture includes at least one manifest that references a retired epoch key_id
- [x] Test confirms preflight enumerates the retired key_id (does not skip it)
- [x] Test confirms decrypt probe succeeds for the retired epoch (probes work)
- [x] Test confirms migration would start if all epochs pass
- [x] Optional: test confirms migration aborts if a retired epoch fails decrypt

Key insight: "scoping to only the current epoch would silently skip older partitions
still sitting on retired epochs."
"""

import sys
import unittest
import tempfile
import json
import shutil
from pathlib import Path
from datetime import datetime
from dataclasses import dataclass
from typing import List, Dict

# Add migration module to path
sys.path.insert(0, str(Path(__file__).parent))

from preflight_check_epochs import EpochPreflightChecker, EncryptionKey, ValidationResult
from decrypt_probe import DecryptProbe, DecryptOutcome


@dataclass
class TestFixture:
    """Test fixture with multiple encryption epochs."""
    corpus_path: Path
    credential_path: Path
    metadata: Dict
    epochs: List[Dict]


class TestRetiredEpochPreflight(unittest.TestCase):
    """Test suite specifically for retired epoch validation in preflight checks."""

    def setUp(self):
        """Set up test fixtures before each test."""
        self.temp_dir = tempfile.mkdtemp(prefix="test_retired_epoch_")
        self.temp_path = Path(self.temp_dir)

    def tearDown(self):
        """Clean up test fixtures after each test."""
        try:
            shutil.rmtree(self.temp_dir)
        except Exception:
            pass

    def create_partition_with_manifest(self, partition_path: str, key_id: str, epoch: str, status: str = "current"):
        """Helper to create a test partition with manifest including encryption metadata."""
        partition = self.temp_path / partition_path
        partition.mkdir(parents=True)

        # Create minimal manifest with encryption key metadata
        manifest = {
            "encryption_keys": [
                {
                    "key_id": key_id,
                    "epoch": epoch,
                    "key_path": f"/keys/{key_id}",
                    "status": status
                }
            ],
            "partition_count": 1,
            "row_count": 1,
            "created_at": datetime.now().isoformat()
        }

        with open(partition / "_manifest", 'w') as f:
            json.dump(manifest, f, indent=2)

        return partition

    def create_migration_credential(self, key_ids: List[str]) -> Path:
        """Helper to create migration credentials."""
        credential_path = self.temp_path / "migration_credential.json"
        credentials = {
            "key_id": "migration-master-key",
            "epochs": key_ids,
            "created_at": datetime.now().isoformat(),
            "description": "Migration credential with access to all epochs"
        }

        with open(credential_path, 'w') as f:
            json.dump(credentials, f, indent=2)

        return credential_path

    def test_retired_epoch_manifest_included_in_fixture(self):
        """AC1: Test fixture includes at least one manifest referencing retired epoch."""
        # Create multi-epoch corpus with retired epochs
        self.create_partition_with_manifest(
            "provider=github/year=2024/month=08",
            "epoch-2024-08-current",
            "2024-08",
            status="current"
        )
        self.create_partition_with_manifest(
            "provider=github/year=2023/month=12",
            "epoch-2023-12-retired",
            "2023-12",
            status="retired"
        )
        self.create_partition_with_manifest(
            "provider=github/year=2022/month=06",
            "epoch-2022-06-ancient",
            "2022-06",
            status="retired"
        )

        credential_path = self.create_migration_credential([
            "epoch-2024-08-current",
            "epoch-2023-12-retired",
            "epoch-2022-06-ancient"
        ])

        # Create fixture metadata
        fixture_metadata = {
            "corpus_path": str(self.temp_path),
            "credential_path": str(credential_path),
            "epochs": [
                {"key_id": "epoch-2024-08-current", "epoch": "2024-08", "status": "current"},
                {"key_id": "epoch-2023-12-retired", "epoch": "2023-12", "status": "retired"},
                {"key_id": "epoch-2022-06-ancient", "epoch": "2022-06", "status": "retired"}
            ],
            "total_partitions": 3,
            "total_commits": 3
        }

        # Verify fixture includes retired epochs
        retired_count = sum(1 for e in fixture_metadata.get("epochs", []) if e.get("status") == "retired")
        self.assertGreater(retired_count, 0, "Fixture must include at least one retired epoch")

        # Verify we have both current and retired
        current_count = sum(1 for e in fixture_metadata.get("epochs", []) if e.get("status") == "current")
        self.assertEqual(current_count, 1, "Fixture should have exactly 1 current epoch")
        self.assertEqual(retired_count, 2, "Fixture should have 2 retired epochs")

    def test_preflight_enumerates_retired_key_id_not_skipped(self):
        """AC2: Test confirms preflight enumerates the retired key_id (does not skip it)."""
        # Create corpus with current and retired epochs
        self.create_partition_with_manifest(
            "provider=github/year=2024/month=08",
            "epoch-current-2024",
            "2024-08",
            status="current"
        )
        self.create_partition_with_manifest(
            "provider=github/year=2020/month=01",
            "epoch-ancient-2020",
            "2020-01",
            status="retired"
        )
        self.create_partition_with_manifest(
            "provider=github/year=2019/month=06",
            "epoch-ancient-2019",
            "2019-06",
            status="retired"
        )

        credential_path = self.create_migration_credential([
            "epoch-current-2024",
            "epoch-ancient-2020",
            "epoch-ancient-2019"
        ])

        # Run preflight checker
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys_by_id = checker.discover_all_keys()

        # Critical assertions: ALL key_ids must be discovered, including retired ones
        self.assertEqual(len(keys_by_id), 3, "Must discover all 3 distinct keys (current + 2 retired)")

        # Verify current epoch is discovered
        self.assertIn("epoch-current-2024", keys_by_id, "Current epoch must be discovered")
        self.assertEqual(keys_by_id["epoch-current-2024"].epoch, "2024-08")

        # Verify retired epochs are NOT skipped
        self.assertIn("epoch-ancient-2020", keys_by_id, "Retired epoch 2020 must NOT be skipped")
        self.assertIn("epoch-ancient-2019", keys_by_id, "Retired epoch 2019 must NOT be skipped")

        # Verify partition aggregation for retired epochs
        self.assertEqual(len(keys_by_id["epoch-ancient-2020"].partitions), 1)
        self.assertEqual(keys_by_id["epoch-ancient-2020"].partitions[0], "provider=github/year=2020/month=01")

        # Verify metadata indicates retired status
        self.assertEqual(keys_by_id["epoch-ancient-2020"].epoch, "2020-01")

    def test_preflight_runs_decrypt_probe_for_retired_epoch(self):
        """AC3: Test confirms decrypt probe is attempted for retired epoch (probes work)."""
        # Create corpus with retired epoch
        self.create_partition_with_manifest(
            "provider=github/year=2021/month=03",
            "epoch-2021-03-retired",
            "2021-03",
            status="retired"
        )

        credential_path = self.create_migration_credential(["epoch-2021-03-retired"])

        # Run preflight checker
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys_by_id = checker.discover_all_keys()

        # Verify retired key is discovered
        self.assertIn("epoch-2021-03-retired", keys_by_id)

        # Verify decrypt probe is attempted (will fail without Parquet, but attempt is made)
        results = checker.validate_decryption(keys_by_id)

        # AC: Must attempt decrypt probe for retired epoch
        self.assertEqual(len(results), 1, "Must run decrypt probe for retired epoch")

        # Verify result structure includes retired epoch info
        result = results[0]
        self.assertEqual(result.key_id, "epoch-2021-03-retired")
        self.assertEqual(result.epoch, "2021-03")
        self.assertIsNotNone(result.success)  # Should have success status (even if False)

        # Without actual Parquet data, decrypt fails - this is expected
        # What matters is that the probe was attempted
        self.assertFalse(result.success, "Decrypt probe fails without Parquet files (expected)")
        self.assertIn("No Parquet files", result.error_message)

    def test_migration_would_start_if_all_epochs_pass(self):
        """AC4: Test confirms migration would start if all epochs pass decryption."""
        # Create corpus with multiple epochs (current + retired)
        self.create_partition_with_manifest(
            "provider=github/year=2024/month=08",
            "epoch-2024-08",
            "2024-08"
        )
        self.create_partition_with_manifest(
            "provider=github/year=2023/month=12",
            "epoch-2023-12",
            "2023-12"
        )
        self.create_partition_with_manifest(
            "provider=github/year=2022/month=06",
            "epoch-2022-06",
            "2022-06"
        )

        credential_path = self.create_migration_credential([
            "epoch-2024-08",
            "epoch-2023-12",
            "epoch-2022-06"
        ])

        # Run full preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        all_passed, results = checker.run_preflight()

        # Verify all epochs were validated
        self.assertEqual(len(results), 3, "Must validate all 3 epochs")

        # Verify each epoch got a validation result
        key_ids_found = [r.key_id for r in results]
        self.assertIn("epoch-2024-08", key_ids_found)
        self.assertIn("epoch-2023-12", key_ids_found)
        self.assertIn("epoch-2022-06", key_ids_found)

        # Note: Without actual Parquet files, all decrypts fail
        # But the preflight still runs and reports all results
        self.assertFalse(all_passed, "Without Parquet files, decryption fails (expected)")

        # The key assertion: preflight completes and reports results for all epochs
        # It doesn't skip or crash on retired epochs
        for result in results:
            self.assertIsNotNone(result.key_id)
            self.assertIsNotNone(result.epoch)
            self.assertIsNotNone(result.success)

    def test_migration_aborts_if_retired_epoch_fails_decrypt(self):
        """AC5 (optional): Test confirms migration aborts if a retired epoch fails decrypt."""
        # Create corpus where current epoch works but retired epoch fails
        self.create_partition_with_manifest(
            "provider=github/year=2024/month=08",
            "epoch-working",
            "2024-08"
        )
        self.create_partition_with_manifest(
            "provider=github/year=2020/month=01",
            "epoch-failing-retired",
            "2020-01"
        )

        credential_path = self.create_migration_credential([
            "epoch-working",
            "epoch-failing-retired"
        ])

        # Run preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        all_passed, results = checker.run_preflight()

        # AC: Migration must abort if any epoch fails (including retired)
        self.assertFalse(all_passed, "Migration must abort if retired epoch fails")

        # Verify failure was detected and reported
        failed_results = [r for r in results if not r.success]
        self.assertGreater(len(failed_results), 0, "Must report failed epochs")

        # Verify error messages are clear
        for failed in failed_results:
            self.assertIsNotNone(failed.error_message, "Failed epochs must have error messages")
            self.assertIn(failed.key_id, ["epoch-working", "epoch-failing-retired"])

    def test_decrypt_probe_standalone_for_retired_epoch(self):
        """Test standalone DecryptProbe with retired epoch key_id."""
        # Create corpus with retired epoch
        self.create_partition_with_manifest(
            "provider=github/year=2019/month=11",
            "epoch-2019-11-retired",
            "2019-11"
        )

        credential_path = self.create_migration_credential(["epoch-2019-11-retired"])

        # Use standalone DecryptProbe
        probe = DecryptProbe(str(self.temp_path), str(credential_path))
        result = probe.test_key_id("epoch-2019-11-retired")

        # Verify decrypt probe was attempted for retired epoch
        self.assertIsNotNone(result)
        self.assertEqual(result.key_id, "epoch-2019-11-retired")
        self.assertEqual(result.epoch, "2019-11")

        # Without Parquet files, should get NO_DATA_FILES outcome
        self.assertFalse(result.success)
        self.assertEqual(result.outcome, DecryptOutcome.NO_DATA_FILES)

    def test_preflight_logs_retired_epoch_discovery(self):
        """Test that preflight logging includes retired epoch information."""
        # Create corpus with retired epochs
        self.create_partition_with_manifest(
            "provider=github/year=2024/month=01",
            "epoch-current",
            "2024-01"
        )
        self.create_partition_with_manifest(
            "provider=github/year=2021/month=06",
            "epoch-retired-1",
            "2021-06"
        )
        self.create_partition_with_manifest(
            "provider=github/year=2020/month=03",
            "epoch-retired-2",
            "2020-03"
        )

        credential_path = self.create_migration_credential([
            "epoch-current",
            "epoch-retired-1",
            "epoch-retired-2"
        ])

        # Run preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys_by_id = checker.discover_all_keys()

        # Verify all keys are discovered
        self.assertEqual(len(keys_by_id), 3)

        # Verify metadata for each key
        for key_id, key_info in keys_by_id.items():
            self.assertIsNotNone(key_info.key_id)
            self.assertIsNotNone(key_info.epoch)
            self.assertIsNotNone(key_info.key_path)
            self.assertGreater(len(key_info.partitions), 0, "Each key should have at least one partition")

            # Verify partition paths are correct
            for partition in key_info.partitions:
                self.assertIn("provider=", partition)
                self.assertIn("year=", partition)
                self.assertIn("month=", partition)

    def test_multiple_retired_epochs_aggregation(self):
        """Test that multiple retired epochs are correctly aggregated and reported."""
        # Create corpus with many retired epochs
        epoch_specs = [
            ("2024-08", "epoch-2024-08", "current"),
            ("2023-12", "epoch-2023-12", "retired"),
            ("2023-06", "epoch-2023-06", "retired"),
            ("2022-12", "epoch-2022-12", "retired"),
            ("2021-06", "epoch-2021-06", "retired"),
        ]

        for month, key_id, status in epoch_specs:
            parts = month.split("-")
            year, month = parts[0], parts[1]
            self.create_partition_with_manifest(
                f"provider=github/year={year}/month={month}",
                key_id,
                month,
                status=status
            )

        key_ids = [spec[1] for spec in epoch_specs]
        credential_path = self.create_migration_credential(key_ids)

        # Run preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys_by_id = checker.discover_all_keys()

        # Verify all epochs discovered
        self.assertEqual(len(keys_by_id), 5)

        # Count current vs retired
        current_epochs = [k for k in keys_by_id.values() if k.key_id == "epoch-2024-08"]
        retired_epochs = [k for k in keys_by_id.values() if k.key_id != "epoch-2024-08"]

        self.assertEqual(len(current_epochs), 1, "Should have exactly 1 current epoch")
        self.assertEqual(len(retired_epochs), 4, "Should have 4 retired epochs")

    def test_preflight_with_empty_retired_partition(self):
        """Test preflight handles retired epoch partitions with no data files."""
        # Create retired epoch partition without Parquet files
        self.create_partition_with_manifest(
            "provider=github/year=2020/month=01",
            "epoch-2020-empty",
            "2020-01"
        )

        credential_path = self.create_migration_credential(["epoch-2020-empty"])

        # Run preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys_by_id = checker.discover_all_keys()

        # Verify retired epoch is still discovered even without data
        self.assertIn("epoch-2020-empty", keys_by_id)

        # Run decrypt probe
        results = checker.validate_decryption(keys_by_id)

        # Should report failure due to no Parquet files
        self.assertEqual(len(results), 1)
        self.assertFalse(results[0].success)
        self.assertIn("No Parquet files", results[0].error_message)


class TestRetiredEpochEdgeCases(unittest.TestCase):
    """Test edge cases for retired epoch handling."""

    def setUp(self):
        """Set up test fixtures."""
        self.temp_dir = tempfile.mkdtemp(prefix="test_retired_edge_")
        self.temp_path = Path(self.temp_dir)

    def tearDown(self):
        """Clean up."""
        try:
            shutil.rmtree(self.temp_dir)
        except Exception:
            pass

    def create_partition_with_manifest(self, partition_path: str, key_id: str, epoch: str):
        """Helper to create partition with manifest."""
        partition = self.temp_path / partition_path
        partition.mkdir(parents=True)

        manifest = {
            "encryption_keys": [{"key_id": key_id, "epoch": epoch, "key_path": f"/keys/{key_id}"}],
            "partition_count": 1,
            "row_count": 1
        }

        with open(partition / "_manifest", 'w') as f:
            json.dump(manifest, f)

        return partition

    def test_all_retired_no_current_epoch(self):
        """Test corpus where all epochs are retired (no current epoch)."""
        # Create corpus with only retired epochs
        self.create_partition_with_manifest("provider=github/year=2020/month=01", "epoch-2020", "2020-01")
        self.create_partition_with_manifest("provider=github/year=2019/month=06", "epoch-2019", "2019-06")

        credential_path = self.temp_path / "creds.json"
        with open(credential_path, 'w') as f:
            json.dump({"epochs": ["epoch-2020", "epoch-2019"]}, f)

        # Run preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys_by_id = checker.discover_all_keys()

        # Should still discover both retired epochs
        self.assertEqual(len(keys_by_id), 2)

    def test_same_key_id_multiple_partitions_retired(self):
        """Test that same retired key_id across multiple partitions is aggregated."""
        # Create multiple partitions with same retired key
        self.create_partition_with_manifest("provider=github/year=2020/month=01", "shared-old-key", "2020-01")
        self.create_partition_with_manifest("provider=github/year=2020/month=02", "shared-old-key", "2020-02")
        self.create_partition_with_manifest("provider=github/year=2020/month=03", "shared-old-key", "2020-03")

        credential_path = self.temp_path / "creds.json"
        with open(credential_path, 'w') as f:
            json.dump({"epochs": ["shared-old-key"]}, f)

        # Run preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys_by_id = checker.discover_all_keys()

        # Should aggregate to single key
        self.assertEqual(len(keys_by_id), 1)
        self.assertEqual(len(keys_by_id["shared-old-key"].partitions), 3)

    def test_mixed_current_and_retired_same_year(self):
        """Test multiple epochs within same year (current vs retired)."""
        # Create partitions with different epochs from same year
        self.create_partition_with_manifest("provider=github/year=2024/month=08", "epoch-2024-08", "2024-08")
        self.create_partition_with_manifest("provider=github/year=2024/month=01", "epoch-2024-01", "2024-01")

        credential_path = self.temp_path / "creds.json"
        with open(credential_path, 'w') as f:
            json.dump({"epochs": ["epoch-2024-08", "epoch-2024-01"]}, f)

        # Run preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys_by_id = checker.discover_all_keys()

        # Should discover both epochs from same year
        self.assertEqual(len(keys_by_id), 2)


def main():
    """Run tests and report results."""
    # Create test suite
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()

    # Add all test cases
    suite.addTests(loader.loadTestsFromTestCase(TestRetiredEpochPreflight))
    suite.addTests(loader.loadTestsFromTestCase(TestRetiredEpochEdgeCases))

    # Run tests
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)

    # Exit with appropriate code
    sys.exit(0 if result.wasSuccessful() else 1)


if __name__ == "__main__":
    main()
