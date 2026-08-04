# Postgres computes the leaderboard ranking directly — DuckDB is not a compute engine here

## The question

Once the rollup lives in Postgres, should the aggregator (a) query Postgres
directly for rank/window/percentile computation, or (b) export the rollup
to Parquet and let DuckDB do that computation over the exported snapshot —
DuckDB already being a proven, familiar analytical engine elsewhere in this
project's history (the predecessor's aggregator used a DuckDB Stage 2 for
exactly this kind of SQL)?

## The answer: Postgres computes it directly

`RANK() OVER`, 30-day window sums, tiebreak-by-recency, and
`PERCENTILE_CONT` distribution across all users are all native Postgres SQL
capabilities, more than sufficient at this row count (low hundreds of
thousands to low tens of millions of rows). This is also exactly what the
proven predecessor system, claude-leaderboard, already does — see
`docs/research/claude-leaderboard-comparison.md` — its
`generate_leaderboard_v3.py` computes rank/window/alias-merge/percentiles
directly against SQLite, no Parquet/DuckDB round-trip in the computation
path at all.

Routing computation through DuckDB-over-an-exported-Parquet-snapshot would
mean: export Postgres → Parquet, *then* query that snapshot. That's an
extra ETL hop sitting directly in the path of the thing this whole redesign
exists to fix — freshness. Every export cycle reintroduces a lag window,
just a shorter one than the predecessor's 24h.

The usual reason to separate a write-store from an analytical-query engine
(OLTP/OLAP isolation, so heavy read queries don't contend with live writes)
doesn't apply here: public traffic never touches Postgres at all. The
public-facing artifact is a static snapshot published to ARMOR (see
`armor-vs-postgres-placement.md`); only the aggregator's own periodic job
queries Postgres, on an interval — a light, bounded read load, nothing like
a live serving path with real contention risk.

## DuckDB/Parquet's actual role

Purely as an **output format**. The aggregator computes the full ranking in
Postgres SQL, then exports the already-computed result to Parquet for
publishing to ARMOR — matching the existing `query_leaderboard.py
--parquet` read-path shape for compatibility. DuckDB is never in the
compute path, only ever a downstream reader of a static artifact this
pipeline produces.
