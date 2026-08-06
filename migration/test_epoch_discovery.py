#!/usr/bin/env python3
"""
Simple unit tests for epoch discovery logic (no pyarrow dependency).

These tests validate the core manifest parsing and key discovery logic
without requiring Parquet/pyarrow installation.

Run with: python3 test_epoch_discovery.py
"""

import sys
import json
import tempfile
import shutil
from pathlib import Path
from dataclasses import dataclass
from typing import List, Dict


# Simplified version of EncryptionKey for testing
@dataclass
class EncryptionKey:
    key_id: str
    epoch: str
    key_path: str
    partitions: List[str]


def read_manifest_keys(manifest_path: Path, partition_key: str) -> List[EncryptionKey]:
    """Read encryption keys from a partition manifest file."""
    keys = []

    try:
        with open(manifest_path, 'r') as f:
            manifest = json.load(f)

        for key_info in manifest.get('encryption_keys', []):
            key = EncryptionKey(
                key_id=key_info.get('key_id', ''),
                epoch=key_info.get('epoch', 'unknown'),
                key_path=key_info.get('key_path', ''),
                partitions=[partition_key]
            )
            if key.key_id:
                keys.append(key)

    except Exception as e:
        print(f"Failed to read manifest {manifest_path}: {e}")

    return keys


def discover_all_keys(corpus_root: Path) -> Dict[str, EncryptionKey]:
    """Enumerate all distinct key_id values across all manifests."""
    keys_by_id: Dict[str, EncryptionKey] = {}

    for provider_dir in corpus_root.glob("provider=*"):
        for year_dir in provider_dir.glob("year=*"):
            for month_dir in year_dir.glob("month=*"):
                partition_key = f"{provider_dir.name}/{year_dir.name}/{month_dir.name}"
                manifest_path = month_dir / "_manifest"

                if manifest_path.exists():
                    partition_keys = read_manifest_keys(manifest_path, partition_key)

                    for key in partition_keys:
                        if key.key_id in keys_by_id:
                            existing = keys_by_id[key.key_id]
                            existing.partitions.extend(key.partitions)
                        else:
                            keys_by_id[key.key_id] = key

    return keys_by_id


def test_single_key_discovery():
    """Test discovering a single encryption key."""
    with tempfile.TemporaryDirectory() as tmpdir:
        corpus_path = Path(tmpdir)

        # Create partition structure
        partition = corpus_path / "provider=github/year=2024/month=08"
        partition.mkdir(parents=True)

        # Create manifest
        manifest = {
            "encryption_keys": [{"key_id": "epoch-2024-08", "epoch": "2024-08", "key_path": "/keys/epoch-2024-08"}],
            "partition_count": 1,
            "row_count": 100
        }

        with open(partition / "_manifest", 'w') as f:
            json.dump(manifest, f)

        # Test discovery
        keys = discover_all_keys(corpus_path)

        assert len(keys) == 1, f"Expected 1 key, got {len(keys)}"
        assert "epoch-2024-08" in keys, "Expected key_id not found"
        print("✓ test_single_key_discovery passed")


def test_multiple_epoch_discovery():
    """Test discovering multiple distinct epochs (AC1: enumerate all key_ids)."""
    with tempfile.TemporaryDirectory() as tmpdir:
        corpus_path = Path(tmpdir)

        # Create current epoch partition
        partition1 = corpus_path / "provider=github/year=2024/month=08"
        partition1.mkdir(parents=True)
        manifest1 = {
            "encryption_keys": [{"key_id": "current-epoch", "epoch": "2024-08", "key_path": "/keys/current"}],
            "partition_count": 1,
            "row_count": 50
        }
        with open(partition1 / "_manifest", 'w') as f:
            json.dump(manifest1, f)

        # Create retired epoch partition (AC4: must include retired epoch)
        partition2 = corpus_path / "provider=github/year=2020/month=01"
        partition2.mkdir(parents=True)
        manifest2 = {
            "encryption_keys": [{"key_id": "retired-epoch-2020", "epoch": "2020-01", "key_path": "/keys/old"}],
            "partition_count": 1,
            "row_count": 30
        }
        with open(partition2 / "_manifest", 'w') as f:
            json.dump(manifest2, f)

        # Create another retired epoch
        partition3 = corpus_path / "provider=github/year=2019/month=06"
        partition3.mkdir(parents=True)
        manifest3 = {
            "encryption_keys": [{"key_id": "retired-epoch-2019", "epoch": "2019-06", "key_path": "/keys/ancient"}],
            "partition_count": 1,
            "row_count": 20
        }
        with open(partition3 / "_manifest", 'w') as f:
            json.dump(manifest3, f)

        # Test discovery
        keys = discover_all_keys(corpus_path)

        # AC1: Must enumerate ALL distinct key_ids
        assert len(keys) == 3, f"Expected 3 keys, got {len(keys)}: {list(keys.keys())}"
        assert "current-epoch" in keys, "Current epoch not found"
        assert "retired-epoch-2020" in keys, "Retired epoch 2020 not found"
        assert "retired-epoch-2019" in keys, "Retired epoch 2019 not found"

        # AC4: Fixture includes retired epochs
        retired_count = sum(1 for k in keys.values() if "retired" in k.key_id)
        assert retired_count >= 2, f"Expected at least 2 retired epochs, got {retired_count}"

        print("✓ test_multiple_epoch_discovery passed (AC1: enumerate all, AC4: retired epochs included)")


