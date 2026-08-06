#!/usr/bin/env python3
"""
Test Invariant 4 against a fixture database.

This test validates that the three SQL assertions correctly detect:
(a) Orphan user_id in repo_user_daily_tool (FK integrity)
(b) user_aliases.target_login not in users.login (referential integrity)
(c) Chained aliases and cycles (acyclic + one-level-deep)

Acceptance criteria for cg-2eq3:
- Query (a) finds any repo_user_daily_tool.user_id with no matching users.user_id
- Query (b) finds any user_aliases.target_login not present in users.login
- Query (c) finds any user_aliases.source_login that is itself a target_login
- CI fixture includes deliberately chained alias (A to B to C) and cycle (A to B, B to A)
"""

import psycopg
from psycopg import sql
import subprocess
import sys
from pathlib import Path
import tempfile
import os


def run_invariant_4_queries(db_conn) -> dict:
    """
    Run all three Invariant 4 queries against the database.

    Returns a dict with keys 'query_a', 'query_b', 'query_c' containing
    violation counts and details for each query.
    """
    invariant_path = Path("/home/coding/commitgraph/migrations/invariant_4_identity_referential_integrity.sql")

    with open(invariant_path, 'r') as f:
        sql_content = f.read()

    results = {
        'query_a': {'violations': [], 'count': 0},
        'query_b': {'violations': [], 'count': 0},
        'query_c': {'violations': [], 'count': 0},
    }

    # Extract and run Query (a): Rollup user_id FK integrity
    # Look for the SELECT query after the comment block
    lines = sql_content.split('\n')

    # Query (a) - starts around line 62
    query_a_start = None
    query_a_end = None
    for i, line in enumerate(lines):
        if 'SELECT' in line and 'rut.repo_id' in line and query_a_start is None:
            query_a_start = i
        if query_a_start is not None and query_a_end is None:
            if i > query_a_start and ('--' in line or 'Query (b)' in line):
                query_a_end = i
                break

    if query_a_start is not None:
        query_a_end = query_a_end if query_a_end else len(lines)
        query_a = '\n'.join(lines[query_a_start:query_a_end])

        cursor = db_conn.cursor()
        cursor.execute(query_a)
        results['query_a']['violations'] = cursor.fetchall()
        results['query_a']['count'] = len(results['query_a']['violations'])
        cursor.close()

    # Query (b) - starts around line 120
    query_b_start = None
    query_b_end = None
    for i, line in enumerate(lines):
        if 'SELECT' in line and 'ua.source_login' in line and 'ua.target_login' in line and query_b_start is None:
            query_b_start = i
        if query_b_start is not None and query_b_end is None:
            if i > query_b_start and ('--' in line or 'Query (c)' in line):
                query_b_end = i
                break

    if query_b_start is not None:
        query_b_end = query_b_end if query_b_end else len(lines)
        query_b = '\n'.join(lines[query_b_start:query_b_end])

        cursor = db_conn.cursor()
        cursor.execute(query_b)
        results['query_b']['violations'] = cursor.fetchall()
        results['query_b']['count'] = len(results['query_b']['violations'])
        cursor.close()

    # Query (c) - starts around line 179
    query_c_start = None
    query_c_end = None
    for i, line in enumerate(lines):
        if 'SELECT' in line and 'ua1.source_login' in line and 'ua2.source_login' in line and query_c_start is None:
            query_c_start = i
        if query_c_start is not None and query_c_end is None:
            if i > query_c_start and ('--' in line or 'Test fixture' in line):
                query_c_end = i
                break

    if query_c_start is not None:
        query_c_end = query_c_end if query_c_end else len(lines)
        query_c = '\n'.join(lines[query_c_start:query_c_end])

        cursor = db_conn.cursor()
        cursor.execute(query_c)
        results['query_c']['violations'] = cursor.fetchall()
        results['query_c']['count'] = len(results['query_c']['violations'])
        cursor.close()

    return results


