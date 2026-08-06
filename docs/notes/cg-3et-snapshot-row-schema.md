# Published Snapshot Row Schema (cg-3et)

**Decision finalized:** 2026-08-05

**Purpose:** Single source of truth for the Parquet snapshot row schema consumed by `agg-snapshot-export-parquet`.

## Per-Row Fields

The published Parquet snapshot row consists of 11 scalar/array fields derived from the live JSON schema, plus one 30-element histogram array.

### Core Fields (inherited from live JSON schema)

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| `rank` | INT | Computed by `ROW_NUMBER() OVER (ORDER BY ai_commits_30d DESC)` | Ranking field |
| `username` | TEXT | `users.login` | Canonical GitHub login |
| `ai_commits_30d` | BIGINT | `SUM(commits)` from `repo_user_daily_tool` WHERE `day > current_date - 30` | Primary ranking metric |
| `ai_commits_total` | BIGINT | `SUM(commits)` from `repo_user_daily_tool` (all-time) | Lifetime AI commits |
| `ship_streak` | INT | Computed from `repo_user_daily_tool` | Max consecutive days with AI commits |
| `tools` | TEXT[] | `ARRAY_AGG(DISTINCT tool)` from `repo_user_daily_tool` | Distinct AI tools used by developer |
| `providers` | TEXT[] | `ARRAY_AGG(DISTINCT provider)` from `repos` joined via `repo_user_daily_tool.repo_id` | Git hosts (e.g., `["github"]`) |
| `last_active` | DATE | `MAX(day)` from `repo_user_daily_tool` | Latest day with AI commits |
| `verified` | BOOLEAN | Future hardening (see `docs/runbooks/repo-exclusion.md#L220`) | Whether GitHub email is verified |

### New Fields (added for snapshot)

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| `last_scanned_at` | TIMESTAMPTZ | `MAX(insert_time)` from `repo_user_daily_tool` | When developer's repos were last scanned (scan recency, not commit recency) |
| `daily_ai_commits` | INT[30] | Computed from `repo_user_daily_tool` | 30-day trailing histogram of AI commits per day |

### Explicitly Excluded Field

- **`top_repo`** - Deliberately removed (see `docs/plan/plan.md#L855-906`). This field computed an all-time argmax repo name, which was asymmetrical with the 30-day windowed `ai_commits_30d`. Reinstating it requires a deliberate product decision on the window scope.

## File-Level Metadata (Parquet footer / Parquet metadata)

These fields are stored once per snapshot file, not per row:

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| `window_start` | DATE | `current_date - 29` | First day of the 30-day histogram window (all rows share the same window) |
| `totals.commits` | BIGINT | `corpus_stats` WHERE `stat = 'commits'` | Total AI commits in corpus |
| `totals.developers` | BIGINT | `corpus_stats` WHERE `stat = 'developers'` | Total developers with AI commits |
| `totals.repositories` | BIGINT | `corpus_stats` WHERE `stat = 'repositories'` | Total repositories with AI commits |
| `percentiles.p50_ai_commits_30d` | BIGINT | Computed from `ai_commits_30d` distribution | Median AI commits (30d) |
| `percentiles.p75_ai_commits_30d` | BIGINT | Computed from `ai_commits_30d` distribution | 75th percentile |
| `percentiles.p90_ai_commits_30d` | BIGINT | Computed from `ai_commits_30d` distribution | 90th percentile |
| `percentiles.p95_ai_commits_30d` | BIGINT | Computed from `ai_commits_30d` distribution | 95th percentile |
| `percentiles.p99_ai_commits_30d` | BIGINT | Computed from `ai_commits_30d` distribution | 99th percentile |

## Field Sourcing Detail

### `tools[]`
```sql
-- From repo_user_daily_tool, grouped by user_id
ARRAY_AGG(DISTINCT tool ORDER BY tool)
```
Distinct AI tools the developer has used, alphabetically ordered.

### `providers[]`
```sql
-- From repos joined via repo_user_daily_tool
ARRAY_AGG(DISTINCT provider ORDER BY provider)
```
Git hosts for repositories containing the developer's AI commits. Typically `["github"]` alone.

### `ship_streak`
```sql
-- Computed from repo_user_daily_tool
-- Max consecutive days with commits > 0
-- Implementation: window function over day sequences
```
Longest streak of consecutive days with at least one AI commit. Derived from the rollup's `day` series.

### `verified`
**Status:** Future hardening, not currently implemented. See `docs/runbooks/repo-exclusion.md#L220` for discussion. Would indicate whether the developer's GitHub email is verified via GitHub's verification system.

### `daily_ai_commits[30]`
```sql
-- Histogram: one element per day in trailing 30-day window
-- Index 0 = oldest day (window_start), index 29 = most recent day (current_date)
-- Modeled after claude-leaderboard's implementation (vibecodeleaderboard-backend/src/generate_leaderboard_v3.py:387-398)
```
Array of 30 integers representing AI commits per day, anchored on a board-wide window (same `window_start` for all rows). Zeros for days with no activity.

## Cross-References

- **`top_repo` exclusion rationale:** `docs/plan/plan.md#L855-906` (decided 2026-08-05)
- **agg-top-repo-exclusion-guard:** Implementation that prevents `top_repo` from being added without deliberate reconsideration
- **agg-snapshot-export-parquet:** Consumer of this schema definition
- **Live JSON schema reference:** `docs/plan/plan.md#L616-617` (verified against `~/backups/commitgraph-cutover/leaderboard.json`)

## Sizing

- **Per-row uncompressed:** ~110-115 bytes (scalar fields)
- **Per-row compressed:** ~35-50 bytes (Parquet with RLE/dictionary encoding)
- **Histogram uncompressed:** ~120 bytes (30 × 4-byte INT)
- **Histogram compressed:** ~10-30 bytes/row (heavy zeros, RLE-optimized)
- **Full snapshot estimate:** 15-50 MB for hundreds of thousands of rows

See `docs/plan/plan.md#L989-1006` for detailed sizing breakdown.

## Implementation Notes

1. **`window_start` is singular** - all rows share the same 30-day window, computed once per snapshot
2. **`daily_ai_commits` indexing** - index 0 is oldest, index 29 is most recent (matches claude-leaderboard)
3. **Progressive-recency tiebreaker** - optional enhancement for ranking ties (plan.md#L980-982)
4. **Scan vs. commit recency** - `last_scanned_at` (scan time) and `last_active` (commit day) serve different purposes (plan.md#L776-793)
