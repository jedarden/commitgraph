#!/usr/bin/env python3
"""
Unit tests for ARMOR client warm-start artifact fetch.

Tests the fetch_warmstart_artifact method covering three cases:
1. Artifact present — returns tar bytes
2. Artifact absent — returns None (not an exception)
3. ARMOR unreachable/erroring — returns None (never raises)

Acceptance criteria from bead cg-4v07.
"""

import sys
import os
from unittest.mock import Mock, MagicMock, patch
from io import BytesIO

# Mock boto3 before importing armor_client (not available in this environment)
sys.modules['boto3'] = MagicMock()
sys.modules['botocore.client'] = MagicMock()
sys.modules['botocore.exceptions'] = MagicMock()

# Add migration directory to path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from armor_client import ArmorClient


def test_warmstart_key_generation():
    """
    Test that warm-start key generation uses the correct namespace.

    Warm-start artifacts should use a distinct key namespace from
    Parquet commit-history artifacts:
    - Parquet: .../commits.parquet
    - Warm-start: .../warmstart.tar
    """
    client = ArmorClient(
        armor_url='http://test:9000',
        access_key='test',
        secret_key='test',
        bucket='test-bucket'
    )

    # Test warm-start key generation
    warmstart_key = client.get_warmstart_key('github', 'owner/repo')
    expected = 'commitgraph/repo-artifacts/github/owner/repo/warmstart.tar'
    assert warmstart_key == expected, \
        f"Expected warm-start key '{expected}', got '{warmstart_key}'"

    # Verify it's distinct from Parquet artifact key
    parquet_key = client.get_artifact_key('github', 'owner/repo')
    assert warmstart_key != parquet_key, \
        "Warm-start key must be distinct from Parquet artifact key"

    print("✓ Warm-start key generation test passed!")
    print(f"  - Warm-start key: {warmstart_key}")
    print(f"  - Parquet key: {parquet_key}")
    print(f"  - Keys are distinct: True")

    return True


def test_fetch_warmstart_artifact_present():
    """
    Test fetching a warm-start artifact when it exists in ARMOR.

    Should return the tarball bytes, not None.
    """
    # Mock S3 client
    mock_s3 = MagicMock()
    mock_response = {
        'Body': BytesIO(b'fake tarball content')
    }
    mock_s3.get_object.return_value = mock_response

    client = ArmorClient(
        armor_url='http://test:9000',
        access_key='test',
        secret_key='test',
        bucket='test-bucket'
    )
    client.s3_client = mock_s3

    # Fetch warm-start artifact
    result = client.fetch_warmstart_artifact('github', 'owner/repo')

    # Verify we got bytes back
    assert result is not None, \
        "Should return bytes when artifact exists"
    assert isinstance(result, bytes), \
        f"Should return bytes, got {type(result)}"
    assert result == b'fake tarball content', \
        "Should return the exact artifact bytes"

    # Verify S3 get_object was called correctly
    mock_s3.get_object.assert_called_once_with(
        Bucket='test-bucket',
        Key='commitgraph/repo-artifacts/github/owner/repo/warmstart.tar'
    )

    print("✓ Fetch present artifact test passed!")
    print("  - Returned bytes: True")
    print("  - Artifact content length:", len(result))

    return True


def test_fetch_warmstart_artifact_absent():
    """
    Test fetching a warm-start artifact when it doesn't exist.

    Should return None (clean signal), not raise an exception.
    """
    # Mock S3 client to raise NoSuchKey
    mock_s3 = MagicMock()

    # Create a proper exception class
    class MockNoSuchKey(Exception):
        pass

    mock_s3.exceptions.NoSuchKey = MockNoSuchKey
    mock_s3.get_object.side_effect = mock_s3.exceptions.NoSuchKey("No such key")

    client = ArmorClient(
        armor_url='http://test:9000',
        access_key='test',
        secret_key='test',
        bucket='test-bucket'
    )
    client.s3_client = mock_s3

    # Fetch warm-start artifact
    result = client.fetch_warmstart_artifact('github', 'owner/repo')

    # Verify we got None (not an exception)
    assert result is None, \
        "Should return None when artifact doesn't exist"

    print("✓ Fetch absent artifact test passed!")
    print("  - Returned None: True (clean 'not found' signal)")
    print("  - No exception raised: True")

    return True


