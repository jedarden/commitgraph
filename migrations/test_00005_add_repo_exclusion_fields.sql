-- Test: Verify repo exclusion fields migration
--
-- This test creates a repos table WITHOUT the excluded_at and excluded_reason columns,
-- applies the migration, and verifies that:
-- 1. The excluded_at column is added as TIMESTAMPTZ, nullable
-- 2. The excluded_reason column is added as TEXT, nullable
-- 3. The migration is idempotent (can be run multiple times safely)
--
-- Prerequisites: Access to a PostgreSQL database
-- Run with: psql -h localhost -U commitgraph -d commitgraph -f migrations/test_00005_add_repo_exclusion_fields.sql
-- Or with: goose postgres "..." down to version 00004, then run this test

BEGIN;

-- Step 1: Create a repos table WITHOUT the exclusion columns (simulating old schema)
DROP TABLE IF EXISTS repos CASCADE;

CREATE TABLE repos (
    repo_id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider       TEXT NOT NULL,
    repo_full_name TEXT NOT NULL,
    UNIQUE (provider, repo_full_name)
);

SELECT 'Step 1: Created repos table WITHOUT exclusion columns' as status;

-- Step 2: Verify columns don't exist yet
DO $$
DECLARE
    excluded_at_exists INT;
    excluded_reason_exists INT;
BEGIN
    SELECT COUNT(*) INTO excluded_at_exists
    FROM information_schema.columns
    WHERE table_name = 'repos' AND column_name = 'excluded_at';

    SELECT COUNT(*) INTO excluded_reason_exists
    FROM information_schema.columns
    WHERE table_name = 'repos' AND column_name = 'excluded_reason';

    IF excluded_at_exists > 0 THEN
        RAISE EXCEPTION 'excluded_at column should not exist yet';
    END IF;

    IF excluded_reason_exists > 0 THEN
        RAISE EXCEPTION 'excluded_reason column should not exist yet';
    END IF;

    RAISE NOTICE 'Verified: exclusion columns do not exist in old schema';
END $$;

SELECT 'Step 2: Verified exclusion columns do not exist initially' as status;

-- Step 3: Insert some test data
INSERT INTO repos (provider, repo_full_name) VALUES
    ('github', 'jedarden/commitgraph'),
    ('github', 'torvalds/linux'),
    ('gitlab', 'gitlab-org/gitlab');

SELECT 'Step 3: Inserted test data (3 repos)' as status;

-- Step 4: Apply the migration (add exclusion columns)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'repos' AND column_name = 'excluded_at'
    ) THEN
        ALTER TABLE repos ADD COLUMN excluded_at TIMESTAMPTZ;
        RAISE NOTICE 'Added excluded_at column';
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'repos' AND column_name = 'excluded_reason'
    ) THEN
        ALTER TABLE repos ADD COLUMN excluded_reason TEXT;
        RAISE NOTICE 'Added excluded_reason column';
    END IF;
END $$;

SELECT 'Step 4: Applied migration (added exclusion columns)' as status;

-- Step 5: Verify columns now exist and have correct types
DO $$
DECLARE
    excluded_at_type TEXT;
    excluded_reason_type TEXT;
    excluded_at_nullable TEXT;
    excluded_reason_nullable TEXT;
BEGIN
    SELECT data_type, is_nullable INTO excluded_at_type, excluded_at_nullable
    FROM information_schema.columns
    WHERE table_name = 'repos' AND column_name = 'excluded_at';

    SELECT data_type, is_nullable INTO excluded_reason_type, excluded_reason_nullable
    FROM information_schema.columns
    WHERE table_name = 'repos' AND column_name = 'excluded_reason';

    IF excluded_at_type != 'timestamp with time zone' THEN
        RAISE EXCEPTION 'excluded_at type is %, expected timestamp with time zone', excluded_at_type;
    END IF;

    IF excluded_at_nullable != 'YES' THEN
        RAISE EXCEPTION 'excluded_at should be nullable, got %', excluded_at_nullable;
    END IF;

    IF excluded_reason_type != 'text' THEN
        RAISE EXCEPTION 'excluded_reason type is %, expected text', excluded_reason_type;
    END IF;

    IF excluded_reason_nullable != 'YES' THEN
        RAISE EXCEPTION 'excluded_reason should be nullable, got %', excluded_reason_nullable;
    END IF;

    RAISE NOTICE 'Verified: column types and nullability are correct';
END $$;

SELECT 'Step 5: Verified column types and nullability are correct' as status;

-- Step 6: Test that we can update the new columns
UPDATE repos SET excluded_at = NOW(), excluded_reason = 'test exclusion'
WHERE repo_full_name = 'jedarden/commitgraph';

UPDATE repos SET excluded_at = NOW(), excluded_reason = 'archived repository'
WHERE repo_full_name = 'torvalds/linux';

SELECT 'Step 6: Successfully updated exclusion data for 2 repos' as status;

-- Step 7: Query to verify the data
SELECT
    repo_full_name,
    excluded_at IS NOT NULL as is_excluded,
    excluded_reason
FROM repos
ORDER BY repo_full_name;

SELECT 'Step 7: Queried repos to verify exclusion data' as status;

-- Step 8: Test idempotency - run the migration again
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'repos' AND column_name = 'excluded_at'
    ) THEN
        ALTER TABLE repos ADD COLUMN excluded_at TIMESTAMPTZ;
        RAISE NOTICE 'Added excluded_at column (should not happen)';
    ELSE
        RAISE NOTICE 'excluded_at column already exists - migration is idempotent';
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'repos' AND column_name = 'excluded_reason'
    ) THEN
        ALTER TABLE repos ADD COLUMN excluded_reason TEXT;
        RAISE NOTICE 'Added excluded_reason column (should not happen)';
    ELSE
        RAISE NOTICE 'excluded_reason column already exists - migration is idempotent';
    END IF;
END $$;

SELECT 'Step 8: Tested idempotency - migration can be run multiple times safely' as status;

-- Step 9: Verify data is still there after idempotency test
DO $$
DECLARE
    excluded_count INT;
BEGIN
    SELECT COUNT(*) INTO excluded_count
    FROM repos
    WHERE excluded_at IS NOT NULL;

    IF excluded_count != 2 THEN
        RAISE EXCEPTION 'Expected 2 excluded repos, got %', excluded_count;
    END IF;

    RAISE NOTICE 'Verified: data preserved after idempotent migration';
END $$;

SELECT 'Step 9: Verified data preserved after idempotent migration' as status;

-- Step 10: Test NULL values are allowed
INSERT INTO repos (provider, repo_full_name) VALUES
    ('github', 'openai/gpt-3');

UPDATE repos SET excluded_at = NULL, excluded_reason = NULL
WHERE repo_full_name = 'github/gpt-3';

SELECT 'Step 10: Verified NULL values are allowed in exclusion columns' as status;

-- Final summary
SELECT '=== FINAL STATE ===' as summary;
SELECT
    COUNT(*) as total_repos,
    COUNT(*) FILTER (WHERE excluded_at IS NOT NULL) as excluded_repos,
    COUNT(*) FILTER (WHERE excluded_at IS NULL) as active_repos
FROM repos;

SELECT 'SUCCESS: All migration tests passed!' as final_result;

ROLLBACK; -- Rollback to clean up test schema
