# Histogram Scope Decision (cg-1gx)

**Decision finalized:** 2026-08-06

## Decision

**Scope:** The published Parquet snapshot carries the 30-element `daily_ai_commits` histogram array for **every ranked row** in the full list (hundreds of thousands of users), NOT just for a bounded top-N subset.

## Rationale

### Storage cost is negligible
- Full list histogram: **~3-9MB** (300K rows × 10-30 bytes/row compressed)
- Top-N subset (e.g., N=10K): **~0.1-0.3MB**
- The ~3-9MB difference is not a material storage concern for ARMOR, which is designed for multi-gigabyte corpus objects

### Completeness enables flexibility
- The snapshot is an **internal pipeline artifact**, not a public-facing web asset
- Downstream consumers (e.g., devimprint.com) can filter to their own top-N threshold without recomputing the histogram
- Future consumers may need different cutoffs or want to analyze long-tail patterns
- Computing once for all rows is more efficient than supporting multiple N values or recomputing on demand

### Alignment with prior decisions
- The snapshot already publishes the **full ranked list** (not top-N) — see `plan.md` ~L835: "full ranked list (every user with rollup activity, expected to be hundreds of thousands of rows, not a top-N cut)"
- Having per-row data for rank/username/ai_commits_30d but not for the histogram would create an **inconsistency**: consumers can see a user's rank but not their activity pattern
- If rank #10,001 has `ai_commits_30d` = 42, consumers can reasonably ask "how are those 42 commits distributed across the 30 days?" — the histogram should answer that

### Consumer simplicity
- claude-leaderboard's `top_usernames`/`top_candidates` scoping exists because it directly serves a public leaderboard UI
- This snapshot serves an **internal pipeline** whose consumer (devimprint.com generator) applies its own presentation logic (curated top-N, pagination, anti-scraping, etc.)
- That consumer should decide what subset to display, not have the upstream data pre-filtered

### Future-proofing
- Analytics use cases (trend analysis, user cohort studies, burst detection) benefit from complete data
- Recomputing the histogram after the fact would require re-querying `repo_user_daily_tool` for 30 days across all users — far more expensive than storing it upfront
- The histogram enables queries like "show me users who had a burst of activity in the last 7 days" across the entire corpus, not just top-N

## Sizing Consequence

From `plan.md` L1008-1013:

- **Per-row compressed:** ~10-30 bytes (30 small integers, heavily zeros, RLE/dictionary encoding)
- **Full list (300K rows):** **+3-9MB**, taking snapshot from ~15-50MB (scalars) to **~18-59MB total**
- **Still not a storage concern**, but doubles the per-row payload

## Implementation

### Aggregation query
- Compute `daily_ai_commits[30]` for **all rows** in the ranking query, no `LIMIT` or `WHERE` filter on rank
- Use the same window logic as claude-leaderboard (board-wide 30-day window anchored on `current_date`)

### Schema consistency
- `cg-3et-snapshot-row-schema.md` already defines the histogram as a per-row field — this confirms that field applies to **all rows** in the published snapshot
- No divergence between the schema definition and the aggregation query

### Cross-references
- **Aggregation query:** `agg-histogram-query` (to be implemented) — must compute for full list
- **Schema decision:** `cg-3et-snapshot-row-schema.md` — already specifies histogram as per-row field
- **Plan context:** `plan.md` L975-979 (consequence 2: "Scope the computation to published rows") — this decision resolves that open question

## Revisit triggers

This decision should be revisited if:
1. ARMOR storage costs become a constraint (unlikely: corpus objects are multi-GB)
2. A consumer demonstrates that the full histogram is never used AND the storage difference becomes material
3. The histogram computation time becomes a bottleneck (unlikely: one 30-day aggregation per user vs. continuous writes)

## Alternative rejected: Top-N subset

**What was rejected:** Computing the histogram only for a bounded subset (e.g., top 10K users by rank).

**Why rejected:**
- Creates inconsistency: rank and ai_commits_30d exist for all users, but their activity pattern does not
- Requires choosing an arbitrary N; different consumers may want different cutoffs
- Forces recomputation if N changes or if a new consumer wants a different slice
- Minimal storage savings (~3-9MB) do not justify the completeness trade-off
- Misaligns with the "full ranked list" decision already made for the snapshot

## Decision record links

- **Bead:** cg-1gx (per-user histogram scope decision)
- **Schema:** cg-3et (snapshot row schema)
- **Plan:** `plan.md` L967-979 (consequence 2), L997-1013 (sizing)
