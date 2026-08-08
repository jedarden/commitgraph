#!/usr/bin/env python3
"""
Enumerate distinct key_id values across all corpus manifests.

This module provides a focused interface for scanning all manifests in the
corpus and extracting the complete set of distinct key_id values referenced.

Unlike the integrated preflight checker, this module only performs enumeration
without any decryption testing, making it suitable for:
- Listing all encryption epochs in the corpus
- Pre-flight scanning before migration
- Audit and verification workflows
- Feeding key_id lists to other tools

Usage:
    from enumerate_key_ids import enumerate_key_ids

    result = enumerate_key_ids("/data/corpus")
    print(f"Scanned {result.manifests_scanned} manifests")
    print(f"Found {result.unique_key_ids_count} distinct key_ids")
    for key_id in sorted(result.key_ids):
        print(f"  - {key_id}")
"""

import json
import logging
from pathlib import Path
from typing import Set, List
from dataclasses import dataclass

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


@dataclass
class EnumerationResult:
    """
    Result of enumerating key_ids across corpus manifests.

    Provides clear counts and the deduplicated set of key_ids for
    downstream consumption by validation or migration tools.
    """
    manifests_scanned: int
    manifests_missing: int
    manifests_failed: int
    key_ids: Set[str]
    unique_key_ids_count: int

    def __repr__(self):
        return (f"EnumerationResult(manifests_scanned={self.manifests_scanned}, "
                f"manifests_missing={self.manifests_missing}, "
                f"manifests_failed={self.manifests_failed}, "
                f"unique_key_ids_count={self.unique_key_ids_count})")

    def to_dict(self) -> dict:
        """Convert result to dict for JSON serialization."""
        return {
            'manifests_scanned': self.manifests_scanned,
            'manifests_missing': self.manifests_missing,
            'manifests_failed': self.manifests_failed,
            'unique_key_ids_count': self.unique_key_ids_count,
            'key_ids': sorted(list(self.key_ids))
        }


def enumerate_key_ids(corpus_root: str) -> EnumerationResult:
    """
    Scan all manifests in the corpus and extract distinct key_id values.

    This function walks the entire corpus directory structure, reads every
    partition manifest, and extracts all key_id values from the encryption_keys
    metadata. Returns a deduplicated set with counts for reporting.

    Args:
        corpus_root: Root directory of Hive-partitioned corpus

    Returns:
        EnumerationResult with manifest count, key_id set, and unique count

    Raises:
        ValueError: If corpus_root does not exist
    """
    corpus_path = Path(corpus_root)

    if not corpus_path.exists():
        raise ValueError(f"Corpus root does not exist: {corpus_root}")

    logger.info(f"Scanning corpus at: {corpus_path}")

    key_ids: Set[str] = set()
    manifests_scanned = 0
    manifests_missing = 0
    manifests_failed = 0

    # Walk provider/year/month structure
    for provider_dir in corpus_path.glob("provider=*"):
        for year_dir in provider_dir.glob("year=*"):
            for month_dir in year_dir.glob("month=*"):
                partition_key = f"{provider_dir.name}/{year_dir.name}/{month_dir.name}"
                manifest_path = month_dir / "_manifest"

                if not manifest_path.exists():
                    manifests_missing += 1
                    logger.debug(f"Manifest missing for partition: {partition_key}")
                    continue

                try:
                    with open(manifest_path, 'r') as f:
                        manifest = json.load(f)

                    manifests_scanned += 1

                    # Extract key_ids from encryption_keys array
                    for key_info in manifest.get('encryption_keys', []):
                        key_id = key_info.get('key_id', '')
                        if key_id:  # Only add non-empty key_ids
                            key_ids.add(key_id)

                except json.JSONDecodeError as e:
                    manifests_failed += 1
                    logger.warning(f"Invalid JSON in manifest {manifest_path}: {e}")
                except Exception as e:
                    manifests_failed += 1
                    logger.warning(f"Failed to read manifest {manifest_path}: {e}")

    # Log summary
    logger.info(f"Scanned {manifests_scanned} manifests successfully")
    if manifests_missing > 0:
        logger.warning(f"Found {manifests_missing} partitions without manifests")
    if manifests_failed > 0:
        logger.warning(f"Failed to parse {manifests_failed} manifests")
    logger.info(f"Discovered {len(key_ids)} distinct key_ids")

    return EnumerationResult(
        manifests_scanned=manifests_scanned,
        manifests_missing=manifests_missing,
        manifests_failed=manifests_failed,
        key_ids=key_ids,
        unique_key_ids_count=len(key_ids)
    )


def main():
    """CLI entry point for key_id enumeration."""
    import sys

    if len(sys.argv) < 2:
        print("Usage: python enumerate_key_ids.py <corpus_root>")
        print("\nExamples:")
        print("  python enumerate_key_ids.py /data/corpus")
        print("\nOutput:")
        print("  JSON with manifest count and sorted list of unique key_ids")
        sys.exit(1)

    corpus_root = sys.argv[1]

    try:
        result = enumerate_key_ids(corpus_root)

        # Output as JSON for downstream consumption
        import json
        print(json.dumps(result.to_dict(), indent=2))

    except Exception as e:
        logger.error(f"Enumeration failed: {e}")
        sys.exit(2)


if __name__ == "__main__":
    main()
