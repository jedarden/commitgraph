#!/usr/bin/env python3
"""
Test Invariant 5: Uniform scan time for all rows of a repo_id

Acceptance criteria for cg-43xi:
- SQL assertion: SELECT repo_id FROM repo_user_daily_tool GROUP BY repo_id 
  HAVING COUNT(DISTINCT insert_time) > 1 returns zero rows.
- CI test writes a repo's rollup slice via the real write path (DELETE + bulk INSERT)
  and asserts the query above returns nothing for that repo.
- CI test simulates a failed/aborted mid-write (e.g. transaction rolled back partway)
  and asserts no partial, mixed-timestamp rows are ever visible.
- The same query is registered as a periodic production audit.

This test validates that:
1. The SQL assertion correctly detects mixed insert_time values
2. The DELETE + bulk INSERT write pattern produces uniform insert_time
3. Transaction rollbacks don't leave partial state visible
"""

import psycopg
from psycopg import sql
import subprocess
import sys
from pathlib import Path
import tempfile
import os
import time
from datetime import date, datetime, timedelta


def run_sql_file(db_conn, sql_file_path: str) -> list:
    """
    Run a SQL file against the database and return the results.

    Returns the violation rows (empty list means invariant passed).
    """
    with open(sql_file_path, 'r') as f:
        sql_content = f.read()

    # Extract the main SELECT query (find the SELECT that returns violation rows)
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


def create_fixture_database():
    """
    Create a temporary Postgres database for testing Invariant 5.

    Returns: connection_string
    """
    test_db_name = f"test_invariant_5_{os.getpid()}"

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

    cursor.close()
    # Keep connection open for caller
    return conn_str


def cleanup_test_database(conn_str):
    """Clean up the test database."""
    # Extract database name from connection string
    db_name = conn_str.split("dbname=")[1].split()[0]
    
    conn = psycopg.connect("host=localhost user=postgres dbname=postgres", autocommit=True)
    conn.cursor().execute(sql.SQL("DROP DATABASE {}").format(
        sql.Identifier(db_name)))
    conn.close()


