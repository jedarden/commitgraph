-- Test: Verify exclusion_audit_log table migration
--
-- This test creates the necessary prerequisite tables (repos), applies the migration,
-- and verifies that:
-- 1. The exclusion_audit_log table is created with all required columns
-- 2. All indexes are created correctly
-- 3. Foreign key relationship to repos table is enforced
-- 4. The migration is reversible (down migration works)
--
-- Prerequisites: Access to a PostgreSQL database
-- Run with: psql -h localhost -U commitgraph -d commitgraph -f migrations/test_00007_create_exclusion_audit_log.sql

BEGIN;

-- Step 1: Create prerequisite repos table
DROP TABLE IF EXISTS repos CASCADE;

CREATE TABLE repos (
    repo_id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider       TEXT NOT NULL,
    repo_full_name TEXT NOT NULL,
    excluded_at    TIMESTAMPTZ,
    excluded_reason TEXT,
    UNIQUE (provider, repo_full_name)
);

SELECT 'Step 1: Created prerequisite repos table' as status;

-- Step 2: Insert test repos
INSERT INTO repos (provider, repo_full_name, excluded_at, excluded_reason) VALUES
    ('github', 'jedarden/commitgraph', NOW(), 'test exclusion'),
    ('github', 'torvalds/linux', NULL, NULL),
    ('gitlab', 'gitlab-org/gitlab', NOW(), 'archived');

SELECT 'Step 2: Inserted 3 test repos (2 excluded, 1 active)' as status;

-- Step 3: Apply the migration (create exclusion_audit_log table)
CREATE TABLE IF NOT EXISTS exclusion_audit_log (
  id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  repo_id             BIGINT NOT NULL REFERENCES repos(repo_id) ON DELETE CASCADE,
  actor               TEXT NOT NULL,
  timestamp           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  event_type          TEXT NOT NULL,
  old_excluded_at     TIMESTAMPTZ,
  old_excluded_reason TEXT,
  new_excluded_at     TIMESTAMPTZ,
  new_excluded_reason TEXT
);

