#!/usr/bin/env python3
"""
End-to-end test proving migration starts successfully when all epochs pass preflight.

This test validates the complete happy path flow: enumeration → decrypt probes → migration start.
It confirms that migration proceeds past the preflight phase when all epochs (including retired)
can be successfully decrypted.

Acceptance criteria:
- [x] Test confirms migration would start if all epochs pass preflight
- [x] Test includes both current and retired epochs in the corpus
- [x] Test asserts migration proceeds past preflight phase
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

# Add migration module to path
sys.path.insert(0, str(Path(__file__).parent))

from preflight_check_epochs import EpochPreflightChecker, ValidationResult
from migrate_corpus import CorpusMigrator
from decrypt_probe import DecryptProbe, DecryptOutcome


@dataclass
class MigrationTestFixture:
    """Test fixture simulating a multi-epoch corpus."""
    corpus_path: Path
    credential_path: Path
    postgres_conn: str
    epochs: List[Dict]
    total_partitions: int
    total_commits: int


class TestMigrationSuccessAllEpochsPass(unittest.TestCase):
    """Test suite for migration success when all epochs pass preflight."""

    def setUp(self):
        """Set up test fixtures before each test."""
        self.temp_dir = tempfile.mkdtemp(prefix="test_migration_success_")
        self.temp_path = Path(self.temp_dir)

    def tearDown(self):
        """Clean up test fixtures after each test."""
        try:
            shutil.rmtree(self.temp_dir)
        except Exception:
            pass

    def create_test_parquet(self, partition_path: Path, num_commits: int = 10) -> Path:
        """
        Create a realistic test Parquet file with commit data.

        Args:
            partition_path: Directory to write the Parquet file
            num_commits: Number of sample commits to generate

        Returns:
            Path to the created Parquet file
        """
        import pyarrow as pa
        import pyarrow.parquet as pq

        # Create sample commit data matching the corpus schema
        commits_data = {
            'sha': [f'abc123{idx}' for idx in range(num_commits)],
            'repo_full_name': ['test/example'] * num_commits,
            'provider': ['github'] * num_commits,
            'author_name': [f'Test Author {idx}' for idx in range(num_commits)],
            'author_email': [f'test{idx}@example.com' for idx in range(num_commits)],
            'committed_at': [1609459200000 + idx * 1000 for idx in range(num_commits)],  # Timestamps
            'message': [f'Test commit message {idx}' for idx in range(num_commits)]
        }

        # Create Arrow table
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

        # Write to Parquet
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
        """
        Helper to create a test partition with manifest and Parquet data.

        Args:
            partition_path: Hive partition path
            key_id: Encryption key identifier
            epoch: Epoch identifier
            status: Epoch status (current/retired)
            num_commits: Number of commits in test Parquet

        Returns:
            Path to the created partition directory
        """
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
            "row_count": num_commits,
            "created_at": datetime.now().isoformat()
        }

        with open(partition / "_manifest", 'w') as f:
            json.dump(manifest, f, indent=2)

        # Create test Parquet file with commit data
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

    def test_migration_success_when_all_epochs_pass_preflight(self):
        """
        AC1 & AC3: Test confirms migration starts when all epochs pass preflight.

        This is the main end-to-end test validating the complete flow:
        1. Create corpus with current + retired epochs and valid data
        2. Run preflight check
        3. Verify all epochs pass decrypt probe
        4. Confirm migration would proceed (past preflight phase)
        """
        # Create corpus with both current and retired epochs
        # All epochs have valid Parquet data, so all decrypt probes should succeed

        # Current epoch partition
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2024/month=08",
            "epoch-2024-08-current",
            "2024-08",
            status="current",
            num_commits=15
        )

        # Retired epoch partition 1
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2023/month=12",
            "epoch-2023-12-retired",
            "2023-12",
            status="retired",
            num_commits=20
        )

        # Retired epoch partition 2 (ancient)
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2020/month=06",
            "epoch-2020-06-ancient",
            "2020-06",
            status="retired",
            num_commits=10
        )

        # Another retired epoch
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2022/month=03",
            "epoch-2022-03-retired",
            "2022-03",
            status="retired",
            num_commits=12
        )

        credential_path = self.create_migration_credential([
            "epoch-2024-08-current",
            "epoch-2023-12-retired",
            "epoch-2020-06-ancient",
            "epoch-2022-03-retired"
        ])

        # Step 1: Run preflight check
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        all_passed, results = checker.run_preflight()

        # Step 2: Verify all epochs were discovered
        self.assertEqual(len(results), 4, "Must discover all 4 distinct epoch keys")

        # Step 3: Verify all epochs passed decrypt probe
        self.assertTrue(all_passed, "All epochs must pass decrypt probe for migration to proceed")

        # Verify each epoch's success status
        key_to_result = {r.key_id: r for r in results}
        self.assertTrue(key_to_result["epoch-2024-08-current"].success, "Current epoch must pass")
        self.assertTrue(key_to_result["epoch-2023-12-retired"].success, "Retired epoch 2023 must pass")
        self.assertTrue(key_to_result["epoch-2020-06-ancient"].success, "Ancient retired epoch must pass")
        self.assertTrue(key_to_result["epoch-2022-03-retired"].success, "Retired epoch 2022 must pass")

        # Step 4: Confirm migration would proceed past preflight
        # In the real migration code, this is where CorpusMigrator would be initialized
        # and migration would start. The key assertion is that preflight returns True,
        # allowing migration to proceed.

        # Simulate the migration startup check from migrate_corpus.py
        if not all_passed:
            self.fail("Migration would abort due to preflight failure")

        # Migration would proceed at this point
        # (In production, CorpusMigrator would be created and run_migration() called)

    def test_both_current_and_retired_epochs_included_in_corpus(self):
        """
        AC2: Test includes both current and retired epochs in the corpus.

        Validates that the test fixture properly represents a real-world corpus
        with mixed current and retired epochs.
        """
        # Create comprehensive fixture with multiple current and retired epochs
        epoch_specs = [
            # Current epoch (most recent)
            ("2024-08", "epoch-current-latest", "current", 25),
            # Recent retired epochs
            ("2024-01", "epoch-2024-01", "retired", 30),
            ("2023-12", "epoch-2023-12", "retired", 28),
            ("2023-06", "epoch-2023-06", "retired", 22),
            # Ancient retired epochs
            ("2022-11", "epoch-2022-11", "retired", 18),
            ("2021-05", "epoch-2021-05", "retired", 15),
            ("2020-02", "epoch-2020-02", "retired", 10),
        ]

        for month, key_id, status, num_commits in epoch_specs:
            parts = month.split("-")
            year, month = parts[0], parts[1]
            self.create_partition_with_manifest_and_data(
                f"provider=github/year={year}/month={month}",
                key_id,
                month,
                status=status,
                num_commits=num_commits
            )

        key_ids = [spec[1] for spec in epoch_specs]
        credential_path = self.create_migration_credential(key_ids)

        # Verify fixture includes both current and retired
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys_by_id = checker.discover_all_keys()

        # Count current vs retired
        current_count = sum(1 for k in keys_by_id.values() if k.key_id == "epoch-current-latest")
        retired_count = len(keys_by_id) - current_count

        self.assertEqual(current_count, 1, "Fixture should have exactly 1 current epoch")
        self.assertEqual(retired_count, 6, "Fixture should have 6 retired epochs")
        self.assertEqual(len(keys_by_id), 7, "Fixture should have 7 total epochs")

        # Verify all epochs have valid Parquet data
        all_passed, results = checker.validate_decryption(keys_by_id)
        self.assertTrue(all_passed, "All epochs (current + retired) must successfully decrypt")

    def test_migration_proceeds_past_preflight_phase(self):
        """
        AC3: Test asserts migration proceeds past preflight phase.

        This test verifies that successful preflight allows migration to proceed,
        simulating the actual migration startup flow from migrate_corpus.py.
        """
        # Create corpus with mix of epochs that all have valid data
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2024/month=06",
            "epoch-2024-06",
            "2024-06",
            status="current",
            num_commits=50
        )

        self.create_partition_with_manifest_and_data(
            "provider=github/year=2023/month=11",
            "epoch-2023-11",
            "2023-11",
            status="retired",
            num_commits=40
        )

        credential_path = self.create_migration_credential([
            "epoch-2024-06",
            "epoch-2023-11"
        ])

        # Run preflight (this is the critical gate)
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        all_passed, results = checker.run_preflight()

        # CRITICAL ASSERTION: Preflight must pass for migration to proceed
        self.assertTrue(all_passed, "Preflight check must pass for migration to start")

        # Verify migration would proceed (simulating the migration startup logic)
        # This is the exact check from migrate_corpus.py lines 750-780
        if not all_passed:
            # Migration would abort here
            self.fail("Migration aborted: Preflight encryption validation failed")

        # At this point, migration would proceed
        # CorpusMigrator would be initialized and run_migration() would start

        # Verify we have successful results for all epochs
        self.assertEqual(len(results), 2, "Must have results for all epochs")
        for result in results:
            self.assertTrue(result.success, f"Epoch {result.key_id} must pass for migration to proceed")
            self.assertTrue(result.epoch.startswith("20"), "Should have realistic epoch years starting with '20'")
            self.assertEqual(result.error_message, "", "Successful epochs have no error message")

    def test_decrypt_probe_success_for_all_epochs(self):
        """
        Detailed test validating decrypt probe success for all epoch types.

        This test uses the standalone DecryptProbe to verify that decrypt probes
        succeed for both current and retired epochs when valid data exists.
        """
        # Create test corpus with multiple epoch types
        self.create_partition_with_manifest_and_data(
            "provider=github/year=2024/month=07",
            "current-key-2024",
            "2024-07",
            status="current"
        )

        self.create_partition_with_manifest_and_data(
            "provider=github/year=2021/month=09",
            "retired-key-2021",
            "2021-09",
            status="retired"
        )

        self.create_partition_with_manifest_and_data(
            "provider=github/year=2019/month=04",
            "ancient-key-2019",
            "2019-04",
            status="retired"
        )

        credential_path = self.create_migration_credential([
            "current-key-2024",
            "retired-key-2021",
            "ancient-key-2019"
        ])

        # Use standalone DecryptProbe to test each epoch
        probe = DecryptProbe(str(self.temp_path), str(credential_path))

        # Test current epoch
        current_result = probe.test_key_id("current-key-2024")
        self.assertTrue(current_result.success, "Current epoch decrypt must succeed")
        self.assertEqual(current_result.outcome, DecryptOutcome.SUCCESS)
        self.assertEqual(current_result.epoch, "2024-07")

        # Test retired epoch
        retired_result = probe.test_key_id("retired-key-2021")
        self.assertTrue(retired_result.success, "Retired epoch decrypt must succeed")
        self.assertEqual(retired_result.outcome, DecryptOutcome.SUCCESS)
        self.assertEqual(retired_result.epoch, "2021-09")

        # Test ancient retired epoch
        ancient_result = probe.test_key_id("ancient-key-2019")
        self.assertTrue(ancient_result.success, "Ancient retired epoch decrypt must succeed")
        self.assertEqual(ancient_result.outcome, DecryptOutcome.SUCCESS)
        self.assertEqual(ancient_result.epoch, "2019-04")

        # Test all keys in batch
        all_keys = ["current-key-2024", "retired-key-2021", "ancient-key-2019"]
        batch_results = probe.test_multiple_keys(all_keys)

        all_passed, failed = probe.validate_all_passed(batch_results)
        self.assertTrue(all_passed, "All epoch decrypt probes must succeed")
        self.assertEqual(len(failed), 0, "No epochs should fail decrypt probe")

    def test_complete_migration_flow_simulation(self):
        """
        Complete end-to-end simulation of the migration flow.

        This test simulates the entire migration startup sequence:
        1. Create corpus with mixed epochs
        2. Discover all encryption keys
        3. Run decrypt probes (preflight)
        4. Verify all pass
        5. Confirm migration would start
        """
        # Step 1: Create realistic multi-epoch corpus
        epochs_to_create = [
            ("2024-08", "epoch-2024-08-current", "current", 100),
            ("2023-06", "epoch-2023-06-retired", "retired", 85),
            ("2022-04", "epoch-2022-04-retired", "retired", 72),
            ("2021-02", "epoch-2021-02-retired", "retired", 58),
            ("2020-01", "epoch-2020-01-retired", "retired", 45),
        ]

        for month, key_id, status, num_commits in epochs_to_create:
            parts = month.split("-")
            year, month = parts[0], parts[1]
            self.create_partition_with_manifest_and_data(
                f"provider=github/year={year}/month={month}",
                key_id,
                month,
                status=status,
                num_commits=num_commits
            )

        key_ids = [spec[1] for spec in epochs_to_create]
        credential_path = self.create_migration_credential(key_ids)

        # Step 2: Discover all encryption keys (enumeration phase)
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys_by_id = checker.discover_all_keys()

        self.assertEqual(len(keys_by_id), 5, "Must enumerate all 5 epoch keys")

        # Verify current and retired are both discovered
        current_keys = [k for k in keys_by_id.values() if "current" in k.key_id]
        retired_keys = [k for k in keys_by_id.values() if "retired" in k.key_id]

        self.assertEqual(len(current_keys), 1, "Must have 1 current epoch")
        self.assertEqual(len(retired_keys), 4, "Must have 4 retired epochs")

        # Step 3: Run decrypt probes (preflight phase)
        all_passed, results = checker.validate_decryption(keys_by_id)

        # Step 4: Verify all decrypt probes succeed
        self.assertTrue(all_passed, "All decrypt probes must pass for migration to proceed")
        self.assertEqual(len(results), 5, "Must have validation results for all epochs")

        # Verify each result
        for result in results:
            self.assertTrue(
                result.success,
                f"Epoch {result.key_id} must successfully decrypt"
            )
            self.assertEqual(
                result.error_message,
                "",
                f"Successful epoch {result.key_id} should have no error message"
            )
            self.assertGreater(
                result.test_partition,
                "",
                f"Successful epoch {result.key_id} should have test partition info"
            )

        # Step 5: Confirm migration would start (past preflight phase)
        # This simulates the exact logic from migrate_corpus.py main() function
        if not all_passed:
            # This would trigger migration abort (lines 751-780 in migrate_corpus.py)
            self.fail("Migration would abort: preflight check failed")

        # At this point, migration would proceed with:
        # migrator = CorpusMigrator(...)
        # migrator.run_migration()

        # Final verification: all critical assertions passed
        self.assertTrue(
            all_passed,
            "CRITICAL: Migration startup requires all epochs to pass preflight"
        )


class TestMigrationSuccessEdgeCases(unittest.TestCase):
    """Test edge cases for migration success scenarios."""

    def setUp(self):
        """Set up test fixtures."""
        self.temp_dir = tempfile.mkdtemp(prefix="test_migration_edge_")
        self.temp_path = Path(self.temp_dir)

    def tearDown(self):
        """Clean up."""
        try:
            shutil.rmtree(self.temp_dir)
        except Exception:
            pass

    def create_partition_with_manifest_and_data(
        self, partition_path: str, key_id: str, epoch: str, num_commits: int = 5
    ):
        """Helper to create partition with manifest and Parquet data."""
        import pyarrow as pa
        import pyarrow.parquet as pq

        partition = self.temp_path / partition_path
        partition.mkdir(parents=True)

        manifest = {
            "encryption_keys": [{"key_id": key_id, "epoch": epoch, "key_path": f"/keys/{key_id}"}],
            "partition_count": 1,
            "row_count": num_commits
        }

        with open(partition / "_manifest", 'w') as f:
            json.dump(manifest, f)

        # Create simple Parquet file
        data = {
            'sha': [f'test{idx}' for idx in range(num_commits)],
            'repo_full_name': ['test/repo'] * num_commits,
            'provider': ['github'] * num_commits,
            'author_name': ['Test'] * num_commits,
            'author_email': ['test@example.com'] * num_commits,
            'committed_at': [1609459200000] * num_commits,
            'message': ['Test'] * num_commits
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
        parquet_path = partition / "test.parquet"
        pq.write_table(table, parquet_path)

    def test_all_retired_epochs_no_current(self):
        """Test migration success when all epochs are retired (no current epoch)."""
        # Create corpus with only retired epochs
        self.create_partition_with_manifest_and_data("provider=github/year=2020/month=01", "epoch-2020", "2020-01")
        self.create_partition_with_manifest_and_data("provider=github/year=2019/month=06", "epoch-2019", "2019-06")

        credential_path = self.temp_path / "creds.json"
        with open(credential_path, 'w') as f:
            json.dump({"epochs": ["epoch-2020", "epoch-2019"]}, f)

        # Run preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        all_passed, results = checker.run_preflight()

        # Should pass even with no current epoch
        self.assertTrue(all_passed, "Migration should succeed with only retired epochs if all pass decrypt")
        self.assertEqual(len(results), 2)

    def test_many_epochs_all_pass(self):
        """Test migration success with many epochs (10+) all passing."""
        # Create 12 epochs covering a full year
        for month_num in range(1, 13):
            month_str = f"2024-{month_num:02d}"
            key_id = f"epoch-{month_str}"
            partition = f"provider=github/year=2024/month={month_num:02d}"
            self.create_partition_with_manifest_and_data(partition, key_id, month_str)

        credential_path = self.temp_path / "creds.json"
        key_ids = [f"epoch-2024-{m:02d}" for m in range(1, 13)]
        with open(credential_path, 'w') as f:
            json.dump({"epochs": key_ids}, f)

        # Run preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        all_passed, results = checker.run_preflight()

        # All 12 epochs should pass
        self.assertTrue(all_passed, "All 12 epochs must pass for migration to proceed")
        self.assertEqual(len(results), 12)


def main():
    """Run tests and report results."""
    # Create test suite
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()

    # Add all test cases
    suite.addTests(loader.loadTestsFromTestCase(TestMigrationSuccessAllEpochsPass))
    suite.addTests(loader.loadTestsFromTestCase(TestMigrationSuccessEdgeCases))

    # Run tests
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)

    # Exit with appropriate code
    sys.exit(0 if result.wasSuccessful() else 1)


if __name__ == "__main__":
    main()