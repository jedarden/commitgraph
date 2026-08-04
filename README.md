# commitgraph-v2

A redesign of [`commitgraph`](https://git.ardenone.com/jedarden/commitgraph)
(deprecated), the AI-coding-tool-attribution data platform: discovers
repositories and developers across git hosting providers, clones repos,
detects AI-coding-tool footprints (Claude Code, Cursor, Aider, Codex, Devin,
and 10+ others) in commit history, and rolls the results up into
per-developer / per-repo / per-tool statistics at the scale of the full
public GitHub corpus (the predecessor reached 76.6M commits / 1M+ developers
/ 98,747 repos before this redesign began).

## Why a rewrite, not an iteration

The predecessor's write path (`clone-worker` → stage → `compactor` merge →
`filter-worker` detect → `aggregator` rollup) round-trips every commit
through a single serialized SQLite coordination service (`queue-api`)
multiple times per commit, across 9+ concurrently-running pods. That
contention was live-verified (2026-08-03/04) as the direct cause of a
concrete accuracy problem: a user with 5,000+ real AI commits in the last 30
days saw only ~124 reflected on the public board — partly identity
fragmentation, partly up to 24h of aggregation lag, both downstream of the
same structural bottleneck. The predecessor's aggregator also suffered five
separate OOM-crash incidents over its lifetime, each caused by having to
decrypt and materialize large encrypted Parquet partitions in-process to
compute a rollup that a proven predecessor system (claude-leaderboard, see
`docs/research/`) had already shown could be maintained incrementally at a
fraction of the cost.

This redesign collapses the write path to one hop (clone-worker computes
and writes the rollup directly, in the same pass as extraction) backed by a
concurrent-writer-safe datastore (Postgres) instead of serialized SQLite,
while deliberately preserving the one capability the predecessor's
multi-stage design got right that a naive collapse would destroy:
retroactive re-detection of AI-tool footprints across already-cloned history
when a new tool signature is added, without re-cloning from GitHub.

See `docs/plan/plan.md` for the full architecture, `docs/notes/` for the
specific decisions and their rationale, and `docs/research/` for the
comparative analysis of the claude-leaderboard predecessor that grounded
this design in a system already proven at comparable scale.

## Status

Design/planning stage — no application code yet. The predecessor
(`jedarden/commitgraph`) remains live and running; see its README for
current deprecation status. Cutover follows the phased rollout in
`docs/plan/plan.md` (build in isolation → validate → migrate the existing
corpus → shadow/dual-write burn-in → cutover → decommission the
predecessor).

## Structure

- `docs/notes/` — architecture decisions specific to this redesign, and why
- `docs/research/` — comparative analysis of claude-leaderboard, the proven
  predecessor system this redesign's core patterns are drawn from
- `docs/plan/plan.md` — the complete architecture and phased rollout plan
