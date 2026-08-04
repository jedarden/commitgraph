# claude-leaderboard: the proven predecessor this redesign's core patterns are drawn from

Source: `vibecodeleaderboard-backend` (private repo, backend for the
now-offline claude-leaderboard.ardenone.com), specifically
`src/extractor_v4.py`, `src/generate_leaderboard_v3.py`, and
`src/build_hot_db.py`. Frozen data snapshot examined directly:
`~/backups/claude-leaderboard/hot.db` and `leaderboard.db`
(2026-07-27/28 freeze).

This system reached **37.9M commits, 695,569 users, 1.1M repos** on a
single SQLite file with no distributed coordination and no equivalent of
`commitgraph`'s queue-api contention. Understanding exactly how it achieved
that, measured directly against real data rather than assumed, is what
grounded most of this redesign's core decisions.

## The idempotent rollup pattern

`extractor_v4.py`'s `clone_and_extract_v4` re-derives a repo's **complete**
commit history fresh on every (re)scan — no incremental diffing against
previously-seen SHAs. Commits are bucketed into `(repo_id, username, day)`
counts (`_compute_rollups`), then the repo's entire existing slice of the
`repo_user_daily` rollup table is deleted and replaced in one transaction
(`_update_repo_user_daily`: `DELETE FROM repo_user_daily WHERE repo_id=?`
→ bulk `INSERT`).

This is the core idempotency trick this redesign's clone-worker reuses:
because a rescan always regenerates the same true history, wholesale
slice-replace is inherently safe under repeated rescans — a repo scanned
ten times never double-counts, since each pass wipes and rewrites only
that repo's own rows.

`users.total_commits` (the lifetime counter) is updated by **delta**, not
recomputed: `_update_user_total_commits` diffs old vs. new per-user counts
and applies `UPDATE users SET total_commits = total_commits + delta`.
`build_hot_db.py`'s header states the invariant this maintains: `total =
archived_commits + SUM(repo_user_daily.commits)` holds by construction, so
a full `COUNT(*)` rebuild is never needed — a real fix in this codebase's
own history, replacing an earlier version that did recompute from scratch
and both undercounted and risked wiping the table via an orphan-user DELETE.

**What doesn't transfer directly**: claude-leaderboard's delta-update was
never tested under concurrent writers, because it never had any — single
process, single SQLite connection. This redesign's Postgres schema
reproduces the DELETE+INSERT rollup pattern for the write-mostly slice
replace, but adds explicit row locking (`SELECT ... FOR UPDATE`) around the
`users` delta-update specifically, because multiple `clone-worker`
replicas touching the same user across different repos is exactly the race
Postgres is being brought in to survive.

## Measured size breakdown (the finding that reframed the whole storage discussion)

Queried directly via `sqlite3`/`dbstat` against the real frozen databases,
not estimated:

| Structure | Size | Rows | File |
|---|---|---|---|
| `repo_user_daily` (rollup) + its one index | 209 MB | 3.48M | `hot.db` (frozen, pruned working set — no raw `commits` table) |
| `repo_user_daily` (rollup) + its two indexes | 738 MB | 7.97M | `leaderboard.db` |
| Full `leaderboard.db` (raw `commits` + 6 index structures + rollup + caches) | 27.9 GB | 52.5M commits | `leaderboard.db` |

**Corrected 2026-08-04 (gap-review round 3): the original 95.8%/2.6%
breakdown divided `hot.db`'s rollup size by `leaderboard.db`'s total — two
different physical files treated as one measurement — and the resulting
2.6% didn't even follow from that division (209MB/27.9GB is actually
~0.8%).** Re-measured self-consistently within `leaderboard.db` alone, the
one file that holds the raw commit log and the rollup side by side: 95.8%
of the full database is the raw `commits` table plus its six index
structures (**corrected 2026-08-04, gap-review round 4**: 5 explicit
indexes plus the implicit `sqlite_autoindex_commits_1` its `PRIMARY KEY`
creates on this rowid table — `.indexes commits` in `sqlite3` lists all
six; summing only the 5 explicit indexes gives 85.3% of the file, not the
95.8% cited here or in `plan.md`, so the sixth, implicit one has to be
counted for either figure to reproduce);
**2.8%** is the rollup (not 2.6%). `hot.db`'s separately-measured
209MB/3.48M-row rollup figure is real and still the baseline this redesign's
Postgres sizing extrapolates from — it's just a different file than the
27.9GB total, so the two shouldn't be divided against each other. The
row-count multiplier (`hot.db`'s own ratio: ~11x, one row per commit vs.
one row per `(repo,user,day)` bucket, 37.9M source commits / 3.48M rollup
rows) and row-width multiplier (full SHA + full `owner/repo` string + up to
200 bytes of message per row, vs. an int repo_id + short strings for the
rollup) both point the same direction — fewer, narrower rows — and index
proliferation adds more on top (six structures on the `commits` table,
several overlapping composites that a later index likely made partially
redundant). The previously-stated "roughly 130x" aggregate was itself
downstream of the same file-mixing bug (`hot.db`'s 209MB divided into
`leaderboard.db`'s ~26.7GB commits-table size); measured self-consistently
within `leaderboard.db` alone, the raw `commits` table + its six index
structures (26.7GB) is **~34.5x** the size of the rollup table + its two indexes
(738MB) — smaller than the earlier cross-file estimate, but the qualitative
conclusion is unchanged: the rollup that actually drives ranking is a small
fraction of the real cost.

This is the concrete evidence behind `build_hot_db.py`'s own design
decision to ship `hot.db` (the rollup + `users` + a few small caches, no
raw `commits` table at all) as the thing that actually serves rankings —
and it's the evidence this redesign leans on to justify keeping the raw
commit-history artifact and the rollup in genuinely different storage
tiers (ARMOR, cold/archival vs. Postgres, hot/queryable) rather than one
undifferentiated store.

## Read-time ranking computation

`generate_leaderboard_v3.py`'s `fetch_leaderboard_data` never touches raw
commits for ranking — only the rollup and the incrementally-maintained
`users.total_commits` counter:

- **Candidates are selected by 30-day rollup activity, not lifetime
  total** — a documented fix for a real bug: an earlier version filtered
  candidates by lifetime total first, which measurably dropped 28 of the
  true top-100 users whose activity was concentrated in the recent window
  (one excluded user had 2,371 recent commits against a published cutoff
  of 942, because their *lifetime* total ranked below the old candidate-pool
  cutoff).
- **Identity/alias merging happens at read time**, via a small
  hand-maintained map (`_load_username_aliases`), applied case-insensitively
  and folded into ranking before the final sort — not a live background
  writer materializing merged identities into storage. This redesign's
  Postgres `users`/alias-join design follows the same shape, though against
  a live queue-api-hosted alias table rather than a static file.
- Final rank = 30-day commits desc, tiebroken by a cumulative
  recency-weighted sum (more activity closer to "today" wins ties among
  equal 30-day totals).

## What claude-leaderboard never had to solve, and why that matters here

Single-tool detection (`Co-Authored-By: Claude` only), single-provider
(GitHub only), single process. `commitgraph`'s 21-tool multi-provider
scope is real added complexity claude-leaderboard's architecture doesn't
address at all — this redesign keeps that breadth (via
`shared/detection.py` and the `repo_user_daily_tool` sparse table, since
only ~0.3% of commits are AI-tagged) while borrowing claude-leaderboard's
proven storage/write patterns for the parts of the problem that actually
overlap.
