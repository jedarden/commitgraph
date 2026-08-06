-- Test migration for cg-aiyi4: Verify UNIQUE constraint on (provider, repo_full_name, kind)
-- This tests the migration from commit da022fc against the live schema

-- Step 1: Create a test database from the live schema (without the constraint)
DROP TABLE IF EXISTS repo_queue;

CREATE TABLE repo_queue (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider        TEXT NOT NULL,
  repo_full_name  TEXT NOT NULL,
  kind            TEXT NOT NULL DEFAULT 'normal-clone',
  status          TEXT NOT NULL DEFAULT 'pending',
  claimed_at      TIMESTAMPTZ,
  completed_at    TIMESTAMPTZ,
  error_message   TEXT,
  created_at      TIMESTAMPTZ NOT DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Step 2: Insert test data representing existing rows in the database
INSERT INTO repo_queue (provider, repo_full_name, kind, status) VALUES
('github', 'owner/repo1', 'normal-clone', 'pending'),
('github', 'owner/repo2', 'normal-clone', 'completed'),
('gitlab', 'owner/repo3', 'fork-clone', 'pending'),
('github', 'owner/repo4', 'redetect', 'pending'),
('github', 'owner/repo1', 'redetect', 'pending');  -- Same repo, different kind - should be allowed

-- Step 3: Verify data was inserted correctly
SELECT 'Before migration - row count:' as step, COUNT(*) as count FROM repo_queue;
SELECT 'Before migration - sample data:' as step, provider, repo_full_name, kind, status FROM repo_queue ORDER BY id;

-- Step 4: Apply the migration - add the UNIQUE constraint
ALTER TABLE repo_queue ADD CONSTRAINT repo_queue_provider_repo_kind UNIQUE (provider, repo_full_name, kind);

-- Step 5: Verify the constraint was applied successfully
SELECT 'After migration - constraint exists:' as step,
       constraint_name, constraint_type
FROM information_schema.table_constraints
WHERE table_name = 'repo_queue' AND constraint_name = 'repo_queue_provider_repo_kind';

-- Step 6: Verify all existing data is preserved
SELECT 'After migration - all rows preserved:' as step, COUNT(*) as count FROM repo_queue;
SELECT 'After migration - data integrity:' as step, provider, repo_full_name, kind, status FROM repo_queue ORDER BY id;

-- Step 7: Test that the constraint works - try to insert duplicate (should fail)
-- This should fail because we already have (github, owner/repo1, normal-clone)
DO $$
BEGIN
    INSERT INTO repo_queue (provider, repo_full_name, kind, status)
    VALUES ('github', 'owner/repo1', 'normal-clone', 'pending');
    RAISE EXCEPTION 'Constraint did NOT prevent duplicate - FAIL';
EXCEPTION WHEN unique_violation THEN
    RAISE NOTICE 'SUCCESS: Constraint correctly prevented duplicate of same kind';
END $$;

-- Step 8: Test that different kinds for same repo are allowed (should succeed)
INSERT INTO repo_queue (provider, repo_full_name, kind, status)
VALUES ('github', 'owner/repo2', 'redetect', 'pending');

-- Step 9: Verify the insert succeeded
SELECT 'After test insert - different kind allowed:' as step, provider, repo_full_name, kind, status
FROM repo_queue
WHERE repo_full_name = 'owner/repo2'
ORDER BY kind;

-- Final summary
SELECT '=== VERIFICATION COMPLETE ===' as step;
SELECT 'Total rows in repo_queue:' as step, COUNT(*) as count FROM repo_queue;
SELECT 'Unique (provider, repo_full_name, kind) combinations:' as step, COUNT(DISTINCT (provider, repo_full_name, kind)) as count FROM repo_queue;