def create_fixture_database_with_violations():
    """
    Create a temporary Postgres database with deliberate invariant violations.

    This fixture includes:
    1. Normal data (should NOT be flagged)
    2. Query (a) violation: rollup row with orphan user_id
    3. Query (b) violation: alias targeting non-existent login
    4. Query (c) violations: chained aliases (A->B->C) and cycle (X->Y, Y->X)

    Returns: (connection_string, expected_counts dict)
    """
    test_db_name = f"test_invariant_4_{os.getpid()}"

    # Connect to postgres to create test database
    conn = psycopg.connect("host=localhost user=postgres dbname=postgres", autocommit=True)
    conn.cursor().execute(sql.SQL("DROP DATABASE IF EXISTS {}").format(
        sql.Identifier(test_db_name)))
    conn.cursor().execute(sql.SQL("CREATE DATABASE {}").format(
        sql.Identifier(test_db_name)))
    conn.close()

    # Connect to test database and create schema
    conn_str = f"host=localhost user=postgres dbname={test_db_name}"
    conn = psycopg.connect(conn_str, autocommit=True)
    cursor = conn.cursor()

    # Create schema (simplified version of 00001_initial_schema.sql)
    cursor.execute("""
        CREATE TABLE repos (
          repo_id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
          provider       TEXT NOT NULL,
          repo_full_name TEXT NOT NULL,
          excluded_at    TIMESTAMPTZ,
          excluded_reason TEXT,
          UNIQUE (provider, repo_full_name)
        );

        CREATE TABLE users (
          user_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
          login      TEXT NOT NULL UNIQUE,
          profile_url TEXT,
          avatar_url  TEXT
        );

        CREATE TABLE repo_user_daily_tool (
          repo_id     BIGINT NOT NULL REFERENCES repos(repo_id),
          user_id     BIGINT NOT NULL REFERENCES users(user_id),
          tool        TEXT   NOT NULL,
          day         DATE   NOT NULL,
          commits     INT    NOT NULL,
          insert_time TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
          PRIMARY KEY (repo_id, user_id, tool, day)
        );

        CREATE TABLE user_aliases (
          source_login TEXT PRIMARY KEY,
          target_login TEXT NOT NULL,
          reason       TEXT NOT NULL,
          created_at   TIMESTAMPTZ NOT NULL
        );
    """)

    # Insert test data
    cursor.execute("""
        -- Valid users
        INSERT INTO users (login) VALUES
            ('canonical-user'),
            ('alice'),
            ('bob'),
            ('canonical-user-c');

        -- Valid repos
        INSERT INTO repos (provider, repo_full_name) VALUES
            ('github', 'test/normal-repo'),
            ('github', 'test/orphan-user-repo');

        -- Normal rollup rows (should NOT be flagged)
        INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits) VALUES
            (1, 1, 'claude-code', '2024-08-01'::DATE, 5),
            (1, 1, 'cursor', '2024-08-01'::DATE, 3);

        -- VIOLATION for Query (a): Orphan user_id in rollup
        -- user_id 9999 doesn't exist in users table
        INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits) VALUES
            (2, 9999, 'claude-code', '2024-08-01'::DATE, 1);

        -- Valid aliases (should NOT be flagged)
        INSERT INTO user_aliases (source_login, target_login, reason, created_at) VALUES
            ('alice-old', 'alice', 'admin', NOW()),
            ('bob-bot', 'bob', 'admin', NOW());

        -- VIOLATION for Query (b): Alias targeting non-existent login
        INSERT INTO user_aliases (source_login, target_login, reason, created_at) VALUES
            ('old-login-fixture', 'non-existent-login', 'admin', NOW());

        -- VIOLATIONS for Query (c): Chained aliases and cycle
        -- Chain: A -> B -> C
        INSERT INTO user_aliases (source_login, target_login, reason, created_at) VALUES
            ('chained-alias-a', 'chained-alias-b', 'admin', NOW()),
            ('chained-alias-b', 'canonical-user-c', 'admin', NOW());

        -- Cycle: X -> Y, Y -> X
        INSERT INTO user_aliases (source_login, target_login, reason, created_at) VALUES
            ('cycle-alias-x', 'cycle-alias-y', 'admin', NOW()),
            ('cycle-alias-y', 'cycle-alias-x', 'admin', NOW());
    """)

    cursor.close()

    expected_counts = {
        'query_a': 1,  # 1 orphan user_id
        'query_b': 1,  # 1 alias with non-existent target
        'query_c': 4,  # 4 rows: 2 for chain (a->b->c, b->c), 2 for cycle (x->y->x, y->x->y)
    }

    return conn_str, expected_counts


