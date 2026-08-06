-- queue-api schema
-- Work queue for repository cloning and processing tasks
-- This is a separate service from the main commitgraph database

CREATE TABLE IF NOT EXISTS repo_queue (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider        TEXT NOT NULL,
  repo_full_name  TEXT NOT NULL,       -- e.g. owner/name
  kind            TEXT NOT NULL DEFAULT 'normal-clone', -- 'normal-clone' | 'fork-clone' | 'mirror-clone' | etc.
  status          TEXT NOT NULL DEFAULT 'pending',     -- 'pending' | 'processing' | 'completed' | 'failed'
  claimed_at      TIMESTAMPTZ,
  completed_at    TIMESTAMPTZ,
  error_message   TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp()
);

CREATE INDEX IF NOT EXISTS repo_queue_status_idx ON repo_queue (status);
CREATE INDEX IF NOT EXISTS repo_queue_kind_idx ON repo_queue (kind);
CREATE INDEX IF NOT EXISTS repo_queue_created_at_idx ON repo_queue (created_at);
CREATE INDEX IF NOT EXISTS repo_queue_provider_repo_idx ON repo_queue (provider, repo_full_name);
