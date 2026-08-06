-- Invariant 4: Identity referential integrity + acyclic one-level alias graph
--
-- This invariant validates three aspects of identity data integrity:
--
-- (a) Rollup user_id FK integrity: Every user_id in repo_user_daily_tool
--     must resolve to a users row.
--
-- (b) user_aliases.target_login existence: Every target_login in user_aliases
--     must exist in users.login.
--
-- (c) Alias graph acyclic + one-level-deep: No alias source_login may be
--     itself a target_login of another alias (no chains), and no cycles
--     (A to B, B to A) may exist.
--
-- Usage:
--   Run these assertions in CI against fixture databases
--   Run periodically against production as an audit
--
-- Expected: 0 rows (no violations)

-- ============================================================
-- Query (a): Rollup user_id FK integrity
-- ============================================================
-- Finds any repo_user_daily_tool.user_id with no matching users.user_id
-- This should never happen if the write path is working correctly.
--
-- Expected: 0 rows on healthy database
-- Returns: Violating rollup rows with their orphan user_id

DO $$
DECLARE
    violation_count BIGINT;
BEGIN
    -- Count violations
    SELECT COUNT(*) INTO violation_count
    FROM repo_user_daily_tool rut
    LEFT JOIN users u ON rut.user_id = u.user_id
    WHERE u.user_id IS NULL;

    -- Log violations
    IF violation_count > 0 THEN
        RAISE NOTICE 'Invariant 4(a) violation: % rollup rows with orphan user_id', violation_count;

        -- Show sample violations for debugging
        RAISE NOTICE 'Sample violations (up to 10):';
        FOR v IN
            SELECT rut.repo_id, rut.user_id, rut.tool, rut.day, rut.commits
            FROM repo_user_daily_tool rut
            LEFT JOIN users u ON rut.user_id = u.user_id
            WHERE u.user_id IS NULL
            LIMIT 10
        LOOP
            RAISE NOTICE '  repo_id=%, user_id=%, tool=%, day=%, commits=%',
                v.repo_id, v.user_id, v.tool, v.day, v.commits;
        END LOOP;
    ELSE
        RAISE NOTICE 'Invariant 4(a): PASS (0 orphan user_id violations)';
    END IF;
END $$;

-- Direct query for CI (returns 0 rows on pass, violation rows on fail)
SELECT
    rut.repo_id,
    r.provider,
    r.repo_full_name,
    rut.user_id,           -- This user_id doesn't exist in users
    rut.tool,
    rut.day,
    rut.commits,
    rut.insert_time
FROM repo_user_daily_tool rut
JOIN repos r ON rut.repo_id = r.repo_id
LEFT JOIN users u ON rut.user_id = u.user_id
WHERE u.user_id IS NULL
ORDER BY rut.repo_id, rut.user_id, rut.day;

-- ============================================================
-- Query (b): user_aliases.target_login existence in users
-- ============================================================
-- Finds any user_aliases.target_login not present in users.login
-- This can happen if:
--  - A user is deleted from users but their alias remains
--  - An alias is created targeting a non-existent login (bug in ingest)
--
-- Expected: 0 rows on healthy database
-- Returns: Violating alias rows with their non-existent target_login

DO $$
DECLARE
    violation_count BIGINT;
BEGIN
    -- Count violations
    SELECT COUNT(*) INTO violation_count
    FROM user_aliases ua
    LEFT JOIN users u ON ua.target_login = u.login
    WHERE u.login IS NULL;

    -- Log violations
    IF violation_count > 0 THEN
        RAISE NOTICE 'Invariant 4(b) violation: % aliases target non-existent login', violation_count;

        -- Show sample violations for debugging
        RAISE NOTICE 'Sample violations (up to 10):';
        FOR v IN
            SELECT ua.source_login, ua.target_login, ua.reason, ua.created_at
            FROM user_aliases ua
            LEFT JOIN users u ON ua.target_login = u.login
            WHERE u.login IS NULL
            LIMIT 10
        LOOP
            RAISE NOTICE '  source_login=%, target_login=%, reason=%, created_at=%',
                v.source_login, v.target_login, v.reason, v.created_at;
        END LOOP;
    ELSE
        RAISE NOTICE 'Invariant 4(b): PASS (0 non-existent target_login violations)';
    END IF;
END $$;

-- Direct query for CI (returns 0 rows on pass, violation rows on fail)
SELECT
    ua.source_login,
    ua.target_login,     -- This target_login doesn't exist in users.login
    ua.reason,
    ua.created_at
FROM user_aliases ua
LEFT JOIN users u ON ua.target_login = u.login
WHERE u.login IS NULL
ORDER BY ua.source_login;

