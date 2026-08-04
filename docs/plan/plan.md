# commitgraph v2 — redesign plan

## Context

commitgraph (the live AI-coding-attribution pipeline on `ord-devimprint`, 76.6M
commits / 234K AI-tagged / 98,747 repos / 1M+ developers as of 2026-08-04) has
spent most of its life fighting its own infrastructure rather than its actual
job. The recurring pattern, verified live this session: queue-api's single
serialized SQLite write connection (`SetMaxOpenConns(1)`) is contended by 9+
pods (search-worker, 4 clone-worker replicas, user-enrichment-worker,
aggregator, filter-worker, compactor), producing constant `context canceled`
/ `Read timed out` errors. That contention is a proximate cause of a concrete
user-facing symptom: a user with 5,000+ real AI commits in the last 30 days
sees only ~124 on the public board — a ~40x gap, only partially explained by
identity fragmentation (raw email vs. resolved GitHub login) and partly by
the pipeline's freshness lag (up to 24h between clone and rollup).

Investigating why led to comparing commitgraph against its predecessor,
claude-leaderboard (`vibecodeleaderboard-backend`), whose architecture is
radically simpler: single SQLite file, a `(repo, user, day)` rollup table
maintained by idempotent whole-repo-slice replace on every rescan, read-time
identity aliasing, no distributed coordination. That system reached 37.9M
commits / 695K users on one box with no equivalent contention. Live-measured
this session: its rollup table is 209MB for 3.48M rows; the full 27.9GB
database is 95.8% raw commit log + indexes, only 2.6% rollup — confirming
the rollup itself was never the expensive part.

This plan applies that lesson to commitgraph without discarding the things
commitgraph got right that claude-leaderboard never needed: multi-tool
detection (12 signatures vs. claude-leaderboard's single hardcoded
`Co-Authored-By: Claude` trailer) and the ability to retroactively re-detect
history when a new tool signature is added — which is a real, already-built
capability (`catalog_version` / `last_filter_catalog_version` in queue-api's
schema) that a naive "just inline everything into clone-worker" redesign
would have destroyed permanently for the inherited corpus. That mechanism is
preserved below in a lighter-weight form.

