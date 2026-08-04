# Considered: a public top-100 page with rank explainability

**Status: captured for later consideration, not adopted into the core plan.**
This directly resolves a real open decision the plan itself flags (Phase 5:
what happens to public serving at cutover, given the new pipeline's ranked
snapshot is internal-only) — but it does so by expanding this plan's scope
into public-serving territory the plan otherwise deliberately excludes.
Recorded here rather than folded into Architecture/Storage placement so the
current internal-only design stays the settled default until this is
explicitly decided.

## The idea

Publish a small, curated public page — top 100 only, not the full
hundreds-of-thousands-row internal snapshot — directly from this pipeline,
alongside a per-row "explain this ranking" drill-down showing the actual
commits/rows behind a user's number. This is exactly option (c) the plan's
own Phase 5 section already names as a live possibility: "have this pipeline
additionally publish a minimal public artifact... purely to avoid a hard
outage, despite the internal-only decision."

## Why it's tempting

- Directly closes the Phase 5 gap: without *something* public, decommissioning
  the old pipeline's dashboard at cutover leaves public serving dark until
  the separate, not-yet-started devimprint presentation layer ships.
- The explainability half directly serves the problem that started this
  whole redesign — a user with 5,000+ real commits seeing ~124 on the board
  is exactly the kind of confusion a "here's the exact rows behind your
  number" view would surface and make debuggable, by the user themselves.

## Why it's not a clean adoption

- It's a second public-serving surface, maintained by *this* pipeline, that
  the plan's own architecture section is explicit was never supposed to
  exist here: "no `leaderboard.json` or other public-serving format is
  produced by this pipeline — that's the downstream devimprint presentation
  layer's concern."
- **A required amendment, not optional, if this is ever built**: without an
  explicit anti-scraping design (rate-limiting, or keeping the *full* dataset
  off this surface even though only top-100 is here), this reintroduces
  exactly the scraping risk that motivated keeping the full snapshot
  internal-only in the first place — a bulk-downloadable public artifact is
  the worst shape for that regardless of row count. Cloudflare's
  rate-limiting/bot-management (already in the ARMOR request path via
  `ARMOR_CF_DOMAIN`) is the natural fit, but this needs to be designed, not
  assumed.
- Top-100 specifically (not the full list) sidesteps the storage/format
  problems already solved for the internal snapshot, but reopens the
  question of who computes and refreshes this second, smaller artifact and
  on what cadence — not free just because it's small.

## If this gets picked up later

Scope it explicitly as *its own* small feature with its own Phase, not a
Phase 1 addition — it's genuinely a different concern (public product
surface) from the pipeline redesign this plan is about. The anti-scraping
design should be written before any code, not after.
