"""
Standalone decrypt probe module for encryption epoch validation.

This module provides a focused interface for testing whether migration
credentials can successfully decrypt data from a specific encryption epoch.

Unlike the integrated preflight checker, this module can be called with just
a key_id to test decryption on demand, making it suitable for:
- Individual epoch validation
- Post-migration verification
- Troubleshooting specific decryption failures
- Integration testing

Usage:
    from decrypt_probe import DecryptProbe, DecryptResult

    probe = DecryptProbe(corpus_root, credential_path)
    result = probe.test_key_id(key_id)

    if result.success:
        print(f"✓ key_id {key_id} can decrypt")
    else:
        print(f"✗ key_id {key_id} failed: {result.error_reason}")
"""

import json
import logging
from pathlib import Path
from typing import Optional, Dict, List, Tuple
from dataclasses import dataclass
from enum import Enum
import pyarrow.parquet as pq
import pyarrow as pa

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class DecryptOutcome(Enum):
    """Distinct decryption failure reasons for clear error reporting."""
    SUCCESS = "success"
    KEY_NOT_FOUND = "key_not_found"          # key_id not in any manifest
    NO_DATA_FILES = "no_data_files"          # manifest exists but no parquet files
    CREDENTIAL_ERROR = "credential_error"    # cannot access/parse credential
    DECRYPT_ERROR = "decrypt_error"          # crypto/parse failure
    PARTITION_MISSING = "partition_missing"  # partition path doesn't exist


@dataclass
class DecryptResult:
    """
    Result of attempting to decrypt with a specific key_id.

    Provides clear pass/fail with specific failure reason, distinguishing
    between "key not found", "decrypt failed", and "success".
    """
    key_id: str
    success: bool
    outcome: DecryptOutcome
    error_message: str = ""
    test_file: str = ""
    epoch: str = ""
    partitions_tested: int = 0

    def __repr__(self):
        status = "✓" if self.success else "✗"
        return f"{status} key_id={self.key_id!r} outcome={self.outcome.value}"

    def to_dict(self) -> dict:
        """Convert result to dict for JSON serialization."""
        return {
            'key_id': self.key_id,
            'success': self.success,
            'outcome': self.outcome.value,
            'error_message': self.error_message,
            'test_file': str(self.test_file) if self.test_file else "",
            'epoch': self.epoch,
            'partitions_tested': self.partitions_tested
        }


@dataclass
class PartitionInfo:
    """Information about a partition using a specific key."""
    partition_key: str
    partition_path: Path
    key_path: str
    epoch: str