**Outcome of this redesign:** freshness drops from up to 24h to effectively
immediate (rollup written in the same pass as extraction), the aggregator's
five-incident OOM history goes away (it becomes a thin SQL query instead of a
DuckDB engine decrypting/materializing large partitions), and queue-api's
write contention drops to its original, much lower-volume job-coordination
role — which plausibly also fixes a live bug where that same contention
silently broke identity resolution's feed (`fed 0 new emails`). What this
redesign does **not** fix: discovery and clone throughput, both hard
ceilings on the shared GitHub API budget and clone-worker's own measured
per-replica rate; and identity resolution *throughput*, capped by that same
shared API budget regardless of storage architecture (commitgraph already
reuses claude-leaderboard's identity-resolution design — cached per-email
resolution + read-time alias merge — this isn't a missing piece to borrow).
See "Explicitly out of scope" below for the full accounting, including a
concrete, not-yet-built follow-up (seeding `email_resolution` from
claude-leaderboard's own frozen resolution cache) that would help
independently of this redesign's timeline.

## Architecture

**clone-worker** (rewritten, absorbs filter-worker's logic):
1. **Warm-starts from a stored snapshot before falling back to a full
   clone.** Checks ARMOR for a previously-stored **warm-start artifact**
   (see below) for this repo. If present: materializes it into a fresh
   working directory and runs `git fetch origin` — retrieving only commits
   added since the last scan, not the full history. If absent (first-ever
   scan) or the warm-start fails for any reason: falls back to a full
   `git clone --bare --filter=blob:none <url>` — always a safe fallback,
   never a hard failure. Commit-history only either way, blob-filtered, so
   file contents are never fetched, only commit/tree/trailer metadata.
   **Empirically validated, 2026-08-04** — see
   `docs/research/incremental-fetch-warm-start.md`: tested end-to-end
   against a real repo and the real GitHub remote; a correctly-materialized
   warm start fetched the delta in under a second with a ~300-byte
   negotiation, versus re-downloading full history every scan.
2. Walks full history, extracts `(sha, author_name, author_email,
   committed_at, message)` per commit — the same shape clone-worker already
   produces today (`containers/clone-worker/worker.py:338`).
3. Runs `shared/detection.py` inline per commit (reused as-is — it already
   operates on a single commit's message/trailer text, no interface change
   needed). **Adopted (2026-08-04, plan-idea-gen run 1): poison-pill
   isolation** — the per-commit detection call is wrapped so a single
   malformed commit message that crashes or hangs detection is caught,
   logged, and skipped, rather than blocking extraction for the whole repo.
   Real, low-cost hardening; not built until a first real incident makes it
   worth prioritizing over other Phase 1 work.
4. Computes `(user, repo, tool, day, count)` rollup rows for the whole repo.
5. In one pass: **(a)** upserts the rollup into Postgres, **(b)** writes the
   full per-repo commit-history artifact (the extracted rows from step 2,
   as Parquet — not a raw git bundle, so no git tooling is needed to re-scan
   it later) to ARMOR at a per-repo key, **(c)** writes the updated
   warm-start artifact (see below) to ARMOR at a separate per-repo key —
   **overwritten wholesale** on every rescan, same idempotency pattern as
   (b). All writes happen in the same job; if any fails the job fails and
   gets re-claimed (no partial-state).

This mirrors claude-leaderboard's core idempotency trick — full re-derive +
whole-slice replace — applied to three destinations instead of one SQLite
file.

**The warm-start artifact is a raw pack-file transport, not a `git
bundle`.** This was tested both ways. `git bundle create` on a
`--filter=blob:none` clone produced a bundle **~127x larger** than the
source repo's own pack (105MB from an 833KB source, for a sub-1,000-commit
repo), took 22+ CPU-seconds to create, and left a bloated pack behind in
the source repo as an undesirable side effect — confirmed specific to
partial/blob-filtered clones via a control test (an ordinary unfiltered
clone bundled smaller than its source, instantly). Rejected as a transport
for exactly the reason this optimization exists: under the
whole-object-overwrite pattern, that cost would be paid on every single
clone-worker job. The validated alternative: package the raw pack files
(`objects/pack/*.pack`, `.idx`, `.promisor`, `.rev`) directly, plus the
specific ref (the **loose** ref file, not `packed-refs` — the latter is a
stale clone-time snapshot that doesn't reflect a later ref update), plus
three git config values that turned out to be required —
`core.repositoryformatversion`, `remote.origin.promisor`,
`remote.origin.partialclonefilter` — without which the pack is present but
git refuses to use it (`fatal: pack has 49 unresolved deltas`, verified as
the actual cause by adding just those three values and re-testing with no
other change). Tarring all of it together stayed at 796KB — matching the
source almost exactly, no bloat. Full methodology, exact numbers, and every
failure mode encountered are in
`docs/research/incremental-fetch-warm-start.md` — not yet verified at real
corpus scale (NEEDLE, the test repo, is a few hundred commits; the corpus
includes far larger repos) or under the fleet's actual multi-replica
claim/lease conditions.

**Retroactive re-detection (the capability that must not be lost):**
queue-api keeps `catalog_version`. When a new tool signature is added to
`shared/detection.py`, the version bumps and a **redetect job** is enqueued
per affected repo (a new lightweight job kind, likely a `kind` column on the
existing `repo_queue` table rather than a whole new table — a Phase 1
implementation detail). clone-worker (or a thin variant using the same
detection code) claims it, reads the **already-stored** Parquet artifact
from ARMOR — no re-clone, no GitHub API cost — re-runs detection, and
upserts only the rollup rows that changed. Same code path, same Postgres
target, triggered rarely instead of running as an always-on service.
**filter-worker and compactor are retired as standalone deployments**; their
logic lives inside clone-worker's two job kinds.

**aggregator** (simplified): periodically queries Postgres directly — SQL
rollup + a read-time identity-alias join, mirroring
`generate_leaderboard_v3.py`'s `_apply_aliases` pattern — and publishes the
**full ranked list** (every user with rollup activity, expected to be
hundreds of thousands of rows, not a top-N cut) as a single Parquet snapshot
to ARMOR. No direct B2 SDK calls anywhere in the new pipeline, corpus or
snapshot. This removes the DuckDB decrypt/materialize-large-partition step
that caused all five prior OOM incidents.

**This snapshot is an internal pipeline artifact, not a public-facing
one.** A separate, not-yet-built downstream pipeline consumes it to
generate the actual devimprint.com presentation layer — that layer's own
design (curated top-N vs. full list, anti-scraping via rate-limiting/
pagination, etc.) is explicitly out of scope here (see "Explicitly out of
scope" below). Because the consumer is one internal process reading
occasionally rather than many public browser clients doing fine-grained
range reads, ARMOR's whole-file-decrypt-on-read is a good fit with no
efficiency tradeoff worth carving an exception for — unlike the corpus,
this was never a case where public range-read performance was the design
constraint.

**Postgres computes the ranking, not DuckDB.** Rank/30-day-window/tiebreak/
percentile-distribution all run as native Postgres SQL (window functions,
`RANK() OVER`, `PERCENTILE_CONT`) against the live rollup — the same thing
`generate_leaderboard_v3.py` already does against SQLite directly, no
Parquet round-trip in the computation path. DuckDB/Parquet's role in this
design is **output-format only**: the aggregator exports the already-computed
result (a few thousand rows) to Parquet + JSON for public serving, matching
the existing `query_leaderboard.py --parquet` read path. Routing the
computation itself through an export-to-Parquet-then-DuckDB-query step would
add an ETL hop directly in the path of the thing this redesign exists to fix
(freshness), for no benefit — the usual reason to separate write-store from
analytical-query-store (OLTP/OLAP isolation) doesn't apply here, since public
traffic never touches Postgres at all; only the aggregator's own periodic job
reads it, on an interval, which is a light, bounded load.

**queue-api**: kept as-is (39K LOC, well-tested) for `search_queue` /
`repo_queue` / `user_queue` claim-lease-complete semantics and
`catalog_version`. `dirty_partitions` goes away (nothing left to coordinate
once compactor/filter-worker are gone). `email_resolution` / `user_aliases`
stay in queue-api verbatim; aggregator reads them at query time rather than
duplicating identity data into Postgres. This is a materially lighter write
workload than today — no more dirty-partition-bump storm from every
clone-worker/filter-worker/aggregator cycle — so queue-api's existing SQLite
should be adequate; re-measure contention after Phase 2 before considering
any change here.

**Postgres**: new CNPG cluster on a **dedicated large Rackspace Spot node in
ord-devimprint** (user's explicit choice — keeps clone-worker's rollup
upserts same-cluster, no cross-cluster network hop on the hot write path).

**Confirmed via `./k8s/capacity-check.sh ord-devimprint` (2026-08-04, live):**
the existing 6-node `compute1-4` fleet is already committed 41% CPU / 50%
memory, and the largest pod that fits anywhere on it is 1.20 CPU / 1.64 GiB
— there genuinely is no room to squeeze Postgres onto existing capacity.
This validates the dedicated-node decision; it wasn't just a preference.

**Node-class naming, corrected (2026-08-04):** `compute1-4` (used above and
in `k8s/ord-devimprint/CLAUDE.md`) is the Kubernetes-facing generic
instance-type label, not the actual Rackspace Spot server class the pricing
API keys on. Live-checked via node labels (`servers.ngpc.rxt.io/class`):
ord-devimprint's current class is **`ch.vs1.medium-ord`**, not
`gp.vs1.medium-ord` as an earlier pass through this document assumed —
those are different hardware tiers at the identical advertised shape (2
CPU/3.75GB) and different prices. Corrected pricing comparison against
`us-central-ord-1` percentile data: `ch.vs1.medium-ord` (current) prices at
p50 **$0.006/hr**; `mh.vs1.large-ord` (proposed, 4 CPU/30GB) also prices at
p50 **$0.006/hr** — same expected cost tier, not cheaper as previously
stated, while still offering 2x CPU and ~8x memory. The capacity/headroom
case for this class stands; the "cheaper" framing was wrong and is corrected
here.

**Percentile pricing is a historical bid-clearing distribution, not an
availability guarantee or a reservation** — p20/p50/p80 describe what a Spot
market *has* cleared at, not what it's guaranteed to clear at when this
class is actually requested. Rackspace Spot nodepools are bid-based; a
nodepool's `fulfilled` count can come in under `desired`
(`rackspace-spot-terraform/notes/bf-1qt.md` shows this field exists because
it happens in practice on this account). No fallback class is specified if
`mh.vs1.large-ord` doesn't fulfill — decide one before attempting
provisioning, don't discover this live. This matters more than usual here
because the plan proposes a **single dedicated node** (`instances: 1`,
matching the `iad-ci/queue-db` CNPG precedent, which also runs single-
instance with no built-in HA) as the sole write target for every
clone-worker replica's rollup upserts — a preemption event on this node is a
hard outage for the entire rollup path, not a graceful degrade. The
`queue-db` precedent mitigates this with `barmanObjectStore` backups to B2
via ARMOR on a daily `ScheduledBackup`; this Postgres instance needs the
equivalent wired before it's trusted with anything, not left unaddressed as
a "someday" item.

**Adopted (2026-08-04, plan-idea-gen run 1): a backup/restore runbook is now
a required Phase 0 deliverable, not just a flagged risk.** Mirror the
`queue-db` precedent directly — `barmanObjectStore` + daily `ScheduledBackup`
to ARMOR — and additionally **document and rehearse** the manual
promote-from-backup procedure with a stated target RTO before this instance
carries any real traffic. This is also the mechanism storage/compute
expansion depends on (see the Postgres-node-expansion question raised in
conversation): a Cinder volume can't grow in place, so restoring into a
larger PVC from this same backup is the actual expansion path — meaning no
expansion path exists at all until this is done.

**Adopted (2026-08-04, plan-idea-gen run 1): infra cost must be surfaced,
tool-agnostic — explicitly not a Grafana panel by default.** Visibility into
actual Rackspace Spot bid spend for this node plus ARMOR write volume,
against the percentile pricing already cited above — but the mechanism is
deliberately unspecified here; pick whatever fits the org's existing
observability stack when this is built, not a prescribed tool choice made
during planning.

This is comfortably more capacity than CNPG + the rollup table actually
need — see the corrected sizing note in the schema section below.
Allocatable-vs-capacity ratio for this specific class hasn't been
empirically confirmed (only `compute1-4`'s ~75% CPU / ~70% memory ratio is
known from live clusters) — verify once actually provisioned, don't assume
it holds exactly.

Two known costs, both one-time and still unstarted: (1) node provisioning is
a manual, out-of-band step — Rackspace Spot node pools are provisioned via
the Spot web UI or a locally-run Terraform apply from the separate
`jedarden/rackspace-spot-terraform` repo, **not** a declarative-config PR
(in-cluster Terraform automation for this was retired org-wide 2026-04-22
after a reliability incident); (2) CNPG operator does not exist on
ord-devimprint yet and needs installing fresh (it already runs on
ardenone-cluster, apexalgo-iad, iad-ci, **and rs-manager** — this would be a
5th install, not a reuse).

### Postgres schema

Two tables, not one — claude-leaderboard's single-tool design doesn't fit;
only ~0.3% of commits are AI-tagged (234K of 76.6M), so tool-tagged data is
sparse relative to total activity:

```sql
CREATE TABLE repo_user_daily (       -- tool-agnostic totals
  repo_id    BIGINT NOT NULL,
  author_key TEXT   NOT NULL,        -- resolved login, or raw email fallback
  day        DATE   NOT NULL,
  commits    INT    NOT NULL,
  PRIMARY KEY (repo_id, author_key, day)
);
CREATE INDEX ON repo_user_daily (author_key, day);

CREATE TABLE repo_user_daily_tool (  -- sparse: only AI-tool-tagged rows
  repo_id    BIGINT NOT NULL,
  author_key TEXT   NOT NULL,
  tool       TEXT   NOT NULL,        -- plain TEXT, not enum — catalog grows
  day        DATE   NOT NULL,
  commits    INT    NOT NULL,
  PRIMARY KEY (repo_id, author_key, tool, day)
);
CREATE INDEX ON repo_user_daily_tool (author_key, tool, day);
CREATE INDEX ON repo_user_daily_tool (tool, day);

CREATE TABLE users (
  login             TEXT PRIMARY KEY,
  total_commits     BIGINT NOT NULL DEFAULT 0,  -- delta-updated, never COUNT(*)
  total_ai_commits  BIGINT NOT NULL DEFAULT 0,
  first_seen        TIMESTAMPTZ,
  last_seen         TIMESTAMPTZ
);
```

**Sizing, corrected twice now — this is the current best estimate.** An
earlier pass through this document claimed the rollup "measures in the tens
of MB," undercounting by not extrapolating from data this same document
already cites — claude-leaderboard's own directly-comparable rollup
measured 209MB for 3.48M rows from 37.9M commits (see Context above and the
research doc). A first correction applied that ratio to commitgraph's 76.6M
commits and landed on "several hundred MB." That correction was itself too
low: it carried over SQLite's per-row cost without adjusting for Postgres,
which has no equivalent to SQLite's `WITHOUT ROWID` clustered storage (the
mechanism that made claude-leaderboard's 209MB figure so compact —
~32 bytes/row for the table, ~31 for its index, per the exact `dbstat`
breakdown measured earlier). Every Postgres row instead carries its own
~24-byte tuple header plus alignment padding on top of the real column
data. Worked through per table, at current 76.6M-commit / 1,094,043-developer
scale (commits-to-rollup-row ratio held constant from the claude-leaderboard
baseline, ~10.9 commits/row):

- `repo_user_daily` (tool-agnostic, ~7.03M rows extrapolated): ~75-85
  bytes/row for the table (~560-600MB) + a comparable-magnitude
  `(author_key, day)` index (~250-280MB) → **~800MB-1.1GB**, the dominant
  cost by far.
- `repo_user_daily_tool` (sparse, AI-tagged only — ~21-25K rows from
  234,263 AI-tagged commits at the same ratio): negligible, well under
  10MB including both indexes.
- `users` (one row per developer, 1,094,043 live count): ~90-100
  bytes/row including its primary-key index → **~150-170MB**.

**Total at current scale: roughly 1.0-1.2GB** — a materially different
number from "several hundred MB," though the provisioning conclusion still
doesn't change: the `mh.vs1.large-ord` node's 30GB of RAM absorbs even a
10x-scale rollup (~10-12GB) with room to spare. The number keeps getting
corrected because each pass checked the previous one against real data
rather than trusting it — worth remembering next time a "trivial" sizing
claim shows up in this document.

**Leaderboard snapshot sizing (ARMOR, Parquet, the full ranked list):**
per-row shape matches the live JSON schema (rank, username,
ai_commits_30d, ai_commits_total, ship_streak, tools[], providers[],
top_repo, last_active, verified) — roughly 110-115 bytes/row uncompressed,
compressing to an estimated 35-50 bytes/row in Parquet given heavy
repetition in low-cardinality columns (`providers` is almost always just
`["github"]`) and small-range integers. At "hundreds of thousands of
rows" (per the earlier full-list-not-top-N decision): **roughly 10-40MB**
— never a real storage concern, unlike the rollup.

**No re-evaluation trigger is defined for "Postgres computes ranking
directly" past current scale.** The "trivial at this row count" claim is
calibrated against today's snapshot. commitgraph's own history shows the
corpus went from 1,313 commits to 402,980 in the flywheel's first hour, then
to 76.6M within about two weeks — and discovery/clone throughput are
explicitly *unclosed* ceilings (see "Explicitly out of scope" below), so
continued growth at a non-trivial rate should be assumed, not treated as a
one-time snapshot. This plan does not state a row-count or query-latency
threshold at which the direct-SQL-ranking approach should be revisited in
favor of, e.g., a materialized/precomputed ranking table refreshed on an
interval. Flagged as an open gap rather than silently assumed away — worth
picking a concrete trigger (a row count, or a measured query-latency
ceiling) before this becomes a live problem instead of a planning question.

**Adopted (2026-08-04, plan-idea-gen run 1): the trigger is a measured
query-latency SLO, not a row count.** Row count was the more obvious choice
but is a proxy for the thing that actually matters — pick one number now
(e.g., the ranking query's p99 latency staying under 2s) and monitor it;
when it's breached, that's the signal to build the materialized/precomputed
ranking table this section already names as the fallback, not before. This
deliberately doesn't build that materialization now — the SLO decides
whether it's ever needed at all, rather than assuming it will be.

Write pattern per repo, one transaction: `DELETE ... WHERE repo_id=$1` on
both rollup tables, then a single set-based bulk `INSERT` via
`UNNEST($1::bigint[], ...)` (not row-by-row — matters once multiple
clone-worker replicas write concurrently), then update `users` by delta with
explicit row locking (`SELECT ... FOR UPDATE` per touched login) —
claude-leaderboard never needed this because it was single-writer;
concurrent clone-worker replicas touching the same user across different
repos is exactly the race Postgres is being brought in to survive.

## Storage placement

- **Raw per-repo commit-history artifact** (Parquet, sha/author/email/day/
  message): ARMOR, per-repo key, whole-object overwrite on every rescan.
  ADR-009's stated objection to ARMOR (whole-file encryption defeats DuckDB
  range-read pruning) was measured against the **old** architecture, where
  the corpus was hot — read every aggregator/filter-worker cycle. In this
  redesign the corpus becomes cold/archival: written once per clone/rescan,
  read back only for rare catalog-triggered redetect jobs, so the
  cold-access argument for using ARMOR here holds regardless of the
  following correction.
  **Correction (2026-08-04, from adversarial review): the "whole-file
  encryption" characterization of ARMOR was never independently
  re-verified against ARMOR's actual current behavior.** ARMOR's own
  README claims the opposite is true today — seekable AES-256-CTR with
  64KB blocks, explicit DuckDB range-read/column-pruning compatibility.
  Either ADR-009 mischaracterized ARMOR at the time it was written, or
  ARMOR gained seekability afterward; this was never checked before being
  repeated as settled fact in this plan's earlier notes. This doesn't
  overturn the decision to keep this artifact cold/internal (the
  access-pattern argument above is independent of it), but the plan's
  stated *technical justification* was weaker than presented. Given
  ARMOR's own repo also documents a recent, non-trivial history of
  multipart-encryption corruption bugs (as recently as 2026-07-18 per its
  own ADRs), verify ARMOR's actual current range-read behavior empirically
  before relying on any specific performance characteristic of it, rather
  than trusting either ADR-009's or ARMOR's own README's claims
  uncritically.
  **ADR-009's other two objections to ARMOR — proxy-as-SPOF and
  cross-namespace coupling — are not addressed by this reversal and should
  be named rather than silently reintroduced.** This design's actual ARMOR
  exposure is much narrower than what ADR-009 was reacting to: ARMOR sits
  in the write path for clone-worker's raw-history artifact and the
  aggregator's periodic snapshot publish, but is **not** in the hot query
  path at all — every live ranking query goes to Postgres directly, so
  ARMOR being briefly unavailable delays extraction/publishing, it doesn't
  take down ranking. That materially shrinks the SPOF concern versus
  ADR-009's original worry (every corpus read routing through one proxy).
  The cross-namespace coupling is real and knowingly accepted, not
  resolved: clone-worker (namespace `commitgraph`) depends on ARMOR running
  in namespace `devimprint`, the exact shape ADR-009 wanted to move away
  from. Worth an explicit decision — accept the coupling, or give this
  redesign its own ARMOR deployment scoped to `commitgraph` — rather than
  leaving it as an unstated assumption.
- **Warm-start artifact** (raw pack files + loose ref + three promisor
  config values, tarred — see Architecture above and
  `docs/research/incremental-fetch-warm-start.md` for the full
  methodology): ARMOR, per-repo key **distinct from** the Parquet
  commit-history artifact above — these are two different artifacts with
  two different purposes (Parquet: redetection, no git tooling needed;
  this: warm-starting the *next* clone so it fetches only new commits
  instead of re-downloading full history). Same cold/whole-object-overwrite
  reasoning as the Parquet artifact applies equally here. Validated
  end-to-end against a real repo at real repo size (a few hundred commits);
  **not yet validated at the corpus's actual large-repo scale** or under
  concurrent multi-replica access — treat as a real, working mechanism with
  a real, unclosed scale question, not as fully proven.
  **Adopted (2026-08-04, plan-idea-gen run 1): a two-part mitigation, in
  order, not both at once.** (1) Run the smoke test this gap already calls
  for — against the corpus's genuinely large repos — before assuming
  mitigation is even needed; the mechanism may simply hold at scale. (2)
  Only if that smoke test finds a real problem: sticky worker affinity
  scoped *specifically* to large repos (not the fleet generally, which
  deliberately has no affinity today) enabling local caching, combined with
  keeping any repack step append-only — never invoking `git gc`/`repack`
  the way the rejected bundle transport did, since that's the exact
  mechanism that produced the 127x bloat this design already worked around.
  Framed as a fallback gated on evidence, not a default addition.
- **Leaderboard snapshot** (`aggregates/leaderboard.parquet`, full ranked
  list, hundreds of thousands of rows): also via ARMOR, not direct B2 — no
  component in the new pipeline talks to B2 directly. No `leaderboard.json`
  or other public-serving format is produced by this pipeline — that's the
  downstream devimprint presentation layer's concern (out of scope here).
- **Pre-checks before depending on ARMOR** (from architecture review, not
  yet verified live): `ARMOR_PREFIX` is currently unset on the live
  `devimprint`-namespace ARMOR instance (dedicated-bucket mode) — decide and
  wire explicit scoping for commitgraph's objects before writing. Confirm
  which ARMOR instance is in scope (there are at least two org-wide —
  `devimprint` namespace on ord-devimprint vs. `armor` namespace on iad-ci).
  Object sizes for the Parquet artifact are small relative to ARMOR's
  historical multipart corruption bug's trigger conditions (typically KBs
  to low tens of MB even for large repos) — low risk. **The warm-start
  artifact's size at scale is a real, separate open question** — unlike
  the compact Parquet extraction, it contains actual git pack data, and
  the one real repo tested was a few hundred commits; a smoke test against
  the corpus's genuinely large repos is needed before assuming this stays
  cheap, not just for the Parquet artifact.

## Corpus migration (inherit, don't rediscover)

GitHub's shared ~30 req/min search budget makes rediscovery from zero take a
comparable amount of time to how long it took to reach current scale — the
existing corpus must be inherited, not rebuilt via re-discovery.

The existing corpus (old architecture, direct-B2, encrypted Hive
`provider/year/month` partitions) already retains full commit `message` text
per commit (confirmed: `pa.field('message', pa.string())` in the current
clone-worker). The migration:

1. Streams the existing corpus partition-by-partition (Arrow batch API, not
   `fetchall()` — a prior incident already OOM'd a 2Gi pod materializing
   400K commits' message bodies at once; this must not be repeated at 76M
   scale).
2. Enumerates all distinct `key_id` (epoch key) values across manifests
   first and confirms migration credentials can decrypt all of them —
   scoping to only the current epoch would silently skip older partitions
   still sitting on retired epochs.
3. Per repo: re-runs `shared/detection.py` (Python, not reimplemented in
   SQL — two sources of truth would drift) to compute `(user, repo, tool,
   day, count)`, and **also** re-packages that repo's already-extracted
   commit rows into the new per-repo ARMOR artifact — so every repo, old and
   new, has raw history reachable through the one uniform mechanism, and
   redetect jobs work identically regardless of when a repo was first
   scanned. This is a data-movement pass (existing corpus → Postgres +
   ARMOR), not a re-clone — it does not touch GitHub's search/API budget.
4. Writes rollups to Postgres using the same DELETE+bulk-INSERT pattern as
   live clone-worker, so the migration and live traffic are logically
   identical operations.
5. Tracks progress in a `migration_progress(partition_key, completed_at)`
   table so a killed job resumes at the next partition instead of restarting
   a multi-hour job from zero.
6. Is idempotent and must be tested as such: run twice, assert
   `users.total_commits` is unchanged on the second run (the delta-update
   pattern is not naturally idempotent the way the DELETE+INSERT rollup
   rows are — verify explicitly before trusting it).

## Phased rollout

1. **Phase 0 — capacity & provisioning.** Confirm real available node
   classes/pricing for a large node in the ORD region (only
   `gp.vs1.medium-ord`-class pricing is currently known). Provision the
   dedicated Postgres node manually (Spot UI or `rackspace-spot-terraform`).
   Install CNPG operator on ord-devimprint. Stand up the Postgres cluster
   with the schema above.
2. **Phase 1 — isolated build.** New Forgejo repo, new clone-worker
   (Postgres rollup write + ARMOR per-repo artifact write, detection logic
   inlined), new aggregator (Postgres read + ARMOR-published snapshot).
   Built and tested against a handful of real repos, zero production
   traffic — queue-api and the live pipeline are completely untouched at
   this stage.
3. **Phase 2 — subset validation.** Drive a bounded repo set through the
   new path in parallel with the untouched old pipeline. Cross-check rollup
   counts against the existing corpus for the same repos. **Load-test
   concurrent clone-worker replicas writing Postgres simultaneously** — this
   is the entire reason Postgres was chosen over SQLite, so prove the
   concurrent-write path actually holds up before trusting it. Test the
   catalog-version-bump → redetect-job path end-to-end with a synthetic new
   tool signature.
4. **Phase 3 — bulk corpus migration (offline, read-only against old
   pipeline).** Run the migration described above against the full existing
   corpus. Old pipeline keeps running untouched throughout.
5. **Phase 4 — shadow / dual-write burn-in.** Repoint discovery workers
   (search-worker, user-worker) at the new queue-api using the
   already-proven `QUEUE_API_URL`-swap pattern from this project's own
   history. Run both pipelines in parallel for a defined burn-in window —
   long enough to see at least one catalog-version bump and one full
   aggregator publish cycle.
   **Corrected diff mechanism (2026-08-04):** the acceptance gate as
   originally written here — "diff `leaderboard.json` output between old
   and new" — directly contradicted the Architecture/Storage-placement
   sections, which say this pipeline produces **no public-serving artifact
   at all** (the ranked-list snapshot is internal-only, in ARMOR, for a
   separate downstream pipeline to consume — see above). There is nothing
   of that shape to diff. The actual gate: a validation script queries the
   new pipeline's Postgres ranking directly (same SQL the aggregator runs)
   and compares its output against the old pipeline's public
   `leaderboard.json`, row-for-row on rank/username/counts — comparing two
   different representations of the same computed ranking, not two files
   of identical shape.
6. **Phase 5 — final delta + cutover.** The old pipeline keeps discovering
   and cloning new repos through phases 1-4 — the corpus is not a static
   migration target. Run one final delta migration pass immediately before
   cutover to catch everything scanned during the migration window.
   **Open decision, not yet resolved (flagged by adversarial review,
   2026-08-04): what happens to public serving at the moment of cutover?**
   The old pipeline's public `leaderboard.json`/dashboard is the only
   public-facing artifact that exists today. Under the internal-only
   snapshot decision, the new pipeline has no artifact to replace it with
   at cutover — the downstream devimprint presentation layer that would
   consume the internal snapshot is explicitly out of scope and not
   started. So literally as designed, decommissioning the old pipeline's
   public serving at cutover (Phase 6) leaves public serving **dark** until
   that separate downstream effort ships. This may be an acceptable,
   deliberate tradeoff — but it hasn't been decided, only exposed by
   working through the mechanism in detail. The alternatives are: (a)
   accept the gap, treat "public dashboard" as intentionally offline
   between old-pipeline shutdown and the downstream pipeline shipping; (b)
   keep the old pipeline's public serving alive past Phase 6 even after its
   data pipeline is decommissioned, pointed at a bridge export from the new
   corpus, until the downstream piece is ready; (c) have this pipeline
   additionally publish a minimal public artifact (small top-N, not the
   full list) purely to avoid a hard outage, despite the internal-only
   decision. Needs an explicit choice before Phase 5 can actually execute.
7. **Phase 6 — shutdown & decommission.** Per user's explicit instruction:
   shut down the live pipeline once parity is confirmed. **The repo rename
   already happened (2026-08-04, ahead of the rest of this phase)** —
   `jedarden/commitgraph` → `commitgraph-deprecated`, this repo took over
   the canonical `commitgraph` name, on both Forgejo and GitHub, with
   push-mirrors, local remotes, and most found references to the old name
   updated in the same pass. **This was not fully complete — a live CI gap
   from it is still open as of this writing; see
   `docs/notes/repo-rename-2026-08-04.md` for the full account, including
   what the reference sweep did and didn't cover.** ArgoCD's `repoURL`
   needed no change (it syncs from `declarative-config`, never referenced
   `jedarden/commitgraph` directly) — that part held up under scrutiny.
   What's left for this phase is unchanged: shut down the live pipeline once
   parity is confirmed, then decommission old k8s manifests via the existing
   "disable in git (`.disabled` suffix) → push → prune, direct-kubectl-delete
   only for objects git no longer declares" playbook already used once
   before for a structurally identical teardown in this project's own
   history.

## Explicitly out of scope

- **Discovery and clone throughput are hard, confirmed ceilings, not gaps
  this redesign closes.** Discovery is capped at GitHub's shared ~30 req/min
  search budget. Clone throughput has its own independently-measured
  ceiling: a single clone-worker replica processes roughly 1,000 repos/hour,
  against a discovered-repo backlog that has run into the tens of thousands
  (28.9k+ pending at last measurement). Freshness of already-discovered
  repos improves under this redesign (rollup written in the same pass as
  extraction, not up to 24h later); the *rate* new repos get discovered and
  cloned does not change at all.
- **Identity resolution — architecture is already shared with
  claude-leaderboard, not something this redesign needs to newly borrow.**
  commitgraph's `email_resolution` (cached per-email API resolution) and
  `user_aliases` (hand-curated read-time merge) are already the same
  two-layer design claude-leaderboard's `author_login_cache` +
  `_load_username_aliases()` used — this redesign explicitly keeps both
  verbatim and applies the alias join at read time in the aggregator,
  matching `generate_leaderboard_v3.py`'s `_apply_aliases` pattern exactly.
  What genuinely doesn't change: resolution *throughput* is capped by the
  **same shared ~30 req/min GitHub API budget as discovery search** — this
  redesign adds no resolution capacity, and the backlog (363k pending emails
  at last measurement) is structurally bounded by that shared resource, the
  same way discovery is. What plausibly *does* improve: a live bug where the
  resolution feed silently failed (`fed 0 new emails`) traced directly to
  queue-api write contention — the exact thing this redesign eliminates — so
  already-rate-limited resolution progress should land reliably post-cutover
  instead of intermittently dropping.
  **Concrete, low-risk follow-up identified but not yet built:**
  claude-leaderboard's frozen `author_login_cache` (349,425 already-resolved
  email→login pairs, `~/backups/claude-leaderboard/hot.db`, spanning
  2026-03-14 to 2026-06-29) can seed `email_resolution` directly — zero
  GitHub API cost, immediate backlog relief. Positive resolutions only (no
  negative-cache equivalent in the source), and coverage is necessarily
  partial (claude-leaderboard only ever searched `Co-Authored-By: Claude`,
  so this helps pre-freeze Claude-Code-tagged identities specifically, not
  the other 11 tools or post-freeze activity). Needs a small new queue-api
  endpoint (`POST /email-resolution/seed` or equivalent) — the existing
  `/resolve` endpoint enforces claim/lease state (`ErrClaimConflict` is a
  real return path per `internal/server/email_resolution.go`), so it's not
  a fit for a one-time bulk historical import; `/upsert` would just re-queue
  these as pending work, wasting the exact budget being saved.
  **Conflict-handling rule, specified (2026-08-04, from adversarial
  review):** an earlier pass through this document left the seed's write
  semantics unstated, which matters — the schema's `claimed_by` /
  `lease_expires_at` / `attempted_at` columns exist precisely to prevent
  two writers racing the same email. The correct rule: the seed endpoint
  only writes a row where **`status = 'pending'`** (i.e., never claimed by
  any worker, ever) — skip everything else outright, including rows
  currently `claimed`, even if their lease looks expired. This sidesteps
  the race with a live enrichment worker entirely rather than trying to
  detect and resolve it, at the cost of very rarely leaving a handful of
  already-in-flight rows unseeded — acceptable, since those resolve
  normally through the live worker regardless. This also makes the seed
  naturally idempotent: a second run only ever touches rows still pending,
  never re-touches what it or a live worker already resolved. Do **not**
  seed by overwriting `claimed` or already-`resolved`/`unresolvable` rows —
  the frozen cache is 5-8+ weeks stale by the time it would run, so a stale
  seed value must never be allowed to beat a live worker's fresher result.
  This can run against the **currently live** deprecated pipeline
  immediately, doesn't need to wait for any phase of this redesign, and the
  new pipeline should do the same seed during its own bootstrap. Not built
  yet — deferred until
  Phase 1 implementation starts.
- **The detection footprint gap is real but deliberately not fixed here.**
  `shared/detection.py`'s catalog covers 15 tools; `search-worker`'s active
  discovery footprints (`GITHUB_FOOTPRINTS`) cover 12 — windsurf, codeium,
  replit, tabnine, codestral, sweep, and netlify-coding have detection
  patterns but no dedicated discovery query. Every commit in any cloned repo
  is still checked against the full 15-tool catalog regardless (detection
  runs one call per commit, all patterns at once — see
  `docs/notes/detection-inlined-not-lost.md`), so these tools are detected
  whenever a repo surfaces via some *other* footprint; they just can't be
  the sole reason a repo gets discovered. Closing this gap is a pure
  discovery-breadth-vs.-already-maxed-API-budget tradeoff — adding entries
  to `GITHUB_FOOTPRINTS` trades search calls away from the existing 12
  tools' freshness/history depth toward broader coverage. This tradeoff
  predates this redesign, isn't caused or worsened by it, and is an
  independent decision for whoever owns the discovery footprint list — not
  bundled into this plan.
- **The public devimprint.com presentation layer** — curated top-N vs. full
  list, anti-scraping design (rate-limiting, pagination, profile-lookup vs.
  bulk export), the general revival of devimprint.com as a display layer —
  is a separate, not-yet-started downstream effort. This plan's
  responsibility ends at publishing a correct, complete Parquet snapshot to
  ARMOR; what consumes that snapshot and how it's exposed publicly is out
  of scope here.

## Verification

- Migration idempotency test (Phase 3, step 6 above) must pass before any
  dual-write phase begins.
- **Adopted (2026-08-04, plan-idea-gen run 1): the idempotency check becomes
  a permanent CI gate, not a one-time pre-flight.** The one-time migration
  check above stays as written — but rollup-writing code isn't only the
  migration, it's clone-worker's every-job write path, which keeps changing
  after launch as the codebase evolves. Add an automated test (run the same
  repo through clone-worker's rollup write twice, assert `users.total_commits`
  is unchanged the second time) to CI, so this stays enforced on an ongoing
  basis rather than verified once and assumed to hold forever.
- Phase 4's continuous `leaderboard.json` diff is the primary correctness
  gate for cutover — not "pods are Running" (the same false-positive
  pattern that cost real time earlier in this project's history).
- Phase 2's concurrent-write load test is the primary validation that
  Postgres actually solves the problem this redesign exists to fix — if it
  doesn't hold up under concurrent clone-worker replicas, the core premise
  needs revisiting before going further.

## Critical files referenced

- `/home/coding/commitgraph/shared/detection.py` — reused as-is by the new
  clone-worker
- `/home/coding/commitgraph/containers/clone-worker/worker.py` — current
  extraction logic and Parquet schema to build on
- `/home/coding/commitgraph/containers/queue-api/schema.sql` — current
  `catalog_version` / `dirty_partitions` mechanism
- `/home/coding/commitgraph/docs/adr/009-encrypted-public-b2-storage.md` —
  ADR-009, being partially reversed for the reasons stated above
- `/home/coding/vibecodeleaderboard-backend/src/extractor_v4.py` — the
  reference idempotent DELETE+INSERT rollup pattern
- `/home/coding/declarative-config/k8s/iad-ci/queue-db/cnpg-cluster.yaml` —
  closest existing CNPG manifest precedent (schema/resources shape, not
  cluster placement)
- `/home/coding/declarative-config/k8s/ord-devimprint/commitgraph/` — old
  manifests to decommission in Phase 6
