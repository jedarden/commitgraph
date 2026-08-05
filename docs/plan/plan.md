# commitgraph v2 — redesign plan

> **Read "Status — the old pipeline is gone" first.** The predecessor was
> torn down on 2026-08-05. This Context section is written in the present
> tense of 2026-08-04, when the pipeline was still running; it is retained
> because the diagnosis it records is the entire reason for this redesign,
> but "the live pipeline" below now means "the pipeline as it last ran."

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

**No acceptance target is set for that 40x gap, and that is deliberate
(operator, 2026-08-05: "to be determined").** It is worth being explicit
about why the target is hard to set rather than leaving the silence to be
read as an oversight: this redesign closes the freshness half of the gap
(24h → a 15-minute publish cycle) and improves the identity half only
insofar as inheriting `email_resolution` and merging aliases before ranking
resolves fragmentation that already had answers. It adds **no new resolution
throughput** — that remains capped by the same shared ~30 req/min GitHub
budget as discovery (see "Explicitly out of scope"). So the post-cutover
number depends on which half dominates for any given user, which nothing has
yet measured. The golden-snapshot comparison in "Verification" is the first
opportunity to measure it: its own rank 1 is an unresolved raw email, which
suggests fragmentation is not a minor term.

What the operator does want post-cutover, and what this plan now delivers:
**per-user visibility into how recently their repos were scanned** — see
"`insert_time` — scan recency, not commit recency". A user seeing a number
they think is too low should be able to tell whether the pipeline has looked
at their work recently, which is a different and more answerable question
than "is this number right."