def test_invariant_4_detection():
    """
    Test that all three Invariant 4 queries correctly detect violations.
    """
    print("Testing Invariant 4: Identity referential integrity + acyclic alias graph\n")

    # Create fixture database with violations
    print("1. Creating fixture database with deliberate violations...")
    conn_str, expected_counts = create_fixture_database_with_violations()
    print(f"   ✓ Created test database")
    print(f"   Expected violations: Query (a)={expected_counts['query_a']}, Query (b)={expected_counts['query_b']}, Query (c)={expected_counts['query_c']}")

    # Run all three invariant queries
    print("\n2. Running all three invariant SQL assertions...")
    conn = psycopg.connect(conn_str)
    results = run_invariant_4_queries(conn)
    conn.close()

    print(f"   Query (a) violations: {results['query_a']['count']}")
    print(f"   Query (b) violations: {results['query_b']['count']}")
    print(f"   Query (c) violations: {results['query_c']['count']}")

    success = True

    # Verify Query (a)
    print("\n3. Verifying Query (a) - Rollup user_id FK integrity...")
    if results['query_a']['count'] != expected_counts['query_a']:
        print(f"   ❌ FAIL: Expected {expected_counts['query_a']} violations, found {results['query_a']['count']}")
        success = False
    else:
        print(f"   ✓ PASS: Correctly detected {results['query_a']['count']} orphan user_id violation(s)")
        if results['query_a']['violations']:
            for v in results['query_a']['violations']:
                print(f"      repo_id={v[0]}, user_id={v[3]} (orphan)")

    # Verify Query (b)
    print("\n4. Verifying Query (b) - user_aliases.target_login exists in users...")
    if results['query_b']['count'] != expected_counts['query_b']:
        print(f"   ❌ FAIL: Expected {expected_counts['query_b']} violations, found {results['query_b']['count']}")
        success = False
    else:
        print(f"   ✓ PASS: Correctly detected {results['query_b']['count']} non-existent target_login")
        if results['query_b']['violations']:
            for v in results['query_b']['violations']:
                print(f"      source_login={v[0]}, target_login={v[1]} (non-existent)")

    # Verify Query (c)
    print("\n5. Verifying Query (c) - Alias graph acyclic + one-level-deep...")
    if results['query_c']['count'] != expected_counts['query_c']:
        print(f"   ❌ FAIL: Expected {expected_counts['query_c']} violations, found {results['query_c']['count']}")
        success = False
    else:
        print(f"   ✓ PASS: Correctly detected {results['query_c']['count']} chain/cycle violations")
        # Show chain violations
        chain_count = sum(1 for v in results['query_c']['violations'] if v[0] == 'chained-alias-a' or v[0] == 'chained-alias-b')
        cycle_count = sum(1 for v in results['query_c']['violations'] if v[0] == 'cycle-alias-x' or v[0] == 'cycle-alias-y')
        print(f"      Chain violations (A->B->C): {chain_count}")
        print(f"      Cycle violations (X->Y->X): {cycle_count}")

    # Clean up
    print("\n6. Cleaning up test database...")
    conn = psycopg.connect("host=localhost user=postgres dbname=postgres", autocommit=True)
    test_db_name = f"test_invariant_4_{os.getpid()}"
    conn.cursor().execute(sql.SQL("DROP DATABASE {}").format(
        sql.Identifier(test_db_name)))
    conn.close()
    print("   ✓ Test database cleaned up")

    if success:
        print("\n✅ PASS: All three Invariant 4 assertions work correctly")
        print("   - Query (a): Detects orphan user_id in rollup")
        print("   - Query (b): Detects aliases targeting non-existent logins")
        print("   - Query (c): Detects chained aliases and cycles")
    else:
        print("\n❌ FAIL: Some invariant assertions did not work correctly")

    return success


