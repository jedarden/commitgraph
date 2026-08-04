# Ideas ledger

Dated run sections from `/plan-idea-gen`. Each run's full pool with verdicts, so
future runs can dedupe against what's already been considered.

---

## 2026-08-04 — run 1 (first run, no PRIOR)

**Adoption outcome**, decided same day: 6 finalists folded directly into
`docs/plan/plan.md` (backup/restore runbook, tool-agnostic cost visibility,
SLO-driven ranking-refresh trigger, large-repo warm-start mitigation,
CI idempotency harness, poison-pill detection isolation) — each marked
inline where it resolves a gap the plan had already flagged in its own
text. 3 finalists captured as standalone notes docs instead, deliberately
kept out of the core plan (public top-100 + explainability expands scope
into public-serving territory the plan otherwise excludes; the
why-am-I-not-on-the-leaderboard diagnostic and GitLab/Bitbucket parity are
both genuine additive scope, not fixes to the current redesign):
`public-leaderboard-with-explainability.md`,
`why-am-i-not-on-the-leaderboard.md`, `gitlab-bitbucket-discovery-parity.md`.
The live-migration-progress-dashboard finalist was explicitly declined —
no action taken. No `.beads/` workspace exists in this repo yet, so no beads
were created this run; adoption tracking lives in `plan.md` and these notes
docs instead.

POOL 100, generated across 8 lenses, none skipped. Full pool + clusters + triage
reasoning archived at
`/home/coding/.tmp/claude-1000/-home-coding/bcbe9644-8d09-468e-9dd3-fd33794653cb/scratchpad/commitgraph-ideas-pool.md`
(scratch, not durable — this ledger is the durable record).

### Clusters (15)
Discovery-mechanism · Ranking-methodology · Identity-resolution · Rollup/schema
architecture · Infra-simplification · Warm-start/caching · ARMOR/storage-path ·
Public-serving-gap · Detection-catalog governance · Coordination/queue-api ·
Reliability/ops hardening · Power-user API surface · Novice UX/clarity ·
Scheduling/triggering · Growth/marketing/monetization

### Triage (100 → 25)
25 survived initial triage. Cut highlights: **27** (rewrite queue-api) violates
the plan's explicit "stays as-is, not being rewritten" constraint. **48**
(Parquet-only ranking, no Postgres) contradicts the plan's settled, extensively-
justified Postgres-computes-ranking decision. **41** (litestream-SQLite instead
of Postgres) contradicts the settled concurrent-writer requirement that is the
entire reason Postgres was chosen. The full growth/marketing cluster (17, 20,
87–99 — embeddable widgets, paid tiers, SEO profiles, social content, etc.) cut
as out-of-scope-by-design: these belong to the downstream devimprint
presentation layer the plan explicitly excludes, not because they're bad ideas.

### Crossover (4 hybrids)
- **H1**: public top-100 (28) + explain-ranking API (62) → public top-100 with
  per-row rank explainability drill-down.
- **H2**: materialized ranking snapshot (36) + freshness SLO (26) → SLO-driven
  materialized ranking refresh, not a blind fixed interval.
- **H3**: sticky large-repo affinity (38) + incremental append-compact (33) →
  combined mitigation for the plan's own flagged large-repo warm-start-at-scale
  risk.
- **H4**: diagnostic tool (79) + fuzzy-match dedup (21) → diagnostic tool with
  self-service fuzzy-match merge suggestions. **Later killed as a hybrid** (see
  below) — the fuzzy-match half carries a real correctness risk; 79 survives
  standalone without it.

### Pairwise ranking (29 candidates → 15 advanced, 2/cluster cap enforced)
Advanced: 31, 32, 21, 33, H2, H3, 34, H1, 8, 70, 66, 75, 53, H4, 80.
Trimmed at the cap or on relative strength: 15, 40, 35 (real ideas, lost pairwise
comparisons to stronger same-cluster or cross-cluster contenders — not killed,
just didn't advance).

### Adversarial kill pass (CONSTRAINTS re-read first)
- **KILLED — 21 (fuzzy-match identity dedup, standalone/unhybridized)**: fuzzy
  matching on names/emails is false-positive-prone; a wrong auto-merge silently
  corrupts rankings in a way that's hard to detect or undo. Directly conflicts
  with the plan's own hard-won philosophy (the `email_resolution` seed's
  conflict rule: never let an uncertain value silently override a correct one).
  Would need a human-reviewed-suggestion-only redesign to be safe — not what
  was proposed.
- **KILLED — 34 (direct-B2 escape hatch for the warm-start artifact)**: flatly
  violates the plan's explicit hard constraint, "No direct B2 SDK calls
  anywhere in the new pipeline." If ARMOR proves inadequate, the correct
  escalation is fixing/scoping ARMOR (e.g., the already-open `ARMOR_PREFIX`
  decision), not quietly reintroducing exactly what this redesign moved away
  from.
