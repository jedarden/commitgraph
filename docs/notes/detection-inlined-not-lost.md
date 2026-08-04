# Detection moves inline, but retroactive re-detection is preserved

## The decision

`clone-worker` absorbs `filter-worker`'s detection logic directly —
`shared/detection.py` (unchanged) runs against every commit in the same
pass as extraction, not as a separate downstream service reading dirty
partitions. `filter-worker` and `compactor` are retired as standalone
deployments.

## Why this was almost a mistake

An early version of this plan retired `filter-worker` outright, on the
assumption that inlining its logic into clone-worker meant its job was
fully absorbed. It wasn't. `filter-worker`'s live role isn't just "run
detection on new commits" — it's the consumer of a real, already-built
mechanism (`catalog_version` / `last_filter_catalog_version` in queue-api's
schema) that re-dirties the **whole corpus** when the detection catalog
gains a new tool signature, so historical repos get re-scanned for tools
that didn't exist in the catalog when they were first cloned.

`shared/detection.py`'s own docstring is explicit about why this matters:
the catalog is written from memory of tool conventions, which makes it
"structurally weakest on the past" — real 2021-2023 tool conventions were
missed entirely until added later. If detection only ever runs at clone
time, a signature added in month N is permanently invisible for every repo
not re-cloned after month N — and re-cloning the full corpus (98,747+ repos
and growing) to fix that is exactly the expensive operation this whole
redesign exists to avoid.

## The resolution

`catalog_version` stays in queue-api. When a new signature is added, the
version bumps and a **redetect job** — a lightweight job kind, distinct
from a fresh clone — gets enqueued per affected repo. clone-worker (or a
thin variant using the identical detection code) claims it, reads the
**already-stored** raw commit-history artifact back from ARMOR (see
`armor-vs-postgres-placement.md`) — no re-clone, no GitHub API cost —
reruns detection, and upserts only the rollup rows that changed.

Same detection code, same Postgres write target, same idempotent
DELETE+INSERT rollup pattern as a normal clone job. The only difference is
trigger frequency: continuous background service vs. occasional
catalog-bump-driven job. This is what makes the collapse from four services
to one safe: the *capability* survives, only the *always-running process*
goes away.

## What one call to detection actually checks

`detect_tools_for_commit()` (the entry point both a fresh clone and a
redetect job call) checks **all ~15 cataloged tools in one pass** per
commit — four signal tiers (Co-Authored-By trailer emails, author emails,
author name patterns, body text patterns), each a dict/set lookup keyed by
tool, unioned into one result set. It is never clone-once-per-agent or
scan-once-per-agent; one commit, one function call, every known tool
checked simultaneously. This is also why a redetect job is cheap: rerunning
detection against an already-extracted commit is a pure-Python regex pass,
not an I/O-bound operation.

One open gap, not yet resolved: the live discovery footprint list
(`search-worker`'s `GITHUB_FOOTPRINTS`, 12 entries) is narrower than the
full detection catalog (15 tools) — a handful of tools (windsurf, codeium,
replit, tabnine, codestral, sweep, netlify-coding) have detection patterns
but no active GitHub-search discovery query. A repo only enters the
pipeline if *some* footprint matches during discovery, so a tool with zero
discovery footprints could in principle only ever be detected in repos that
happened to be discovered for an unrelated reason. Worth deciding whether
this is intentional (discovery breadth vs. API budget tradeoff) or a gap to
close before relying on the catalog's full breadth for content/reporting
purposes.