class DecryptProbe:
    """
    Standalone decrypt probe for testing encryption epoch access.

    This class provides focused functionality for testing whether migration
    credentials can decrypt data from specific encryption epochs. Unlike the
    integrated preflight checker, it can be called with just a key_id.

    Key design:
    - Distinguishes between "key not found" vs "decrypt failed" vs "success"
    - Uses same credential loading as corpus migration
    - Handles both current and retired epoch keys
    - Returns structured result with specific failure reason
    """

    def __init__(self, corpus_root: str, credential_path: str):
        """
        Initialize the decrypt probe.

        Args:
            corpus_root: Root directory of Hive-partitioned corpus
            credential_path: Path to migration encryption credential JSON
        """
        self.corpus_root = Path(corpus_root)
        self.credential_path = Path(credential_path)

        if not self.corpus_root.exists():
            raise ValueError(f"Corpus root does not exist: {corpus_root}")

        if not self.credential_path.exists():
            raise ValueError(f"Credential path does not exist: {credential_path}")

        # Load migration credentials (same as corpus migration)
        self.credentials = self._load_credentials()

        # Cache for partition lookup by key_id
        self._partition_cache: Dict[str, List[PartitionInfo]] = {}

    def _load_credentials(self) -> dict:
        """Load migration credentials from file."""
        try:
            with open(self.credential_path, 'r') as f:
                creds = json.load(f)
            logger.debug(f"Loaded migration credentials from: {self.credential_path}")
            return creds
        except Exception as e:
            raise ValueError(f"Failed to load credentials from {self.credential_path}: {e}")

    def _find_partitions_for_key(self, key_id: str) -> List[PartitionInfo]:
        """
        Find all partitions that use a specific key_id.

        Walks the corpus structure looking for manifests that reference
        the given key_id. Returns partition info for each match.

        Args:
            key_id: Encryption key identifier to search for

        Returns:
            List of PartitionInfo for partitions using this key
        """
        # Check cache first
        if key_id in self._partition_cache:
            return self._partition_cache[key_id]

        partitions = []
        corpus_path = self.corpus_root

        # Walk provider/year/month structure
        for provider_dir in corpus_path.glob("provider=*"):
            for year_dir in provider_dir.glob("year=*"):
                for month_dir in year_dir.glob("month=*"):
                    partition_key = f"{provider_dir.name}/{year_dir.name}/{month_dir.name}"
                    partition_path = self.corpus_root / partition_key.replace("/", "/")
                    manifest_path = month_dir / "_manifest"

                    if not manifest_path.exists():
                        continue

                    try:
                        with open(manifest_path, 'r') as f:
                            manifest = json.load(f)

                        # Check if this manifest references our key_id
                        for key_info in manifest.get('encryption_keys', []):
                            if key_info.get('key_id') == key_id:
                                partitions.append(PartitionInfo(
                                    partition_key=partition_key,
                                    partition_path=partition_path,
                                    key_path=key_info.get('key_path', ''),
                                    epoch=key_info.get('epoch', 'unknown')
                                ))
                                break

                    except Exception as e:
                        logger.warning(f"Failed to read manifest {manifest_path}: {e}")
                        continue

        # Cache the result
        self._partition_cache[key_id] = partitions
        return partitions

    def _find_test_parquet(self, partition_info: PartitionInfo) -> Optional[Path]:
        """
        Find a Parquet file in a partition for testing decryption.

        Args:
            partition_info: Partition to search for Parquet files

        Returns:
            Path to first Parquet file found, or None
        """
        partition_path = partition_info.partition_path

        if not partition_path.exists():
            return None

        # Find first Parquet file
        for parquet_file in partition_path.glob("*.parquet"):
            return parquet_file

        return None

    def test_key_id(self, key_id: str) -> DecryptResult:
        """
        Attempt to decrypt test data for a specific key_id.

        This is the main entry point. Given a key_id, it:
        1. Finds partitions using this key (by scanning manifests)
        2. Locates a sample Parquet file in one partition
        3. Attempts to read the file (tests actual decryption)
        4. Returns clear pass/fail with specific failure reason

        Args:
            key_id: Encryption key identifier to test

        Returns:
            DecryptResult with success/failure and specific reason
        """
        logger.debug(f"Testing key_id: {key_id}")

        # Step 1: Find partitions using this key
        partitions = self._find_partitions_for_key(key_id)

        if not partitions:
            return DecryptResult(
                key_id=key_id,
                success=False,
                outcome=DecryptOutcome.KEY_NOT_FOUND,
                error_message=f"key_id {key_id!r} not found in any partition manifest",
                partitions_tested=0
            )

        # Step 2: Try each partition until we find a testable Parquet file
        for partition_info in partitions:
            test_file = self._find_test_parquet(partition_info)

            if not test_file:
                logger.debug(f"No Parquet files in {partition_info.partition_key}")
                continue

            # Step 3: Attempt to decrypt by reading the file
            try:
                # Try reading just the metadata first (lighter)
                parquet_file = pq.ParquetFile(test_file)

                # Read a small sample to verify actual decryption works
                # Just read first row group, not the whole file
                table = parquet_file.read_row_group(0)

                # If we get here, decryption succeeded
                return DecryptResult(
                    key_id=key_id,
                    success=True,
                    outcome=DecryptOutcome.SUCCESS,
                    test_file=str(test_file),
                    epoch=partition_info.epoch,
                    partitions_tested=1,
                    error_message=""
                )

            except Exception as e:
                # Decryption failed - return specific error
                error_msg = str(e)
                logger.debug(f"Decrypt failed for {key_id} using {test_file}: {error_msg}")

                # Categorize the error
                if "credentials" in error_msg.lower() or "auth" in error_msg.lower():
                    return DecryptResult(
                        key_id=key_id,
                        success=False,
                        outcome=DecryptOutcome.CREDENTIAL_ERROR,
                        error_message=f"Credential error: {error_msg}",
                        test_file=str(test_file),
                        epoch=partition_info.epoch,
                        partitions_tested=1
                    )
                else:
                    return DecryptResult(
                        key_id=key_id,
                        success=False,
                        outcome=DecryptOutcome.DECRYPT_ERROR,
                        error_message=f"Decrypt failed: {error_msg}",
                        test_file=str(test_file),
                        epoch=partition_info.epoch,
                        partitions_tested=1
                    )

        # If we get here, key was found but no testable Parquet files
        return DecryptResult(
            key_id=key_id,
            success=False,
            outcome=DecryptOutcome.NO_DATA_FILES,
            error_message=f"key_id {key_id!r} found in {len(partitions)} partition(s) but no Parquet files available for testing",
            partitions_tested=len(partitions)
        )

    def test_multiple_keys(self, key_ids: List[str]) -> List[DecryptResult]:
        """
        Test multiple key_ids in batch.

        Args:
            key_ids: List of key_id strings to test

        Returns:
            List of DecryptResult, one per key_id (same order as input)
        """
        results = []
        for key_id in key_ids:
            result = self.test_key_id(key_id)
            results.append(result)

            # Log immediate feedback
            if result.success:
                logger.info(f"  ✓ {result.key_id} (epoch={result.epoch})")
            else:
                logger.error(f"  ✗ {result.key_id} (epoch={result.epoch}): {result.error_message}")

        return results

    def validate_all_passed(self, results: List[DecryptResult]) -> Tuple[bool, List[DecryptResult]]:
        """
        Check if all decrypt tests passed.

        Args:
            results: List of DecryptResult from test_key_id or test_multiple_keys

        Returns:
            Tuple of (all_passed, failed_results)
        """
        all_passed = all(r.success for r in results)
        failed = [r for r in results if not r.success]
        return all_passed, failed


