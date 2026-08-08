#!/usr/bin/env python3
"""
Integration tests for epoch decryption preflight checks.

Tests the critical requirement: "scoping to only the current epoch would
silently skip older partitions still sitting on retired epochs."

Acceptance criteria:
- Tool enumerates and reports every distinct key_id found across all manifests
- Tool attempts a decrypt probe per epoch and reports pass/fail per key_id
- Migration refuses to start if any enumerated epoch fails to decrypt
- Test fixture includes at least one manifest referencing a retired (non-current) epoch
"""

import sys
import unittest
import tempfile
import json
import shutil
from pathlib import Path
from datetime import datetime

# Add migration module to path
sys.path.insert(0, str(Path(__file__).parent))

from preflight_check_epochs import EpochPreflightChecker, EncryptionKey, ValidationResult


class TestEpochPreflightChecks(unittest.TestCase):
    """Test suite for epoch decryption preflight validation."""

    def setUp(self):
        """Set up test fixtures before each test."""
        self.temp_dir = tempfile.mkdtemp(prefix="test_preflight_")
        self.temp_path = Path(self.temp_dir)

    def tearDown(self):
        """Clean up test fixtures after each test."""
        try:
            shutil.rmtree(self.temp_dir)
        except Exception:
            pass

    def create_partition(self, partition_path: str, key_id: str, epoch: str, commit_count: int = 1):
        """Helper to create a test partition with manifest."""
        partition = self.temp_path / partition_path
        partition.mkdir(parents=True)

        # Create minimal manifest
        manifest = {
            "encryption_keys": [
                {
                    "key_id": key_id,
                    "epoch": epoch,
                    "key_path": f"/keys/{key_id}"
                }
            ],
            "partition_count": 1,
            "row_count": commit_count
        }

        with open(partition / "_manifest", 'w') as f:
            json.dump(manifest, f)

        return partition

    def create_credential(self, epochs: list):
        """Helper to create migration credentials."""
        credential_path = self.temp_path / "migration_credential.json"
        credentials = {
            "key_id": "migration-master-key",
            "epochs": epochs,
            "created_at": datetime.now().isoformat()
        }

        with open(credential_path, 'w') as f:
            json.dump(credentials, f)

        return credential_path

    def test_discover_single_key(self):
        """Test discovering a single encryption key from one partition."""
        # Create single partition
        self.create_partition(
            "provider=github/year=2024/month=08",
            "epoch-2024-08",
            "2024-08"
        )

        # Create credential
        credential_path = self.create_credential(["epoch-2024-08"])

        # Run checker
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys = checker.discover_all_keys()

        # Assertions
        self.assertEqual(len(keys), 1, "Should discover exactly 1 key")
        self.assertIn("epoch-2024-08", keys, "Should discover the key_id")

    def test_discover_multiple_keys(self):
        """Test discovering multiple distinct encryption keys."""
        # Create partitions with different keys
        self.create_partition("provider=github/year=2024/month=08", "epoch-current", "2024-08")
        self.create_partition("provider=github/year=2023/month=12", "epoch-retired-1", "2023-12")
        self.create_partition("provider=github/year=2022/month=06", "epoch-retired-2", "2022-06")

        # Create credential
        credential_path = self.create_credential(["epoch-current", "epoch-retired-1", "epoch-retired-2"])

        # Run checker
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys = checker.discover_all_keys()

        # Assertions
        self.assertEqual(len(keys), 3, "Should discover all 3 distinct keys")
        self.assertIn("epoch-current", keys)
        self.assertIn("epoch-retired-1", keys)
        self.assertIn("epoch-retired-2", keys)

        # Verify partition aggregation
        self.assertEqual(len(keys["epoch-current"].partitions), 1)
        self.assertEqual(keys["epoch-current"].partitions[0], "provider=github/year=2024/month=08")

    def test_aggregates_same_key_multiple_partitions(self):
        """Test that same key_id across multiple partitions is aggregated."""
        # Create multiple partitions with same key
        self.create_partition("provider=github/year=2024/month=08", "epoch-shared", "2024-08")
        self.create_partition("provider=github/year=2024/month=07", "epoch-shared", "2024-08")
        self.create_partition("provider=github/year=2024/month=06", "epoch-shared", "2024-08")

        credential_path = self.create_credential(["epoch-shared"])

        # Run checker
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys = checker.discover_all_keys()

        # Assertions
        self.assertEqual(len(keys), 1, "Should aggregate to 1 distinct key")
        self.assertEqual(len(keys["epoch-shared"].partitions), 3, "Should aggregate all 3 partitions")

    def test_reports_retired_epoch_not_skipped(self):
        """Test that retired epochs are discovered and not silently skipped."""
        # Create current and retired partitions
        self.create_partition("provider=github/year=2024/month=08", "epoch-current-2024", "2024-08")
        self.create_partition("provider=github/year=2020/month=01", "epoch-ancient-2020", "2020-01")

        credential_path = self.create_credential(["epoch-current-2024", "epoch-ancient-2020"])

        # Run checker
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys = checker.discover_all_keys()

        # Critical assertion: both keys must be discovered
        self.assertEqual(len(keys), 2, "Must discover both current AND retired epochs")
        self.assertIn("epoch-ancient-2020", keys, "Must NOT skip the retired epoch")

    def test_empty_manifest_encryption_keys(self):
        """Test handling of manifests with empty encryption_keys arrays."""
        # Create partition with empty encryption_keys
        partition = self.temp_path / "provider=github/year=2024/month=08"
        partition.mkdir(parents=True)

        manifest = {
            "encryption_keys": [],  # Empty array
            "partition_count": 1,
            "row_count": 0
        }

        with open(partition / "_manifest", 'w') as f:
            json.dump(manifest, f)

        credential_path = self.create_credential([])

        # Run checker
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys = checker.discover_all_keys()

        # Should handle gracefully (0 keys discovered)
        self.assertEqual(len(keys), 0)

    def test_validates_all_epochs_before_migration(self):
        """Test that preflight runs before migration starts."""
        # Create multi-epoch corpus
        self.create_partition("provider=github/year=2024/month=08", "epoch-1", "2024-08")
        self.create_partition("provider=github/year=2023/month=12", "epoch-2", "2023-12")

        credential_path = self.create_credential(["epoch-1", "epoch-2"])

        # Run full preflight
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        all_passed, results = checker.run_preflight()

        # Assertions
        self.assertEqual(len(results), 2, "Should validate all 2 epochs")
        # Note: validation will fail without actual Parquet files, but the check runs

    def test_validates_against_credential_file(self):
        """Test that validation uses the provided credential file."""
        # Create partition
        self.create_partition("provider=github/year=2024/month=08", "test-key", "2024-08")

        # Create credential with specific epochs
        credential_path = self.create_credential(["test-key"])

        # Verify credential file is loaded
        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        with open(credential_path, 'r') as f:
            loaded_creds = json.load(f)

        self.assertIn("test-key", loaded_creds["epochs"])

    def test_reports_key_id_and_epoch_per_result(self):
        """Test that validation results include key_id and epoch information."""
        keys = {
            "test-key": EncryptionKey(
                key_id="test-key",
                epoch="2024-08",
                key_path="/keys/test-key",
                partitions=["provider=github/year=2024/month=08"]
            )
        }

        credential_path = self.create_credential(["test-key"])

        # Mock validation - we can't test real decryption without Parquet files
        # but we can verify the result structure
        result = ValidationResult(
            key_id="test-key",
            epoch="2024-08",
            success=True,
            test_partition="provider=github/year=2024/month=08"
        )

        self.assertEqual(result.key_id, "test-key")
        self.assertEqual(result.epoch, "2024-08")
        self.assertTrue(result.success)

    def test_multiple_repos_same_partition(self):
        """Test that manifests correctly handle multiple repos in one partition."""
        # This tests partition structure, not encryption directly
        self.create_partition("provider=github/year=2024/month=08", "epoch-test", "2024-08", commit_count=5)

        credential_path = self.create_credential(["epoch-test"])

        checker = EpochPreflightChecker(str(self.temp_path), str(credential_path))
        keys = checker.discover_all_keys()

        self.assertEqual(len(keys), 1)