- **KILLED — H4 (hybrid specifically)**: inherits 21's fuzzy-match risk. **79
  reinstated standalone** (the diagnostic tool without auto-merge suggestions
  is sound on its own) in H4's place.
- **SURVIVES — 31, 32, 33, H2, H3, 8, 70, 66, 75, 53, 80, H1** — strongest
  remaining objections noted per-idea in the dossiers delivered to the user
  this run.

### Completeness gap round
Surviving set was thin on: cost/spend observability for the new infra, live
visibility into the one-time corpus migration's progress (distinct from its
correctness/idempotency), and security/access-control for the new endpoints
this plan introduces (`/email-resolution/seed`, H1's public page). Ran one
targeted batch:
- **G1** (survives): cost dashboard/alert for the new Postgres node + ARMOR
  write volume, tied to actual Spot bid price — spend visibility, not just
  capacity.
- **G2** (survives): live migration-progress dashboard over the
  already-planned `migration_progress` table (partitions done/total, ETA).
- **G3** (survives, framed as a required amendment to H1 rather than a
  standalone idea): explicit auth/rate-limiting design for H1's public page —
  without it, H1 reintroduces the exact scraping risk the internal-only
  snapshot decision was built to avoid.

### Final selection (KEEP 10, delivered as dossiers)
66, 75, H2, H3, H1+G3, 79, 70, G1, G2, 31.

**Strong runners-up, not selected this round** (survived the kill pass, lost
only on final cut): 32 (multiple ranking views), 33 (standalone — folded into
H3), 8 (continuous redetect, throttled), 53 (commitgraph-cli), 80 (repo-owner
onboarding docs).

### Full pool with individual verdicts

| # | Idea | Cluster | Verdict |
|---|------|---------|---------|
| 1 | Push-model discovery via GitHub webhooks | Discovery | cut-triage: real but out of scope for this plan's throughput-ceiling framing |
| 2 | Lazy/on-demand rank computation | Scheduling | cut-triage: conflicts with "Postgres computes ranking, periodic aggregator" design |
| 3 | AI-TOOLS.md self-report manifest convention | Discovery | cut-triage: changes detection philosophy from passive to opt-in, large scope change |
| 4 | Bulk GH Archive ingestion | Discovery | cut-triage: interesting, not pursued this round |
| 5 | User-key-only rollup (drop repo granularity) | Schema | cut-triage: loses data the plan needs (top_repo field) |
| 6 | Impact/lines-changed ranking | Ranking | cut-triage: needs blob content, conflicts with blob:none design |
| 7 | P2P coordination via Postgres advisory locks | Coordination | advanced, pairwise-trimmed: queue-api already works, added complexity w/o clear payoff |
| 8 | Continuous rolling redetection | Detection governance | kill-pass: SURVIVES (throttle to low-priority background, not literally continuous) |
| 9 | Client-side-only ARMOR decryption | ARMOR | cut-triage: re-architects ARMOR's own trust model, out of this plan's scope |
| 10 | Per-org/private opt-in leaderboards | Public-serving | cut-triage: superseded in spirit by H1 |
| 11 | Self-claim/verify identity flow | Identity | cut-triage: overlaps 77, deferred to devimprint-layer |
| 12 | Delta-only storage (inverts overwrite pattern) | Schema | cut-triage: conflicts with the plan's core idempotency philosophy |
| 13 | Metadata-first, clone-on-promising-trailer | Discovery | cut-triage: GitHub commit API metadata doesn't include trailers reliably |
| 14 | ELO-style rating | Ranking | cut-triage: reward-shape change, needs product sign-off beyond this plan |
| 15 | Trending/velocity view | Ranking | pairwise-trimmed: real, lost to stronger contenders under the cap |
| 16 | CDC-driven incremental snapshot publish | ARMOR | cut-triage: snapshot is internal/periodic already, CDC is overbuilt for that access pattern |
| 17 | Full gamification (streaks/badges/levels) | Growth | cut-triage: downstream devimprint layer, out of scope |
| 18 | OpenTelemetry tracing across jobs | Reliability | cut-triage: real, folded conceptually into G1/G2's observability theme |
| 19 | CDN-style cache invalidation | ARMOR | cut-triage: single internal consumer, no cache-invalidation problem exists yet |
| 20 | Spotify-Wrapped-style recap | Growth | cut-triage: downstream devimprint layer, out of scope |
| 21 | Fuzzy-match identity dedup | Identity | **kill-pass: KILLED** — silent-misattribution risk, conflicts with plan's own conflict-handling philosophy |
| 22 | Time-series-DB-style auto-partitioning | Schema | cut-triage: premature at current ~1GB scale |
| 23 | Feature-flag gradual Phase 4 rollout | Scheduling | cut-triage: Phase 4 already has a burn-in window; marginal value |
| 24 | Content-addressed warm-start artifacts | Warm-start | cut-triage: folded conceptually into H3's incremental-compact direction |
| 25 | commitgraph.txt opt-out convention | ARMOR | cut-triage: real but not urgent, no opt-out requests exist yet |
| 26 | SRE error-budget freshness SLO | Scheduling | absorbed into H2 |
| 27 | Rewrite queue-api async | Coordination | cut-triage: **violates constraint** — plan states queue-api "stays as-is, not being rewritten" |
| 28 | Public top-100 from this pipeline | Public-serving | absorbed into H1 |
| 29 | Multi-AZ managed Postgres | Infra | cut-triage: real, but no managed-Postgres option confirmed available/affordable in this org |
| 30 | Community-contributed detection patterns | Detection governance | cut-triage: process/governance change beyond this plan's scope |
| 31 | GitLab/Bitbucket parity | Discovery | **kill-pass: SURVIVES** — selected as finalist |
| 32 | Multiple ranking views | Ranking | kill-pass: SURVIVES, strong runner-up not selected this round |
| 33 | Incremental append+compact warm-start | Schema/warm-start | absorbed into H3 |
| 34 | Direct-B2 escape hatch | ARMOR | **kill-pass: KILLED** — violates explicit "no direct B2" constraint |
| 35 | Self-service rescan-now trigger | Scheduling | pairwise-trimmed: real, lost to stronger contenders under the cap |
| 36 | Materialized ranking snapshot in Postgres | Schema | absorbed into H2 |
| 37 | Per-org private data export | Power-user | cut-triage: real, deferred — no org customers yet |
| 38 | Sticky affinity for large repos | Warm-start | absorbed into H3 |
| 39 | Move queue-api onto Postgres | Coordination | cut-triage: plan already says "re-measure after Phase 2 before considering" — premature |
| 40 | Fix SQLite contention before new infra | Infra | pairwise-trimmed: real sequencing idea, largely already reflected in the plan's compactor/filter-worker retirement |
| 41 | litestream-SQLite instead of Postgres | Infra | cut-triage: **contradicts settled decision** — concurrent-writer requirement is why Postgres was chosen |
| 42 | Adaptive resync interval | Warm-start | cut-triage: real, absorbed conceptually into H2's SLO framing |
| 43 | Shared RWX PVC cache instead of ARMOR | Warm-start | cut-triage: this project has real documented RWO/multi-attach pain; RWX adds a new failure class |
| 44 | Managed Postgres-as-a-service | Infra | cut-triage: no confirmed option in this org's infra |
| 45 | Fold tool table into JSONB column | Schema | cut-triage: loses index-ability the sparse table's design intentionally has |
| 46 | Manual psql import for identity seed | Identity | cut-triage: real simpler alternative, but the seed's conflict-handling rule (status='pending' only) is easier to enforce via endpoint than ad hoc SQL |
| 47 | Defer redetect-job mechanism entirely | Detection governance | cut-triage: **conflicts with plan's core preserved capability** — retroactive redetection was explicitly saved from an earlier near-miss |
| 48 | Parquet-only ranking, no Postgres | Schema | cut-triage: **contradicts settled decision** — extensively justified Postgres-computes-ranking design |
| 49 | Bare postgres:16, no CNPG operator | Infra | cut-triage: loses the backup/ScheduledBackup pattern this plan explicitly needs |
| 50 | Skip sparse table's second index | Schema | cut-triage: table is negligibly small either way, no real savings |
| 51 | Defer ARMOR warm-start artifact entirely | Infra | cut-triage: **contradicts already-validated, already-planned mechanism** |
| 52 | Public read-only query API | Power-user | cut-triage: real, deferred — no external consumer demand yet |
| 53 | commitgraph-cli | Power-user | kill-pass: SURVIVES, strong runner-up not selected this round |
| 54 | Rank-milestone notifications | Power-user | cut-triage: devimprint-layer feature, out of scope |
| 55 | Bulk CSV/Parquet export endpoint | Power-user | cut-triage: real, deferred — no consumer demand yet |
| 56 | Compare-orgs aggregate views | Power-user | cut-triage: devimprint-layer feature, out of scope |
| 57 | Per-user historical trend charts | Power-user | cut-triage: devimprint-layer feature, out of scope |
| 58 | Third-party API key system | Power-user | cut-triage: no external integrators yet |
| 59 | Grafana-over-Postgres dashboard | Reliability | cut-triage: folded into G1/G2's observability theme |
| 60 | Saved/custom leaderboard filters | Power-user | cut-triage: devimprint-layer feature, out of scope |
| 61 | Webhook push to external dashboards | Power-user | cut-triage: no external consumer yet |
| 62 | Explain-ranking API | Power-user | absorbed into H1 |
| 63 | Bulk identity-claim API for orgs | Power-user | cut-triage: no org customers yet |
| 64 | Circuit breaker on ARMOR calls | Reliability | cut-triage: real, deferred — Phase 1 build should surface whether this is actually needed |
| 65 | Dead-letter quarantine queue | Reliability | kill-pass: SURVIVES, strong idea, not selected this round (queue-api's oversized-repo pattern partially covers this already) |
| 66 | Backup/restore runbook | Reliability | **kill-pass: SURVIVES** — selected as finalist |
| 67 | Chaos-test concurrent-write load test | Reliability | cut-triage: real, folds into Phase 2's already-planned load test as a variant |
| 68 | Reconciliation job (drift detection) | Reliability | kill-pass: SURVIVES, strong runner-up not selected this round |
| 69 | Shared token-bucket rate limiter | Reliability | cut-triage: plan already says search-worker/enrichment self-pace on 403s; real but not urgent |
| 70 | Poison-pill isolation for detection.py | Detection governance | **kill-pass: SURVIVES** — selected as finalist |
| 71 | Versioned-migrations tooling | Reliability | cut-triage: real, standard practice, low novelty as a distinct "idea" |
| 72 | PVC-attach retry/backoff | Reliability | cut-triage: real, but node-level concern better owned by cluster ops than this plan |
| 73 | Health-check dashboard per worker type | Reliability | cut-triage: folded into G1/G2 |
| 74 | Backpressure signal queue-api→search-worker | Reliability | cut-triage: real, but search-worker already self-paces on its own budget |
| 75 | Idempotency test harness in CI | Reliability | **kill-pass: SURVIVES** — selected as finalist |
| 76 | "How is my rank calculated" explainer | Novice UX | cut-triage: devimprint-layer content, out of scope |
| 77 | OAuth "claim your profile" flow | Novice UX | cut-triage: overlaps 11, devimprint-layer, out of scope |
| 78 | "Last updated" timestamp everywhere | Novice UX | cut-triage: devimprint-layer UI concern, out of scope |
| 79 | "Why am I not on the leaderboard" diagnostic | Novice UX | **kill-pass: SURVIVES (standalone, reinstated from killed H4)** — selected as finalist |
| 80 | Repo-owner onboarding docs | Novice UX | kill-pass: SURVIVES, strong runner-up not selected this round |
| 81 | Friendly empty/zero-data states | Novice UX | cut-triage: devimprint-layer UI concern, out of scope |
| 82 | Guided first-time-visitor tour | Novice UX | cut-triage: devimprint-layer UI concern, out of scope |
| 83 | Plain-language tool-name glossary | Novice UX | cut-triage: devimprint-layer content, out of scope |
| 84 | "Did you mean" username suggestions | Novice UX | cut-triage: devimprint-layer UI concern, out of scope |
| 85 | Visual tool-mix breakdown | Novice UX | cut-triage: devimprint-layer UI concern, out of scope |
| 86 | Plain-language status page | Novice UX | cut-triage: devimprint-layer concern; G1/G2 cover the pipeline-side version |
| 87 | Embeddable live-counter widget | Growth | cut-triage: downstream devimprint layer, out of scope |
| 88 | Automated "biggest movers" content gen | Growth | cut-triage: downstream devimprint layer, out of scope |
| 89 | AI-vendor co-marketing partnerships | Growth | cut-triage: downstream devimprint layer, out of scope |
| 90 | Paid team/company analytics tier | Growth | cut-triage: downstream devimprint layer, out of scope |
| 91 | Real-time live-activity ticker | Growth | cut-triage: downstream devimprint layer, out of scope |
| 92 | SEO-optimized public profile pages | Growth | cut-triage: downstream devimprint layer, out of scope |
| 93 | Compare-a-friend shareable tool | Growth | cut-triage: downstream devimprint layer, out of scope |
| 94 | Mobile PWA | Growth | cut-triage: downstream devimprint layer, out of scope |
| 95 | Referral/invite mechanic | Growth | cut-triage: downstream devimprint layer, out of scope |
| 96 | Public API changelog/versioning promise | Growth | cut-triage: no external API surface exists yet to version |
| 97 | Shareable "rank card" image generator | Growth | cut-triage: downstream devimprint layer, out of scope |
| 98 | Seasonal competitions/leaderboard resets | Growth | cut-triage: downstream devimprint layer, out of scope |
| 99 | Enterprise SSO / private-cloud tier | Growth | cut-triage: downstream devimprint layer, out of scope |
| 100 | Open-source the detection catalog | Detection governance | cut-triage: real, separate decision (licensing/maintenance) beyond this plan |
