#!/usr/bin/env python3
"""
Test enumerate_key_ids module.

Tests the manifest scanning logic for:
- Correct extraction of key_ids from valid manifests
- Deduplication of key_ids across multiple manifests
- Graceful handling of missing manifests
- Graceful handling of malformed manifest JSON
- Correct counting of manifests scanned
"""

import json
import tempfile
import shutil
from pathlib import Path
import unittest

from enumerate_key_ids import enumerate_key_ids, EnumerationResult


class TestEnumerateKeyIds(unittest.TestCase):
    """Test suite for key_id enumeration across corpus manifests."""

    def setUp(self):
        """Create a temporary corpus directory with test fixtures."""
        self.test_dir = tempfile.mkdtemp()
        self.corpus_path = Path(self.test_dir)

    def tearDown(self):
        """Clean up temporary directory."""
        shutil.rmtree(self.test_dir)

    def _create_partition(self, provider: str, year: str, month: str,
                          key_ids: list, create_manifest: bool = True,
                          manifest_valid: bool = True) -> Path:
        """
        Helper to create a partition directory with optional manifest.

        Args:
            provider: Provider name (e.g., 'github')
            year: Year string (e.g., '2024')
            month: Month string (e.g., '01')
            key_ids: List of key_id strings to include in manifest
            create_manifest: Whether to create a manifest file
            manifest_valid: Whether manifest should be valid JSON

        Returns:
            Path to the created partition directory
        """
        partition_path = self.corpus_path / f"provider={provider}" / f"year={year}" / f"month={month}"
        partition_path.mkdir(parents=True, exist_ok=True)

        if create_manifest:
            manifest_path = partition_path / "_manifest"
            if manifest_valid:
                manifest = {
                    'encryption_keys': [
                        {'key_id': key_id, 'key_path': f'/keys/{key_id}', 'epoch': f'epoch-{key_id}'}
                        for key_id in key_ids
                    ]
                }
                with open(manifest_path, 'w') as f:
                    json.dump(manifest, f)
            else:
                # Write invalid JSON
                with open(manifest_path, 'w') as f:
                    f.write('{invalid json content')

        return partition_path

    def test_single_manifest_single_key(self):
        """Test enumeration with a single manifest containing one key_id."""
        self._create_partition('github', '2024', '01', ['key-001'])

        result = enumerate_key_ids(self.test_dir)

        self.assertEqual(result.manifests_scanned, 1)
        self.assertEqual(result.manifests_missing, 0)
        self.assertEqual(result.manifests_failed, 0)
        self.assertEqual(result.unique_key_ids_count, 1)
        self.assertEqual(result.key_ids, {'key-001'})

    def test_single_manifest_multiple_keys(self):
        """Test enumeration with a single manifest containing multiple key_ids."""
        self._create_partition('github', '2024', '01', ['key-001', 'key-002', 'key-003'])

        result = enumerate_key_ids(self.test_dir)

        self.assertEqual(result.manifests_scanned, 1)
        self.assertEqual(result.unique_key_ids_count, 3)
        self.assertEqual(result.key_ids, {'key-001', 'key-002', 'key-003'})

    def test_multiple_manifests_same_key(self):
        """Test deduplication: same key_id in multiple manifests."""
        self._create_partition('github', '2024', '01', ['key-shared'])
        self._create_partition('github', '2024', '02', ['key-shared'])
        self._create_partition('github', '2024', '03', ['key-shared'])

        result = enumerate_key_ids(self.test_dir)

        self.assertEqual(result.manifests_scanned, 3)
        self.assertEqual(result.unique_key_ids_count, 1)
        self.assertEqual(result.key_ids, {'key-shared'})

    def test_multiple_manifests_different_keys(self):
        """Test enumeration with multiple manifests containing different key_ids."""
        self._create_partition('github', '2024', '01', ['key-001'])
        self._create_partition('github', '2024', '02', ['key-002'])
        self._create_partition('github', '2024', '03', ['key-003'])

        result = enumerate_key_ids(self.test_dir)

        self.assertEqual(result.manifests_scanned, 3)
        self.assertEqual(result.unique_key_ids_count, 3)
        self.assertEqual(result.key_ids, {'key-001', 'key-002', 'key-003'})

    def test_multiple_providers(self):
        """Test enumeration across multiple providers."""
        self._create_partition('github', '2024', '01', ['key-github'])
        self._create_partition('gitlab', '2024', '01', ['key-gitlab'])
        self._create_partition('gitea', '2024', '01', ['key-gitea'])

        result = enumerate_key_ids(self.test_dir)

        self.assertEqual(result.manifests_scanned, 3)
        self.assertEqual(result.unique_key_ids_count, 3)
        self.assertEqual(result.key_ids, {'key-github', 'key-gitlab', 'key-gitea'})

    def test_missing_manifest(self):
        """Test graceful handling of missing manifests."""
        # Create partitions but skip manifest creation
        self._create_partition('github', '2024', '01', ['key-001'], create_manifest=False)
        self._create_partition('github', '2024', '02', ['key-002'], create_manifest=False)
        self._create_partition('github', '2024', '03', ['key-003'], create_manifest=True)

        result = enumerate_key_ids(self.test_dir)

        # Should only scan the one manifest that exists
        self.assertEqual(result.manifests_scanned, 1)
        self.assertEqual(result.manifests_missing, 2)
        self.assertEqual(result.manifests_failed, 0)
        self.assertEqual(result.unique_key_ids_count, 1)
        self.assertEqual(result.key_ids, {'key-003'})

    def test_malformed_manifest(self):
        """Test graceful handling of malformed JSON in manifests."""
        # Create valid manifest
        self._create_partition('github', '2024', '01', ['key-valid'], manifest_valid=True)
        # Create malformed manifest
        self._create_partition('github', '2024', '02', ['key-malformed'], manifest_valid=False)

        result = enumerate_key_ids(self.test_dir)

        # Should successfully parse the valid manifest and skip the malformed one
        self.assertEqual(result.manifests_scanned, 1)
        self.assertEqual(result.manifests_missing, 0)
        self.assertEqual(result.manifests_failed, 1)
        self.assertEqual(result.unique_key_ids_count, 1)
        self.assertEqual(result.key_ids, {'key-valid'})

    def test_empty_key_id(self):
        """Test that empty key_ids are filtered out."""
        # Create manifest with empty key_id
        partition_path = self._create_partition('github', '2024', '01', [])
        manifest_path = partition_path / "_manifest"
        manifest = {
            'encryption_keys': [
                {'key_id': '', 'key_path': '/empty', 'epoch': 'epoch-empty'},
                {'key_id': 'key-001', 'key_path': '/keys/key-001', 'epoch': 'epoch-001'}
            ]
        }
        with open(manifest_path, 'w') as f:
            json.dump(manifest, f)

        result = enumerate_key_ids(self.test_dir)

        # Empty key_id should be filtered out
        self.assertEqual(result.unique_key_ids_count, 1)
        self.assertEqual(result.key_ids, {'key-001'})

    def test_no_encryption_keys_field(self):
        """Test handling manifests without encryption_keys field."""
        partition_path = self._create_partition('github', '2024', '01', [])
        manifest_path = partition_path / "_manifest"
        manifest = {'other_field': 'value'}
        with open(manifest_path, 'w') as f:
            json.dump(manifest, f)

        result = enumerate_key_ids(self.test_dir)

        # Should not crash, just return empty set
        self.assertEqual(result.manifests_scanned, 1)
        self.assertEqual(result.unique_key_ids_count, 0)
        self.assertEqual(result.key_ids, set())

    def test_to_dict(self):
        """Test JSON serialization of result."""
        self._create_partition('github', '2024', '01', ['key-002', 'key-001'])

        result = enumerate_key_ids(self.test_dir)
        result_dict = result.to_dict()

        # Check structure
        self.assertIn('manifests_scanned', result_dict)
        self.assertIn('manifests_missing', result_dict)
        self.assertIn('manifests_failed', result_dict)
        self.assertIn('unique_key_ids_count', result_dict)
        self.assertIn('key_ids', result_dict)

        # Check values
        self.assertEqual(result_dict['manifests_scanned'], 1)
        self.assertEqual(result_dict['manifests_missing'], 0)
        self.assertEqual(result_dict['manifests_failed'], 0)
        self.assertEqual(result_dict['unique_key_ids_count'], 2)
        self.assertEqual(result_dict['key_ids'], ['key-001', 'key-002'])  # Sorted

    def test_nonexistent_corpus(self):
        """Test that nonexistent corpus root raises ValueError."""
        with self.assertRaises(ValueError) as context:
            enumerate_key_ids('/nonexistent/path')

        self.assertIn('does not exist', str(context.exception))

    def test_complex_realistic_scenario(self):
        """Test a realistic scenario with multiple providers, years, and keys."""
        # GitHub: current epoch key across 2024
        self._create_partition('github', '2024', '01', ['github-epoch-2024'])
        self._create_partition('github', '2024', '06', ['github-epoch-2024'])
        self._create_partition('github', '2024', '12', ['github-epoch-2024'])

        # GitLab: retired epoch in early 2023, current in 2024
        self._create_partition('gitlab', '2023', '01', ['gitlab-epoch-2023'])
        self._create_partition('gitlab', '2024', '01', ['gitlab-epoch-2024'])

        # Gitea: single key across all time
        self._create_partition('gitea', '2023', '01', ['gitea-epoch-2023'])
        self._create_partition('gitea', '2024', '01', ['gitea-epoch-2023'])

        # Missing manifest in one partition
        self._create_partition('github', '2024', '03', ['ignored'], create_manifest=False)

        result = enumerate_key_ids(self.test_dir)

        # Should have 5 distinct keys (github-2024, gitlab-2023, gitlab-2024, gitea-2023)
        self.assertEqual(result.manifests_scanned, 7)  # 8 partitions - 1 missing manifest
        self.assertEqual(result.unique_key_ids_count, 4)
        self.assertEqual(result.key_ids, {
            'github-epoch-2024',
            'gitlab-epoch-2023',
            'gitlab-epoch-2024',
            'gitea-epoch-2023'
        })
        # Check error counts
        self.assertEqual(result.manifests_missing, 1)
        self.assertEqual(result.manifests_failed, 0)

    def test_combined_missing_and_failed(self):
        """Test tracking both missing and failed manifests in same corpus."""
        # Valid manifest
        self._create_partition('github', '2024', '01', ['key-001'], manifest_valid=True)
        # Missing manifest (no file created)
        self._create_partition('github', '2024', '02', ['key-002'], create_manifest=False)
        # Failed manifest (malformed JSON)
        self._create_partition('github', '2024', '03', ['key-003'], manifest_valid=False)

        result = enumerate_key_ids(self.test_dir)

        self.assertEqual(result.manifests_scanned, 1)
        self.assertEqual(result.manifests_missing, 1)
        self.assertEqual(result.manifests_failed, 1)
        self.assertEqual(result.unique_key_ids_count, 1)
        self.assertEqual(result.key_ids, {'key-001'})


if __name__ == '__main__':
    unittest.main()
