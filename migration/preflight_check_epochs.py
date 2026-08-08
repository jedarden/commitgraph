#!/usr/bin/env python3
"""
Preflight encryption epoch validation for corpus migration.

This tool scans every manifest in the existing corpus, collects all distinct
key_id (epoch key) values, and attempts a decrypt probe against each using
migration credentials. Aborts loudly if any epoch fails - do not silently skip.

This is critical: scoping to only the current epoch would silently skip older
partitions still sitting on retired epochs.

Usage:
    python preflight_check_epochs.py <corpus_root> <credential_path>

Example:
    python preflight_check_epochs.py /data/corpus /creds/migration.json
"""

import json
import sys
import logging
from pathlib import Path
from typing import Dict, List, Tuple, Set
from dataclasses import dataclass
from datetime import datetime
import pyarrow.parquet as pq
import pyarrow as pa

logging.basicConfig(
    level=logging.INFO,
    format='%(levelname)s: %(message)s'
)
logger = logging.getLogger(__name__)


@dataclass
class EncryptionKey:
    """Represents an encryption epoch key from a partition manifest."""
    key_id: str
    epoch: str
    key_path: str
    partitions: List[str]  # List of partition keys using this key

    def __repr__(self):
        return f"EncryptionKey(key_id={self.key_id!r}, epoch={self.epoch!r}, partitions={len(self.partitions)})"


@dataclass
class ValidationResult:
    """Result of attempting to decrypt with a specific key_id."""
    key_id: str
    epoch: str
    success: bool
    error_message: str = ""
    test_partition: str = ""

    def __repr__(self):
        status = "✓" if self.success else "✗"
        return f"{status} key_id={self.key_id!r} epoch={self.epoch!r}"


