#!/usr/bin/env python3
"""
Test that migration aborts when a retired epoch fails decrypt.

This test validates the critical error path where a retired epoch cannot be
decrypted during preflight checks, ensuring migration aborts correctly.

Acceptance criteria:
- [x] Test confirms migration aborts when a retired epoch fails preflight
- [x] Test validates error message or abort condition is correctly triggered
- [x] Test distinguishes between "current epoch failed" and "retired epoch failed"
- [x] Test is automated and runs as part of the test suite

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
from unittest.mock import patch, MagicMock

# Add migration module to path
sys.path.insert(0, str(Path(__file__).parent))

from preflight_check_epochs import EpochPreflightChecker, ValidationResult


class TestMigrationAbortRetiredEpochDecrypt(unittest.TestCase):
    """Test suite specifically for migration abort when retired epoch fails decrypt."""

    def setUp(self):
        """Set up test fixtures before each test."""
        self.temp_dir = tempfile.mkdtemp(prefix="test_abort_retired_")
        self.temp_path = Path(self.temp_dir)

    def tearDown(self):
        """Clean up test fixtures after each test."""
        try:
            shutil.rmtree(self.temp_dir)
        except Exception:
            pass

    def create_test_parquet(self, partition_path: Path, num_commits: int = 10) -> Path:
        """Create a realistic test Parquet file with commit data."""
        import pyarrow as pa
        import pyarrow.parquet as pq

        commits_data = {
            'sha': [f'abc123{idx}' for idx in range(num_commits)],
            'repo_full_name': ['test/example'] * num_commits,
            'provider': ['github'] * num_commits,
            'author_name': [f'Test Author {idx}' for idx in range(num_commits)],
            'author_email': [f'test{idx}@example.com' for idx in range(num_commits)],
            'committed_at': [1609459200000 + idx * 1000 for idx in range(num_commits)],
            'message': [f'Test commit message {idx}' for idx in range(num_commits)]
        }

        schema = pa.schema([
            ('sha', pa.string()),
            ('repo_full_name', pa.string()),
            ('provider', pa.string()),
            ('author_name', pa.string()),
            ('author_email', pa.string()),
            ('committed_at', pa.int64()),
            ('message', pa.string())
        ])

        table = pa.table(commits_data, schema=schema)
        parquet_path = partition_path / f"commits_{datetime.now().strftime('%Y%m%d%H%M%S')}.parquet"
        pq.write_table(table, parquet_path)

        return parquet_path

    def create_partition_with_manifest_and_data(
        self,
        partition_path: str,
        key_id: str,
        epoch: str,
        status: str = "current",
        num_commits: int = 10
    ) -> Path:
        """Helper to create a test partition with manifest and Parquet data."""
        partition = self.temp_path / partition_path
        partition.mkdir(parents=True)

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
            "row_count": num_commits,
            "created_at": datetime.now().isoformat()
        }

        with open(partition / "_manifest", 'w') as f:
            json.dump(manifest, f, indent=2)

        parquet_file = self.create_test_parquet(partition, num_commits)
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

    def test_migration_aborts_when_retired_epoch_fails_decrypt(self):
        """
        AC1: Test confirms migration aborts when a retired epoch fails decrypt.

        This test creates a corpus where:
        - Current epoch has valid, decryptable data
        - Retired epoch has data that cannot be decrypted (simulated failure)
        - Migration should abort with clear error message
        """
        # Create current epoch partition with valid data
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2024/month=08",
            "epoch-current-2024",
            "2024-08",
            status="current",
            num_commits=15
        )

        # Create retired epoch partition
        retired_partition = self.create_partition_with_manifest_and_data(
            "provider=github/year=2020/month=01",
            "epoch-retired-failing-2020",
            "2020-01",
            status="retired",
            num_commits=10
        )

        credential_path = self.create_migration_credential([
            "epoch-current-2024",
            "epoch-retired-failing-2020"
        ])

        # Mock the decrypt probe to simulate failure for retired epoch
        original_test_one = EpochPreflightChecker._test_decrypt_one_key

        def mock_decrypt(self, key_info):
            """Simulate decrypt failure for retired epoch, success for current."""
            if "retired" in key_info.key_id:
                return ValidationResult(
                    key_id=key_info.key_id,
                    epoch=key_info.epoch,
                    success=False,
                    error_message="Decryption failed: Invalid key or corrupted data",
                    test_partition=key_info.partitions[0] if key_info.partitions else ""
                )
            else:
                # Current epoch succeeds
                return ValidationResult(
                    key_id=key_info.key_id,
                    epoch=key_info.epoch,
                    success=True,
                    test_partition=key_info.partitions[0] if key_info.partitions else ""
                )

        with patch.object(EpochPreflightChecker, '_test_decrypt_one_key', mock_decrypt):
            # Run preflight
            checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
            all_passed, results = checker.run_preflight()

            # AC1: Migration must abort when retired epoch fails
            self.assertFalse(all_passed, "Migration MUST abort when retired epoch fails decrypt")

            # Verify both epochs were tested
            self.assertEqual(len(results), 2, "Must test both current and retired epochs")

            # AC2: Validate error message is clear and specific
            failed_results = [r for r in results if not r.success]
            self.assertEqual(len(failed_results), 1, "Exactly one epoch should fail (retired)")

            failed_epoch = failed_results[0]
            self.assertEqual(failed_epoch.key_id, "epoch-retired-failing-2020")
            self.assertIn("Decryption failed", failed_epoch.error_message)
            self.assertIsNotNone(failed_epoch.test_partition, "Failed epoch should report which partition failed")

            # Verify current epoch passed
            passed_results = [r for r in results if r.success]
            self.assertEqual(len(passed_results), 1, "Current epoch should pass")
            self.assertEqual(passed_results[0].key_id, "epoch-current-2024")

    def test_distinguishes_current_vs_retired_epoch_failure(self):
        """
        AC3: Test distinguishes between "current epoch failed" and "retired epoch failed".

        This test validates that the preflight system correctly identifies which
        specific epoch failed and provides epoch-specific error information.
        """
        # Create multiple retired epochs
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2024/month=08",
            "epoch-current",
            "2024-08",
            status="current"
        )
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2022/month=06",
            "epoch-retired-2022",
            "2022-06",
            status="retired"
        )
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2020/month=01",
            "epoch-retired-2020",
            "2020-01",
            status="retired"
        )
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2018/month=03",
            "epoch-retired-2018",
            "2018-03",
            status="retired"
        )

        credential_path = self.create_migration_credential([
            "epoch-current",
            "epoch-retired-2022",
            "epoch-retired-2020",
            "epoch-retired-2018"
        ])

        # Mock decrypt: current fails, one retired fails, others succeed
        def mock_decrypt(self, key_info):
            key_to_outcome = {
                "epoch-current": (False, "Current epoch key not available"),
                "epoch-retired-2022": (False, "Retired 2022 key corrupted"),
                "epoch-retired-2020": (True, ""),
                "epoch-retired-2018": (True, ""),
            }

            success, error = key_to_outcome.get(key_info.key_id, (True, ""))
            return ValidationResult(
                key_id=key_info.key_id,
                epoch=key_info.epoch,
                success=success,
                error_message=error,
                test_partition=key_info.partitions[0] if key_info.partitions else ""
            )

        with patch.object(EpochPreflightChecker, '_test_decrypt_one_key', mock_decrypt):
            checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
            all_passed, results = checker.run_preflight()

            # AC3: Must distinguish which specific epochs failed
            self.assertFalse(all_passed, "Migration aborts when any epoch fails")

            failed_epochs = [r for r in results if not r.success]
            self.assertEqual(len(failed_epochs), 2, "Two epochs should fail")

            # Verify we can distinguish current vs retired failure
            failed_key_ids = {r.key_id for r in failed_epochs}
            self.assertIn("epoch-current", failed_key_ids, "Must identify current epoch failure")
            self.assertIn("epoch-retired-2022", failed_key_ids, "Must identify retired epoch failure")

            # Verify each failed epoch has distinct error message
            for failed in failed_epochs:
                self.assertIsNotNone(failed.error_message)
                self.assertNotEqual(failed.error_message, "", "Each failed epoch should have specific error")

                # AC3: Error message should indicate which epoch failed
                if failed.key_id == "epoch-current":
                    self.assertIn("Current", failed.error_message)
                elif failed.key_id == "epoch-retired-2022":
                    self.assertIn("2022", failed.error_message)

    def test_migration_aborts_on_any_retired_epoch_failure_even_if_current_succeeds(self):
        """
        Test that migration aborts if ANY retired epoch fails, even when current succeeds.

        This validates the critical safety property: we cannot silently skip retired epochs.
        If any retired partition cannot be decrypted, migration must abort.
        """
        # Create corpus with successful current epoch but one failed retired epoch
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2024/month=08",
            "epoch-current-success",
            "2024-08",
            status="current"
        )
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2023/month=12",
            "epoch-retired-success-2023",
            "2023-12",
            status="retired"
        )
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2022/month=06",
            "epoch-retired-success-2022",
            "2022-06",
            status="retired"
        )
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2020/month=01",
            "epoch-retired-fail-2020",
            "2020-01",
            status="retired"
        )

        credential_path = self.create_migration_credential([
            "epoch-current-success",
            "epoch-retired-success-2023",
            "epoch-retired-success-2022",
            "epoch-retired-fail-2020"
        ])

        # Mock: current succeeds, most retired succeed, one retired fails
        def mock_decrypt(self, key_info):
            if "fail-2020" in key_info.key_id:
                return ValidationResult(
                    key_id=key_info.key_id,
                    epoch=key_info.epoch,
                    success=False,
                    error_message="Retired 2020 key permanently unavailable",
                    test_partition=key_info.partitions[0] if key_info.partitions else ""
                )
            else:
                return ValidationResult(
                    key_id=key_info.key_id,
                    epoch=key_info.epoch,
                    success=True,
                    test_partition=key_info.partitions[0] if key_info.partitions else ""
                )

        with patch.object(EpochPreflightChecker, '_test_decrypt_one_key', mock_decrypt):
            checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
            all_passed, results = checker.run_preflight()

            # CRITICAL: Migration must abort even though current epoch succeeds
            self.assertFalse(all_passed, "Migration MUST abort when ANY retired epoch fails")

            # Verify 3 epochs passed, 1 failed
            passed = [r for r in results if r.success]
            failed = [r for r in results if not r.success]

            self.assertEqual(len(passed), 3, "3 epochs should succeed (current + 2 retired)")
            self.assertEqual(len(failed), 1, "1 retired epoch should fail")

            # Verify the failed one is the retired epoch
            self.assertEqual(failed[0].key_id, "epoch-retired-fail-2020")
            self.assertIn("permanently unavailable", failed[0].error_message)

            # Verify current epoch is in the passed list
            passed_key_ids = {r.key_id for r in passed}
            self.assertIn("epoch-current-success", passed_key_ids)

    def test_error_message_clarity_for_retired_epoch_failure(self):
        """
        AC2: Test validates error message clarity for retired epoch failure.

        Ensures that when a retired epoch fails, the error message is actionable
        and identifies the specific problem.
        """
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2019/month=06",
            "epoch-ancient-retired",
            "2019-06",
            status="retired"
        )

        credential_path = self.create_migration_credential(["epoch-ancient-retired"])

        # Mock with specific, actionable error message
        def mock_decrypt(self, key_info):
            return ValidationResult(
                key_id=key_info.key_id,
                epoch=key_info.epoch,
                success=False,
                error_message="Decrypt failed for epoch '2019-06': Key expired on 2023-01-01. Restore from key archive.",
                test_partition=key_info.partitions[0] if key_info.partitions else ""
            )

        with patch.object(EpochPreflightChecker, '_test_decrypt_one_key', mock_decrypt):
            checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
            all_passed, results = checker.run_preflight()

            self.assertFalse(all_passed)
            self.assertEqual(len(results), 1)

            failed = results[0]
            self.assertEqual(failed.key_id, "epoch-ancient-retired")
            self.assertEqual(failed.epoch, "2019-06")

            # AC2: Error message should be clear and actionable
            self.assertIn("Key expired", failed.error_message)
            self.assertIn("2019-06", failed.error_message)
            self.assertIn("Restore from key archive", failed.error_message)

    def test_all_epochs_including_retired_are_validated(self):
        """
        Test that all epochs (current and retired) are validated, not just current.

        This validates the core safety property: preflight checks ALL epochs.
        """
        # Create corpus with many epochs
        epoch_specs = [
            ("2024-08", "epoch-2024-08", "current"),
            ("2023-12", "epoch-2023-12", "retired"),
            ("2023-06", "epoch-2023-06", "retired"),
            ("2022-12", "epoch-2022-12", "retired"),
            ("2021-06", "epoch-2021-06", "retired"),
            ("2020-01", "epoch-2020-01", "retired"),
        ]

        for month, key_id, status in epoch_specs:
            parts = month.split("-")
            year, month = parts[0], parts[1]
            self.create_partition_with_manifest_and_data(
                f"provider=github/year={year}/month={month}",
                key_id,
                month,
                status=status
            )

        key_ids = [spec[1] for spec in epoch_specs]
        credential_path = self.create_migration_credential(key_ids)

        # Run preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        all_passed, results = checker.run_preflight()

        # All epochs should be validated
        self.assertEqual(len(results), 6, "All 6 epochs must be validated")

        # Verify we have results for both current and retired
        result_key_ids = {r.key_id for r in results}
        self.assertEqual(result_key_ids, set(key_ids))

        # Count current vs retired in results
        current_in_results = sum(1 for r in results if r.key_id == "epoch-2024-08")
        retired_in_results = sum(1 for r in results if r.key_id != "epoch-2024-08")

        self.assertEqual(current_in_results, 1, "Current epoch must be validated")
        self.assertEqual(retired_in_results, 5, "All 5 retired epochs must be validated")


class TestMigrationAbortScenarios(unittest.TestCase):
    """Additional test scenarios for migration abort behavior."""

    def setUp(self):
        """Set up test fixtures."""
        self.temp_dir = tempfile.mkdtemp(prefix="test_abort_scenarios_")
        self.temp_path = Path(self.temp_dir)

    def tearDown(self):
        """Clean up."""
        try:
            shutil.rmtree(self.temp_dir)
        except Exception:
            pass

    def create_partition_with_manifest_and_data(self, partition_path: str, key_id: str, epoch: str):
        """Helper to create partition with manifest and data."""
        import pyarrow as pa
        import pyarrow.parquet as pq

        partition = self.temp_path / partition_path
        partition.mkdir(parents=True)

        manifest = {
            "encryption_keys": [{"key_id": key_id, "epoch": epoch, "key_path": f"/keys/{key_id}"}],
            "partition_count": 1,
            "row_count": 1
        }

        with open(partition / "_manifest", 'w') as f:
            json.dump(manifest, f)

        data = {
            'sha': ['test'],
            'repo_full_name': ['test/repo'],
            'provider': ['github'],
            'author_name': ['Test'],
            'author_email': ['test@example.com'],
            'committed_at': [1609459200000],
            'message': ['Test']
        }

        schema = pa.schema([
            ('sha', pa.string()),
            ('repo_full_name', pa.string()),
            ('provider', pa.string()),
            ('author_name', pa.string()),
            ('author_email', pa.string()),
            ('committed_at', pa.int64()),
            ('message', pa.string())
        ])

        table = pa.table(data, schema=schema)
        pq.write_table(table, partition / "test.parquet")

    def create_migration_credential(self, key_ids: List[str]) -> Path:
        """Helper to create migration credentials."""
        credential_path = self.temp_path / "creds.json"
        with open(credential_path, 'w') as f:
            json.dump({"epochs": key_ids}, f)
        return credential_path

    def test_multiple_retired_epochs_fail_migration_aborts(self):
        """Test that multiple retired epoch failures cause migration abort."""
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2024/month=08",
            "epoch-current-2024",
            "2024-08"
        )
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2020/month=01",
            "epoch-retired-fail-1",
            "2020-01"
        )
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2019/month=06",
            "epoch-retired-fail-2",
            "2019-06"
        )

        credential_path = self.create_migration_credential([
            "epoch-current-2024",
            "epoch-retired-fail-1",
            "epoch-retired-fail-2"
        ])

        # Mock: current succeeds, both retired fail
        def mock_decrypt(self, key_info):
            if "retired" in key_info.key_id:
                return ValidationResult(
                    key_id=key_info.key_id,
                    epoch=key_info.epoch,
                    success=False,
                    error_message=f"Key {key_info.key_id} not found",
                    test_partition=key_info.partitions[0] if key_info.partitions else ""
                )
            return ValidationResult(
                key_id=key_info.key_id,
                epoch=key_info.epoch,
                success=True,
                test_partition=key_info.partitions[0] if key_info.partitions else ""
            )

        from unittest.mock import patch
        with patch.object(EpochPreflightChecker, '_test_decrypt_one_key', mock_decrypt):
            checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
            all_passed, results = checker.run_preflight()

            self.assertFalse(all_passed, "Must abort when multiple retired epochs fail")

            failed = [r for r in results if not r.success]
            self.assertEqual(len(failed), 2, "Both retired epochs should fail")

    def test_only_retired_epochs_current_epoch_missing(self):
        """Test corpus with only retired epochs (no current epoch)."""
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2020/month=01",
            "epoch-retired-2020",
            "2020-01"
        )
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2019/month=06",
            "epoch-retired-2019",
            "2019-06"
        )

        credential_path = self.create_migration_credential([
            "epoch-retired-2020",
            "epoch-retired-2019"
        ])

        # Mock: both retired fail
        def mock_decrypt(self, key_info):
            return ValidationResult(
                key_id=key_info.key_id,
                epoch=key_info.epoch,
                success=False,
                error_message="Key unavailable",
                test_partition=key_info.partitions[0] if key_info.partitions else ""
            )

        from unittest.mock import patch
        with patch.object(EpochPreflightChecker, '_test_decrypt_one_key', mock_decrypt):
            checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
            all_passed, results = checker.run_preflight()

            self.assertFalse(all_passed, "Must abort when retired epochs fail (even with no current)")
            self.assertEqual(len(results), 2, "Both retired epochs must be validated")


def main():
    """Run tests and report results."""
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()

    suite.addTests(loader.loadTestsFromTestCase(TestMigrationAbortRetiredEpochDecrypt))
    suite.addTests(loader.loadTestsFromTestCase(TestMigrationAbortScenarios))

    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)

    sys.exit(0 if result.wasSuccessful() else 1)


if __name__ == "__main__":
    main()
