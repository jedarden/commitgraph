#!/usr/bin/env python3
"""
Create test fixture with a 2170-dated commit to test quarantine behavior.

This fixture reproduces the historical incident where a single 2170-dated
commit zeroed the board-wide AI-commit count (quarantine bf-jyctj/93dc8d1;
aggregator fix 946e815).

The fixture contains:
1. Normal commits with valid dates (2024-08-01 to 2024-08-05)
2. One commit dated 2170-01-01 (the incident reproduction)
3. One commit dated 2004-12-31 (below minimum bound)

Expected behavior after migration:
- Rollup (repo_user_daily_tool): only commits 1-5 counted (5 AI-tagged commits)
- Parquet artifact: all 7 commits preserved with raw committed_at values
"""

import pyarrow as pa
import pyarrow.parquet as pq
from datetime import datetime, date, timedelta
from pathlib import Path
import tempfile
import os

def create_fixture_corpus():
    """Create a minimal test corpus with out-of-range dates."""

    # Create temporary directory structure
    with tempfile.TemporaryDirectory() as tmpdir:
        corpus_path = Path(tmpdir) / "corpus"
        partition_path = corpus_path / "provider=github" / "year=2024" / "month=08"
        partition_path.mkdir(parents=True)

        # Define test commits with various dates
        commits = [
            # Normal AI-tagged commits (should be in rollup)
            {
                "sha": "aa" * 20,
                "provider": "github",
                "repo_full_name": "test/quarantine-fixture",
                "author_name": "Test User",
                "author_email": "test@example.com",
                "committed_at": datetime(2024, 8, 1, 12, 0, 0),
                "message": "feat: add feature\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
            },
            {
                "sha": "bb" * 20,
                "provider": "github",
                "repo_full_name": "test/quarantine-fixture",
                "author_name": "Test User",
                "author_email": "test@example.com",
                "committed_at": datetime(2024, 8, 2, 12, 0, 0),
                "message": "fix: bug fix\n\nCo-Authored-By: Cursor AI <info@cursor.sh>"
            },
            {
                "sha": "cc" * 20,
                "provider": "github",
                "repo_full_name": "test/quarantine-fixture",
                "author_name": "Another User",
                "author_email": "another@example.com",
                "committed_at": datetime(2024, 8, 3, 12, 0, 0),
                "message": "docs: update readme\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
            },
            {
                "sha": "dd" * 20,
                "provider": "github",
                "repo_full_name": "test/quarantine-fixture",
                "author_name": "Test User",
                "author_email": "test@example.com",
                "committed_at": datetime(2024, 8, 4, 12, 0, 0),
                "message": "refactor: code cleanup\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
            },
            {
                "sha": "ee" * 20,
                "provider": "github",
                "repo_full_name": "test/quarantine-fixture",
                "author_name": "Test User",
                "author_email": "test@example.com",
                "committed_at": datetime(2024, 8, 5, 12, 0, 0),
                "message": "test: add tests\n\nCo-Authored-By: GitHub Copilot <github-copilot@github.com>"
            },
            # The 2170 incident commit (should be quarantined from rollup, preserved in Parquet)
            {
                "sha": "ff" * 20,
                "provider": "github",
                "repo_full_name": "test/quarantine-fixture",
                "author_name": "Test User",
                "author_email": "test@example.com",
                "committed_at": datetime(2170, 1, 1, 0, 0, 0),  # The incident date
                "message": "chore: the 2170 incident commit\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
            },
            # Pre-2005 commit (should be quarantined from rollup, preserved in Parquet)
            {
                "sha": "00" * 20,
                "provider": "github",
                "repo_full_name": "test/quarantine-fixture",
                "author_name": "Test User",
                "author_email": "test@example.com",
                "committed_at": datetime(2004, 12, 31, 23, 59, 59),  # Below minimum bound
                "message": "ancient: commit from before 2005\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
            },
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

        # Convert commits to Arrow format
        data = {field: [] for field in schema.names}
        for commit in commits:
            data['sha'].append(commit['sha'])
            data['provider'].append(commit['provider'])
            data['repo_full_name'].append(commit['repo_full_name'])
            data['author_name'].append(commit['author_name'])
            data['author_email'].append(commit['author_email'])
            data['committed_at'].append(commit['committed_at'])
            data['message'].append(commit['message'])

        table = pa.table(data, schema=schema)

        # Write to Parquet
        parquet_path = partition_path / "part-00000.parquet"
        pq.write_table(table, parquet_path)

        # Create _manifest file
        manifest = {
            "encryption_keys": [],
            "partition_count": 1,
            "row_count": len(commits),
            "schema": schema.to_string()
        }

        import json
        with open(partition_path / "_manifest", 'w') as f:
            json.dump(manifest, f, indent=2)

        print(f"Created fixture corpus at: {corpus_path}")
        print(f"Total commits: {len(commits)}")
        print(f"  - Normal AI-tagged commits: 5 (should be in rollup)")
        print(f"  - 2170-dated commit: 1 (should be quarantined from rollup, preserved in Parquet)")
        print(f"  - Pre-2005 commit: 1 (should be quarantined from rollup, preserved in Parquet)")
        print(f"\nExpected migration results:")
        print(f"  - repo_user_daily_tool rows: 5 (only valid dates)")
        print(f"  - Parquet artifact rows: 7 (all commits preserved)")

        # Write the fixture path to a file for use in tests
        fixture_marker = Path("/home/coding/commitgraph/migration/fixtures/.2170_fixture_path")
        fixture_marker.write_text(str(corpus_path))

        return corpus_path

if __name__ == "__main__":
    create_fixture_corpus()