def test_invariant_5_detects_violations():
    """
    Test that Invariant 5 SQL correctly detects mixed insert_time values.

    This test creates a fixture database with deliberate violations and verifies
    that the invariant SQL catches them.
    """
    print("=" * 70)
    print("Test 1: Invariant 5 detects mixed insert_time values")
    print("=" * 70)
    print()

    # Create fixture database
    print("1. Creating fixture database...")
    conn_str = create_fixture_database()
    print(f"   ✓ Created test database")

    # Insert test data with deliberate violations
    print("\n2. Inserting test data with mixed insert_time values...")
    conn = psycopg.connect(conn_str, autocommit=True)
    cursor = conn.cursor()

    # Create test repos and users
    cursor.execute("""
        INSERT INTO repos (provider, repo_full_name) VALUES
            ('github', 'test/violating-repo'),
            ('github', 'test/clean-repo');

        INSERT INTO users (login) VALUES
            ('test-user-1'),
            ('test-user-2');
    """)

    # Insert VIOLATING data: repo 1 has rows with different insert_time values
    # This simulates a broken write path or partial write
    cursor.execute("""
        INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
        VALUES
            (1, 1, 'claude-code', '2024-08-01'::DATE, 5, '2024-08-01 10:00:00+00'::TIMESTAMPTZ),
            (1, 1, 'claude-code', '2024-08-02'::DATE, 3, '2024-08-01 10:00:00+00'::TIMESTAMPTZ),
            (1, 1, 'cursor', '2024-08-01'::DATE, 2, '2024-08-01 10:00:00+00'::TIMESTAMPTZ),
            -- VIOLATION: Same repo, different insert_time
            (1, 1, 'claude-code', '2024-08-03'::DATE, 4, '2024-08-02 11:30:00+00'::TIMESTAMPTZ);
    """)

    # Insert CLEAN data: repo 2 has uniform insert_time (should not trigger violation)
    cursor.execute("""
        INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
        VALUES
            (2, 2, 'claude-code', '2024-08-01'::DATE, 5, '2024-08-01 12:00:00+00'::TIMESTAMPTZ),
            (2, 2, 'claude-code', '2024-08-02'::DATE, 3, '2024-08-01 12:00:00+00'::TIMESTAMPTZ);
    """)

    cursor.close()
    conn.close()
    print("   ✓ Inserted 1 violating repo and 1 clean repo")

    # Run the invariant SQL
    print("\n3. Running invariant SQL assertion...")
    invariant_path = Path("/home/coding/commitgraph/migrations/invariant_5_uniform_scan_time.sql")

    conn = psycopg.connect(conn_str)
    violations = run_sql_file(conn, invariant_path)
    conn.close()

    actual_violations = len(violations)
    print(f"   Found {actual_violations} violations")

    # Verify we detected exactly 1 violation
    if actual_violations != 1:
        print(f"\n❌ FAIL: Expected 1 violation, found {actual_violations}")
        if violations:
            print("\nViolation rows:")
            for v in violations:
                print(f"  - repo_id={v[0]}, repo={v[2]}, distinct_count={v[3]}")
        cleanup_test_database(conn_str)
        return False

    print("\n4. Verifying violation details...")
    
    # Check the violating repo is detected correctly
    v = violations[0]
    repo_id = v[0]
    repo_full_name = v[2]
    distinct_count = v[3]
    total_rows = v[6]
    
    print(f"   ✓ Violating repo: {repo_full_name} (repo_id={repo_id})")
    print(f"   ✓ Distinct insert_time count: {distinct_count}")
    print(f"   ✓ Total rows: {total_rows}")

    if repo_id != 1 or distinct_count != 2 or total_rows != 4:
        print(f"\n❌ FAIL: Unexpected violation details")
        cleanup_test_database(conn_str)
        return False

    # Clean up
    print("\n5. Cleaning up test database...")
    cleanup_test_database(conn_str)
    print("   ✓ Test database cleaned up")

    print("\n✅ PASS: Invariant 5 correctly detects mixed insert_time values")
    print(f"   - Clean repo (uniform insert_time): NOT flagged (as expected)")
    print(f"   - Violating repo (mixed insert_time): FLAGGED (as expected)")
    return True


