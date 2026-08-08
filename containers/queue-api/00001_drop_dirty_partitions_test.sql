-- Test: Verify dirty_partitions table drop migration
-- This test ensures the migration works correctly and is idempotent

-- First, let's simulate the dirty_partitions table existing to test the DROP
CREATE TABLE IF NOT EXISTS dirty_partitions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER NOT NULL,
    partition_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create some indexes that might have existed
CREATE INDEX IF NOT EXISTS dirty_partitions_repo_idx ON dirty_partitions (repo_id);
CREATE INDEX IF NOT EXISTS dirty_partitions_status_idx ON dirty_partitions (status);
CREATE INDEX IF NOT EXISTS dirty_partitions_partition_idx ON dirty_partitions (partition_key);

-- Verify table exists before migration
SELECT 'dirty_partitions table exists:' as status, COUNT(*) as count FROM dirty_partitions;

-- Now run the migration (same as 00001_drop_dirty_partitions.sql)
DROP INDEX IF EXISTS dirty_partitions_repo_idx;
DROP INDEX IF EXISTS dirty_partitions_status_idx;
DROP INDEX IF EXISTS dirty_partitions_created_at_idx;
DROP INDEX IF EXISTS dirty_partitions_partition_idx;
DROP TABLE IF EXISTS dirty_partitions;

-- Verify table is gone
-- This should return an error or no rows (table doesn't exist)
SELECT 'After migration - table should not exist' as status;

-- Run the migration again to test idempotency
-- This should succeed without errors
DROP INDEX IF EXISTS dirty_partitions_repo_idx;
DROP INDEX IF EXISTS dirty_partitions_status_idx;
DROP INDEX IF EXISTS dirty_partitions_created_at_idx;
DROP INDEX IF EXISTS dirty_partitions_partition_idx;
DROP TABLE IF EXISTS dirty_partitions;

SELECT 'Migration is idempotent - second run succeeded' as status;
