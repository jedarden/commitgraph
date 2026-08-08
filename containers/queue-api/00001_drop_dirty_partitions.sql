-- Migration: Drop dirty_partitions table
-- Description: Removes the dirty_partitions table and any associated indexes
-- Database: queue-api (SQLite)
-- Idempotent: Safe to run multiple times

-- Drop any indexes that may have been created on dirty_partitions
-- These use IF EXISTS to be idempotent
DROP INDEX IF EXISTS dirty_partitions_repo_idx;
DROP INDEX IF EXISTS dirty_partitions_status_idx;
DROP INDEX IF EXISTS dirty_partitions_created_at_idx;
DROP INDEX IF EXISTS dirty_partitions_partition_idx;

-- Drop the dirty_partitions table itself
-- Uses IF EXISTS to be idempotent
DROP TABLE IF EXISTS dirty_partitions;
