#!/usr/bin/env python3
"""
Create test fixture with multiple encryption epochs including retired ones.

This fixture creates a corpus with partitions encrypted with different key_ids,
including at least one "retired" (non-current) epoch to prove the preflight tool
does not silently skip older partitions.

Fixture structure:
- provider=github/year=2024/month=08/  (current epoch)
- provider=github/year=2023/month=12/  (retired epoch)
- provider=github/year=2022/month=06/  (another retired epoch)

This tests the critical requirement: "scoping to only the current epoch would
silently skip older partitions still sitting on retired epochs."
"""

import pyarrow as pa
import pyarrow.parquet as pq
from datetime import datetime
from pathlib import Path
import tempfile
import json
import shutil


def create_multi_epoch_fixture():
    """Create a test corpus with multiple encryption epochs."""

    # Create temporary directory structure
    tmpdir = tempfile.mkdtemp(prefix="corpus_fixture_")
    corpus_path = Path(tmpdir) / "corpus"
    corpus_path.mkdir(parents=True)

    print(f"Creating multi-epoch fixture at: {corpus_path}")

    # Define partitions with different epochs
    partitions = [
        {
            "path": "provider=github/year=2024/month=08",
            "key_id": "epoch-2024-08-current",
            "epoch": "2024-08",
            "commits": [
                {
                    "sha": "aaa" + "a" * 37,
                    "provider": "github",
                    "repo_full_name": "test/current-repo",
                    "author_name": "Current User",
                    "author_email": "current@example.com",
                    "committed_at": datetime(2024, 8, 15, 12, 0, 0),
                    "message": "feat: new feature\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
                }
            ]
        },
        {
            "path": "provider=github/year=2023/month=12",
            "key_id": "epoch-2023-12-retired",
            "epoch": "2023-12",
            "commits": [
                {
                    "sha": "bbb" + "b" * 37,
                    "provider": "github",
                    "repo_full_name": "test/old-repo",
                    "author_name": "Old User",
                    "author_email": "old@example.com",
                    "committed_at": datetime(2023, 12, 15, 12, 0, 0),
                    "message": "fix: old bug\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
                }
            ]
        },
        {
            "path": "provider=github/year=2022/month=06",
            "key_id": "epoch-2022-06-ancient",
            "epoch": "2022-06",
            "commits": [
                {
                    "sha": "ccc" + "c" * 37,
                    "provider": "github",
                    "repo_full_name": "test/ancient-repo",
                    "author_name": "Ancient User",
                    "author_email": "ancient@example.com",
                    "committed_at": datetime(2022, 6, 15, 12, 0, 0),
                    "message": "feat: ancient feature\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
                }
            ]
        }
    ]

    # Create Arrow schema
    schema = pa.schema([
        ('sha', pa.string()),
        ('provider', pa.string()),
        ('repo_full_name', pa.string()),
        ('author_name', pa.string()),
        ('author_email', pa.string()),
        ('committed_at', pa.timestamp('ns')),
        ('message', pa.string())
    ])

    # Create each partition
    for partition_spec in partitions:
        partition_path = corpus_path / partition_spec["path"]
        partition_path.mkdir(parents=True)

        # Convert commits to Arrow format
        data = {field: [] for field in schema.names}
        for commit in partition_spec["commits"]:
            data['sha'].append(commit['sha'])
            data['provider'].append(commit['provider'])
            data['repo_full_name'].append(commit['repo_full_name'])
            data['author_name'].append(commit['author_name'])
            data['author_email'].append(commit['author_email'])
            data['committed_at'].append(commit['committed_at'])
            data['message'].append(commit['message'])

        table = pa.table(data, schema=schema)

        # Write Parquet file
        parquet_path = partition_path / "part-00000.parquet"
        pq.write_table(table, parquet_path)

        # Create _manifest with encryption key metadata
        manifest = {
            "encryption_keys": [
                {
                    "key_id": partition_spec["key_id"],
                    "epoch": partition_spec["epoch"],
                    "key_path": f"/keys/{partition_spec['key_id']}"
                }
            ],
            "partition_count": 1,
            "row_count": len(partition_spec["commits"]),
            "schema": schema.to_string()
        }

        with open(partition_path / "_manifest", 'w') as f:
            json.dump(manifest, f, indent=2)

        print(f"  Created partition: {partition_spec['path']}")
        print(f"    key_id={partition_spec['key_id']!r} epoch={partition_spec['epoch']!r}")

    # Create fake migration credential file
    credential_path = Path(tmpdir) / "migration_credential.json"
    credentials = {
        "key_id": "migration-master-key",
        "epochs": ["epoch-2024-08-current", "epoch-2023-12-retired", "epoch-2022-06-ancient"],
        "created_at": datetime.now().isoformat(),
        "description": "Migration credential with access to all epochs"
    }

    with open(credential_path, 'w') as f:
        json.dump(credentials, f, indent=2)

    print(f"\nCreated migration credential at: {credential_path}")

    # Write fixture metadata
    fixture_metadata = {
        "corpus_path": str(corpus_path),
        "credential_path": str(credential_path),
        "epochs": [
            {"key_id": p["key_id"], "epoch": p["epoch"], "status": "current" if "2024-08" in p["key_id"] else "retired"}
            for p in partitions
        ],
        "total_partitions": len(partitions),
        "total_commits": sum(len(p["commits"]) for p in partitions)
    }

    metadata_path = Path(tmpdir) / "fixture_metadata.json"
    with open(metadata_path, 'w') as f:
        json.dump(fixture_metadata, f, indent=2)

    print(f"\nFixture metadata:")
    print(f"  Total partitions: {fixture_metadata['total_partitions']}")
    print(f"  Total commits: {fixture_metadata['total_commits']}")
    print(f"  Epochs: {len(fixture_metadata['epochs'])}")
    print(f"    - Current: 1")
    print(f"    - Retired: {sum(1 for e in fixture_metadata['epochs'] if e['status'] == 'retired')}")

    print(f"\n✓ Multi-epoch fixture created successfully")
    print(f"  Corpus: {corpus_path}")
    print(f"  Credential: {credential_path}")
    print(f"  Metadata: {metadata_path}")

    return tmpdir


def cleanup_fixture(fixture_dir: str):
    """Clean up the fixture directory."""
    try:
        shutil.rmtree(fixture_dir)
        print(f"Cleaned up fixture: {fixture_dir}")
    except Exception as e:
        print(f"Failed to clean up fixture: {e}")


if __name__ == "__main__":
    import sys

    try:
        fixture_dir = create_multi_epoch_fixture()

        # Keep the path for tests
        marker_path = Path("/home/coding/commitgraph/migration/fixtures/.multi_epoch_fixture_path")
        marker_path.write_text(fixture_dir)

        print(f"\nFixture path written to: {marker_path}")
        print(f"Use this path for preflight check tests.")

    except Exception as e:
        print(f"Error creating fixture: {e}", file=sys.stderr)
        sys.exit(1)
