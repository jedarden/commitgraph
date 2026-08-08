-- Retention Tiering Trigger Query
--
-- This query measures the current disk size of repo_user_daily_tool,
-- including both the table data and its indexes. This is the trigger
-- metric for determining when to revisit retention tiering.
--
-- Usage:
--   psql -f migrations/check_retention_tiering_trigger.sql
--
-- Output: A single row with the size in bytes and a human-readable size
--
-- Context: See docs/plan.md section "Retention tiering -- gated on measurement"
-- This does NOT implement tiering — it only measures when it would be worth considering.
--
-- HARD CONSTRAINT (must be preserved in any future tiering implementation):
--   The trailing 30 days MUST remain at daily granularity. The per-user
--   activity histogram (see "Per-user 30-day activity histogram" in plan.md)
--   reads exactly that window day-by-day, so collapsing it would break a
--   shipped feature. Any tiering design must preserve daily granularity
--   for days >= current_date - 29.
--
-- Threshold guidance:
--   - Current projected size: ~35.5K rows / ~10MB (data) + ~4MB (indexes) = < 15MB
--   - With 2x bloat multiplier: ~30MB
--   - Trigger threshold: 500MB (when tiering becomes worth implementing)
--   - At 10x corpus scale: ~1GB (which would trigger tiering consideration)
--
-- The 500MB threshold is chosen because:
--   1. It's ~33x larger than the current projected size (15MB)
--   2. It's large enough that tiering would have material storage savings
--   3. It's small enough that we'd act before storage becomes a real problem
--   4. It leaves plenty of time to design and implement tiering properly

SELECT
    pg_total_relation_size('repo_user_daily_tool') AS size_bytes,
    pg_size_pretty(pg_total_relation_size('repo_user_daily_tool')) AS size_pretty,
    CURRENT_TIMESTAMP AS measured_at;