Investigating why led to comparing commitgraph against its predecessor,
claude-leaderboard (`vibecodeleaderboard-backend`), whose architecture is
radically simpler: single SQLite file, a `(repo, user, day)` rollup table
maintained by idempotent whole-repo-slice replace on every rescan, read-time
identity aliasing, no distributed coordination. That system reached 37.9M
commits / 695K users on one box with no equivalent contention. Live-measured
this session: `hot.db` — a separate, frozen, pruned working-set file with no
raw `commits` table at all — has a rollup table of 209MB for 3.48M rows, a
~10.9-commits/row ratio. **Corrected 2026-08-04 (gap-review round 5): the
Postgres sizing section below no longer extrapolates from this hot.db
ratio** — it now uses `leaderboard.db`'s own self-consistent ratio instead,
for the same reason the round-3 correction just below rejects hot.db/
leaderboard.db mixing for this exact class of measurement (see the sizing
section for the corrected figure). **Corrected 2026-08-04 (gap-review round 3): the full-database
breakdown below was previously computed by dividing that 209MB figure into
a *different* file's total, and the resulting percentage didn't even follow
from that division.** Measured self-consistently instead, against
`leaderboard.db` — the one file that actually holds the raw commit log and
the rollup side by side (27.9GB total, 52.5M commits at last measurement,
grown since `hot.db`'s freeze): the raw commit log + its six index
structures (5 explicit indexes plus the implicit `sqlite_autoindex_commits_1`
its `PRIMARY KEY` creates on this rowid table) is 95.8% of the file; the
rollup (table + its two indexes) is 738MB / 7.97M rows, **2.8%** — not the
previously-stated 2.6% (which came from dividing
`hot.db`'s 209MB by `leaderboard.db`'s 27.9GB, two different files, and
doesn't even arithmetically yield 2.6% — that division is actually ~0.8%).
Either way, correctly computed: confirming the rollup itself was never the
expensive part. Full measurement in
`docs/research/claude-leaderboard-comparison.md`.

This plan applies that lesson to commitgraph without discarding the things
commitgraph got right that claude-leaderboard never needed: multi-tool
detection (21 tools in the full catalog, per `shared/detection.py`'s
`ALL_TOOLS` — a different number from, and not to be confused with, the
separate 11-tool discovery-footprint gap covered in "Explicitly out of
scope" below, which is about coverage of the *discovery* query list, not
the size of the detection catalog itself — vs. claude-leaderboard's single
hardcoded `Co-Authored-By: Claude` trailer) and the ability to retroactively
re-detect
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
role. **Corrected 2026-08-04 (gap-review round 5): the `fed 0 new emails`
bug cited in earlier drafts here already has its own independent fix,
predating this plan.** `commitgraph-deprecated` commit `e37bb5b`
("queue-api: split reader/writer DB pools, add export cursor (2.8.0)",
2026-07-31 — four days before this plan's 2026-08-04 origin) diagnosed the
exact symptom (`/email-resolution/export` pulled the full 365K+ row table,
unfiltered, on every aggregator cycle) and shipped a fix — a separate
reader pool plus a `since=` cursor; queue-api runs at 2.8.0 with this fix
live today. This redesign's write-contention relief is broader and still
real (see the `context canceled`/`Read timed out` evidence above), just no
longer evidenced by this one already-patched instance. What this redesign
does **not** fix: discovery and clone throughput, both hard
ceilings on the shared GitHub API budget and clone-worker's own measured
per-replica rate; and identity resolution *throughput*, capped by that same
shared API budget regardless of storage architecture (commitgraph already
reuses claude-leaderboard's identity-resolution design — cached per-email
resolution + read-time alias merge — this isn't a missing piece to borrow).
See "Explicitly out of scope" below for the full accounting, including a
concrete, not-yet-built follow-up (seeding `email_resolution` from
claude-leaderboard's own frozen resolution cache) that would help
independently of this redesign's timeline.

## Status — the old pipeline is gone

**2026-08-05: the predecessor pipeline was torn down.** Per the operator's
explicit instruction — *"The old system is gone and deprecated. This version
has to work. There is no turning back."* — every processing workload in the
`commitgraph` namespace on ord-devimprint was disabled in declarative-config
and pruned by ArgoCD: aggregator, compactor, filter-worker, all three
clone-worker variants, search-worker, user-worker, user-enrichment-worker,
admin-ui, oauth2-proxy, admin-alias-sync.

**Deliberately kept alive: `queue-api`, its Service, and its PVC.**
`queue-api-data` holds `email_resolution` — 365K+ resolved email→login pairs
representing months of spent GitHub API budget against a shared ~30 req/min
ceiling — which this pipeline inherits rather than re-earns (see "Identity
lives in Postgres" below). The `sata` StorageClass has
`reclaimPolicy: Delete`, so pruning that PVC destroys the Cinder volume and
every row in it. Extraction is blocked on a refreshed
`ord-devimprint-admin.kubeconfig` (currently 401). Full completion checklist
in `declarative-config/k8s/ord-devimprint/commitgraph/TEARDOWN.md`.

Hand-curated aliases needed no extraction — they live in
`admin-alias-configmap.yml` in git, not only in the database.

**Consequences that ripple through this plan:**

1. **There is no rollback target.** "Roll back" now means *forward recovery*
   — restore Postgres from backup and replay — not "switch back to the old
   pipeline." This raises the bar on the backup/restore rehearsal rather
   than lowering it; see "Durability" below.
2. **The old pipeline can no longer serve as a live correctness baseline.**
   Its aggregator had already been failing readiness for ~5 days at teardown
   (`Available: False / MinimumReplicasUnavailable`), so its output was
   stale before it stopped. Phase 4's original gate — continuously diff
   against the old pipeline's `leaderboard.json` — is void. Replaced with
   two gates that don't need a running predecessor; see "Verification".
3. **A golden snapshot was frozen before teardown.** The last published
   `leaderboard.json` (generated 2026-08-03T22:05:42Z, 100 rows, sha256
   `cf2ef378…77cc8`) is archived at
   `~/backups/commitgraph-cutover/` on ex44 as the comparison baseline.
   `commitgraph.jedarden.com` still serves that frozen file.

## Open decisions

Pointers only — each item is discussed in full where it's flagged inline
elsewhere in this document.

- **Spot fallback node class** if `mh.vs1.large-ord` doesn't fulfill on the
  bid market (Postgres provisioning section; Phase 0).
- **`mh.vs1.large-ord` bid price**: a low bid (mirroring the current
  `ch.vs1.medium-ord` nodepool's $0.001/hr) rather than the $0.006/hr p50 is
  the reasonable default, but hasn't been explicitly decided (Postgres
  provisioning section; Phase 0).
- **Postgres replica topology**: `instances: 1` (matching the `queue-db`
  precedent) versus `instances: 3` with synchronous replication. With no
  fallback system, a preemption on a single Spot node is a hard outage of
  the only write target (Durability section; Phase 0).
- ~~**Identity ingest breadth**~~ — **resolved 2026-08-05: AI-relevant only.**
  See "The rollup holds AI-relevant commits only".
- **Write-path admission control**: lease-concurrency only, PgBouncer, or a
  purpose-built rate-limiting write API in front of Postgres (Write-path
  admission control section).
- **ARMOR cross-namespace coupling**: accept `commitgraph` depending on
  ARMOR in `devimprint`, or give this redesign its own scoped ARMOR
  deployment (Storage placement section).
- **ARMOR instance/prefix scoping**: `ARMOR_PREFIX` is unset and four
  org-wide ARMOR instances exist (verified via `declarative-config/k8s/`:
  `devimprint` ns on ord-devimprint, `armor` ns on iad-ci, `armor` ns on
  iad-kalshi, `armor` ns on rs-manager) — confirm which is in scope before
  writing (Storage placement section).
- **How long the frozen public `leaderboard.json` can stay frozen** before
  the downstream presentation layer must ship or the file must be pulled
  (Phase 5 section).

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
   committed_at, message)` per commit. **Corrected 2026-08-04 (gap-review
   round 3): this is a deliberate subset of clone-worker's current 10-field
   Parquet schema — `schema_version, sha, provider, repo, username,
   author_name, author_email, committed_at, subject, message`
   (`commitgraph-deprecated/containers/clone-worker/worker.py:328-339`; the
   previously-cited line 338 alone is only the schema's trailing `message`
   field, not evidence of the claimed shape).** The four dropped fields are
   intentional, not an oversight: `provider`/`repo` are redundant with the
   per-repo ARMOR key the new artifact is already written under (step 5b
   below); `username` is dropped because clone-worker doesn't meaningfully
   resolve it today either (`resolve_username()`, in that same `worker.py`
   cited just above, is noreply-regex-only, zero network cost — everything
   else is deferred to user-enrichment-worker, a separate container with
   its own file) and the new design keeps identity
   resolution read-time via `email_resolution`/`user_aliases` (aggregator,
   below) rather than carrying a raw-parsed username forward; `subject` is
   dropped because today's own `msg = f"{subj}\n\n{body}"` construction
   already puts it at the top of `message`, making a separate column
   redundant with it.
3. Runs `shared/detection.py` inline per commit (reused as-is — it already
   operates on a single commit's message/trailer text, no interface change
   needed).
4. Computes `(user, repo, tool, day, count, insert_time)` rollup rows for the
   whole repo — **AI-tool-tagged commits only** (decided 2026-08-05, see
   "The rollup holds AI commits only" below). Total commit counts are not
   rolled up; they stay with the raw git data persisted to ARMOR.
5. In one pass: **(a)** upserts the rollup into Postgres, **(b)** writes the
   full per-repo commit-history artifact (the extracted rows from step 2,
   as Parquet — not a raw git bundle, so no git tooling is needed to re-scan
   it later) to ARMOR at a per-repo key, **(c)** writes the updated
   warm-start artifact (see below) to ARMOR at a separate per-repo key —
   **overwritten wholesale** on every rescan, same idempotency pattern as
   (b). All writes happen in the same job; if any fails the job fails and
   gets re-claimed (no partial-state).

**Deferred hardening, not Phase 1 scope (2026-08-04, plan-idea-gen run 1):
poison-pill isolation around step 3's detection call.** Wrap the per-commit
`shared/detection.py` call so a single malformed commit message that
crashes or hangs detection is caught, logged, and skipped, rather than
blocking extraction for the whole repo. Real, low-cost hardening — but not
built until a first real incident makes it worth prioritizing over other
Phase 1 work, so step 3 above describes the call as it actually ships in
Phase 1, without this wrapper.

This mirrors claude-leaderboard's core idempotency trick — full re-derive +
whole-slice replace — applied to three destinations instead of one SQLite
file.

**The warm-start artifact is a raw pack-file transport, not a `git
bundle`.** This was tested both ways. `git bundle create` on a
`--filter=blob:none` clone produced a bundle **~127x larger** than the
source repo's own pack (105MB from an 833KB source, for a sub-1,000-commit
repo), took 22.6 seconds (~60 CPU-seconds, multi-threaded) to create, and
left a bloated pack behind in the source repo as an undesirable side effect — confirmed specific to
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
implementation detail). **Verified against the live schema
(`commitgraph-deprecated/containers/queue-api/schema.sql`): adding a `kind`
column alone is not sufficient.** `repo_queue`'s existing `UNIQUE (provider,
repo_full_name)` constraint is keyed on repo identity alone, so a bare new
column would make a repo's pending redetect-job row collide with its
pending normal-clone-job row the first time both are due at once — e.g. a
catalog-version bump firing a redetect job for a repo that's also
independently due for its normal rescan — and the second `INSERT` would
violate the unique constraint. The constraint must widen to `UNIQUE
(provider, repo_full_name, kind)` (or equivalent) as part of this change,
not just "add a column." clone-worker (or a thin variant using the same
detection code) claims it, reads the **already-stored** Parquet artifact
from ARMOR — no re-clone, no GitHub API cost — re-runs detection, and
upserts only the rollup rows that changed. Same code path, same Postgres
target, triggered rarely instead of running as an always-on service.
**filter-worker and compactor are retired as standalone deployments**; their
logic lives inside clone-worker's two job kinds.

**Gap identified and closed (2026-08-04, gap-review round 4): "their logic"
above only accounts for filter-worker.** filter-worker's tool-detection is
inlined into clone-worker's step 3 above, but compactor's other real job —
quarantining commits with malformed/out-of-range `committed_at` — has no
replacement anywhere in this design. The old compactor
(`commitgraph-deprecated/containers/compactor/worker.py`) routed any commit
dated before 2005-01-01 or more than 24h in the future to a
`year=0000/month=00` partition, excluded from every downstream aggregate
while still preserving the raw value on the row. This isn't a hypothetical
risk: a single 2170-dated commit once reached the old aggregator unguarded
and pulled its 30-day anchor 144 years forward, zeroing the board-wide
AI-commit count until a defensive clamp was added (quarantine
introduced in `bf-jyctj`/commit `93dc8d1`; aggregator incident fix in commit
`946e815`). **This can recur here just as easily**: clone-worker's step 2
extracts `committed_at` from `git log` against the raw clone, not the
GitHub API — commit dates are entirely client-set (clock skew or a crafted
`--date`) regardless of which pipeline reads them, so nothing about this
redesign's ingestion path is structurally safer than the one that already
needed this guard once. And Postgres's `day DATE` column has no bound
constraint, so a malformed value wouldn't fail the INSERT — it would sit
there silently corrupting any MIN/MAX-day-based read (`users.first_seen`/
`last_seen`, any 30-day-window ranking), the same silent-corruption shape
as the 2170 incident, just in Postgres instead of the old Python
aggregator. **Fix: clone-worker's step 4 rollup computation must exclude
commits with `committed_at` outside `[2005-01-01, today+1]`** from the
`(user, repo, tool, day, count)` rollup — mirroring compactor's exact old
bound — before any `day` value reaches Postgres. The raw per-repo Parquet
artifact (step 5b) still retains the unclamped value verbatim, matching the
old design's preserve-raw/exclude-from-aggregate split, since that artifact
is read back for redetection, not for ranking.

**aggregator** (simplified): every 15 minutes, queries Postgres directly —
SQL rollup joined to `email_resolution` and `user_aliases` in the same
database, so the alias merge is a real join evaluated before `RANK()` rather
than a post-hoc pass (see "Identity lives in Postgres" above; this is the
same *semantics* as `generate_leaderboard_v3.py`'s `_apply_aliases`, which
achieved it with an in-memory dict merge only because its rollup and its
alias table were in one SQLite file) — and publishes the
**full ranked list** (every user with rollup activity, expected to be
hundreds of thousands of rows, not a top-N cut) as a single Parquet snapshot
to ARMOR. No direct B2 SDK calls anywhere in the new pipeline, corpus or
snapshot. This removes the DuckDB decrypt/materialize-large-partition step
that caused all five prior OOM incidents. **The "effectively immediate"
freshness claim in Context above applies to the rollup write only** (same
pass as extraction) — the published snapshot itself is only as fresh as
this 15-minute publish cycle, so end-to-end freshness for anything reading
the snapshot is bounded by that interval, not by the rollup's immediacy.

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
the existing `commitgraph-deprecated/query_leaderboard.py --parquet` read
path. Routing the
computation itself through an export-to-Parquet-then-DuckDB-query step would
add an ETL hop directly in the path of the thing this redesign exists to fix
(freshness), for no benefit — the usual reason to separate write-store from
analytical-query-store (OLTP/OLAP isolation) doesn't apply here, since public
traffic never touches Postgres at all; only the aggregator's own periodic job
reads it, every 15 minutes, which is a light, bounded load.

**queue-api**: kept as-is (39K LOC, well-tested) for `search_queue` /
`repo_queue` / `user_queue` claim-lease-complete semantics and
`catalog_version`. `dirty_partitions` goes away (nothing left to coordinate
once compactor/filter-worker are gone). This is a materially lighter write
workload than today — no more dirty-partition-bump storm from every
clone-worker/filter-worker/aggregator cycle — so queue-api's existing SQLite
should be adequate; re-measure contention after Phase 2 before considering
any change here.

### Identity lives in Postgres, not queue-api

**Decided 2026-08-05. This reverses an earlier draft of this plan**, which
kept `email_resolution` / `user_aliases` in queue-api verbatim and had the
aggregator "read them at query time." That was not implementable as written:
the rollup would be in Postgres while the identity tables sat in queue-api's
SQLite behind an HTTP API, and there is no such thing as a SQL join across
that boundary.

**The decisive argument is ordering, not convenience: you cannot rank first
and merge identities afterward.** When one person's commits sit under two
email keys, ranking before the merge produces two wrong rows, and merging
afterward changes their totals — which changes their rank, and everyone
else's. Rank must be computed *after* alias resolution, so the identity data
has to live wherever ranking happens. That is Postgres.

The alternative — merging downstream, in the aggregator, at assembly time —
was rejected for a second reason: it means pulling the rollup into aggregator
memory to merge in Python, which is the exact materialize-everything shape
behind the five prior OOM incidents this redesign exists to end.

**The split is by role, not by table:**

- **queue-api keeps the resolution *work queue*** — `claimed_by`,
  `lease_expires_at`, `attempted_at`, and the pending backlog the enrichment
  worker drains. That is job coordination, which is queue-api's actual job
  and what its claim/lease machinery is for.
- **Postgres owns the resolution *results*** — `email_resolution`
  (email → login) and `user_aliases` (login → canonical login), both
  read-only from the pipeline's perspective and written only through the
  ingest path below.

### Identity ingest endpoint

Resolved mappings reach Postgres through one bulk-upsert ingest path, with
three writers: the live enrichment worker as it resolves, the one-time
claude-leaderboard seed (349,425 frozen pairs, see "Explicitly out of scope"),
and manual alias curation.

Every row carries a **`source`** (`live` / `seed` / `manual`) and a
**`resolved_at`**. Conflict resolution is then a single rule:

```sql
ON CONFLICT (email) DO UPDATE
  SET login = excluded.login, source = excluded.source,
      resolved_at = excluded.resolved_at
  WHERE excluded.source = 'manual'
     OR (email_resolution.source <> 'manual'
         AND excluded.resolved_at > email_resolution.resolved_at)
```

This is strictly better than the pending-only rule an earlier draft
specified. A seed that is 5-8+ weeks stale by the time it runs can never beat
a live result; hand curation is never clobbered by either; the operation is
naturally idempotent; and because `source` is retained, every row's
provenance stays auditable — which the pending-only rule could not offer.

**Caller/trust boundary:** the ingest path is reachable only from inside the
cluster, called by the enrichment worker and by one-off migration scripts. It
is never exposed on a public or authenticated-user-facing surface, and the
downstream devimprint presentation layer has no reason to call it.

**Hand-curated aliases already survive outside the database.**
`declarative-config/k8s/ord-devimprint/commitgraph/admin-alias-configmap.yml`
holds the operator-authored `source_login → target_login` map in git;
`user_aliases` rows with `reason='name-match'` are auto-derived and
reproducible. Only `email_resolution` is genuinely irreplaceable, which is
why the teardown preserved its volume.

**Superseded 2026-08-05 by teardown — the preserved instance is reused, not
duplicated.** A 2026-08-04 clarification specified that Phase 1 would stand
up a *new, second* queue-api deployment, so the new pipeline could build and
test without disturbing the instance the live old pipeline depended on. That
rationale no longer exists: the old workers are gone, so the surviving
instance is idle and uncontended.

Reusing it is strictly better than a fresh deployment, because it already
holds state a new instance would have to be given anyway — `repo_queue` with
the 98,747 discovered repos, `repo_head_cursors`, and `catalog_version`. The
`QUEUE_API_URL`-swap pattern is correspondingly unnecessary: there is only
one instance, and discovery workers point at it when Phase 5 re-enables them.

"Kept as-is" therefore now means both the codebase *and* the running
instance — with the one change described above, that identity **results**
move out of it into Postgres while the resolution **work queue** stays.

**Postgres**: new CNPG cluster on a **dedicated large Rackspace Spot node in
ord-devimprint** (user's explicit choice — keeps clone-worker's rollup
upserts same-cluster, no cross-cluster network hop on the hot write path).
**Storage class, stated explicitly (2026-08-04, closing a gap flagged by
adversarial review): `storageClassName: sata`** — this org's hard rule for
Rackspace Spot Cinder storage is always `sata`/`sata-large`, never
`ssd`/`ssd-large`, with the class always set explicitly rather than left to
a cluster default; this section discussed the PVC at length without ever
stating one. `sata`'s 5-20GB range comfortably covers the corrected sizing
below (**corrected 2026-08-04, gap-review round 5**: ~1.4-1.6GB now,
~14-16GB projected at 10x scale — see the sizing section for why this grew
from the previous ~0.9-1.0GB / ~9-10GB figures); `sata-large` (75GB
minimum) would be the wrong tier to provision up front at this size. Mirrors
`queue-db`'s CNPG cluster (`storageClass: sata`, see "Critical files
referenced" below). If growth ever exceeds `sata`'s 20GB ceiling, the
backup/restore-into-larger-PVC path already described below is how to move
to `sata-large` — Cinder volumes can't grow in place or change class in
place either way.

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

**Sourced and reconciled (2026-08-04, gap-review round 3): the $0.006/hr
figures above come from a live query against Rackspace Spot's public
percentile-pricing endpoint**
(`https://ngpc-prod-public-data.s3.us-east-2.amazonaws.com/percentiles.json`,
`regions.us-central-ord-1.serverclasses`, checked 2026-08-04) — both
`ch.vs1.medium-ord` and `mh.vs1.large-ord` show `50_percentile: 0.006`
there, confirming the figure is real and current, not stale or invented.
**This is a different number from, and doesn't contradict, this account's
own $0.001/hr bid** for the live `ch.vs1.medium-ord` nodepool — recorded in
`rackspace-spot-terraform/clusters/ord-devimprint/main.tf` (`bid_price =
0.001`) and confirmed live in `notes/bf-1qt.md` (`bidPrice=$0.001`,
`fulfilled=6`). A p50 percentile describes what the broader market has
historically cleared at across all bidders; this account's own bid is one
specific choice on that market and can legitimately sit well below p50 when
optimizing for cost over preemption risk — which is exactly what's
happening here (the same percentiles.json response's `market_price` field
for `ch.vs1.medium-ord` is `0.001000`, matching this account's bid exactly).
Provisioning `mh.vs1.large-ord` at a similarly low bid rather than at its
$0.006/hr p50 is the reasonable default to carry forward, but hasn't been
explicitly decided — worth stating as an actual bid-price choice before
Phase 0 provisioning, not left implicit.

**Percentile pricing is a historical bid-clearing distribution, not an
availability guarantee or a reservation** — p20/p50/p80 describe what a Spot
market *has* cleared at, not what it's guaranteed to clear at when this
class is actually requested. Rackspace Spot nodepools are bid-based, and
`fulfilled` can in principle come in under `desired` on any bid-based
market. **Corrected (2026-08-04, from adversarial review): the cited note
doesn't actually show this happening.**
`rackspace-spot-terraform/notes/bf-1qt.md` was previously cited here as
evidence this "happens in practice on this account," but the note documents
exactly one nodepool snapshot —
`desired=6, fulfilled=6`, fully satisfied — in the context of an unrelated
issue (stale Terraform state referencing a deleted cloudspace); it's also
the only note in that repo, and no other file there mentions `fulfilled` at
all. The field exists specifically to track under-fulfillment risk, but
this account's own history doesn't yet show a concrete instance of it — the
risk is real in principle (inherent to how Spot bid markets work,
independent of any one account's history), just not yet evidenced on this
account. No fallback class is specified if
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

### The rollup holds AI-relevant commits only

**Decided 2026-08-05.** Earlier drafts specified two rollup tables — a
tool-agnostic `repo_user_daily` alongside the sparse tool-tagged one. The
tool-agnostic table is **dropped entirely**. Total commit counts stay with
the raw git data persisted to ARMOR; only AI-tool-tagged activity is rolled
up into Postgres.

**Verified against the live artifact before adopting this**, since it would
be an expensive thing to get wrong: the last published `leaderboard.json`
(archived at `~/backups/commitgraph-cutover/`) exposes per-developer fields
`rank, username, ai_commits_30d, ai_commits_total, ship_streak, tools[],
providers[], top_repo, last_active, verified` — **no non-AI commit count on
any row**. Nothing at the presentation layer consumes tool-agnostic
per-repo-per-day activity.

The one place the corpus-wide total does surface is the document's `totals`
object (`commits: 76,614,890`, `developers: 1,094,043`, `repositories:
98,747`). Those are three scalars, not a rollup — maintain them in a small
`corpus_stats` table updated per repo scan, rather than keeping an 11.6M-row
table alive to compute three numbers.

This is by far the largest sizing lever available: the dropped table was
~1.29-1.45GB of the previous ~1.4-1.6GB estimate.

**"AI-relevant" is the scope boundary for the whole database, not just the
rollup** (operator, 2026-08-05). Identity follows the rollup: Postgres
carries `email_resolution` rows for authors of AI-tagged commits, not for
all ~1.09M discovered developers. Emails that only ever appear on non-AI
commits are never resolved and never stored — they cannot affect any
ranking, so resolving them spends rate-limited API budget on data with no
consumer.

**The predecessor had already reached this conclusion, which is strong
corroboration.** `commitgraph-deprecated/containers/aggregator/aggregator.py:1414-1428`
feeds the resolution queue from:

```sql
WHERE r.email IS NULL AND c.n > 0      -- c.n = SUM(ai_commits) per email
```

with the comment *"only emails carrying AI commits are worth resolving —
priority is the AI-commit count, so zero-AI emails queued at priority 0
behind ~945k siblings were never going to be serviced anyway."* The old
pipeline discovered this the hard way, at ~945K distinct emails. This design
adopts it deliberately and pushes it one layer further: not just
*prioritized*, but never stored.

**This materially changes the identity-throughput story** and corrects a
claim made elsewhere in this plan. "Explicitly out of scope" states that
resolution throughput is capped by the shared ~30 req/min GitHub budget with
a 363K-email backlog — the cap is real and unchanged, but the *demand under
it* is far smaller than that figure suggests. 234,263 AI-tagged commits
resolve to a distinct-author set orders of magnitude below 1.09M developers.
Measure the exact count during the `email_resolution` extraction in Phase 1;
it is the number that determines both Postgres sizing and how long the
resolution backlog actually takes to clear.

**Consequence for the claude-leaderboard seed:** it needs no filtering. Every
pair in that frozen cache came from a `Co-Authored-By: Claude` commit, so all
349,425 are AI-relevant by construction.

### Postgres schema

Surrogate integer keys throughout, resolved identity in the same database,
and `insert_time` carried on the rollup:

```sql
CREATE TABLE repos (
  repo_id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider       TEXT NOT NULL,
  repo_full_name TEXT NOT NULL,       -- e.g. owner/name
  excluded_at    TIMESTAMPTZ,         -- non-NULL = excluded from ranking
  excluded_reason TEXT,
  UNIQUE (provider, repo_full_name)
);

CREATE TABLE users (
  user_id    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  login      TEXT NOT NULL UNIQUE,    -- canonical GitHub login
  profile_url TEXT,
  avatar_url  TEXT
);

-- Resolution RESULTS (the work queue stays in queue-api; see above)
CREATE TABLE email_resolution (
  email       TEXT PRIMARY KEY,
  login       TEXT NOT NULL,
  source      TEXT NOT NULL,          -- 'live' | 'seed' | 'manual'
  resolved_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX ON email_resolution (login);

CREATE TABLE user_aliases (
  source_login TEXT PRIMARY KEY,
  target_login TEXT NOT NULL,
  reason       TEXT NOT NULL,         -- 'admin' | 'name-match'
  created_at   TIMESTAMPTZ NOT NULL
);

-- The rollup: AI-tool-tagged commits only
CREATE TABLE repo_user_daily_tool (
  repo_id     BIGINT NOT NULL REFERENCES repos(repo_id),
  user_id     BIGINT NOT NULL REFERENCES users(user_id),
  tool        TEXT   NOT NULL,        -- plain TEXT, not enum — catalog grows
  day         DATE   NOT NULL,
  commits     INT    NOT NULL,
  insert_time TIMESTAMPTZ NOT NULL,   -- when this repo was last scanned
  PRIMARY KEY (repo_id, user_id, tool, day)
);
CREATE INDEX ON repo_user_daily_tool (user_id, tool, day);
CREATE INDEX ON repo_user_daily_tool (tool, day);
CREATE INDEX ON repo_user_daily_tool (user_id, insert_time);

CREATE TABLE corpus_stats (           -- the three `totals` scalars
  stat  TEXT PRIMARY KEY,             -- 'commits' | 'developers' | 'repositories'
  value BIGINT NOT NULL
);
```

**`repo_id` is a surrogate, allocated here** (adopted 2026-08-05, closing the
gap where earlier drafts used `repo_id BIGINT` without ever saying where it
came from — queue-api's schema has no integer repo id at all; `repo_queue` is
keyed on `(provider, repo_full_name)`). Three reasons a surrogate beats the
natural key:

1. **Repos get renamed on GitHub.** A surrogate survives a rename by updating
   one row in `repos`; a natural key fragments that repo's history across the
   old and new names permanently.
2. `repo_full_name` averages ~20-25 bytes and would otherwise sit in the
   primary key of the rollup *and* in every index on it.
3. The name-lookup table is needed anyway — for exclusion (see "Threat
   model"), admin tooling, and debugging.

Allocation is one upsert-returning-id **per repo**, not per commit — a single
round trip per job, negligible against the clone itself. The same reasoning
applies to `user_id` versus a variable-length `author_key`.

**On query speed with surrogate keys** (raised 2026-08-05): integer keys are
*faster* to look up than text, not slower — an 8-byte fixed-width integer
comparison beats variable-length string collation at every level of the
B-tree, and the index is smaller so more of it stays cached. Lookup by name
stays available through `repos.UNIQUE (provider, repo_full_name)` and
`users.login UNIQUE`: name → id is one index probe, and the rollup query then
runs entirely on integers.

**A UUID or hash key would be worse, and is rejected.** A random UUID is 16
bytes rather than 8, and — the real cost — random insertion points destroy
B-tree locality, causing page splits and poor cache behaviour on exactly the
large index this schema depends on. A deterministic hash of the repo name
would additionally reintroduce collision handling and lose the rename
survival that motivates the surrogate in the first place. UUIDv7 fixes the
locality problem but still costs 16 bytes and buys nothing here, since ids
are allocated by one database, not distributed generators.

**`users` is an identity/profile table, not a counter** (decided 2026-08-05).
Earlier drafts carried `total_commits` / `total_ai_commits` as delta-updated
columns, which created a genuine correctness hazard: rollup rows are
idempotent via DELETE+INSERT, but a delta-updated counter is not, so a rescan
double-counts unless the delta is computed against the pre-DELETE value —
a step no draft ever specified. Both counters are dropped. They are
derivable with a `SUM` over the rollup, and if that ever gets slow the answer
is a materialized view refreshed on the aggregator's 15-minute cycle, not a
hand-maintained counter. Removing them also removes the need for
`SELECT ... FOR UPDATE` row locking on the write path — the concurrency
hazard was entirely a property of the counter.

Per the operator: `users` exists primarily to map a resolved email to a
GitHub login that links to a real profile, so people reading the leaderboard
can find the developers behind it.

### `insert_time` — scan recency, not commit recency

**Adopted 2026-08-05.** `repo_user_daily_tool` carries `insert_time
TIMESTAMPTZ`, set to the transaction timestamp of the write that produced the
row. Because every rescan replaces a repo's whole slice (DELETE + bulk
INSERT), `insert_time` is uniformly "when this repo was last scanned" across
that repo's rows — it costs nothing to maintain and cannot drift.

`MAX(insert_time)` grouped by user answers **"how long ago were this
developer's repos last scanned"**, at full timestamp resolution rather than
the day granularity `day DATE` would allow.

This is deliberately *scan* recency, not *commit* recency — the two diverge
exactly when someone is actively committing but their repos haven't been
rescanned yet, which is the case users complain about. `last_active` (latest
commit day) remains available from `MAX(day)`. Surfacing both is what lets
the presentation layer distinguish "this developer stopped committing" from
"we haven't looked recently."

**Sizing, rebaselined 2026-08-05 after the AI-only rollup decision.** The
previous figure — ~1.4-1.6GB, itself corrected three times — was dominated by
the tool-agnostic `repo_user_daily` table that no longer exists. The history
of those corrections is preserved in git; what follows is computed fresh
against the new schema rather than adjusted from the old number.

Measured inputs, all from the frozen 2026-08-03 snapshot: 76,614,890 commits,
**234,263 AI-tagged**, 98,747 repositories, 1,094,043 developers. Rollup ratio
~6.6 commits/row, measured self-consistently against `leaderboard.db`
(52,456,758 commits / 7,965,582 rollup rows). Postgres rows carry a ~24-byte
tuple header plus alignment padding on top of column data.

- **`repo_user_daily_tool`** — 234,263 AI commits at ~6.6 commits/row ≈
  **~35.5K rows**. Per row: 24B header + 8 (`repo_id`) + 8 (`user_id`) + ~12
  (`tool`) + 4 (`day`) + 4 (`commits`) + 8 (`insert_time`) + padding ≈ 80
  bytes → **~2.8MB**. Three indexes at ~30-40 bytes/row ≈ ~4MB.
  **Under 10MB total.** The table this redesign is fundamentally *about* is
  now the smallest thing in the database.
- **`repos`** — 98,747 rows × ~90 bytes ≈ 9MB, plus the
  `(provider, repo_full_name)` unique index ≈ 5MB → **~14MB**.
- **`users`** — bounded by distinct AI-active logins, itself bounded by
  234,263 AI commits, so at absolute worst ~234K rows × ~80 bytes plus
  indexes → **≤ ~30MB**, and realistically far less.
- **`email_resolution`** — scoped to AI-relevant authors (decided
  2026-08-05, see above), so this is bounded by the distinct-author set
  behind 234,263 AI-tagged commits rather than by 1,094,043 discovered
  developers. At ~100 bytes/row plus the email primary-key and login
  indexes, that is **single-digit to low tens of MB**. The exact row count
  is unknown until extraction and is the one figure here worth measuring
  rather than estimating — it is also the input to the claude-leaderboard
  seed's coverage question (349,425 pairs, all AI-relevant by construction).
- `corpus_stats` — three rows. Negligible.

**Total at current scale: roughly 60-90MB** — against ~1.4-1.6GB before, and
against the ~230MB an unscoped identity ingest would have cost. At 10x corpus
growth, under 1GB.

**Two honest consequences of that collapse:**

1. **The PVC is now hugely oversized for the data, and that is fine.**
   `sata`'s 5-20GB range was chosen when the rollup was projected at
   14-16GB at 10x scale. It isn't any more. Since Cinder volumes on Spot
   cannot be expanded or reclassed in place, overshooting is the correct
   direction to be wrong in — but the plan should not pretend the sizing
   drove the choice.
2. **The `mh.vs1.large-ord` node class no longer rests on storage.** Its
   justification is now working memory for ranking queries, connection
   headroom for the clone-worker fleet, and the confirmed absence of room on
   the existing `compute1-4` fleet (largest schedulable pod: 1.20 CPU /
   1.64 GiB) — not the size of the rollup. The conclusion holds; the reason
   changed, and saying so is cheaper than someone re-deriving it later and
   finding the stated reason no longer computes.

**Excluded from every figure above: bloat and WAL.** The write pattern is
DELETE-then-bulk-INSERT of a repo's whole slice on every rescan, which is a
dead-tuple generator by construction. Live-tuple sizing understates real disk
use by whatever autovacuum fails to reclaim. See "Durability and load" for
the autovacuum settings this pattern requires; budget a 2x multiplier over
the figures above until measured.

**`top_repo` is excluded from the snapshot** (decided 2026-08-05).

The field's provenance was checked before dropping it, because earlier
drafts of this plan attributed the snapshot's row shape to the
claude-leaderboard comparison. **That attribution was wrong.**
claude-leaderboard's generated leaderboard has no `top_repo` at all — the
only occurrence anywhere in `vibecodeleaderboard-backend` is
`src/inspect_discovered_users.py:284`, a debug script doing a pandas
`value_counts().head(3)`. The field is commitgraph's own, computed in
`commitgraph-deprecated/containers/aggregator/aggregator.py:968-977`:

```sql
ROW_NUMBER() OVER (PARTITION BY j.login ORDER BY SUM(j.ai_commits) DESC, j.repo) = 1
```

That is argmax over **all-time** AI commits with the repo name as a
deterministic tiebreak — **not** a 30-day ordering. This was queried twice
in discussion on the reasonable assumption that it must be windowed, so the
verification chain is recorded here in full rather than asserted:

- `agg_top_repo` groups `j` by `(login, repo)` and orders by
  `SUM(j.ai_commits)` with no date predicate of its own.
- Its `JOIN agg_base b ON b.login = j.login` filters *which logins* are
  admitted; it does not restrict the date range of the rows summed.
- View `j` (`aggregator.py:932`) spans `2005-01-01 .. current_date + 1` —
  a validity clamp, not a window.
- `summaries`, the table behind `j`, is loaded whole from every partition
  in the store (`ingest_streaming`, `aggregator.py:1394`) with no date
  filter anywhere on the load path.
- The 30-day cutoff exists, but only in `agg_base`, applied to a *different*
  column: `COALESCE(SUM(ai_commits) FILTER (WHERE date > ?), 0) AS ai_30d`
  (`aggregator.py:952`).

So `ai_commits_30d` is windowed and `top_repo` is not — they were computed
over different spans in the same query, which is exactly the kind of
asymmetry that survives unnoticed in a published artifact.

**If `top_repo` is ever reinstated, choose the window deliberately.** The
all-time argmax means a developer's displayed "top repo" can be one they
haven't touched in years, which is probably not what a leaderboard reader
would infer from it. A 30-day argmax — matching the `ai_commits_30d` the row
is ranked by — is the more defensible default, but it is a product decision,
not a restoration.

(Incidentally, `j`'s date clamp is the defensive guard added after the
2170-commit incident, and its bounds match compactor's exactly — independent
corroboration of the quarantine requirement above.)

Excluding it is cheap to reverse: it is a pure function of the rollup plus
`repos`, so reinstating it later is one window query and no new data
capture. Dropping it also removes the last consumer that forced repo *names*
into the published snapshot.

**Leaderboard snapshot sizing (ARMOR, Parquet, the full ranked list):**
per-row shape is the live JSON schema minus `top_repo` (rank, username,
ai_commits_30d, ai_commits_total, ship_streak, tools[], providers[],
last_active, verified) plus `last_scanned_at` from `MAX(insert_time)` —
roughly 110-115 bytes/row uncompressed, compressing to an estimated 35-50
bytes/row in Parquet given heavy repetition in low-cardinality columns
(`providers` is almost always just `["github"]`) and small-range integers.
At "hundreds of thousands of rows" (per the earlier full-list-not-top-N
decision): **roughly 10-40MB** — never a real storage concern.

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

Write pattern per repo, one transaction: upsert `repos` and any new `users`
rows to obtain surrogate ids, `DELETE ... WHERE repo_id=$1` on
`repo_user_daily_tool`, then a single set-based bulk `INSERT` via
`UNNEST($1::bigint[], ...)` (not row-by-row — matters once multiple
clone-worker replicas write concurrently), with `insert_time` set to the
transaction timestamp. **No counter update and no row locking** — dropping
`users.total_commits` (see schema above) removed the only part of this
transaction that needed `SELECT ... FOR UPDATE`, and with it the only
non-idempotent step. The whole transaction is now replace-only, so running
it twice against an unchanged repo is a no-op by construction rather than by
careful bookkeeping.

Concurrent clone-worker replicas writing different repos never contend on
the same rows at all; two replicas touching the same *user* now only race on
an idempotent `users` upsert, not on a shared mutable counter. That is a
materially smaller concurrency surface than earlier drafts of this plan
assumed Postgres would have to survive.

## Write-path admission control

**Raised 2026-08-05:** should a rate-limiting API sit in front of Postgres,
so clone-workers receive `429` with an explicit retry timestamp and the API
absorbs the thundering herd?

The instinct is right — a worker fleet needs *backpressure*, and a plain
connection pool cannot express it. But the mechanism should be layered,
cheapest first, because two of the three layers already exist.

**Layer 1 — lease concurrency (free, already built).** queue-api's
claim/lease semantics already bound how many repos are being processed at
once. Outstanding leases *are* an admission-control knob: a fleet that can
only hold N leases can only have N transactions in flight. This is the
correct first line because it throttles work at the point it is *claimed*,
before a worker has spent a clone on it — rather than at the point it tries
to write, after all the expensive work is already done. Rejecting a write
after the clone wastes the clone.

**Layer 2 — PgBouncer in transaction-pooling mode.** The specific way a
single Postgres instance dies under a worker fleet is connection exhaustion:
N replicas × pool size each, all held open, degrades badly past a few
hundred. A pooler multiplexes them onto a small backend set. Transaction
pooling breaks session state, which is irrelevant here — every write is one
self-contained transaction. This is the standard answer to the actual
failure mode and should be provisioned in Phase 0, not deferred.

**Layer 3 — a purpose-built write API, only if 1 and 2 prove insufficient.**
What it uniquely provides that a pooler cannot: explicit `429` plus
`Retry-After`, so workers back off on a schedule instead of queueing
silently. PgBouncer under load *queues* — from the client's perspective a
slow connection is indistinguishable from a healthy one, so nothing backs
off and the herd persists. An API converts that into a signal.

What it costs, and why it isn't the default: an extra hop directly in the
hot write path, a new single point of failure in front of the only write
target, and a new codebase to maintain — largely duplicating a bound that
Layer 1 already enforces more cheaply and earlier.

**Decision: build layers 1 and 2 in Phase 0; treat layer 3 as gated on
Phase 2's load test.** If the load test shows the fleet can saturate
Postgres while staying within its lease budget, that is the evidence that
justifies building the API — and the load test will also have produced the
concrete numbers (`Retry-After` values, per-replica ceilings) the API would
need in order to be configured sensibly. Building it first means guessing
those numbers.

## Durability and load

Ordered by what actually kills a single Postgres instance under this
workload, not by conceptual tidiness.

1. **Connection management** — see Layer 2 above. The most common failure
   mode, and the cheapest to prevent.
2. **Autovacuum tuned for whole-slice replacement.** The write pattern
   generates dead tuples at exactly the rescan rate. Postgres's default
   `autovacuum_vacuum_scale_factor` of 0.2 means vacuum fires only after 20%
   of the table is dead, so bloat accumulates between runs. Set it per-table
   to ~0.02 with a raised `autovacuum_vacuum_cost_limit`, and alert on
   `pg_stat_user_tables.n_dead_tup`. This is why the sizing section budgets
   a 2x multiplier over live-tuple figures.
3. **No long-running transactions.** A single long transaction pins the xmin
   horizon and defeats vacuum *globally* — not just for its own table. One
   pathological repo holding a transaction open can therefore bloat every
   table in the database. Chunk or cap the largest repos, and set
   `statement_timeout` and `lock_timeout` on the worker role so a stuck job
   fails fast instead of wedging the instance.
4. **Replica topology is the real exposure.** `instances: 1` on a single
   preemptible Spot node, as the sole write target, with **no fallback
   system to fail back to** (see "Status" above). A preemption is a hard
   outage of the entire pipeline, not a graceful degrade. CNPG supports
   `instances: 3` with synchronous replication; given that there is no
   turning back, that cost is worth weighing seriously rather than
   inheriting `queue-db`'s single-instance precedent by default. Flagged in
   "Open decisions".
5. **Give the aggregator its own read target** once a replica exists, so the
   15-minute ranking query stops competing with the write path.
6. **State the clone-worker replica count.** This plan has never specified
   one for the new pipeline — which is precisely why Phase 2's load test has
   no pass/fail numbers. Pick it in Phase 0, load-test at that number plus
   headroom, and record the result as the ceiling.

**Backup/restore is now the only recovery path.** It was already a Phase 0
deliverable (`barmanObjectStore` + daily `ScheduledBackup` to ARMOR, plus a
rehearsed manual promote-from-backup with a stated RTO). With the old
pipeline gone, it is promoted from deliverable to **blocking gate**: no real
traffic until a restore has actually been rehearsed end-to-end and timed. It
remains the expansion path as well, since Cinder volumes cannot grow in
place.

## Retention tiering — gated on measurement

**Raised 2026-08-05:** a supplemental pass in clone-worker could collapse
commits older than 400 days into a coarser tier, bounding growth of the
daily table.

The mechanism is sound and the placement is right — clone-worker already
walks each repo's full history, so a second aggregation pass over the
older-than-400-days slice costs one extra grouping over data already in
memory, not a second read.

**But the AI-only rollup decision above may have obviated it entirely.** The
daily table is now projected at ~35.5K rows / under 10MB, not the ~11.6M
rows / ~1.3GB that motivated tiering. Collapsing 400-day-old rows out of a
10MB table saves nothing worth the code.

**So: do not build this yet.** Revisit when a measured `pg_total_relation_size`
on `repo_user_daily_tool` crosses a threshold worth acting on. If it is ever
needed, the design is a `(repo_id, user_id, tool, month)` tier written by the
same clone-worker transaction, with the daily rows for that window deleted in
the same pass — keeping the whole-slice-replace idempotency property intact.
The leaderboard needs a 30-day window and all-time totals; all-time can be
served from the monthly tier without loss.

Related and independent of table size: **declarative range partitioning by
`day`** would let the 30-day query touch one or two partitions and let old
partitions detach to ARMOR. Worth knowing up front that it does *not* help
the write path — `DELETE ... WHERE repo_id=$1` spans every partition for that
repo, so partition pruning cannot apply to it. It is a read-side and
retention-side tool only.
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
  which ARMOR instance is in scope (**corrected 2026-08-04, gap-review round
  4**: there are four org-wide, not "at least two" — `devimprint` namespace
  on ord-devimprint, plus separate `armor`-namespace deployments on iad-ci,
  iad-kalshi, and rs-manager; verified against every `armor-deployment.y*ml`
  under `declarative-config/k8s/`).
  Object sizes for the Parquet artifact are small relative to ARMOR's
  historical multipart corruption bug's trigger conditions (typically KBs
  to low tens of MB even for large repos) — low risk. **The warm-start
  artifact's size at scale is a real, separate open question** — unlike
  the compact Parquet extraction, it contains actual git pack data, and
  the one real repo tested was a few hundred commits; a smoke test against
  the corpus's genuinely large repos is needed before assuming this stays
  cheap, not just for the Parquet artifact.

### ARMOR storage sizing — an oversight, not an inability

**Named plainly 2026-08-05.** This plan corrected its Postgres sizing three
separate times and never once estimated ARMOR's footprint — not because the
number is unknowable, but because attention went to one side of the storage
question and never came back. The asymmetry is the finding.

The figures are measurable, and cheaply:

1. **Warm-start artifacts.** The research doc already ran the exact
   pack-file-plus-loose-ref-plus-config procedure end-to-end for n=1 (NEEDLE,
   796KB, a few hundred commits). Extend it to a **stratified sample by commit
   count** — p50, p90, p99, and the genuinely largest repos in the corpus —
   and fit artifact size against commit count.
2. **Total footprint** then follows from the real distribution: per-repo
   commit counts are already known from the corpus, so it is a sum over
   measured data rather than an extrapolation from an assumed mean.
3. **Parquet artifacts** are easier still — that corpus already exists and is
   already written. Measure the current partitions and re-express per-repo.
4. **Write volume** = footprint × rescan cadence, and cadence is a parameter
   this plan chooses rather than discovers.

**Sample the tail, not the mean.** A random sample would actively mislead
here. A handful of very large repos can dominate total bytes, per-object
transfer time, and worst-case memory during materialization, and those are
precisely the repos the warm-start research has not touched. The sample must
include the corpus's actual largest repos by commit count.

This measurement is a **Phase 0 deliverable**, gating the ARMOR parts of the
design the same way the load test gates the Postgres parts.

### Supporting the extremes is a design requirement, not just a measurement

**Stated as a requirement 2026-08-05** (operator: *"not much to do but ensure
the system can support the extremes"*). Measuring the tail is only useful if
something is committed to happen when the tail turns out to be ugly. The
requirement: **no single repository, at any size in the corpus, may fail a
job, exhaust a pod's memory, or block the fleet.** Concretely, that means the
following must hold or be built:

1. **Streaming, never materialize-whole.** Extraction already walks history
   commit-by-commit; the Parquet write must be batched rather than
   accumulating all rows in memory first. The predecessor OOM'd a 2Gi pod
   materializing 400K commits' message bodies at once — that incident is the
   proof this requirement is not theoretical.
2. **A warm-start size ceiling with a defined fallback.** If a repo's
   warm-start artifact exceeds a threshold set from the measurement, skip
   storing it and let that repo always take the full-clone path. Degrading to
   the slow-but-correct path is strictly better than failing, and warm-start
   is an optimization — its absence is already a supported state.
3. **Bounded transaction size.** A very large repo must not hold one Postgres
   transaction open long enough to pin the xmin horizon (see "Durability and
   load"); chunk the rollup write if the row count warrants it.
4. **The largest repo is a fixture.** Whatever the measurement finds as the
   corpus maximum becomes a named test case, exercised end-to-end before
   Phase 5, not discovered in production.

If the measurement shows the extremes are comfortably handled, items 2 and 3
cost nothing to skip — but that is a conclusion the measurement licenses,
not an assumption to start from.

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
6. Is idempotent and must be tested as such: run twice, assert the rollup is
   identical after the second run. **Restated 2026-08-05:** this step
   previously asserted on `users.total_commits`, because the delta-updated
   counter was the one part of the write path that wasn't naturally
   idempotent. That counter is gone (see schema), so the whole transaction is
   replace-only and the property holds by construction — but the test stays,
   now asserting on the rollup itself, because "holds by construction" is a
   claim about today's code and this is the check that keeps it true.

**Consequence not yet stated (2026-08-04, gap-review round 3): every
migrated repo hits clone-worker's full-clone fallback on its first
post-cutover scan.** Step 3 above writes the Parquet commit-history
artifact for redetection, but explicitly does not run `git clone` — so
migration never produces the separate warm-start artifact (raw pack files +
loose ref + promisor config, see Architecture above) that clone-worker's
warm-start step depends on. Warm-start's own fallback rule (Architecture,
step 1: absent artifact → full `git clone --bare --filter=blob:none`)
therefore fires for all 98,747+ migrated repos the first time each is
rescanned after cutover — not the fast, sub-second warm-start delta this
plan otherwise emphasizes. Against the independently-measured ~1,000
repos/hour/replica ceiling (see "Explicitly out of scope" below): using the
only clone-worker replica count this plan states anywhere (4, per Context
above, for the old architecture — the new pipeline's replica count isn't
separately stated), even running all 4 flat-out on nothing else, one
full-clone pass over the migrated corpus is ~25 hours (98,747 ÷
4,000/hour) — longer in practice, since those same replicas are also
handling live discovery/rescan traffic concurrently, not dedicated solely
to this one-time wave. A real, one-time throughput cost of cutover, not
solved here — just made explicit so Phase 5/6 timeline expectations
account for it.

## Phased rollout

**Rewritten 2026-08-05 after teardown.** The original seven phases were
built around a live predecessor: shadow it, dual-write against it, diff its
output, then switch over. There is no predecessor to shadow any more. The
sequence below replaces "migrate off a running system" with "rebuild and
restart a stopped one," which is a materially different shape — fewer
coordination hazards, but no safety net and a running clock.

**The clock: the pipeline is dark as of 2026-08-05.** `commitgraph.jedarden.com`
serves a frozen `leaderboard.json` generated 2026-08-03T22:05:42Z, and nothing
is discovering, cloning, or aggregating. Every day of this build is a day the
public data ages. That is an accepted cost of the operator's decision to tear
down rather than run both, but it should drive sequencing: prefer the shortest
path to *something publishing again* over completeness in early phases.

1. **Phase 0 — capacity, provisioning, and the two gates.** Pick the fallback
   node class before provisioning (still open — do not discover this live on
   the bid market) and the `mh.vs1.large-ord` bid price. Decide the replica
   topology (`instances: 1` vs `3`, see "Durability and load"). Provision the
   dedicated Postgres node manually (Spot UI or `rackspace-spot-terraform` —
   not a declarative-config PR). Install the CNPG operator on ord-devimprint
   (a 5th install org-wide, not a reuse). Stand up the cluster with the schema
   above, PgBouncer in front, and the autovacuum settings from "Durability and
   load". **Two blocking gates before Phase 1:** (a) a rehearsed, timed
   restore from `barmanObjectStore` backup — with no fallback system this is
   the only recovery path that exists; (b) the stratified ARMOR sizing
   measurement, including the corpus's largest repos.
2. **Phase 1 — isolated build, reusing the preserved queue-api.** New
   clone-worker (Postgres rollup write + ARMOR artifact writes, detection
   inlined) and new aggregator (Postgres read + ARMOR snapshot publish).
   **Earlier drafts specified standing up a second queue-api instance so the
   new pipeline wouldn't disturb the live one. That rationale is gone** — the
   old workers are torn down, so the preserved instance is idle and
   uncontended. Reuse it directly. This is strictly better than a fresh
   instance: it already holds `repo_queue` with the 98,747 discovered repos,
   `repo_head_cursors`, and `catalog_version` — all of which a fresh instance
   would have to have migrated into it anyway.
   **First real task of this phase: extract `email_resolution` and ingest it
   into Postgres** through the ingest path above (`source='live'`). This is
   blocked on refreshing `ord-devimprint-admin.kubeconfig` and is the single
   highest-value inheritance in the whole migration — 365K+ pairs of already
   spent, rate-limited API budget.
3. **Phase 2 — subset validation and the load test that has numbers.** Drive
   a bounded repo set end-to-end. Cross-check rollup counts against the
   existing corpus for the same repos. **Load-test concurrent clone-worker
   replicas writing Postgres**, at the replica count chosen in Phase 0 plus
   headroom, recording: sustained transactions/sec, p99 write latency,
   connection count at saturation, and `n_dead_tup` growth against autovacuum
   throughput. Those numbers are the pass/fail criteria — and they are also
   the inputs a rate-limiting write API would need if Layer 3 turns out to be
   justified (see "Write-path admission control"). Test the
   catalog-version-bump → redetect-job path with a synthetic tool signature.
4. **Phase 3 — bulk corpus migration against a now-static target.** Run the
   migration described above over the full existing corpus. **This got easier
   with teardown:** the corpus in B2 is frozen, so migration no longer chases
   a moving target and the "final delta pass" the old Phase 5 needed is
   unnecessary. The idempotency test still gates progress.
5. **Phase 4 — validation burn-in with no predecessor to shadow.** Run the
   new pipeline over the migrated corpus and hold it against the two
   replacement gates in "Verification" below — the frozen golden snapshot and
   independent recompute. Long enough to see at least one catalog-version
   bump and several aggregator publish cycles.
6. **Phase 5 — restart discovery.** Point search-worker and user-worker at
   the pipeline and re-enable them. This, not a switchover, is the actual
   cutover: the system goes from dark to live. Expect the one-time full-clone
   wave — migrated repos have Parquet artifacts but no warm-start artifacts,
   so each hits clone-worker's full-clone fallback on its first scan
   (~98,747 repos against a measured ~1,000 repos/hour/replica; see the
   consequence note in "Corpus migration"). Budget for it rather than being
   surprised by it.
   **Public serving:** the frozen `leaderboard.json` keeps serving until the
   downstream devimprint presentation layer ships. How long that is acceptable
   is an open decision — the file only gets staler, and at some point serving
   two-month-old rankings is worse than serving nothing.
7. **Phase 6 — finish the decommission.** Extract the remaining queue-api
   tables if not already done, then `.disabled` the queue-api manifests and
   its PVC, per the checklist in
   `declarative-config/k8s/ord-devimprint/commitgraph/TEARDOWN.md`. **Do not
   remove `queue-api-pvc.yml` before extraction is verified** — `sata` has
   `reclaimPolicy: Delete`, so pruning that PVC destroys the Cinder volume
   and every row on it. Also still open from the 2026-08-04 rename: a working
   CI trigger for `commitgraph-deprecated`, and the general
   `commitgraph-build-workflowtemplate.yml` firing against this docs-only
   repo — see `docs/notes/repo-rename-2026-08-04.md`.

## Relationship to claude-leaderboard

**Decided 2026-08-05: claude-leaderboard is the canonical source for
Claude Code leaders.** commitgraph is not trying to be authoritative for
Claude Code specifically, and should not be positioned as competing with or
superseding claude-leaderboard on that tool.

This resolves a tension that ran through earlier drafts, where
claude-leaderboard appeared only as an architectural predecessor to learn
from and a resolution cache to harvest — as though it were being absorbed.
It isn't. The two systems have different remits:

- **claude-leaderboard** — canonical for Claude Code. Single tool, deep
  history, proven at 37.9M commits on one box.
- **commitgraph** — the multi-tool system. Its distinguishing capability is
  the 21-tool detection catalog (`ALL_TOOLS`, verified by importing the
  module and counting) and retroactive re-detection when a new tool
  signature is added — neither of which claude-leaderboard has or needs.

Consequences worth carrying forward:

1. **The seed is an inheritance, not an acquisition.** Harvesting
   claude-leaderboard's frozen `author_login_cache` (349,425 pairs) into
   `email_resolution` remains correct and valuable — resolved emails are
   tool-agnostic facts about people, not Claude-Code-specific data. But it
   is borrowing a cache, not taking over a domain.
2. **Where the two disagree about a Claude Code number, claude-leaderboard
   wins.** Useful during validation: a divergence on Claude Code counts is
   a signal to investigate commitgraph, not evidence that
   claude-leaderboard drifted.
3. **The presentation layer should not imply otherwise.** Out of scope here,
   but the downstream devimprint layer inherits this positioning.

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
  `_load_username_aliases()` used — this redesign keeps both concepts, with
  the **results** moved into Postgres so the alias merge can be a real join
  evaluated before ranking (see "Identity lives in Postgres"; earlier drafts
  said "kept in queue-api verbatim, joined at read time," which was not
  implementable across that boundary).
  What genuinely doesn't change: resolution *throughput* is capped by the
  **same shared ~30 req/min GitHub API budget as discovery search** — this
  redesign adds no resolution capacity.
  **Corrected 2026-08-05: the demand under that cap is much smaller than the
  "363k pending emails" figure this section previously cited.** That number
  counts pending rows across all discovered emails; the predecessor's own
  aggregator only ever *fed* the queue with emails carrying AI commits
  (`aggregator.py:1414-1428`, `WHERE ... c.n > 0`), and this design goes
  further by not storing the rest at all (see "The rollup holds AI-relevant
  commits only"). The ceiling is unchanged and still real; the queue behind
  it is bounded by the distinct-author set behind 234,263 AI-tagged commits,
  not by 1.09M developers. Measure the true figure during Phase 1 extraction
  rather than carrying 363k forward as if it were the work remaining.
  **Corrected 2026-08-04 (gap-review round 5): the `fed 0 new emails` bug
  cited in earlier drafts here was independently point-fixed upstream
  before this plan even started.** `commitgraph-deprecated` commit
  `e37bb5b` (2026-07-31, four days before this plan's 2026-08-04 origin)
  diagnosed the exact symptom — `/email-resolution/export` pulling the full
  365K+ row table, unfiltered, on every aggregator cycle — and shipped a
  fix: a separate reader DB pool plus a `since=` cursor. queue-api is live
  at 2.8.0 with this change. This redesign doesn't get credit for fixing
  that specific instance; it was already fixed independently. What this
  redesign still contributes is broader, systemic write-contention relief
  (queue-api's write volume drops back to its original job-coordination
  role once compactor/filter-worker/aggregator's dirty-partition-bump
  traffic goes away, per the Architecture section above), not the
  resolution of one already-resolved bug.
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
  **Caller/trust boundary, stated explicitly:** this endpoint is invoked by
  a one-off internal migration script with direct network access to
  queue-api, not a standing service — it is never exposed on any
  authenticated-user-facing or public surface, and the downstream
  devimprint presentation layer has no reason to ever call it.
  This can run against the **currently live** deprecated pipeline
  immediately, doesn't need to wait for any phase of this redesign, and the
  new pipeline should do the same seed during its own bootstrap. Not built
  yet — deferred until
  Phase 1 implementation starts.
- **The detection footprint gap is real but deliberately not fixed here.**
  `shared/detection.py`'s catalog covers 21 tools (`ALL_TOOLS`, counted
  directly from the module — its own docstring's "15+" undercounts it);
  `search-worker`'s `GITHUB_FOOTPRINTS` list has 12 query entries but only
  10 distinct tool names (claude-code and cursor each get two entries — one
  per signal channel). **Counts corrected 2026-08-04**: an earlier pass
  through this document said "15 tools" vs. "cover 12" and then named only
  7 tools as the gap, which never added up (15−12≠7) and undercounted the
  real gap besides. Recounted directly against both files, the real gap is
  11 tools with detection patterns but no dedicated discovery query:
  blackbox, cody, codeium, codeium-bot, codestral, netlify-coding, replit,
  replit-bot, sweep, tabnine, and windsurf. Every commit in any cloned repo
  is still checked against the full 21-tool catalog regardless (detection
  runs one call per commit, all patterns at once — see
  `docs/notes/detection-inlined-not-lost.md`), so these tools are detected
  whenever a repo surfaces via some *other* footprint; they just can't be
  the sole reason a repo gets discovered. Closing this gap is a pure
  discovery-breadth-vs.-already-maxed-API-budget tradeoff — adding entries
  to `GITHUB_FOOTPRINTS` trades search calls away from the existing 10
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

**Rewritten 2026-08-05.** The original primary gate — continuously diff the
new pipeline's ranking against the old pipeline's live `leaderboard.json` —
is void, because there is no old pipeline to diff against. Its aggregator had
already been failing readiness for ~5 days before teardown, so its output was
stale even while it existed. The replacement is two gates, neither of which
needs a running predecessor.

**Gate 1 — the frozen golden snapshot.** The last published
`leaderboard.json` (generated 2026-08-03T22:05:42Z, 100 rows, sha256
`cf2ef378…77cc8`) is archived at `~/backups/commitgraph-cutover/` on ex44. A
validation script runs the aggregator's own ranking SQL against Postgres
restricted to commits on or before that generation timestamp, and compares
row-for-row on rank / username / `ai_commits_30d` / `ai_commits_total`. This
is an as-of comparison, not a live one: it answers "does the new pipeline
reproduce the old pipeline's last known-good answer from the same inputs?"

Known caveat, worth stating rather than discovering during the comparison:
**the golden snapshot's own rank 1 is `noreply@anthropic.com`** — a raw
unresolved email occupying the top of the public board. That is the identity
fragmentation problem in its purest form, and it means the golden file is a
record of what the old pipeline *did*, not of what is *correct*. Expect and
require divergence where the new pipeline resolves an identity the old one
did not; the gate is that every divergence is explainable by a resolution or
alias, not that the two agree everywhere.

**Gate 2 — independent recompute.** For a sample of repos, recompute the
rollup directly from the corpus Parquet with a separate implementation path
and assert it matches what Postgres holds. This checks the pipeline against
ground truth rather than against another pipeline, needs no predecessor, and
keeps working indefinitely as a production audit — unlike Gate 1, which
decays as the golden snapshot ages.

**Idempotency, as a permanent CI gate.** Run the same repo through
clone-worker's rollup write twice and assert the rollup is byte-identical the
second time. **Restated 2026-08-05:** earlier drafts asserted on
`users.total_commits`, which no longer exists — and which, as written, the
test would have *failed* on the first real rescan, since a delta-updated
counter double-counts unless the delta is taken against the pre-DELETE value.
Dropping the counter (see schema) made the whole write transaction
replace-only, so the property now holds by construction and the test verifies
that it stays that way as the code evolves.

**The Phase 2 load test is the premise check.** Postgres was chosen over
SQLite specifically to survive concurrent clone-worker writes. If it doesn't
hold up at the chosen replica count, the core premise needs revisiting before
anything else proceeds — not after.

**Not a gate: "pods are Running."** This project has already lost real time
to that false positive; it is named here so it doesn't get treated as
evidence again. The old aggregator sat `Running` for five days while
`Available: False` and publishing nothing.

## Invariants

Named, testable properties, each one a SQL assertion runnable both in CI
against a fixture database and periodically against production as an audit.
These exist because a data pipeline fails silently — wrong numbers look
exactly like right numbers until someone checks.

1. **Rollup matches artifact.** For any repo, the sum of `commits` in
   `repo_user_daily_tool` equals the count of AI-tagged, non-quarantined rows
   in that repo's Parquet artifact.
2. **No out-of-range days.** No row in `repo_user_daily_tool` has a `day`
   outside `[2005-01-01, current_date + 1]`. This is the 2170-incident guard;
   it has fired for real once already.
3. **Rescan idempotency.** Running the same repo through the write path twice
   leaves the rollup identical.
4. **Referential integrity of identity.** Every `user_id` in the rollup
   resolves to a `users` row; every `user_aliases.target_login` exists in
   `users`; the alias graph is acyclic and one level deep (no alias whose
   target is itself an alias source).
5. **Uniform scan time.** All rows for a given `repo_id` share one
   `insert_time` — a violation means a partial write escaped the
   whole-slice-replace transaction.
6. **Exclusion is honoured.** No repo with `repos.excluded_at IS NOT NULL`
   contributes to any published ranking.

## Edge cases

Enumerated rather than discovered. Each needs a stated behaviour and a test.

1. **Empty repo** — no commits. Must produce zero rollup rows, not a failed
   job.
2. **Force-pushed / rewritten history** — SHAs the previous scan saw no longer
   exist. Handled by construction: whole-slice DELETE+INSERT re-derives from
   current state rather than accumulating. **This is the main reason the
   replace pattern was chosen**, and it belongs in the plan as a stated
   property rather than an incidental benefit.
3. **Repo deleted or made private** between discovery and clone. Must be
   distinguishable from a transient clone failure so it isn't retried
   forever.
4. **Repo renamed on GitHub** — the surrogate `repo_id` survives it; the
   `repos.repo_full_name` row is updated. Without the surrogate this
   fragments the repo's history permanently (see schema).
5. **Commit with no author email**, or a malformed one. Cannot be resolved to
   a login; must not crash extraction.
6. **Login renamed after resolution** — `email_resolution` now points at a
   dead login. Needs revalidation, which the predecessor tracked in a
   `username_revalidation` table; carry the concept forward.
7. **Same commit SHA in multiple repos** (forks). Rollup is keyed per repo,
   so this is counted once per repo by design — which is correct for
   "activity in a repo" and wrong for "distinct commits authored." State
   which one the leaderboard means.
8. **Malformed or out-of-range `committed_at`** — excluded from the rollup,
   preserved verbatim in the Parquet artifact (see the quarantine section).
9. **A repo whose warm-start artifact is impractically large** — the extreme
    the ARMOR stratified sample exists to find.
10. **A commit message large enough to blow a Parquet row group.**

## Failure modes

One entry per dependency, with the recovery this design actually implements.

| Dependency | Failure | Recovery |
|---|---|---|
| GitHub API | rate limit / 5xx | Job fails, lease expires, re-claimed. Throughput is a stated hard ceiling, not a bug. |
| ARMOR | unavailable on write | Whole job fails and is re-claimed — no partial state, since all writes are in one job. Ranking is unaffected: ARMOR is not in the read path. |
| Postgres | unavailable | **Total pipeline outage.** No fallback system exists. Recovery is restore-from-backup; see "Durability and load". |
| Postgres | connection exhaustion | PgBouncer (Layer 2); lease concurrency bounds it upstream (Layer 1). |
| Corpus decrypt | retired key epoch | Migration enumerates all `key_id` values up front and confirms decryptability before starting (migration step 2). |
| queue-api | unavailable | Workers cannot claim; pipeline idles without data loss. |

## Threat model

**The core exposure: commit metadata is entirely attacker-controlled.**
`git config user.email` accepts any string, and `Co-Authored-By:` trailers
are free text in a commit message. Anyone can author commits attributed to
someone else's email, or add an AI-tool trailer to commits no tool touched,
and push them to a repository they own. Detection is pure pattern matching
against that text — there is no verification anywhere in the pipeline.

Two distinct harms follow, and they are not symmetric:

1. **Rank inflation** — a person manufactures AI-tagged commits to climb the
   board. Annoying, self-limited, and the traditional cost of any public
   leaderboard.
2. **Attribution to a non-consenting third party** — commits are authored
   under someone else's email and appear on the board under their resolved
   identity, linked to their real GitHub profile. This is the serious one:
   it publishes claims about a real person who never opted in, and the more
   successfully the identity-resolution layer works, the more convincingly
   the false attribution lands.

**This is not a regression** — the predecessor had the identical exposure —
but a redesign is the right moment to name it rather than inherit it
silently.

**Mitigation: repo-level exclusion** (operator's decision, 2026-08-05). The
practical lever is excluding the offending repository, not adjudicating
individual commits. The schema supports this directly: `repos.excluded_at` /
`repos.excluded_reason`, enforced by invariant 6 above, applied at ranking
time so exclusion takes effect on the next publish without re-scanning
anything. The predecessor's queue-api carries a `blocklist` table serving a
related purpose — carry the concept forward rather than reinventing it.

Exclusion is deliberately reversible: clearing `excluded_at` restores the
repo's contribution on the next aggregation cycle, since the rollup rows were
never deleted, only filtered.

**Residual risk, stated honestly:** exclusion is reactive. It requires
someone to notice, which for third-party attribution most likely means the
affected person noticing themselves. Options that would make it proactive —
requiring the author email to be a *verified* GitHub account email, capping
any single repo's contribution to a user's total, or requiring a minimum
repo signal before it counts — are not adopted here, but are the shape of
what a proactive control would look like if the reactive one proves
insufficient.

## Critical files referenced

**Note (2026-08-04): this repo (`commitgraph`) is design/planning-only —
no application code exists here yet.** The four paths below now under
`commitgraph-deprecated` point at the predecessor pipeline's actual code
(`jedarden/commitgraph` renamed `jedarden/commitgraph-deprecated` on
2026-08-04, when this repo took over the canonical `commitgraph` name —
see `docs/notes/repo-rename-2026-08-04.md`); "reused as-is" means reused
from there, not from this repo, until Phase 1 copies/ports it in.

- `/home/coding/commitgraph-deprecated/shared/detection.py` — reused as-is
  by the new clone-worker
- `/home/coding/commitgraph-deprecated/containers/clone-worker/worker.py` —
  current extraction logic and Parquet schema to build on
- `/home/coding/commitgraph-deprecated/containers/queue-api/schema.sql` —
  current `catalog_version` / `dirty_partitions` mechanism
- `/home/coding/commitgraph-deprecated/docs/adr/009-encrypted-public-b2-storage.md`
  — ADR-009, being partially reversed for the reasons stated above
- `/home/coding/commitgraph-deprecated/query_leaderboard.py` — existing
  `--parquet` read path the new aggregator's public export matches (see
  "Postgres computes the ranking, not DuckDB" above)
- `/home/coding/vibecodeleaderboard-backend/src/extractor_v4.py` — the
  reference idempotent DELETE+INSERT rollup pattern
- `/home/coding/declarative-config/k8s/iad-ci/queue-db/cnpg-cluster.yaml` —
  closest existing CNPG manifest precedent for schema/resources shape and
  the `instances: 1`/no-HA replica topology (see the Postgres section
  above). **Not** precedent for *which Kubernetes cluster* to place this
  new instance on — `queue-db` runs on `iad-ci`, while this instance is
  deliberately placed on a dedicated `ord-devimprint` node instead;
  "cluster placement" here means which physical cluster/node, a separate
  question from the replica/HA topology this file is precedent for
- `/home/coding/commitgraph-deprecated/containers/aggregator/aggregator.py` —
  lines 932-977 hold the `top_repo` argmax and the `j` view's date clamp; the
  reference for what the old ranking actually computed (see the `top_repo`
  exclusion note above)
- `/home/coding/declarative-config/k8s/ord-devimprint/commitgraph/` — the old
  namespace. Workloads were `.disabled` and pruned 2026-08-05; `queue-api` +
  Service + PVC are deliberately still live pending data extraction
- `/home/coding/declarative-config/k8s/ord-devimprint/commitgraph/TEARDOWN.md`
  — teardown state and the completion checklist that gates Phase 6
- `/home/coding/declarative-config/k8s/ord-devimprint/commitgraph/admin-alias-configmap.yml`
  — the hand-curated `source_login → target_login` map, in git rather than
  only in the database; the seed for `user_aliases` with `reason='admin'`
- `~/backups/commitgraph-cutover/leaderboard-golden-2026-08-03T22-05-42Z.json`
  — the frozen golden snapshot, Verification gate 1