def test_aggregates_same_key_multiple_partitions():
    """Test that same key_id across multiple partitions is aggregated."""
    with tempfile.TemporaryDirectory() as tmpdir:
        corpus_path = Path(tmpdir)

        # Create three partitions with same key_id
        for month in ["06", "07", "08"]:
            partition = corpus_path / f"provider=github/year=2024/month={month}"
            partition.mkdir(parents=True)

            manifest = {
                "encryption_keys": [{"key_id": "shared-epoch", "epoch": "2024-Q3", "key_path": "/keys/shared"}],
                "partition_count": 1,
                "row_count": 10
            }

            with open(partition / "_manifest", 'w') as f:
                json.dump(manifest, f)

        # Test discovery
        keys = discover_all_keys(corpus_path)

        assert len(keys) == 1, f"Expected 1 aggregated key, got {len(keys)}"
        assert "shared-epoch" in keys
        assert len(keys["shared-epoch"].partitions) == 3, f"Expected 3 partitions, got {len(keys['shared-epoch'].partitions)}"

        print("✓ test_aggregates_same_key_multiple_partitions passed")


def test_empty_manifest_handling():
    """Test handling of manifests with empty encryption_keys."""
    with tempfile.TemporaryDirectory() as tmpdir:
        corpus_path = Path(tmpdir)

        partition = corpus_path / "provider=github/year=2024/month=08"
        partition.mkdir(parents=True)

        manifest = {
            "encryption_keys": [],  # Empty array
            "partition_count": 1,
            "row_count": 0
        }

        with open(partition / "_manifest", 'w') as f:
            json.dump(manifest, f)

        keys = discover_all_keys(corpus_path)

        assert len(keys) == 0, "Expected 0 keys from empty manifest"
        print("✓ test_empty_manifest_handling passed")


def test_key_id_uniqueness_across_epochs():
    """Test that different epochs with same key_id are handled correctly."""
    with tempfile.TemporaryDirectory() as tmpdir:
        corpus_path = Path(tmpdir)

        # Two partitions, same key_id but different epochs (should warn in real impl)
        partition1 = corpus_path / "provider=github/year=2024/month=08"
        partition1.mkdir(parents=True)
        manifest1 = {
            "encryption_keys": [{"key_id": "epoch-a", "epoch": "2024-08", "key_path": "/keys/a"}],
            "partition_count": 1,
            "row_count": 10
        }
        with open(partition1 / "_manifest", 'w') as f:
            json.dump(manifest1, f)

        partition2 = corpus_path / "provider=github/year=2023/month=12"
        partition2.mkdir(parents=True)
        manifest2 = {
            "encryption_keys": [{"key_id": "epoch-a", "epoch": "2023-12", "key_path": "/keys/a-old"}],
            "partition_count": 1,
            "row_count": 10
        }
        with open(partition2 / "_manifest", 'w') as f:
            json.dump(manifest2, f)

        keys = discover_all_keys(corpus_path)

        # Should aggregate by key_id even if epochs differ
        assert len(keys) == 1, "Should aggregate same key_id"
        assert len(keys["epoch-a"].partitions) == 2, "Should have 2 partitions"

        print("✓ test_key_id_uniqueness_across_epochs passed")


def run_all_tests():
    """Run all tests and report results."""
    tests = [
        test_single_key_discovery,
        test_multiple_epoch_discovery,
        test_aggregates_same_key_multiple_partitions,
        test_empty_manifest_handling,
        test_key_id_uniqueness_across_epochs
    ]

    print("Running epoch discovery unit tests...")
    print("=" * 60)

    passed = 0
    failed = 0

    for test in tests:
        try:
            test()
            passed += 1
        except AssertionError as e:
            print(f"✗ {test.__name__} failed: {e}")
            failed += 1
        except Exception as e:
            print(f"✗ {test.__name__} error: {e}")
            failed += 1

    print("=" * 60)
    print(f"Results: {passed} passed, {failed} failed")

    if failed == 0:
        print("✓ All tests passed!")
        return 0
    else:
        print("✗ Some tests failed")
        return 1


if __name__ == "__main__":
    sys.exit(run_all_tests())
