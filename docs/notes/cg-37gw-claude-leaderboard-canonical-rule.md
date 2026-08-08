# claude-leaderboard is canonical for Claude Code

**Bead:** cg-37gw
**Date:** 2026-08-08
**Decision Date:** 2026-08-05

## Positioning

**claude-leaderboard is the canonical source for Claude Code leaders.**

commitgraph is not authoritative for Claude Code specifically and should not be positioned as competing with or superseding claude-leaderboard on that tool.

The two systems have different remits:

- **claude-leaderboard** — canonical for Claude Code. Single tool (`Co-Authored-By: Claude`), deep history, proven at 37.9M commits on one box.
- **commitgraph** — the multi-tool system. Its distinguishing capability is the 21-tool detection catalog (`ALL_TOOLS`) and retroactive re-detection when a new tool signature is added — capabilities claude-leaderboard neither has nor needs.

## Reconciliation Rule

**Where the two disagree about a Claude Code number, claude-leaderboard wins.**

This rule is useful during validation:

- A divergence on Claude Code counts is a signal to investigate commitgraph
- It is NOT evidence that claude-leaderboard drifted

### Application during verification gates

This rule applies directly to:

1. **Gate 1 — divergence review (frozen golden snapshot comparison)**
   - When comparing commitgraph's output against the frozen `leaderboard.json` (2026-08-03T22:05:42Z)
   - For Claude Code-specific numbers, claude-leaderboard is the baseline
   - Any unexplained divergence suggests a commitgraph ingestion or ranking bug

2. **Gate 2 — independent recompute**
   - When recomputing rollups from the corpus Parquet to verify Postgres state
   - Claude Code counts should still align with claude-leaderboard's expectations
   - Use this as a sanity check that detection patterns haven't drifted

## Seed Relationship: Inheritance, Not Acquisition

Harvesting claude-leaderboard's frozen `author_login_cache` (349,425 email→login pairs) into `email_resolution` is correct and valuable, but it is:

- **Borrowing a cache**, not taking over a domain
- **Inheriting tool-agnostic facts about people**, not Claude-Code-specific data

Resolved emails are facts about people that apply regardless of which AI tool they used. The seed is an input to commitgraph's identity resolution, not a transfer of authority for Claude Code rankings.

### Why the distinction matters

This prevents confusion about:

1. **Ranking authority** — claude-leaderboard remains the source of truth for Claude Code leaderboards
2. **Data ownership** — the seed contributes to identity resolution broadly, not a handoff of Claude Code domain
3. **Validation expectations** — divergence signals investigation of commitgraph, not质疑 of claude-leaderboard

## Downstream implications

The presentation layer (devimprint.com) inherits this positioning:

- Claude Code leaderboards reference claude-leaderboard as canonical
- commitgraph's multi-tool rankings are a separate, complementary view
- Cross-linking between the two should reinforce their different remits, not suggest competition

## Cross-references

- **Decision source:** `/home/coding/commitgraph/docs/plan/plan.md` section "Relationship to claude-leaderboard" (decided 2026-08-05)
- **Related verification:** Gate 1 divergence review, Gate 2 independent recompute
- **Seed data:** claude-leaderboard's frozen `author_login_cache` (349,425 pairs)
- **Claude-leaderboard comparison:** `/home/coding/commitgraph/docs/research/claude-leaderboard-comparison.md`