def test_fetch_warmstart_artifact_armor_error():
    """
    Test fetching when ARMOR itself is unreachable or errors.

    Should return None (treat same as "absent"), not raise an exception.
    This allows clone-worker to fall back to full clone.
    """
    # Mock S3 client to raise a generic error (network, timeout, etc.)
    mock_s3 = MagicMock()

    # Set up the exceptions property properly
    class MockNoSuchKey(Exception):
        pass
    mock_s3.exceptions = MagicMock()
    mock_s3.exceptions.NoSuchKey = MockNoSuchKey

    mock_s3.get_object.side_effect = Exception("ARMOR unreachable")

    client = ArmorClient(
        armor_url='http://test:9000',
        access_key='test',
        secret_key='test',
        bucket='test-bucket'
    )
    client.s3_client = mock_s3

    # Fetch warm-start artifact
    result = client.fetch_warmstart_artifact('github', 'owner/repo')

    # Verify we got None (not an exception)
    assert result is None, \
        "Should return None when ARMOR errors (treat as 'absent')"

    print("✓ Fetch with ARMOR error test passed!")
    print("  - Returned None: True (ARMOR errors treated as 'absent')")
    print("  - No exception raised: True")
    print("  - Caller can fall back to full clone: True")

    return True


def test_fetch_warmstart_all_cases_distinguishable():
    """
    Verify all three cases are distinguishable from the caller's perspective.

    From clone-worker's perspective:
    - Got bytes: use warm-start
    - Got None: fall back to full clone
    - Never: catch an exception

    The implementation folds both "absent" and "error" into the same
    None return, which is correct — both cases mean "fall back to full clone."
    """
    client = ArmorClient(
        armor_url='http://test:9000',
        access_key='test',
        secret_key='test',
        bucket='test-bucket'
    )

    # Case 1: Present → returns bytes
    mock_s3 = MagicMock()
    # Set up exceptions property for all cases
    class MockNoSuchKey(Exception):
        pass
    mock_s3.exceptions = MagicMock()
    mock_s3.exceptions.NoSuchKey = MockNoSuchKey
    mock_s3.get_object.return_value = {'Body': BytesIO(b'tar')}
    client.s3_client = mock_s3
    result_present = client.fetch_warmstart_artifact('github', 'owner/repo')
    has_bytes = isinstance(result_present, bytes)

    # Case 2: Absent → returns None
    class MockNoSuchKey(Exception):
        pass
    mock_s3.exceptions.NoSuchKey = MockNoSuchKey
    mock_s3.get_object.side_effect = mock_s3.exceptions.NoSuchKey("No such key")
    client.s3_client = mock_s3
    result_absent = client.fetch_warmstart_artifact('github', 'owner/repo')
    is_none_absent = result_absent is None

    # Case 3: Error → returns None (same as absent)
    mock_s3.get_object.side_effect = Exception("Network error")
    client.s3_client = mock_s3
    result_error = client.fetch_warmstart_artifact('github', 'owner/repo')
    is_none_error = result_error is None

    # Verify distinguishable outcomes
    assert has_bytes, "Present case should return bytes"
    assert is_none_absent, "Absent case should return None"
    assert is_none_error, "Error case should return None"

    print("✓ All cases distinguishable test passed!")
    print("  - Present → bytes:", has_bytes)
    print("  - Absent → None:", is_none_absent)
    print("  - Error → None:", is_none_error)
    print("  - Caller can distinguish: bytes vs None")

    return True


if __name__ == "__main__":
    print("Testing ARMOR client warm-start fetch (cg-4v07)...\n")

    test_warmstart_key_generation()
    print()

    test_fetch_warmstart_artifact_present()
    print()

    test_fetch_warmstart_artifact_absent()
    print()

    test_fetch_warmstart_artifact_armor_error()
    print()

    test_fetch_warmstart_all_cases_distinguishable()
    print()

    print("✅ All tests passed! ARMOR warm-start fetch is working correctly.")
    print("\nAcceptance criteria met:")
    print("  ✓ Returns tar bytes when artifact present")
    print("  ✓ Returns None (not exception) when absent")
    print("  ✓ Returns None (not exception) when ARMOR errors")
    print("  ✓ Uses per-repo warm-start key namespace")
    print("  ✓ Unit tests with mocked ARMOR client")