def test_invariant_4_passes_on_valid_data():
    """
    Test that Invariant 4 passes on a database with only valid data.
    """
    print("\nTesting Invariant 4 passes on valid data...\n")

    test_db_name = f"test_invariant_4_valid_{os.getpid()}"

    # Create test database
    conn = psycopg.connect("host=localhost user=postgres dbname=postgres", autocommit=True)
    conn.cursor().execute(sql.SQL("DROP DATABASE IF EXISTS {}").format(
        sql.Identifier(test_db_name)))
    conn.cursor().execute(sql.SQL("CREATE DATABASE {}").format(
        sql.Identifier(test_db_name)))
    conn.close()

    conn_str = f"host=localhost user=postgres dbname={test_db_name}"
    conn = psycopg.connect(conn_str, autocommit=True)
    cursor = conn.cursor()

    # Create schema and insert ONLY valid data
    cursor.execute("""
        CREATE TABLE repos (
          repo_id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
          provider       TEXT NOT NULL,
          repo_full_name TEXT NOT NULL,
          UNIQUE (provider, repo_full_name)
        );

        CREATE TABLE users (
          user_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
          login      TEXT NOT NULL UNIQUE
        );

        CREATE TABLE repo_user_daily_tool (
          repo_id     BIGINT NOT NULL REFERENCES repos(repo_id),
          user_id     BIGINT NOT NULL REFERENCES users(user_id),
          tool        TEXT   NOT NULL,
          day         DATE   NOT NULL,
          commits     INT    NOT NULL,
          insert_time TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
          PRIMARY KEY (repo_id, user_id, tool, day)
        );

        CREATE TABLE user_aliases (
          source_login TEXT PRIMARY KEY,
          target_login TEXT NOT NULL,
          reason       TEXT NOT NULL,
          created_at   TIMESTAMPTZ NOT NULL
        );

        -- Valid data (all FKs valid, aliases one-level only)
        INSERT INTO users (login) VALUES
            ('alice'),
            ('bob'),
            ('canonical');

        INSERT INTO repos (provider, repo_full_name) VALUES
            ('github', 'test/valid-repo');

        INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits) VALUES
            (1, 1, 'claude-code', '2024-08-01'::DATE, 5);

        -- Valid aliases: all source_logins point to canonical logins
        -- No source_login is also a target_login (one-level-deep)
        INSERT INTO user_aliases (source_login, target_login, reason, created_at) VALUES
            ('alice-old', 'alice', 'admin', NOW()),
            ('bob-bot', 'bob', 'admin', NOW()),
            ('deprecated', 'canonical', 'admin', NOW());
    """)

    cursor.close()

    # Run invariants
    results = run_invariant_4_queries(conn)
    conn.close()

    # Clean up
    conn = psycopg.connect("host=localhost user=postgres dbname=postgres", autocommit=True)
    conn.cursor().execute(sql.SQL("DROP DATABASE {}").format(
        sql.Identifier(test_db_name)))
    conn.close()

    total_violations = results['query_a']['count'] + results['query_b']['count'] + results['query_c']['count']

    if total_violations > 0:
        print(f"❌ FAIL: Found {total_violations} violations in valid data")
        print(f"   Query (a): {results['query_a']['count']}")
        print(f"   Query (b): {results['query_b']['count']}")
        print(f"   Query (c): {results['query_c']['count']}")
        return False

    print("✅ PASS: Invariant 4 correctly passes on valid data (0 violations)")
    return True


if __name__ == "__main__":
    print("=" * 70)
    print("Invariant 4 Test Suite")
    print("=" * 70)
    print("\nThis test validates cg-2eq3 acceptance criteria:")
    print("- Query (a): Finds orphan user_id in repo_user_daily_tool")
    print("- Query (b): Finds aliases targeting non-existent logins")
    print("- Query (c): Finds chained aliases and cycles")
    print("- CI fixture includes: chained alias (A->B->C) and cycle (X->Y->X)")
    print()

    success = True

    try:
        success = test_invariant_4_detection() and success
    except Exception as e:
        print(f"\n❌ FAIL: Detection test raised exception: {e}")
        import traceback
        traceback.print_exc()
        success = False

    try:
        success = test_invariant_4_passes_on_valid_data() and success
    except Exception as e:
        print(f"\n❌ FAIL: Valid-data test raised exception: {e}")
        import traceback
        traceback.print_exc()
        success = False

    if not success:
        print("\n" + "=" * 70)
        print("❌ TESTS FAILED")
        print("=" * 70)
        sys.exit(1)

    print("\n" + "=" * 70)
    print("✅ ALL TESTS PASSED")
    print("=" * 70)
    sys.exit(0)