CREATE INDEX IF NOT EXISTS exclusion_audit_log_timestamp_idx ON exclusion_audit_log (timestamp DESC);
CREATE INDEX IF NOT EXISTS exclusion_audit_log_repo_idx ON exclusion_audit_log (repo_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS exclusion_audit_log_actor_idx ON exclusion_audit_log (actor, timestamp DESC);
CREATE INDEX IF NOT EXISTS exclusion_audit_log_active_exclusions_idx ON exclusion_audit_log (new_excluded_at) WHERE new_excluded_at IS NOT NULL;

SELECT 'Step 3: Applied migration (created exclusion_audit_log table and indexes)' as status;

-- Step 4: Verify table structure
DO $$
DECLARE
    column_count INT;
BEGIN
    SELECT COUNT(*) INTO column_count
    FROM information_schema.columns
    WHERE table_name = 'exclusion_audit_log';

    IF column_count != 9 THEN
        RAISE EXCEPTION 'Expected 9 columns in exclusion_audit_log, got %', column_count;
    END IF;

    RAISE NOTICE 'Verified: exclusion_audit_log has 9 columns';
END $$;

SELECT 'Step 4: Verified table has 9 columns' as status;

-- Step 5: Verify specific columns exist with correct types
DO $$
DECLARE
    id_type TEXT;
    repo_id_type TEXT;
    actor_type TEXT;
    timestamp_type TEXT;
    event_type_type TEXT;
BEGIN
    SELECT data_type INTO id_type
    FROM information_schema.columns
    WHERE table_name = 'exclusion_audit_log' AND column_name = 'id';

    SELECT data_type INTO repo_id_type
    FROM information_schema.columns
    WHERE table_name = 'exclusion_audit_log' AND column_name = 'repo_id';

    SELECT data_type INTO actor_type
    FROM information_schema.columns
    WHERE table_name = 'exclusion_audit_log' AND column_name = 'actor';

    SELECT data_type INTO timestamp_type
    FROM information_schema.columns
    WHERE table_name = 'exclusion_audit_log' AND column_name = 'timestamp';

    SELECT data_type INTO event_type_type
    FROM information_schema.columns
    WHERE table_name = 'exclusion_audit_log' AND column_name = 'event_type';

    IF id_type != 'bigint' THEN
        RAISE EXCEPTION 'id type is %, expected bigint', id_type;
    END IF;

    IF repo_id_type != 'bigint' THEN
        RAISE EXCEPTION 'repo_id type is %, expected bigint', repo_id_type;
    END IF;

    IF actor_type != 'text' THEN
        RAISE EXCEPTION 'actor type is %, expected text', actor_type;
    END IF;

    IF timestamp_type != 'timestamp with time zone' THEN
        RAISE EXCEPTION 'timestamp type is %, expected timestamp with time zone', timestamp_type;
    END IF;

    IF event_type_type != 'text' THEN
        RAISE EXCEPTION 'event_type type is %, expected text', event_type_type;
    END IF;

    RAISE NOTICE 'Verified: column types are correct';
END $$;

SELECT 'Step 5: Verified column types are correct' as status;

-- Step 6: Verify indexes are created
DO $$
DECLARE
    index_count INT;
BEGIN
    SELECT COUNT(*) INTO index_count
    FROM pg_indexes
    WHERE tablename = 'exclusion_audit_log';

    IF index_count < 4 THEN
        RAISE EXCEPTION 'Expected at least 4 indexes, got %', index_count;
    END IF;

    RAISE NOTICE 'Verified: exclusion_audit_log has % indexes', index_count;
END $$;

SELECT 'Step 6: Verified indexes are created' as status;

-- Step 7: Verify foreign key constraint exists
DO $$
DECLARE
    fk_count INT;
BEGIN
    SELECT COUNT(*) INTO fk_count
    FROM information_schema.table_constraints tc
    JOIN information_schema.constraint_column_usage ccu
        ON tc.constraint_name = ccu.constraint_name
    WHERE tc.table_name = 'exclusion_audit_log'
        AND tc.constraint_type = 'FOREIGN KEY'
        AND ccu.column_name = 'repo_id';

    IF fk_count != 1 THEN
        RAISE EXCEPTION 'Expected 1 foreign key on repo_id, got %', fk_count;
    END IF;

    RAISE NOTICE 'Verified: foreign key constraint on repo_id exists';
END $$;

SELECT 'Step 7: Verified foreign key constraint exists' as status;

-- Step 8: Test inserting audit log entries
INSERT INTO exclusion_audit_log (repo_id, actor, event_type, old_excluded_at, old_excluded_reason, new_excluded_at, new_excluded_reason) VALUES
    ((SELECT repo_id FROM repos WHERE repo_full_name = 'torvalds/linux'), 'admin', 'exclude', NULL, NULL, NOW(), 'policy violation'),
    ((SELECT repo_id FROM repos WHERE repo_full_name = 'jedarden/commitgraph'), 'admin', 'unexclude', NOW(), 'test exclusion', NULL, NULL),
    ((SELECT repo_id FROM repos WHERE repo_full_name = 'gitlab-org/gitlab'), 'system', 'exclude', NULL, NULL, NOW(), 'archived');

SELECT 'Step 8: Inserted 3 audit log entries' as status;

-- Step 9: Query to verify the data
SELECT
    eal.id,
    r.repo_full_name,
    eal.actor,
    eal.event_type,
    eal.old_excluded_at IS NOT NULL as was_excluded,
    eal.new_excluded_at IS NOT NULL as is_excluded,
    eal.new_excluded_reason
FROM exclusion_audit_log eal
JOIN repos r ON eal.repo_id = r.repo_id
ORDER BY eal.timestamp;

SELECT 'Step 9: Queried audit log to verify data' as status;

-- Step 10: Test foreign key constraint (should fail for invalid repo_id)
DO $$
BEGIN
    -- This should fail due to foreign key constraint violation
    INSERT INTO exclusion_audit_log (repo_id, actor, event_type, old_excluded_at, old_excluded_reason, new_excluded_at, new_excluded_reason)
    VALUES (99999, 'admin', 'exclude', NULL, NULL, NOW(), 'test');

    RAISE EXCEPTION 'Foreign key constraint should have prevented insert with invalid repo_id';
EXCEPTION
    WHEN foreign_key_violation THEN
        RAISE NOTICE 'SUCCESS: Foreign key constraint correctly prevented insert with invalid repo_id';
END $$;

SELECT 'Step 10: Tested foreign key constraint enforcement' as status;

-- Step 11: Test event_type values (only 'exclude' and 'unexclude' should be used)
DO $$
DECLARE
    valid_event_count INT;
BEGIN
    -- Check that we have both event types
    SELECT COUNT(DISTINCT event_type) INTO valid_event_count
    FROM exclusion_audit_log
    WHERE event_type IN ('exclude', 'unexclude');

    IF valid_event_count != 2 THEN
        RAISE EXCEPTION 'Expected 2 distinct event types (exclude, unexclude), got %', valid_event_count;
    END IF;

    RAISE NOTICE 'Verified: event_type values are correct (exclude, unexclude)';
END $$;

SELECT 'Step 11: Verified event_type values are correct' as status;

-- Step 12: Test CASCADE delete (when repo is deleted, audit entries should be deleted)
DO $$
DECLARE
    audit_count_before INT;
    audit_count_after INT;
    test_repo_id INT;
BEGIN
    -- Get a repo_id to test with
    SELECT repo_id INTO test_repo_id FROM repos WHERE repo_full_name = 'torvalds/linux';

    -- Count audit entries before
    SELECT COUNT(*) INTO audit_count_before
    FROM exclusion_audit_log
    WHERE repo_id = test_repo_id;

    -- Delete the repo (should cascade to audit log)
    DELETE FROM repos WHERE repo_id = test_repo_id;

    -- Count audit entries after
    SELECT COUNT(*) INTO audit_count_after
    FROM exclusion_audit_log
    WHERE repo_id = test_repo_id;

    IF audit_count_after != 0 THEN
        RAISE EXCEPTION 'CASCADE delete failed: expected 0 audit entries, got %', audit_count_after;
    END IF;

    IF audit_count_before = 0 THEN
        RAISE EXCEPTION 'No audit entries found for test repo';
    END IF;

    RAISE NOTICE 'Verified: CASCADE delete works correctly (deleted % audit entries)', audit_count_before;
END $$;

SELECT 'Step 12: Tested CASCADE delete behavior' as status;

-- Step 13: Test down migration (drop table and indexes)
DROP INDEX IF EXISTS exclusion_audit_log_active_exclusions_idx;
DROP INDEX IF EXISTS exclusion_audit_log_actor_idx;
DROP INDEX IF EXISTS exclusion_audit_log_repo_idx;
DROP INDEX IF EXISTS exclusion_audit_log_timestamp_idx;
DROP TABLE IF EXISTS exclusion_audit_log;

SELECT 'Step 13: Applied down migration (dropped table and indexes)' as status;

-- Step 14: Verify table is gone
DO $$
DECLARE
    table_exists INT;
BEGIN
    SELECT COUNT(*) INTO table_exists
    FROM information_schema.tables
    WHERE table_name = 'exclusion_audit_log';

    IF table_exists != 0 THEN
        RAISE EXCEPTION 'exclusion_audit_log table should not exist after down migration';
    END IF;

    RAISE NOTICE 'Verified: exclusion_audit_log table was dropped successfully';
END $$;

SELECT 'Step 14: Verified table was dropped successfully' as status;

-- Final summary
SELECT '=== FINAL STATE ===' as summary;
SELECT 'Test repos remaining (exclusion_audit_log table was dropped):' as info;
SELECT repo_id, provider, repo_full_name, excluded_at IS NOT NULL as is_excluded, excluded_reason
FROM repos
ORDER BY repo_full_name;

SELECT 'SUCCESS: All migration tests passed!' as final_result;

ROLLBACK; -- Rollback to clean up test schema