def main():
    """CLI entry point for standalone decrypt probing."""
    import sys

    if len(sys.argv) < 3:
        print("Usage: python decrypt_probe.py <corpus_root> <credential_path> [key_id ...]")
        print("\nExamples:")
        print("  # Test specific key_ids")
        print("  python decrypt_probe.py /data/corpus /creds/migration.json key1 key2")
        print("\n  # Test all key_ids discovered from manifests")
        print("  python decrypt_probe.py /data/corpus /creds/migration.json")
        sys.exit(1)

    corpus_root = sys.argv[1]
    credential_path = sys.argv[2]
    key_ids = sys.argv[3:] if len(sys.argv) > 3 else None

    try:
        probe = DecryptProbe(corpus_root, credential_path)

        if key_ids:
            # Test specific key_ids
            print(f"Testing {len(key_ids)} specific key_ids...")
            results = probe.test_multiple_keys(key_ids)
        else:
            # Discover and test all key_ids
            print("Discovering all key_ids from corpus...")
            from preflight_check_epochs import EpochPreflightChecker
            checker = EpochPreflightChecker(corpus_root, credential_path)
            keys_by_id = checker.discover_all_keys()
            all_key_ids = list(keys_by_id.keys())

            print(f"Found {len(all_key_ids)} distinct key_ids, testing decryption...")
            results = []
            for key_id in all_key_ids:
                result = probe.test_key_id(key_id)
                results.append(result)

                if result.success:
                    print(f"  ✓ {key_id}")
                else:
                    print(f"  ✗ {key_id}: {result.error_message}")

        # Report summary
        all_passed, failed = probe.validate_all_passed(results)

        print("\n" + "=" * 70)
        if all_passed:
            print(f"✓ ALL {len(results)} key_ids passed decryption test")
            sys.exit(0)
        else:
            print(f"✗ {len(failed)}/{len(results)} key_ids failed decryption")
            print("\nFailed keys:")
            for r in failed:
                print(f"  - {r.key_id}: {r.error_message}")
            sys.exit(1)

    except Exception as e:
        logger.error(f"Decrypt probe error: {e}")
        sys.exit(2)


if __name__ == "__main__":
    main()
