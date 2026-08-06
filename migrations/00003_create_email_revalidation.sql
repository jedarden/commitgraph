-- +goose Up
-- email_revalidation: track email_resolution rows needing login-liveness rechecks
--
-- Edge case #6 from docs/plan/plan.md: "Login renamed after resolution —
-- email_resolution now points at a dead login. Needs revalidation, which the
-- predecessor tracked in a username_revalidation table; carry the concept
-- forward."
--
-- This table tracks which cached email→login resolutions need periodic
-- revalidation against the GitHub API, to detect renamed or deleted accounts
-- that would otherwise silently corrupt identity resolution.
--
-- Relationship to other tables:
-- - email_resolution: the cached resolution being validated (primary key linkage)
-- - user_aliases: NOT duplicated — user_aliases is a deliberate merge ("these
--   logins are the same person"), while email_revalidation is a correctness
--   check ("is this cached resolution still accurate?"). A status='renamed'
--   result feeds user_aliases but does not replace it.
--
-- Lifecycle:
-- 1. Seed: when email_resolution is populated, register email for revalidation
--    (status='pending', next_check_at=NULL for immediate first check).
-- 2. Claim: a worker picks due rows (next_check_at IS NULL or past, status !=
--    'deleted') in priority order (pending first, then oldest last_checked_at).
-- 3. Record: the worker checks the login against GitHub and records the outcome:
--    - 'validated': login is live and current (refresh last_checked_at,
--      next_check_at = now + 90 days)
--    - 'renamed': login was renamed (set new_login, next_check_at = NULL,
--      caller updates email_resolution and optionally creates user_aliases)
--    - 'deleted': account is gone (next_check_at = NULL, stop rechecking)
--    - 'retry': transient failure (rate limit, network error — short backoff,
--      last_checked_at unchanged so retry doesn't erase prior validation)

CREATE TABLE IF NOT EXISTS email_revalidation (
  email           TEXT NOT NULL PRIMARY KEY,     -- References email_resolution.email
  login           TEXT NOT NULL,                 -- The login being revalidated (denormalized for performance)
  last_checked_at TIMESTAMPTZ NOT NULL,          -- When we last performed a check
  next_check_at   TIMESTAMPTZ,                   -- When to check next (NULL = stop checking)
  status          TEXT NOT NULL,                 -- 'pending' | 'validated' | 'renamed' | 'deleted' | 'retry'
  new_login       TEXT,                          -- If status='renamed', what the account is now called
  check_error     TEXT,                          -- If status='retry', why the check failed (rate limit, network, etc.)
  created_at      TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp()
);

-- Index for claiming due rows (worker's hot path)
CREATE INDEX IF NOT EXISTS email_revalidation_next_check_at_idx
  ON email_revalidation (next_check_at) WHERE next_check_at IS NOT NULL AND status NOT IN ('deleted', 'renamed');

-- Index for finding unvalidated pending rows (seed → first claim path)
CREATE INDEX IF NOT EXISTS email_revalidation_status_idx
  ON email_revalidation (status) WHERE status = 'pending';

-- Index for finding all revalidations for a specific login (debugging / manual review)
CREATE INDEX IF NOT EXISTS email_revalidation_login_idx
  ON email_revalidation (login);

-- Comment for documentation (Postgres stores this as table metadata)
COMMENT ON TABLE email_revalidation IS 'Track email_resolution rows requiring periodic login-liveness revalidation to detect renamed/deleted GitHub accounts';

-- +goose Down
DROP INDEX IF EXISTS email_revalidation_login_idx;
DROP INDEX IF EXISTS email_revalidation_status_idx;
DROP INDEX IF EXISTS email_revalidation_next_check_at_idx;
DROP TABLE IF EXISTS email_revalidation;
