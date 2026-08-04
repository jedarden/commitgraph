# Where each artifact lives, and why it differs by artifact

Three different storage decisions were made in this design, each for a
different reason tied to that artifact's actual access pattern — not one
blanket "ARMOR everywhere" or "Postgres everywhere" rule.

## The rollup: Postgres, not SQLite

The predecessor's rollup coordination lived in queue-api's SQLite, which
enforces `SetMaxOpenConns(1)` — a single serialized write connection. That
was fine for queue-api's original, much lighter job-coordination role, but
became the direct, live-verified cause of contention once compactor,
filter-worker, and the aggregator all piled dirty-partition-bump traffic
onto the same connection on top of it.

Requirements that ruled out just copying claude-leaderboard's single-SQLite
design verbatim: multiple `clone-worker` replicas need to write
concurrently without serializing behind one lock, and the aggregator needs
real relational queries (rank, 30-day window functions, joins against an
alias table) — not something a plain key-value store gives you natively.
RocksDB and similar embedded KV stores were considered and rejected:
they're single-process-locked the same way SQLite is (so concurrent
replica writes hit the identical problem, just with extra steps), and they
have no relational query layer — rank/window/join logic would have to be
hand-built in application code, which is exactly the kind of custom
aggregation work that caused the aggregator's OOM history in the first
place. Postgres solves both natively, at a data size (low hundreds of
thousands to low tens of millions of rows) it handles without strain, and
it's already a mature org pattern (CNPG on 3 other clusters) rather than a
new technology with zero precedent here.

**Placement**: a dedicated large Rackspace Spot node in `ord-devimprint`
(not a shared cluster elsewhere) — deliberate choice to keep clone-worker's
concurrent rollup upserts same-cluster as the writers, avoiding a
cross-cluster network hop on the hot write path. Costs this incurs: node
provisioning is a manual, out-of-band step (Rackspace Spot node pools are
not managed via declarative-config — in-cluster Terraform automation for
this was retired org-wide after a reliability incident), and CNPG needs
installing fresh on this cluster (it doesn't run here today).

## The raw per-repo commit-history artifact: ARMOR

Every clone-worker pass writes a per-repo Parquet artifact (sha, author,
email, date, message per commit) to ARMOR, overwritten wholesale on every
rescan — this is the thing a redetect job reads back when a new tool
signature needs retroactively applying.

The predecessor's own ADR (ADR-009) deliberately avoided routing the corpus
through ARMOR, specifically because whole-file encryption defeats DuckDB's
range-read pruning on Parquet — measured at the time as 0.42MB fetched of a
48MB file for a point query, encrypted vs. plaintext, identical.
**Correction (2026-08-04, gap-review round 3): that characterization was
never independently re-verified against ARMOR's actual current behavior,
and shouldn't be repeated as settled fact.** ARMOR's own README claims the
opposite is true today — seekable AES-256-CTR with 64KB blocks, explicit
DuckDB range-read/column-pruning compatibility. Either ADR-009
mischaracterized ARMOR at the time it was written, or ARMOR gained
seekability afterward; verify empirically before relying on either claim
uncritically (full account, including ARMOR's recent multipart-encryption
corruption history, in `docs/plan/plan.md`'s Storage placement section).
Regardless, that ADR-009 reasoning doesn't transfer to this design anyway,
because it was measured against the OLD architecture's access pattern: the
corpus was hot, read on every aggregator/filter-worker cycle. In this redesign, the raw commit-history
artifact is cold — written once per clone/rescan, read back only for rare
catalog-triggered redetect jobs. Whole-file decrypt-on-read costs nothing
meaningful at that access frequency, so the specific reason ADR-009 avoided
ARMOR doesn't apply to what this artifact has become. ARMOR also has a
concrete upside the old direct-B2 path didn't: its `ARMOR_CF_DOMAIN`
routes downloads through Cloudflare's free-egress edge, which the
predecessor's direct-B2 writes bypass entirely (paying avoidable egress
today).

**Pre-checks before depending on this, not yet done**: `ARMOR_PREFIX` is
currently unset on the live `devimprint`-namespace ARMOR instance
(dedicated-bucket mode) — needs explicit scoping decided before writing.
Confirm which of the (at least two) org-wide ARMOR instances is actually in
scope. Object sizes here (per-repo Parquet, typically KBs to low tens of
MB) are small relative to ARMOR's historical multipart-corruption bug's
trigger conditions — low risk, but worth a smoke test against a handful of
genuinely large repos before trusting it at full scale.

## The full leaderboard snapshot: also ARMOR, and explicitly internal-only

The aggregator publishes the **complete** ranked list — every user with
rollup activity, expected to be hundreds of thousands of rows, not a
top-N cut — as a single Parquet snapshot, written to ARMOR.

This went through two rounds of reconsideration worth recording. First
instinct was to make this snapshot the direct public-facing artifact,
queried client-side via DuckDB-WASM (mirroring the proven
`hetzner-auction-dashboard` pattern) — which would have reintroduced
exactly ADR-009's range-read problem for this artifact specifically, since
DuckDB-WASM's efficiency depends on plain, range-readable bytes, not
whole-file decryption through a proxy on every browser request.

That question turned out to be moot once the actual goal was named
explicitly: this data feeds a downstream content-generation use case (X
posts surfacing top users and their tool/harness patterns), not a public
"browse the full corpus" product. A bulk-downloadable file — regardless of
which storage backend serves it — is also the worst possible shape for
minimizing scraping; publishing the complete dataset as one fetchable
object hands a scraper everything in a single request. So the resolution
isn't a storage-backend choice at all: **this snapshot is internal-only**,
consumed by a separate, not-yet-built downstream pipeline that generates
the actual devimprint.com public presentation (curated top-N, rate-limited,
profile-lookup — whatever shape that effort lands on, which is explicitly
out of scope for this repo). Because the consumer is one internal process
reading occasionally rather than many public clients doing fine-grained
range reads, ARMOR's whole-file-decrypt-on-read is simply the right fit
here, with no tradeoff to reason about.

## Consistent rule across all three

No component in this pipeline talks to B2 directly — every write goes
through ARMOR or Postgres. This was an explicit requirement, not just a
convenience: a single consistent write path, one credential model, no
per-component B2 application keys to provision and rotate.