-- ============================================================
-- Query (c): Alias graph acyclic + one-level-deep
-- ============================================================
-- Finds two classes of violations:
--
-- 1. Chained aliases (depth > 1): Any source_login that is itself a target_login
--    of another alias row. This creates chains like A -> B -> C.
--
-- 2. Direct cycles (A to B, B to A): Two aliases pointing at each other.
--
-- Both violate the one-level-deep requirement. The alias graph must be a
-- simple star graph: all source_logins point directly to canonical logins,
-- with no indirection.
--
-- Expected: 0 rows on healthy database
-- Returns: Violating alias pairs showing the chain/cycle

DO $$
DECLARE
    violation_count BIGINT;
BEGIN
    -- Count violations
    SELECT COUNT(*) INTO violation_count
    FROM user_aliases ua1
    JOIN user_aliases ua2 ON ua1.source_login = ua2.target_login;

    -- Log violations
    IF violation_count > 0 THEN
        RAISE NOTICE 'Invariant 4(c) violation: % chained alias pairs (depth > 1 or cycles)', violation_count;

        -- Show sample violations for debugging
        RAISE NOTICE 'Sample violations (up to 10):';
        FOR v IN
            SELECT ua1.source_login AS level1_source, ua1.target_login AS level1_target,
                   ua2.source_login AS level2_source, ua2.target_login AS level2_target
            FROM user_aliases ua1
            JOIN user_aliases ua2 ON ua1.source_login = ua2.target_login
            LIMIT 10
        LOOP
            RAISE NOTICE '  % -> % -> % -> % (chain or cycle)',
                v.level1_source, v.level1_target, v.level2_source, v.level2_target;
        END LOOP;
    ELSE
        RAISE NOTICE 'Invariant 4(c): PASS (0 chained alias violations)';
    END IF;
END $$;

-- Direct query for CI (returns 0 rows on pass, violation rows on fail)
-- Shows the chain: ua1.source_login -> ua1.target_login (= ua2.source_login) -> ua2.target_login
SELECT
    ua1.source_login AS level1_source,
    ua1.target_login AS level1_target,    -- This is also a source_login (violation!)
    ua2.source_login AS level2_source,   -- Same as level1_target
    ua2.target_login AS level2_target,
    ua1.reason AS level1_reason,
    ua2.reason AS level2_reason,
    ua1.created_at AS level1_created,
    ua2.created_at AS level2_created
FROM user_aliases ua1
JOIN user_aliases ua2 ON ua1.source_login = ua2.target_login
ORDER BY ua1.source_login, ua2.source_login;

-- ============================================================
-- Test fixture data (for CI)
-- ============================================================
-- This section creates deliberate violations to prove the queries work.
-- These should be inserted into a test fixture database to verify
-- that the invariant checks correctly detect broken data.

-- NOTE: These inserts are commented out. In CI, uncomment them
-- to create a fixture database with known violations, verify
-- the queries return exactly these rows, then clean up.

/*
-- Test fixture: Create violations for query (a) - orphan user_id in rollup
-- First ensure we have a user and repo
INSERT INTO users (user_id, login) VALUES (9999, 'test-user-fixture-a');
INSERT INTO repos (repo_id, provider, repo_full_name) VALUES (9999, 'github', 'test/repo-a');

-- Insert rollup row with non-existent user_id (violation for query a)
INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits, insert_time)
VALUES (9999, 8888, 'claude', '2024-01-01', 5, NOW());  -- user_id 8888 doesn't exist

-- Test fixture: Create violations for query (b) - target_login not in users
-- Create alias targeting non-existent login (violation for query b)
INSERT INTO user_aliases (source_login, target_login, reason, created_at)
VALUES ('old-login-fixture', 'non-existent-login', 'admin', NOW());  -- 'non-existent-login' not in users

-- Test fixture: Create violations for query (c) - chained aliases
-- Setup: Ensure canonical user exists
INSERT INTO users (user_id, login) VALUES (1001, 'canonical-user-c') ON CONFLICT (login) DO NOTHING;

-- Create chain: A -> B -> C (violates one-level-deep)
INSERT INTO user_aliases (source_login, target_login, reason, created_at)
VALUES ('chained-alias-a', 'chained-alias-b', 'admin', NOW());
INSERT INTO user_aliases (source_login, target_login, reason, created_at)
VALUES ('chained-alias-b', 'canonical-user-c', 'admin', NOW());

-- Create cycle: X -> Y, Y -> X (violates acyclic requirement)
INSERT INTO user_aliases (source_login, target_login, reason, created_at)
VALUES ('cycle-alias-x', 'cycle-alias-y', 'admin', NOW());
INSERT INTO user_aliases (source_login, target_login, reason, created_at)
VALUES ('cycle-alias-y', 'cycle-alias-x', 'admin', NOW());

-- After inserting this fixture data, run the three queries above.
-- Expected results:
-- Query (a) should return 1 row: the rollup row with user_id 8888
-- Query (b) should return 1 row: the alias targeting 'non-existent-login'
-- Query (c) should return 4 rows:
--   - 2 rows for the A -> B -> C chain (a->b->c, b->c)
--   - 2 rows for the X -> Y -> X cycle (x->y->x, y->x->y)
*/