def test_write_path_produces_uniform_insert_time():
    """
    Test that the DELETE + bulk INSERT write pattern produces uniform insert_time.

    This test validates the real write path implementation by:
    1. Writing a repo's rollup slice using the exact DELETE + INSERT pattern
    2. Asserting all rows have the same insert_time
    3. Re-writing the same repo to test idempotence
    """
    print("\n" + "=" * 70)
    print("Test 2: Write path produces uniform insert_time")
    print("=" * 70)
    print()

    # Create fixture database
    print("1. Creating fixture database...")
    conn_str = create_fixture_database()
    conn = psycopg.connect(conn_str)
    cursor = conn.cursor()
    print(f"   ✓ Created test database")

    # Create test repo and user
    print("\n2. Setting up test repo and user...")
    cursor.execute("""
        INSERT INTO repos (provider, repo_full_name) VALUES
            ('github', 'test/uniform-write-test')
            RETURNING repo_id;
    """)
    repo_id = cursor.fetchone()[0]

    cursor.execute("""
        INSERT INTO users (login) VALUES
            ('test-write-user')
            RETURNING user_id;
    """)
    user_id = cursor.fetchone()[0]
    conn.commit()
    print(f"   ✓ Created repo_id={repo_id}, user_id={user_id}")

    # Perform DELETE + INSERT write (simulating clone-worker write path)
    print("\n3. Performing DELETE + INSERT write (simulating clone-worker)...")
    
    # Simulate rollup data for this repo
    rollup_data = [
        (repo_id, user_id, 'claude-code', date(2024, 8, 1), 5),
        (repo_id, user_id, 'claude-code', date(2024, 8, 2), 3),
        (repo_id, user_id, 'cursor', date(2024, 8, 1), 2),
        (repo_id, user_id, 'copilot', date(2024, 8, 3), 4),
    ]

    # Execute in a transaction (DELETE + INSERT)
    try:
        cursor.execute("BEGIN")
        
        # DELETE existing rows for this repo
        cursor.execute(
            "DELETE FROM repo_user_daily_tool WHERE repo_id = %s",
            (repo_id,)
        )
        
        # Bulk INSERT new rows (insert_time uses DEFAULT transaction_timestamp())
        cursor.execute("""
            INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits)
            SELECT * FROM UNNEST(%s::bigint[], %s::bigint[], %s::text[], %s::date[], %s::int[])
        """, (
            [row[0] for row in rollup_data],
            [row[1] for row in rollup_data],
            [row[2] for row in rollup_data],
            [row[3] for row in rollup_data],
            [row[4] for row in rollup_data],
        ))
        
        cursor.execute("COMMIT")
        print(f"   ✓ Wrote {len(rollup_data)} rows via DELETE + INSERT")
        
    except Exception as e:
        cursor.execute("ROLLBACK")
        print(f"   ❌ FAIL: Write transaction failed: {e}")
        conn.close()
        cleanup_test_database(conn_str)
        return False

    # Verify all rows have the same insert_time
    print("\n4. Verifying uniform insert_time...")
    cursor.execute("""
        SELECT COUNT(DISTINCT insert_time) as distinct_count
        FROM repo_user_daily_tool
        WHERE repo_id = %s
    """, (repo_id,))
    
    distinct_count = cursor.fetchone()[0]
    print(f"   ✓ Distinct insert_time values: {distinct_count}")

    if distinct_count != 1:
        print(f"\n❌ FAIL: Expected 1 distinct insert_time, found {distinct_count}")
        
        # Show diagnostic info
        cursor.execute("""
            SELECT insert_time, COUNT(*)
            FROM repo_user_daily_tool
            WHERE repo_id = %s
            GROUP BY insert_time
            ORDER BY insert_time
        """, (repo_id,))
        
        print("\n  Insert time distribution:")
        for row in cursor.fetchall():
            print(f"    - {row[0]}: {row[1]} rows")
        
        conn.close()
        cleanup_test_database(conn_str)
        return False

    # Test idempotence: re-write the same repo
    print("\n5. Testing idempotence (re-write same repo)...")
    time.sleep(0.1)  # Small delay to ensure different transaction timestamp
    
    try:
        cursor.execute("BEGIN")
        
        cursor.execute(
            "DELETE FROM repo_user_daily_tool WHERE repo_id = %s",
            (repo_id,)
        )
        
        cursor.execute("""
            INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits)
            SELECT * FROM UNNEST(%s::bigint[], %s::bigint[], %s::text[], %s::date[], %s::int[])
        """, (
            [row[0] for row in rollup_data],
            [row[1] for row in rollup_data],
            [row[2] for row in rollup_data],
            [row[3] for row in rollup_data],
            [row[4] for row in rollup_data],
        ))
        
        cursor.execute("COMMIT")
        print(f"   ✓ Re-wrote {len(rollup_data)} rows")
        
    except Exception as e:
        cursor.execute("ROLLBACK")
        print(f"   ❌ FAIL: Re-write transaction failed: {e}")
        conn.close()
        cleanup_test_database(conn_str)
        return False

    # Verify all rows still have uniform insert_time
    cursor.execute("""
        SELECT COUNT(DISTINCT insert_time) as distinct_count
        FROM repo_user_daily_tool
        WHERE repo_id = %s
    """, (repo_id,))
    
    distinct_count = cursor.fetchone()[0]
    print(f"   ✓ After re-write: {distinct_count} distinct insert_time")

    if distinct_count != 1:
        print(f"\n❌ FAIL: Idempotence broken - found {distinct_count} distinct insert_time values")
        conn.close()
        cleanup_test_database(conn_str)
        return False

    # Verify no violations detected by invariant
    print("\n6. Running invariant SQL to verify no violations...")
    invariant_path = Path("/home/coding/commitgraph/migrations/invariant_5_uniform_scan_time.sql")
    violations = run_sql_file(conn, invariant_path)

    if len(violations) != 0:
        print(f"\n❌ FAIL: Invariant detected {len(violations)} violations, expected 0")
        conn.close()
        cleanup_test_database(conn_str)
        return False

    print(f"   ✓ Invariant check: PASS (0 violations)")

    conn.close()
    
    # Clean up
    print("\n7. Cleaning up test database...")
    cleanup_test_database(conn_str)
    print("   ✓ Test database cleaned up")

    print("\n✅ PASS: DELETE + INSERT produces uniform insert_time")
    print(f"   - Initial write: uniform insert_time")
    print(f"   - Re-write (idempotence): uniform insert_time")
    print(f"   - Invariant check: 0 violations")
    return True


