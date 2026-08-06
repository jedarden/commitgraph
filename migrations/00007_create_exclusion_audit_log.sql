-- +goose Up
-- Create exclusion_audit_log table for exclusion action audit trail
--
-- This table provides a durable, queryable audit log for all exclusion and
-- un-exclusion operations on repositories, supporting full state capture with
-- referential integrity to the repos table.
--
-- Every write to repos.excluded_at/excluded_reason must be logged here with
-- before/after state, actor, timestamp, and event type.
--
-- The foreign key to repos(repo_id) ensures that audit entries can only exist
-- for valid repositories and supports cascading deletes if needed.

CREATE TABLE IF NOT EXISTS exclusion_audit_log (
  id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  repo_id             BIGINT NOT NULL REFERENCES repos(repo_id) ON DELETE CASCADE,
  actor               TEXT NOT NULL,               -- who performed the action (e.g., 'admin', 'system')
  timestamp           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  event_type          TEXT NOT NULL,               -- 'exclude' or 'unexclude'

  -- Before state (what was changed from)
  old_excluded_at     TIMESTAMPTZ,
  old_excluded_reason TEXT,

  -- After state (what was changed to)
  new_excluded_at     TIMESTAMPTZ,
  new_excluded_reason TEXT
);

-- Index for querying recent audit events (most recent first)
CREATE INDEX IF NOT EXISTS exclusion_audit_log_timestamp_idx ON exclusion_audit_log (timestamp DESC);

-- Index for querying audit events by repo (for repo-specific audit history)
CREATE INDEX IF NOT EXISTS exclusion_audit_log_repo_idx ON exclusion_audit_log (repo_id, timestamp DESC);

-- Index for querying audit events by actor (for actor-specific audit history)
CREATE INDEX IF NOT EXISTS exclusion_audit_log_actor_idx ON exclusion_audit_log (actor, timestamp DESC);

-- Index for finding active exclusions (where new_excluded_at is NOT NULL)
-- This supports queries like "find all currently excluded repos with their audit trail"
CREATE INDEX IF NOT EXISTS exclusion_audit_log_active_exclusions_idx ON exclusion_audit_log (new_excluded_at) WHERE new_excluded_at IS NOT NULL;

-- Add comments for documentation
COMMENT ON TABLE exclusion_audit_log IS 'Audit log for all repository exclusion/un-exclusion operations with full state capture';
COMMENT ON COLUMN exclusion_audit_log.id IS 'Unique identifier for the audit entry';
COMMENT ON COLUMN exclusion_audit_log.repo_id IS 'Foreign key reference to the repos table';
COMMENT ON COLUMN exclusion_audit_log.actor IS 'Who performed the action (e.g., admin username, system process)';
COMMENT ON COLUMN exclusion_audit_log.timestamp IS 'When the action was performed';
COMMENT ON COLUMN exclusion_audit_log.event_type IS 'Type of event: ''exclude'' or ''unexclude''';
COMMENT ON COLUMN exclusion_audit_log.old_excluded_at IS 'Previous excluded_at value before this action (NULL if not excluded)';
COMMENT ON COLUMN exclusion_audit_log.old_excluded_reason IS 'Previous excluded_reason value before this action (NULL if not excluded or no reason)';
COMMENT ON COLUMN exclusion_audit_log.new_excluded_at IS 'New excluded_at value after this action (NULL if unexcluded)';
COMMENT ON COLUMN exclusion_audit_log.new_excluded_reason IS 'New excluded_reason value after this action (NULL if unexcluded or no reason)';

-- +goose Down
-- Drop exclusion_audit_log table and its indexes

DROP INDEX IF EXISTS exclusion_audit_log_active_exclusions_idx;
DROP INDEX IF EXISTS exclusion_audit_log_actor_idx;
DROP INDEX IF EXISTS exclusion_audit_log_repo_idx;
DROP INDEX IF EXISTS exclusion_audit_log_timestamp_idx;
DROP TABLE IF EXISTS exclusion_audit_log;