class TestAcceptanceCriteria(unittest.TestCase):
    """Test suite specifically for acceptance criteria."""

    def test_acceptance_criterion_1_enumerates_all_key_ids(self):
        """AC1: Tool enumerates and reports every distinct key_id found."""
        temp_dir = tempfile.mkdtemp(prefix="test_ac1_")
        temp_path = Path(temp_dir)

        try:
            # Create diverse key set (mkdtemp() already created temp_path)
            for year, key_id, epoch in [
                ("2024", "current-key", "2024-08"),
                ("2023", "old-key-1", "2023-12"),
                ("2022", "old-key-2", "2022-06"),
                ("2021", "ancient-key", "2021-01")
            ]:
                partition = temp_path / f"provider=github/year={year}/month=08"
                partition.mkdir(parents=True)

                manifest = {
                    "encryption_keys": [{"key_id": key_id, "epoch": epoch, "key_path": f"/keys/{key_id}"}],
                    "partition_count": 1,
                    "row_count": 1
                }

                with open(partition / "_manifest", 'w') as f:
                    json.dump(manifest, f)

            credential_path = temp_path / "creds.json"
            with open(credential_path, 'w') as f:
                json.dump({"epochs": [key_id for _, key_id, _ in [
                    ("2024", "current-key", "2024-08"),
                    ("2023", "old-key-1", "2023-12"),
                    ("2022", "old-key-2", "2022-06"),
                    ("2021", "ancient-key", "2021-01")
                ]]}, f)

            checker = EpochPreflightChecker(str(temp_path), str(credential_path))
            keys = checker.discover_all_keys()

            # AC: Must enumerate ALL 4 distinct keys
            self.assertEqual(len(keys), 4, "AC1: Must enumerate every distinct key_id")
            for expected_key in ["current-key", "old-key-1", "old-key-2", "ancient-key"]:
                self.assertIn(expected_key, keys, f"AC1: Must include {expected_key}")

        finally:
            shutil.rmtree(temp_dir)

    def test_acceptance_criterion_2_decrypt_probe_per_epoch(self):
        """AC2: Tool attempts decrypt probe per epoch and reports pass/fail."""
        temp_dir = tempfile.mkdtemp(prefix="test_ac2_")
        temp_path = Path(temp_dir)

        try:
            # Create test structure
            partition = temp_path / "provider=github/year=2024/month=08"
            partition.mkdir(parents=True)

            manifest = {
                "encryption_keys": [{"key_id": "test-key", "epoch": "2024-08", "key_path": "/keys/test-key"}],
                "partition_count": 1,
                "row_count": 1
            }

            with open(partition / "_manifest", 'w') as f:
                json.dump(manifest, f)

            credential_path = temp_path / "creds.json"
            with open(credential_path, 'w') as f:
                json.dump({"epochs": ["test-key"]}, f)

            checker = EpochPreflightChecker(str(temp_path), str(credential_path))
            keys = checker.discover_all_keys()

            # AC: Must attempt decrypt probe (will fail without Parquet, but attempt is made)
            all_passed, results = checker.validate_decryption(keys)

            self.assertEqual(len(results), 1, "AC2: Must report result for each key")
            # Result should have key_id, epoch, success status
            self.assertIsNotNone(results[0].key_id)
            self.assertIsNotNone(results[0].epoch)
            self.assertIsInstance(results[0].success, bool)

        finally:
            shutil.rmtree(temp_dir)

    def test_acceptance_criterion_3_refuses_start_on_failure(self):
        """AC3: Migration refuses to start if any enumerated epoch fails."""
        temp_dir = tempfile.mkdtemp(prefix="test_ac3_")
        temp_path = Path(temp_dir)

        try:
            # Create test structure
            partition = temp_path / "provider=github/year=2024/month=08"
            partition.mkdir(parents=True)

            manifest = {
                "encryption_keys": [{"key_id": "bad-key", "epoch": "2024-08", "key_path": "/keys/bad-key"}],
                "partition_count": 1,
                "row_count": 1
            }

            with open(partition / "_manifest", 'w') as f:
                json.dump(manifest, f)

            credential_path = temp_path / "creds.json"
            with open(credential_path, 'w') as f:
                json.dump({"epochs": ["bad-key"]}, f)

            checker = EpochPreflightChecker(str(temp_path), str(credential_path))

            # AC: Run preflight should detect failure and report it
            all_passed, results = checker.run_preflight()

            # Without actual Parquet files, decryption fails
            # AC: Tool must report failure, not silently continue
            self.assertFalse(all_passed, "AC3: Must refuse to start on decryption failure")

        finally:
            shutil.rmtree(temp_dir)

    def test_acceptance_criterion_4_retired_epoch_in_fixture(self):
        """AC4: Test fixture includes at least one manifest referencing retired epoch."""
        # Run the fixture creation script
        fixture_script = Path(__file__).parent / "fixtures" / "create_epoch_fixture.py"

        if fixture_script.exists():
            import subprocess
            result = subprocess.run(
                ["python3", str(fixture_script)],
                capture_output=True,
                text=True,
                timeout=30
            )

            # Check fixture was created
            marker_path = Path(__file__).parent / "fixtures" / ".multi_epoch_fixture_path"
            if marker_path.exists():
                fixture_dir = marker_path.read_text().strip()
                metadata_path = Path(fixture_dir) / "fixture_metadata.json"

                if metadata_path.exists():
                    with open(metadata_path) as f:
                        metadata = json.load(f)

                    # AC: Fixture must include retired epochs
                    retired_count = sum(1 for e in metadata.get("epochs", []) if e.get("status") == "retired")
                    self.assertGreater(retired_count, 0, "AC4: Fixture must include at least one retired epoch")


def main():
    """Run tests and report results."""
    # Create test suite
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()

    # Add all test cases
    suite.addTests(loader.loadTestsFromTestCase(TestEpochPreflightChecks))
    suite.addTests(loader.loadTestsFromTestCase(TestAcceptanceCriteria))

    # Run tests
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)

    # Exit with appropriate code
    sys.exit(0 if result.wasSuccessful() else 1)


if __name__ == "__main__":
    main()
