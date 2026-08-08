-- Invariant 5: Uniform scan time per repo
--
-- This invariant validates that all rows for a given repo_id share exactly
-- one insert_time value. This is a critical property of the whole-slice
-- replace pattern: the rollup write path uses DELETE + bulk INSERT in a
-- single transaction, so all rows for a repo must have the same insert_time
-- (the timestamp of when that transaction committed).
--
-- A violation means a partial write escaped the whole-slice-replace
-- transaction, which should never happen. It could indicate:
--  - Rows were inserted outside the transactional write path
--  - A transaction committed but some rows got a different timestamp
--  - Concurrent writes to the same repo (race condition)
--
-- Usage:
--   Run this assertion in CI against fixture databases
--   Run periodically against production as an audit
--
-- Expected: 0 rows (no violations)

-- ============================================================
-- Query: Find repos with mixed insert_time values
-- ============================================================
-- Finds any repo_id that has rows with more than one distinct insert_time.
-- This should never happen if the write path is working correctly.
--
-- Expected: 0 rows on healthy database
-- Returns: repo_ids with mixed insert_time values, with sample data

DO $$
DECLARE
    violation_count BIGINT;
BEGIN
    -- Count repos with mixed insert_time values
    SELECT COUNT(*) INTO violation_count
    FROM (
        SELECT repo_id
        FROM repo_user_daily_tool
        GROUP BY repo_id
        HAVING COUNT(DISTINCT insert_time) > 1
    ) violated_repos;

    -- Log violations
    IF violation_count > 0 THEN
        RAISE NOTICE 'Invariant 5 violation: % repos have mixed insert_time values', violation_count;

        -- Show sample violations for debugging
        RAISE NOTICE 'Sample violations (up to 10):';
        FOR v IN
            SELECT repo_id
            FROM repo_user_daily_tool
            GROUP BY repo_id
            HAVING COUNT(DISTINCT insert_time) > 1
            LIMIT 10
        LOOP
            RAISE NOTICE '  repo_id=%', v.repo_id;
        END LOOP;
    ELSE
        RAISE NOTICE 'Invariant 5: PASS (0 violations)';
    END IF;
END $$;

-- Direct query for CI (returns 0 rows on pass, violation rows on fail)
-- Shows repo_ids with mixed insert_time values, with diagnostic detail
SELECT
    rut.repo_id,
    r.provider,
    r.repo_full_name,
    COUNT(DISTINCT rut.insert_time) AS distinct_insert_time_count,
    ARRAY_AGG(DISTINCT rut.insert_time ORDER BY rut.insert_time) AS insert_time_samples,
    COUNT(*) AS total_rows,
    MIN(rut.day) AS earliest_day,
    MAX(rut.day) AS latest_day
FROM repo_user_daily_tool rut
JOIN repos r ON rut.repo_id = r.repo_id
GROUP BY rut.repo_id, r.provider, r.repo_full_name
HAVING COUNT(DISTINCT rut.insert_time) > 1
ORDER BY distinct_insert_time_count DESC, rut.repo_id;

-- ============================================================
-- Test fixture data (for CI)
-- ============================================================
-- This section creates deliberate violations to prove the query works.
-- These should be inserted into a test fixture database to verify
-- that the invariant check correctly detects broken data.

-- NOTE: These inserts are commented out. In CI, uncomment them
-- to create a fixture database with known violations, verify
-- the query returns exactly these rows, then clean up.

/*
-- Test fixture: Create violations - repo with mixed insert_time values
-- Setup: Ensure we have a user and repo
INSERT INTO users (user_id, login) VALUES (5001, 'test-user-invariant5');
INSERT INTO repos (repo_id, provider, repo_full_name) VALUES (5001, 'github', 'test/repo-invariant5');

-- Insert rollup rows with the SAME insert_time (correct, should not trigger violation)
INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
VALUES
    (5001, 5001, 'claude', '2024-01-01'::DATE, 5, '2024-01-15 10:00:00+00'::TIMESTAMPTZ),
    (5001, 5001, 'claude', '2024-01-02'::DATE, 3, '2024-01-15 10:00:00+00'::TIMESTAMPTZ),
    (5001, 5001, 'copilot', '2024-01-01'::DATE, 2, '2024-01-15 10:00:00+00'::TIMESTAMPTZ);

-- Insert MORE rollup rows for the SAME repo but with a DIFFERENT insert_time (violation!)
-- This simulates a partial write or concurrent write scenario.
INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
VALUES
    (5001, 5001, 'claude', '2024-01-03'::DATE, 4, '2024-01-16 11:30:00+00'::TIMESTAMPTZ),
    (5001, 5001, 'copilot', '2024-01-04'::DATE, 1, '2024-01-16 11:30:00+00'::TIMESTAMPTZ);

-- Now repo_id 5001 has 5 rows with 2 different insert_time values:
-- - 3 rows with insert_time = 2024-01-15 10:00:00+00
-- - 2 rows with insert_time = 2024-01-16 11:30:00+00
-- This should trigger the invariant violation query.

-- Expected result when running the main query:
-- - repo_id = 5001
-- - distinct_insert_time_count = 2
-- - insert_time_samples = ['2024-01-15 10:00:00+00', '2024-01-16 11:30:00+00']
-- - total_rows = 5

-- Test fixture: Create a SECOND repo with uniform insert_time (should NOT appear in results)
INSERT INTO repos (repo_id, provider, repo_full_name) VALUES (5002, 'github', 'test/repo-uniform');
INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
VALUES
    (5002, 5001, 'claude', '2024-01-01'::DATE, 5, '2024-01-17 12:00:00+00'::TIMESTAMPTZ),
    (5002, 5001, 'claude', '2024-01-02'::DATE, 3, '2024-01-17 12:00:00+00'::TIMESTAMPTZ);

-- Repo 5002 has 2 rows with 1 insert_time value (uniform - should NOT trigger violation)

-- After inserting this fixture data, run the query above.
-- Expected results:
-- - Query should return 1 row: repo_id 5001 (the violating repo)
-- - Repo 5002 should NOT appear (it has uniform insert_time)
*/
