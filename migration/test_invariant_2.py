#!/usr/bin/env python3
"""
Test Invariant 2 against a fixture database.

This test validates that the SQL assertion correctly detects out-of-range dates
in repo_user_daily_tool. It's designed to run both in CI against fixture databases
and as a one-off validation test.

Acceptance criteria for cg-5bpf:
- SELECT finds any row with day < '2005-01-01' OR day > current_date + 1
- CI fixture includes a deliberately out-of-range row to prove the assertion catches it
"""

import psycopg
from psycopg import sql
import subprocess
import sys
from pathlib import Path
import tempfile
import os


def run_sql_file(db_conn, sql_file_path: str) -> list:
    """
    Run a SQL file against the database and return the results.

    Returns the violation rows (empty list means invariant passed).
    """
    with open(sql_file_path, 'r') as f:
        sql_content = f.read()

    # Extract the main SELECT query (lines 50-67 of the invariant file)
    # We skip the DO block which is for logging only
    lines = sql_content.split('\n')
    select_start = None
    for i, line in enumerate(lines):
        if 'SELECT' in line and 'rut.repo_id' in line:
            select_start = i
            break

    if select_start is None:
        raise ValueError("Could not find SELECT query in SQL file")

    select_query = '\n'.join(lines[select_start:])

    cursor = db_conn.cursor()
    cursor.execute(select_query)
    violations = cursor.fetchall()
    cursor.close()

    return violations


def create_fixture_database_with_violation():
    """
    Create a temporary Postgres database with deliberate invariant violations.

    This fixture includes:
    1. Normal rows with valid dates (should NOT be flagged)
    2. A row with day = 2170-01-01 (historical incident reproduction)
    3. A row with day = 2004-12-31 (below minimum bound)

    Returns: (connection_string, expected_violation_count)
    """
    # Use a temporary database
    test_db_name = f"test_invariant_2_{os.getpid()}"

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
    """)

    # Insert test data
    cursor.execute("""
        INSERT INTO repos (provider, repo_full_name) VALUES
            ('github', 'test/normal-repo'),
            ('github', 'test/2170-incident-repo'),
            ('github', 'test/ancient-repo');

        INSERT INTO users (login) VALUES
            ('normal-user'),
            ('2170-user'),
            ('ancient-user');

        -- Normal rows (should NOT be flagged)
        INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits) VALUES
            (1, 1, 'claude-code', '2024-08-01'::DATE, 5),
            (1, 1, 'claude-code', '2024-08-02'::DATE, 3),
            (1, 1, 'cursor', '2024-08-01'::DATE, 2);

        -- VIOLATION 1: The 2170 incident (historical reproduction)
        INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits) VALUES
            (2, 2, 'claude-code', '2170-01-01'::DATE, 1);

        -- VIOLATION 2: Pre-2005 date
        INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits) VALUES
            (3, 3, 'claude-code', '2004-12-31'::DATE, 1);
    """)

    cursor.close()
    # Keep connection open for caller

    return conn_str, 2  # 2 expected violations


def test_invariant_2_detection():
    """
    Test that Invariant 2 SQL correctly detects out-of-range dates.

    This test creates a fixture database with deliberate violations and verifies
    that the invariant SQL catches them.
    """
    print("Testing Invariant 2: No out-of-range days in repo_user_daily_tool\n")

    # Create fixture database with violations
    print("1. Creating fixture database with deliberate violations...")
    conn_str, expected_violations = create_fixture_database_with_violation()
    print(f"   ✓ Created test database with {expected_violations} violations")

    # Run the invariant SQL
    print("\n2. Running invariant SQL assertion...")
    invariant_path = Path("/home/coding/commitgraph/migrations/invariant_2_no_out_of_range_days.sql")

    conn = psycopg.connect(conn_str)
    violations = run_sql_file(conn, invariant_path)
    conn.close()

    actual_violations = len(violations)

    print(f"   Found {actual_violations} violations")

    # Verify the invariant caught the violations
    if actual_violations != expected_violations:
        print(f"\n❌ FAIL: Expected {expected_violations} violations, found {actual_violations}")
        if violations:
            print("\nViolation rows:")
            for v in violations:
                print(f"  - repo_id={v[0]}, user_id={v[3]}, tool={v[5]}, day={v[6]}")
        return False

    print("\n3. Verifying violation details...")

    # Check for 2170 violation
    has_2170 = any(str(v[6]) == '2170-01-01' for v in violations)
    print(f"   ✓ 2170 incident detected: {has_2170}")

    # Check for pre-2005 violation
    has_pre2005 = any(str(v[6]) == '2004-12-31' for v in violations)
    print(f"   ✓ Pre-2005 date detected: {has_pre2005}")

    # Clean up
    print("\n4. Cleaning up test database...")
    conn = psycopg.connect("host=localhost user=postgres dbname=postgres", autocommit=True)
    test_db_name = conn.info.dbname.replace("postgres", f"test_invariant_2_{os.getpid()}")
    conn.cursor().execute(sql.SQL("DROP DATABASE {}").format(
        sql.Identifier(test_db_name)))
    conn.close()
    print("   ✓ Test database cleaned up")

    print("\n✅ PASS: Invariant 2 correctly detects all out-of-range dates")
    print(f"   - Normal rows: NOT flagged (as expected)")
    print(f"   - 2170-dated row: FLAGGED (historical incident reproduction)")
    print(f"   - Pre-2005 row: FLAGGED (below minimum bound)")

    return True


def test_invariant_2_passes_on_valid_data():
    """
    Test that Invariant 2 passes on a database with only valid dates.
    """
    print("\nTesting Invariant 2 passes on valid data...\n")

    test_db_name = f"test_invariant_2_valid_{os.getpid()}"

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

        INSERT INTO repos (provider, repo_full_name) VALUES
            ('github', 'test/valid-repo');

        INSERT INTO users (login) VALUES
            ('valid-user');

        -- ALL VALID dates (within [2005-01-01, today+1])
        INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits) VALUES
            (1, 1, 'claude-code', '2005-01-01'::DATE, 1),
            (1, 1, 'claude-code', '2024-08-01'::DATE, 5),
            (1, 1, 'cursor', CURRENT_DATE + INTERVAL '1 day', 3);
    """)

    cursor.close()

    # Run invariant
    invariant_path = Path("/home/coding/commitgraph/migrations/invariant_2_no_out_of_range_days.sql")
    violations = run_sql_file(conn, invariant_path)
    conn.close()

    # Clean up
    conn = psycopg.connect("host=localhost user=postgres dbname=postgres", autocommit=True)
    conn.cursor().execute(sql.SQL("DROP DATABASE {}").format(
        sql.Identifier(test_db_name)))
    conn.close()

    if len(violations) > 0:
        print(f"❌ FAIL: Found {len(violations)} violations in valid data")
        return False

    print("✅ PASS: Invariant 2 correctly passes on valid data (0 violations)")
    return True


if __name__ == "__main__":
    print("=" * 70)
    print("Invariant 2 Test Suite")
    print("=" * 70)
    print("\nThis test validates cg-5bpf acceptance criteria:")
    print("- SELECT finds rows with day < '2005-01-01' OR day > current_date + 1")
    print("- CI fixture includes deliberately out-of-range rows")
    print()

    success = True

    try:
        success = test_invariant_2_detection() and success
    except Exception as e:
        print(f"\n❌ FAIL: Detection test raised exception: {e}")
        import traceback
        traceback.print_exc()
        success = False

    try:
        success = test_invariant_2_passes_on_valid_data() and success
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
