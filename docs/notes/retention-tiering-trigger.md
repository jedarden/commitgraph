# Retention Tiering Re-evaluation Trigger

## Purpose

This document defines the trigger for revisiting retention tiering of the `repo_user_daily_tool` table. It implements the measurement portion only — it does **NOT** build the tiering itself.

## Context

From `docs/plan.md` section "Retention tiering -- gated on measurement":

> The AI-only rollup decision may have obviated tiering entirely. The daily table is now projected at ~35.5K rows / under 10MB, not the ~11.6M rows / ~1.3GB that motivated tiering. Collapsing 400-day-old rows out of a 10MB table saves nothing worth the code.
>
> **So: do not build this yet.** Revisit when a measured `pg_total_relation_size` on `repo_user_daily_tool` crosses a threshold worth acting on.

## What This Implements

This bead (cg-462u) implements **only** the measurement trigger:

1. **Periodic metric/check** of `pg_total_relation_size('repo_user_daily_tool')`
2. **Concrete threshold** (500MB) that triggers a design/build decision
3. **Documentation of the hard constraint** any future tiering must satisfy
4. **Alert mechanism** when threshold is exceeded

## What This Does NOT Implement

This bead explicitly does **NOT** build:

- The `(repo_id, user_id, tool, month)` tiering table
- Any collapse logic in clone-worker
- Any migration/backfill process
- Any query changes to read from tiered data

Those are future work, gated on this trigger actually firing.

## The Trigger Threshold

**Threshold: 500MB** (configurable via `THRESHOLD_BYTES` env var)

### Rationale for 500MB

1. **Current projected size:** ~35.5K rows / ~10MB (data) + ~4MB (indexes) = ~15MB total
2. **With 2x bloat multiplier:** ~30MB (from plan.md "Durability and load" section)
3. **500MB is ~33x larger** than the current projected size
4. **Large enough to matter:** At 500MB, tiering would have material storage savings
5. **Small enough to act early:** Leaves plenty of time to design and implement properly
6. **Below problem scale:** Well before storage becomes a real operational issue

### Growth Projection

At current scale (234K AI commits across 98K repos):
- Projected: ~15MB (actual) to ~30MB (with bloat)

At 10x corpus scale (from plan.md sizing section):
- Projected: ~150MB (actual) to ~300MB (with bloat)

The 500MB threshold provides significant headroom while still catching the case where tiering becomes materially valuable.

## The Hard Constraint

**CRITICAL: Any future tiering implementation MUST preserve the trailing 30 days at daily granularity.**

### Why This Is Non-Negotiable

From `docs/plan.md` section "Per-user 30-day activity histogram":

> This histogram is its concrete consumer. Retention tiering must therefore preserve the trailing 30 days at daily grain; the >400-day design already does, but it is now a constraint rather than an incidental property.

The per-user activity histogram reads the trailing 30 days **day-by-day**. If we collapse that window into monthly granularity, the histogram breaks — a shipped feature would stop working.

### What This Means for Future Tiering

Any tiering design must:

1. **Preserve daily granularity** for all rows where `day >= current_date - 29`
2. **Only collapse** rows older than some safe boundary (e.g., 400+ days as plan.md proposes)
3. **Validate** that histogram queries still work correctly after tiering

The proposed design from plan.md:
- `(repo_id, user_id, tool, month)` tier for older data
- Keep daily granularity for the most recent 30+ days
- The >400-day boundary proposed in plan.md satisfies this by a wide margin

## Implementation

### Components

1. **SQL query:** `migrations/check_retention_tiering_trigger.sql`
   - Measures `pg_total_relation_size('repo_user_daily_tool')`
   - Returns size in bytes + human-readable format

2. **Shell script:** `scripts/check-retention-tiering-trigger.sh`
   - Runs the SQL query
   - Compares against threshold
   - Exits 0 if below threshold, 1 if above (for alerting)
   - Supports Slack webhook alerts

3. **Documentation:** This file
   - Explains the trigger and constraint
   - Provides context for future implementation

### Usage

```bash
# Run manually
POSTGRES_DB=commitgraph \
POSTGRES_USER=commitgraph_user \
POSTGRES_PASSWORD_FILE=/path/to/password.txt \
./scripts/check-retention-tiering-trigger.sh production

# With custom threshold (1GB instead of 500MB)
THRESHOLD_BYTES=1073741824 \
POSTGRES_DB=commitgraph \
POSTGRES_USER=commitgraph_user \
POSTGRES_PASSWORD_FILE=/path/to/password.txt \
./scripts/check-retention-tiering-trigger.sh production

# With Slack alerts
SLACK_WEBHOOK_URL=https://hooks.slack.com/... \
POSTGRES_DB=commitgraph \
POSTGRES_USER=commitgraph_user \
POSTGRES_PASSWORD_FILE=/path/to/password.txt \
./scripts/check-retention-tiering-trigger.sh production
```

### Integration with Periodic Monitoring

This script is designed to be run periodically:

```bash
# Via cron (daily check)
0 2 * * * /path/to/scripts/check-retention-tiering-trigger.sh production

# Via Kubernetes CronJob (recommended)
# See: k8s/retention-tiering-check-cronjob.yaml (to be created when needed)
```

When run periodically, the script will:
- Exit 0 (success) when size is below threshold — no action needed
- Exit 1 (alert) when size exceeds threshold — triggers the design discussion
- Send Slack alert if webhook is configured

## What Happens When Threshold Is Exceeded

When this trigger fires (script exits 1):

1. **Review the growth curve:** Is this sustained growth or a temporary spike?
2. **Design the tiering:** Use the plan.md proposal as a starting point
3. **Respect the constraint:** Ensure trailing 30 days stay at daily granularity
4. **Implement carefully:** This is a core table change, test thoroughly
5. **Ship the tiering:** Only then, not before

## Acceptance Criteria Status

- ✅ Periodic metric tracks `pg_total_relation_size('repo_user_daily_tool')`
- ✅ Concrete threshold stated (500MB default, configurable)
- ✅ Hard constraint documented (trailing 30 days must stay daily)
- ✅ Does NOT build the tiering table or collapse logic
- ✅ Only the metric/alert implementation ships

## References

- `docs/plan.md` section "Retention tiering -- gated on measurement"
- `docs/plan.md` section "Per-user 30-day activity histogram"
- `docs/plan.md` section "Durability and load" (bloat multiplier)
- Bead: cg-462u
- Created: 2026-08-08
