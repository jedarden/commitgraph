-- +goose Up
-- commitgraph v2 initial Postgres schema
-- Source of truth: docs/plan/plan.md#Postgres-schema
--
-- This migration creates all six identity/rollup tables that form the core
-- of the commitgraph v2 database:
-- - repos: repository identity with exclusion tracking
-- - users: developer identity (no counter columns - that's in repo_user_daily_tool)
-- - email_resolution: email→login resolution results
-- - user_aliases: login→login alias mapping
-- - repo_user_daily_tool: the main rollup (AI-tool-tagged commits only)
-- - corpus_stats: global scalar totals

CREATE TABLE IF NOT EXISTS repos (
  repo_id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider       TEXT NOT NULL,
  repo_full_name TEXT NOT NULL,       -- e.g. owner/name
  excluded_at    TIMESTAMPTZ,         -- non-NULL = excluded from ranking
  excluded_reason TEXT,
  UNIQUE (provider, repo_full_name)
);

CREATE TABLE IF NOT EXISTS users (
  user_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  login      TEXT NOT NULL UNIQUE,    -- canonical GitHub login
  profile_url TEXT,
  avatar_url  TEXT
);

-- Resolution RESULTS (the work queue stays in queue-api)
CREATE TABLE IF NOT EXISTS email_resolution (
  email       TEXT PRIMARY KEY,
  login       TEXT NOT NULL,
  source      TEXT NOT NULL,          -- 'live' | 'seed' | 'manual'
  resolved_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS email_resolution_login_idx ON email_resolution (login);

CREATE TABLE IF NOT EXISTS user_aliases (
  source_login TEXT PRIMARY KEY,
  target_login TEXT NOT NULL,
  reason       TEXT NOT NULL,         -- 'admin' | 'name-match'
  created_at   TIMESTAMPTZ NOT NULL
);

-- The rollup: AI-tool-tagged commits only
CREATE TABLE IF NOT EXISTS repo_user_daily_tool (
  repo_id     BIGINT NOT NULL REFERENCES repos(repo_id),
  user_id     BIGINT NOT NULL REFERENCES users(user_id),
  tool        TEXT   NOT NULL,        -- plain TEXT, not enum — catalog grows
  day         DATE   NOT NULL,
  commits     INT    NOT NULL,
  insert_time TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (repo_id, user_id, tool, day)
);
CREATE INDEX IF NOT EXISTS repo_user_daily_tool_user_tool_day_idx ON repo_user_daily_tool (user_id, tool, day);
CREATE INDEX IF NOT EXISTS repo_user_daily_tool_tool_day_idx ON repo_user_daily_tool (tool, day);
CREATE INDEX IF NOT EXISTS repo_user_daily_tool_user_insert_time_idx ON repo_user_daily_tool (user_id, insert_time);

CREATE TABLE IF NOT EXISTS corpus_stats (           -- the three `totals` scalars
  stat  TEXT PRIMARY KEY,             -- 'commits' | 'developers' | 'repositories'
  value BIGINT NOT NULL
);

-- +goose Down
-- Reverse of initial schema (not used for forward-only migrations)

DROP INDEX IF EXISTS repo_user_daily_tool_user_insert_time_idx;
DROP INDEX IF EXISTS repo_user_daily_tool_tool_day_idx;
DROP INDEX IF EXISTS repo_user_daily_tool_user_tool_day_idx;
DROP TABLE IF EXISTS repo_user_daily_tool;
DROP TABLE IF EXISTS user_aliases;
DROP INDEX IF EXISTS email_resolution_login_idx;
DROP TABLE IF EXISTS email_resolution;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS repos;
DROP TABLE IF EXISTS corpus_stats;
