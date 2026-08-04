# Considered: a "why am I not on the leaderboard" self-diagnostic tool

**Status: captured for later consideration, not adopted into the core plan.**
A genuinely additive capability, not a fix to anything currently broken in
the architecture — recorded as a notes-level idea rather than folded into
Architecture, since it's a new small surface (a query/endpoint), not a
change to how clone-worker, Postgres, or aggregator already work.

## The idea

A read-only diagnostic — given an email or username, answer: is this
identity resolved to a GitHub login? Does it have any AI-tool-tagged
commits at all? Are the repos it's active in public? Is it fragmented
across multiple unmerged email addresses? Surface the actual state, not
just "not found."

## Why it's worth having

This directly answers the *original* question that started this entire
redesign investigation: a user with 5,000+ real AI commits in the last 30
days saw only ~124 on the public board. The root causes turned out to be a
mix of identity fragmentation, freshness lag, and (at the time) a live
queue-api contention bug silently breaking the resolution feed — all
diagnosable in principle, none of it visible to the user who's actually
experiencing the gap. This tool would have made that specific investigation
self-service instead of requiring a deep pipeline audit to explain.

Notably: most of what this needs already exists as data — `email_resolution`
status, `user_aliases`, and the Postgres rollup are all already being built
for other reasons. This is a thin read-only view over existing state, not
new data collection.

## Why it's not adopted directly into the plan

It's a genuinely separate capability from what this plan is scoped to
build (a rewritten data pipeline) — closer to a small, focused API endpoint
that could be built any time after Phase 1's Postgres schema exists, not
something the core Architecture description needs to account for now.

## The one real caution

Without care, a diagnostic that just says "your email isn't resolved yet"
for a still-fragmented identity system risks *relocating* confusion rather
than resolving it — the user learns *that* something's wrong without being
able to fix it themselves (they can't force their own email resolution to
happen faster; that's still bounded by the shared GitHub API budget). Worth
pairing with realistic expectation-setting in the tool's own output ("this
typically resolves within N days") rather than just a bare status flag.

## Required amendment, not optional

This idea shipped from the same ideation run as the public top-100 page
(`public-leaderboard-with-explainability.md`), which got an explicit
"required amendment, not optional" note on anti-scraping/rate-limiting
(G3) before it could ever be built. This idea got no equivalent treatment
here, which is an oversight, not a signal that it's lower-risk — if
anything the risk shape is worse: a per-email/username lookup is a direct
oracle for "does this identity exist in our system, and what's its
status," which is more individually-identifying than a bulk ranked list.
Unauthenticated and unthrottled, this is free infrastructure for
enumerating which specific emails or usernames are active AI-coding-tool
users, one guess at a time.

Two things are required, not optional, if this is ever built, mirroring
what H1 already requires: **(1) rate-limiting** on the endpoint, same as
the public page's mandatory anti-scraping design; and **(2)** either
**authentication** (only the account owner — once some identity-claim
flow exists — can query their own email) **or** a response shape that
never confirms or denies whether a raw email exists in the system at all
(e.g., only accepting an already-resolved GitHub login rather than
arbitrary raw emails, or returning an identical generic response
regardless of whether the input is unseen, unresolved, or
resolved-but-unranked). Absent one of those two, this tool is an
enumeration oracle over every email address the corpus has ever touched.
