-- +goose Up
-- Add exclusion tracking fields to repos table
--
-- This migration ensures the repos table has excluded_at and excluded_reason columns
-- to support repo-level exclusion. These columns allow repositories to be marked
-- as excluded from ranking with a timestamp and optional reason.
--
-- While these columns exist in the initial schema (00001_initial_schema.sql), this
-- migration provides backwards compatibility for any databases that may have been
-- created before the schema was finalized. The migration is idempotent and can be
-- run safely even if the columns already exist.
--
-- Columns:
-- - excluded_at: TIMESTAMPTZ, nullable — when non-NULL, the repo is excluded from ranking
-- - excluded_reason: TEXT, nullable — human-readable explanation for exclusion

-- Add excluded_at column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'repos'
        AND column_name = 'excluded_at'
    ) THEN
        ALTER TABLE repos ADD COLUMN excluded_at TIMESTAMPTZ;
    END IF;
END $$;

-- Add excluded_reason column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'repos'
        AND column_name = 'excluded_reason'
    ) THEN
        ALTER TABLE repos ADD COLUMN excluded_reason TEXT;
    END IF;
END $$;

-- Add comment for documentation (only if column exists)
COMMENT ON COLUMN repos.excluded_at IS 'Timestamp when repo was excluded from ranking (NULL = not excluded)';
COMMENT ON COLUMN repos.excluded_reason IS 'Human-readable reason for exclusion (e.g., ''fork'', ''archived'', ''policy'')';

-- +goose Down
-- Remove exclusion tracking fields from repos table
--
-- WARNING: This will permanently delete any exclusion data.
-- Consider backing up excluded repos before running this migration.

DO $$
BEGIN
    -- Check if column exists before dropping
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'repos'
        AND column_name = 'excluded_at'
    ) THEN
        ALTER TABLE repos DROP COLUMN IF EXISTS excluded_at;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'repos'
        AND column_name = 'excluded_reason'
    ) THEN
        ALTER TABLE repos DROP COLUMN IF EXISTS excluded_reason;
    END IF;
END $$;
