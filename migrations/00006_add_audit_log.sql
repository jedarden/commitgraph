-- +goose Up
-- Create audit_log table for exclusion action audit trail
--
-- This table provides a durable, queryable audit log for all exclusion and
-- un-exclusion operations, supporting the accountability mechanism described
-- in plan.md's threat model section ("residual risk... exclusion is reactive").
--
-- Every write to repos.excluded_at/excluded_reason must be logged here with
-- before/after state, actor, timestamp, and reason.

CREATE TABLE IF NOT EXISTS audit_log (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  operation       TEXT NOT NULL,          -- 'exclude' or 'clear'
  provider        TEXT NOT NULL,
  repo_full_name  TEXT NOT NULL,
  operator        TEXT NOT NULL,          -- who performed the action
  reason          TEXT,                   -- exclusion reason (NULL for clear operations)
  incident_id     TEXT,                   -- optional incident tracking reference

  -- Before state (what was changed from)
  excluded_before TIMESTAMPTZ,
  reason_before   TEXT,

  -- After state (what was changed to)
  excluded_after  TIMESTAMPTZ,
  reason_after    TEXT,

  rows_affected   INT NOT NULL           -- 1 if repo existed, 0 if not found
);

-- Index for querying recent audit events
CREATE INDEX IF NOT EXISTS audit_log_timestamp_idx ON audit_log (timestamp DESC);

-- Index for querying audit events by repo (for repo-specific audit history)
CREATE INDEX IF NOT EXISTS audit_log_repo_idx ON audit_log (provider, repo_full_name, timestamp DESC);

-- Index for querying audit events by operator (for operator-specific audit history)
CREATE INDEX IF NOT EXISTS audit_log_operator_idx ON audit_log (operator, timestamp DESC);

-- Index for finding long-standing exclusions (for periodic alerting)
-- This supports queries like "find repos excluded for > 30 days without a clear event"
CREATE INDEX IF NOT EXISTS audit_log_excluded_after_idx ON audit_log (excluded_after) WHERE excluded_after IS NOT NULL;

-- +goose Down
-- Drop audit_log table and its indexes

DROP INDEX IF EXISTS audit_log_excluded_after_idx;
DROP INDEX IF EXISTS audit_log_operator_idx;
DROP INDEX IF EXISTS audit_log_repo_idx;
DROP INDEX IF EXISTS audit_log_timestamp_idx;
DROP TABLE IF EXISTS audit_log;
