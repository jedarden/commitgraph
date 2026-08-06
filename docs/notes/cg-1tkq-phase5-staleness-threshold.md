# Phase 5 Public Staleness Threshold Decision

**Decision Date:** 2026-08-06  
**Bead:** cg-1tkq  
**Status:** OPERATOR DECISION - FINAL

## The Question

From `docs/plan/plan.md` Phase 5: *"How long the frozen public `leaderboard.json` can stay frozen before the downstream presentation layer must ship or the file must be pulled."*

## The Baseline

**Golden snapshot generation time:** `2026-08-03T22:05:42Z`  
**File location:** `~/backups/commitgraph-cutover/leaderboard-golden-2026-08-03T22-05-42Z.json`  
**Current serving:** `commitgraph.jedarden.com` serves this frozen file  
**Age as of 2026-08-06:** 3 days

## The Decision

### Maximum Staleness Threshold: **30 calendar days**

**Absolute deadline:** `2026-09-02T22:05:42Z` (30 days after golden snapshot generation)

**Intermediate review checkpoint:** `2026-08-17T22:05:42Z` (14 days after golden snapshot generation)

### Concrete Trigger Rules

1. **Before 14 days (2026-08-17):** No action required. Pipeline build continues normally.

2. **At 14 days (2026-08-17) - REVIEW CHECKPOINT:**
   - Assess Phase 5 progress: has discovery restarted?
   - Assess downstream presentation layer: has it started?
   - If both are "no progress" → escalate to operator with two options:
     - **Option A:** Implement the minimal public-serving fallback (H1 from ideas-ledger: top-100 with anti-scraping design)
     - **Option B:** Pull the frozen `leaderboard.json` and serve a "under reconstruction" message instead

3. **At 30 days (2026-09-02) - HARD DEADLINE:**
   - If the new pipeline is not publishing fresh snapshots by this date → **pull the frozen leaderboard.json**
   - Replace public serving with a static page stating:
     - "The leaderboard is temporarily offline during a major infrastructure upgrade"
     - Show the golden snapshot generation date (2026-08-03) so users understand the context
     - No estimated completion date (to avoid setting expectations we may miss)

## Rationale

### Why 30 days?

1. **Well below "clearly unacceptable":** The plan explicitly states "two-month-old rankings is worse than serving nothing." Thirty days is safely under that threshold while giving the build time to complete.

2. **Realistic build timeline:** Looking at the phased rollout (Phases 0-5), a 30-day window acknowledges:
   - Phase 0 (provisioning, gates): 3-5 days
   - Phase 1 (isolated build): 5-7 days
   - Phase 2 (subset validation): 3-5 days
   - Phase 3 (bulk corpus migration): 7-10 days
   - Phase 4 (validation burn-in): 5-7 days
   - Phase 5 (restart discovery): **cutover must happen by day 30 to meet this deadline**

3. **Clear escalation point:** The 14-day checkpoint prevents a surprise at day 30. If we're not halfway there by day 14, we know we need to act.

4. **Better to pull than serve garbage:** Serving month-old rankings without a timestamp is misleading. The "under reconstruction" message is honest and manages expectations.

### Why not shorter?

- **14 days is too tight:** The full migration (Phase 3) alone is estimated at 7-10 days for 76.6M commits across 98,747 repos. A 14-day total window creates unnecessary pressure and risk of cutting corners.
- **21 days is arbitrary:** 30 days is a round number that maps cleanly to one month, making it easier to communicate and remember.

### Why not longer?

- **45 days is too close to "two months":** The plan uses "two months" as the clearly unacceptable threshold. Forty-five days is only halfway there, but it signals we're willing to let data get very stale.
- **No downstream presentation layer exists:** The devimprint presentation layer is explicitly "not-yet-started" and out of scope for this plan. Waiting for it is not a viable strategy.

## Implementation Reference

**Status: IMPLEMENTED (2026-08-06)**

The `mig-phase5-staleness-alert` implementation exists at:
`migration/mig_phase5_staleness_alert.py`

The script:
- Monitors staleness of the frozen leaderboard.json
- References this decision document in its docstring and output
- Implements all three alert levels (INFO/WARNING/CRITICAL)
- Supports both human-readable and JSON output formats
- Can be integrated with monitoring systems via `--exit-code-on-critical`

Usage:
```bash
# Check current staleness status
python3 migration/mig_phase5_staleness_alert.py

# JSON output for automation
python3 migration/mig_phase5_staleness_alert.py --format json

# Exit with code 1 on CRITICAL (for monitoring integration)
python3 migration/mig_phase5_staleness_alert.py --exit-code-on-critical
```

The implementation correctly references this decision's:
- Golden snapshot time: `2026-08-03T22:05:42Z`
- Maximum staleness threshold: 30 days
- Review checkpoint: 14 days
- All three alert level triggers and actions

## Communication

If the hard deadline is reached and public serving is pulled:

- **Commitgraph repository:** Update README.md with a banner explaining the downtime
- **Commitgraph.jedarden.com:** Serve the reconstruction message with the golden snapshot date
- **Internal communication:** Escalate to operator with the status of all phases and what's blocking

## Related Decisions

- **H1 (public top-100 with explainability):** Not adopted into core plan, but remains available as the "minimal public-serving fallback" if the 14-day review shows we need it. See `docs/notes/public-leaderboard-with-explainability.md`.
- **Downstream presentation layer:** Explicitly out of scope. This decision assumes that layer will not ship in time to affect this threshold.

## Revision History

- **2026-08-06:** Initial decision. Set 30-day maximum with 14-day review checkpoint.