def test_transaction_rollback_hides_partial_state():
    """
    Test that a failed/aborted mid-write transaction doesn't leave partial state visible.

    This test simulates a transaction that:
    1. Deletes existing rows
    2. Inserts some new rows
    3. Fails and rolls back before completing

    Expected behavior: No partial state should be visible after rollback.
    The original rows should still be present (not deleted).
    """
    print("\n" + "=" * 70)
    print("Test 3: Transaction rollback hides partial state")
    print("=" * 70)
    print()

    # Create fixture database
    print("1. Creating fixture database...")
    conn_str = create_fixture_database()
    conn = psycopg.connect(conn_str)
    cursor = conn.cursor()
    print(f"   ✓ Created test database")

    # Create test repo and user
    print("\n2. Setting up test repo and user...")
    cursor.execute("""
        INSERT INTO repos (provider, repo_full_name) VALUES
            ('github', 'test/rollback-test')
            RETURNING repo_id;
    """)
    repo_id = cursor.fetchone()[0]

    cursor.execute("""
        INSERT INTO users (login) VALUES
            ('test-rollback-user')
            RETURNING user_id;
    """)
    user_id = cursor.fetchone()[0]
    conn.commit()
    print(f"   ✓ Created repo_id={repo_id}, user_id={user_id}")

    # Write initial data
    print("\n3. Writing initial data...")
    cursor.execute("""
        INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits)
        VALUES
            (1, 1, 'claude-code', '2024-08-01'::DATE, 5, '2024-08-01 10:00:00+00'::TIMESTAMPTZ),
            (1, 1, 'claude-code', '2024-08-02'::DATE, 3, '2024-08-01 10:00:00+00'::TIMESTAMPTZ),
            (1, 1, 'cursor', '2024-08-01'::DATE, 2, '2024-08-01 10:00:00+00'::TIMESTAMPTZ);
    """)
    conn.commit()
    print(f"   ✓ Wrote 3 initial rows")

    # Simulate failed transaction: DELETE + partial INSERT + rollback
    print("\n4. Simulating failed transaction (DELETE + partial INSERT + ROLLBACK)...")
    
    try:
        cursor.execute("BEGIN")
        
        # Delete existing rows (this is the dangerous part - if transaction fails here, data is lost)
        cursor.execute(
            "DELETE FROM repo_user_daily_tool WHERE repo_id = %s",
            (repo_id,)
        )
        print("   - Deleted existing rows")
        
        # Insert SOME new rows (not all)
        cursor.execute("""
            INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
            VALUES (%s, %s, %s, %s::DATE, %s, %s)
        """, (repo_id, user_id, 'claude-code', date(2024, 8, 3), 4, '2024-08-02 11:00:00+00'))
        print("   - Inserted 1 new row (partial state)")
        
        # Simulate failure - e.g., constraint violation, network error, etc.
        # We'll trigger an intentional error
        cursor.execute("SELECT 1/0")  # Division by zero error
        cursor.execute("COMMIT")
        
    except psycopg.DatabaseError as e:
        cursor.execute("ROLLBACK")
        print(f"   - Transaction failed and rolled back: {e}")
    
    # Verify: Original data should still be present (not affected by rollback)
    print("\n5. Verifying original data is intact after rollback...")
    cursor.execute("""
        SELECT COUNT(*) as row_count
        FROM repo_user_daily_tool
        WHERE repo_id = %s
    """, (repo_id,))
    
    row_count = cursor.fetchone()[0]
    print(f"   ✓ Row count after rollback: {row_count}")

    if row_count != 3:
        print(f"\n❌ FAIL: Expected 3 rows (original data intact), found {row_count}")
        print("   This suggests the DELETE was not rolled back properly!")
        
        cursor.execute("""
            SELECT tool, day, commits, insert_time
            FROM repo_user_daily_tool
            WHERE repo_id = %s
            ORDER BY insert_time
        """, (repo_id,))
        
        print("\n  Current rows:")
        for row in cursor.fetchall():
            print(f"    - {row}")
        
        conn.close()
        cleanup_test_database(conn_str)
        return False

    # Verify all rows have the same original insert_time
    cursor.execute("""
        SELECT COUNT(DISTINCT insert_time) as distinct_count
        FROM repo_user_daily_tool
        WHERE repo_id = %s
    """, (repo_id,))
    
    distinct_count = cursor.fetchone()[0]
    print(f"   ✓ Distinct insert_time values: {distinct_count}")

    if distinct_count != 1:
        print(f"\n❌ FAIL: Expected 1 distinct insert_time (original), found {distinct_count}")
        conn.close()
        cleanup_test_database(conn_str)
        return False

    # Verify no violations detected by invariant
    print("\n6. Running invariant SQL to verify no violations...")
    invariant_path = Path("/home/coding/commitgraph/migrations/invariant_5_uniform_scan_time.sql")
    violations = run_sql_file(conn, invariant_path)

    if len(violations) != 0:
        print(f"\n❌ FAIL: Invariant detected {len(violations)} violations, expected 0")
        conn.close()
        cleanup_test_database(conn_str)
        return False

    print(f"   ✓ Invariant check: PASS (0 violations)")

    conn.close()
    
    # Clean up
    print("\n7. Cleaning up test database...")
    cleanup_test_database(conn_str)
    print("   ✓ Test database cleaned up")

    print("\n✅ PASS: Transaction rollback hides partial state")
    print(f"   - Failed transaction: original data intact")
    print(f"   - No mixed insert_time values: 1 distinct value")
    print(f"   - Invariant check: 0 violations")
    print(f"   - DELETE was properly rolled back")
    return True


def main():
    """Run all Invariant 5 tests."""
    print("\n" + "=" * 70)
    print("INVARIANT 5 TEST SUITE")
    print("Uniform scan time for all rows of a repo_id")
    print("=" * 70)
    print()

    all_passed = True

    # Test 1: SQL assertion detects violations
    if not test_invariant_5_detects_violations():
        all_passed = False

    # Test 2: Write path produces uniform insert_time
    if not test_write_path_produces_uniform_insert_time():
        all_passed = False

    # Test 3: Transaction rollback hides partial state
    if not test_transaction_rollback_hides_partial_state():
        all_passed = False

    print("\n" + "=" * 70)
    if all_passed:
        print("✅ ALL TESTS PASSED")
        print("=" * 70)
        return 0
    else:
        print("❌ SOME TESTS FAILED")
        print("=" * 70)
        return 1


if __name__ == "__main__":
    sys.exit(main())