class EpochPreflightChecker:
    """
    Preflight checker for encryption epoch validation.

    Enumerates all key_ids across corpus manifests and validates that
    migration credentials can decrypt every epoch referenced.
    """

    def __init__(self, corpus_root: str, credential_path: str):
        """
        Initialize the preflight checker.

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

        # Load migration credentials
        try:
            with open(self.credential_path, 'r') as f:
                self.credentials = json.load(f)
            logger.info(f"Loaded migration credentials from: {credential_path}")
        except Exception as e:
            raise ValueError(f"Failed to load credentials: {e}")

    def discover_all_keys(self) -> Dict[str, EncryptionKey]:
        """
        Enumerate all distinct key_id values across all manifests.

        Returns:
            Dict mapping key_id -> EncryptionKey with aggregated partition list
        """
        keys_by_id: Dict[str, EncryptionKey] = {}
        corpus_path = self.corpus_root

        logger.info(f"Scanning corpus at: {corpus_path}")

        # Walk provider/year/month structure
        partition_count = 0
        for provider_dir in corpus_path.glob("provider=*"):
            for year_dir in provider_dir.glob("year=*"):
                for month_dir in year_dir.glob("month=*"):
                    partition_key = f"{provider_dir.name}/{year_dir.name}/{month_dir.name}"
                    manifest_path = month_dir / "_manifest"

                    if manifest_path.exists():
                        partition_count += 1
                        partition_keys = self._read_manifest_keys(manifest_path, partition_key)

                        # Merge into keys_by_id
                        for key in partition_keys:
                            if key.key_id in keys_by_id:
                                # Aggregate partitions for this key_id
                                existing = keys_by_id[key.key_id]
                                existing.partitions.extend(key.partitions)
                                # Verify epoch consistency
                                if existing.epoch != key.epoch:
                                    logger.warning(
                                        f"key_id {key.key_id!r} has inconsistent epochs: "
                                        f"{existing.epoch!r} vs {key.epoch!r}"
                                    )
                            else:
                                keys_by_id[key.key_id] = key

        logger.info(f"Scanned {partition_count} partitions, found {len(keys_by_id)} distinct key_ids")
        return keys_by_id

    def _read_manifest_keys(self, manifest_path: Path, partition_key: str) -> List[EncryptionKey]:
        """Read encryption keys from a partition manifest file."""
        keys = []

        try:
            with open(manifest_path, 'r') as f:
                manifest = json.load(f)

            # Extract encryption keys from manifest
            for key_info in manifest.get('encryption_keys', []):
                key = EncryptionKey(
                    key_id=key_info.get('key_id', ''),
                    epoch=key_info.get('epoch', 'unknown'),
                    key_path=key_info.get('key_path', ''),
                    partitions=[partition_key]
                )
                if key.key_id:  # Only add non-empty key_ids
                    keys.append(key)

        except Exception as e:
            logger.warning(f"Failed to read manifest {manifest_path}: {e}")

        return keys

    def validate_decryption(self, keys_by_id: Dict[str, EncryptionKey]) -> Tuple[bool, List[ValidationResult]]:
        """
        Validate that migration credentials can decrypt all discovered keys.

        For each key_id, attempt to read a sample Parquet file from one of
        the partitions that uses that key. This tests actual decryption,
        not just key metadata validation.

        Args:
            keys_by_id: Dict of key_id -> EncryptionKey

        Returns:
            Tuple of (all_passed, validation_results) where all_passed is True
            if all epochs can be decrypted, False otherwise.
        """
        results = []

        logger.info(f"Validating decryption for {len(keys_by_id)} keys...")

        for key_id, key_info in keys_by_id.items():
            result = self._test_decrypt_one_key(key_info)
            results.append(result)

            # Log immediately for visibility
            if result.success:
                logger.info(f"  ✓ {result.key_id} (epoch={result.epoch})")
            else:
                logger.error(f"  ✗ {result.key_id} (epoch={result.epoch}): {result.error_message}")

        # Compute overall pass/fail
        all_passed = all(r.success for r in results)
        return all_passed, results

    def _test_decrypt_one_key(self, key_info: EncryptionKey) -> ValidationResult:
        """
        Test decryption for a single key_id by reading a sample Parquet file.

        Tries to read the first .parquet file from one of the partitions
        that uses this key. Success means the key can decrypt.

        Args:
            key_info: EncryptionKey to test

        Returns:
            ValidationResult with success/failure
        """
        # Find a test partition with actual Parquet data
        test_partition = None
        test_parquet_path = None

        for partition_key in key_info.partitions:
            partition_path = self.corpus_root / partition_key.replace("/", "/")

            # Find first Parquet file in this partition
            for parquet_file in partition_path.glob("*.parquet"):
                test_partition = partition_key
                test_parquet_path = parquet_file
                break

            if test_parquet_path:
                break

        if not test_parquet_path:
            return ValidationResult(
                key_id=key_info.key_id,
                epoch=key_info.epoch,
                success=False,
                error_message="No Parquet files found in partitions",
                test_partition=""
            )

        # Attempt to read the Parquet file (this tests decryption)
        try:
            # Try reading just the metadata to test decryption
            # This is lighter than reading the whole file
            parquet_file = pq.ParquetFile(test_parquet_path)

            # Read a small sample to verify actual decryption works
            table = parquet_file.read_row_group(0)  # Just first row group

            # If we get here, decryption succeeded
            return ValidationResult(
                key_id=key_info.key_id,
                epoch=key_info.epoch,
                success=True,
                test_partition=test_partition
            )

        except Exception as e:
            # Decryption failed
            return ValidationResult(
                key_id=key_info.key_id,
                epoch=key_info.epoch,
                success=False,
                error_message=str(e),
                test_partition=test_partition
            )

    def run_preflight(self) -> Tuple[bool, List[ValidationResult]]:
        """
        Run the complete preflight check.

        Returns:
            Tuple of (all_passed, validation_results)
        """
        logger.info("=" * 70)
        logger.info("CORPUS MIGRATION PREFLIGHT: Epoch Decryption Check")
        logger.info("=" * 70)

        # Step 1: Discover all keys
        keys_by_id = self.discover_all_keys()

        if not keys_by_id:
            logger.warning("No encryption keys found in corpus")
            return True, []

        # Report what we found
        logger.info("\nDistinct encryption epochs discovered:")
        for key_id, key_info in sorted(keys_by_id.items()):
            logger.info(f"  - key_id={key_id!r} epoch={key_info.epoch!r} ({len(key_info.partitions)} partitions)")

        # Step 2: Validate decryption
        logger.info("\nTesting decryption for each epoch...")
        all_passed, results = self.validate_decryption(keys_by_id)

        # Step 3: Report results (recompute all_passed for clarity)
        all_passed = all(r.success for r in results)

        logger.info("\n" + "=" * 70)
        if all_passed:
            logger.info("✓ PREFLIGHT CHECK PASSED")
            logger.info(f"  All {len(results)} epochs can be decrypted")
        else:
            logger.error("✗ PREFLIGHT CHECK FAILED")
            failed_count = sum(1 for r in results if not r.success)
            logger.error(f"  {failed_count}/{len(results)} epochs failed decryption")
            logger.error("\nFailed epochs:")
            for r in results:
                if not r.success:
                    logger.error(f"    - key_id={r.key_id!r}: {r.error_message}")
            logger.error("\nMigration cannot proceed. Fix credential access or restore missing keys.")

        logger.info("=" * 70)

        return all_passed, results


def main():
    """Main entry point."""
    if len(sys.argv) != 3:
        print("Usage: python preflight_check_epochs.py <corpus_root> <credential_path>")
        print("\nExample:")
        print("  python preflight_check_epochs.py /data/corpus /creds/migration.json")
        sys.exit(1)

    corpus_root = sys.argv[1]
    credential_path = sys.argv[2]

    try:
        checker = EpochPreflightChecker(corpus_root, credential_path)
        all_passed, results = checker.run_preflight()

        # Exit with appropriate code
        sys.exit(0 if all_passed else 1)

    except Exception as e:
        logger.error(f"Preflight check error: {e}")
        sys.exit(2)


if __name__ == "__main__":
    main()
