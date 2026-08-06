# `insert_time` Defaulting Strategy (cg-31cg)

## Decision

**Use column `DEFAULT transaction_timestamp()`** — the application code does not compute or pass `insert_time` at all.

## Why This Approach

1. **Uniform timestamp per transaction**: `transaction_timestamp()` is guaranteed to return the same value for every statement within a single transaction. Since every rescan writes one repo's entire rollup slice in a single transaction (DELETE + bulk INSERT), all rows for that repo automatically share exactly one `insert_time` value.

2. **No application-side bugs**: By removing `insert_time` from the application code entirely, we eliminate an entire class of bugs where the app might forget to reuse the same timestamp across rows, compute different values per row, or drift from the actual transaction time.

3. **Postgres guarantees correctness**: The database is the authoritative source of when a transaction committed. Using `transaction_timestamp()` means `insert_time` reflects the actual database transaction time, not an application-layer clock that can skew or be set incorrectly.

4. **Works with bulk INSERT via `UNNEST`**: The plan's write pattern uses `INSERT INTO repo_user_daily_tool (...) SELECT UNNEST($1::bigint[]), ...`. Column defaults apply seamlessly to this pattern — each row inserted by the bulk INSERT gets the same default value because they're all in the same transaction.

## Why Not `now()` or `clock_timestamp()`

- `now()` (statement timestamp) is constant within a statement but can vary between statements in the same transaction. For a multi-statement transaction (upsert repos → DELETE → bulk INSERT), this would not guarantee uniformity.
- `clock_timestamp()` is the actual wall-clock time and changes for every statement. Using it would give different `insert_time` values to rows inserted in the same transaction, violating the invariant that all rows for a repo share one scan time.

## What Changed

- **Migration (`00001_initial_schema.sql`)**: Already defines `insert_time TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp()` — no change needed.
- **Application code (`pkg/rollup/rollup.go`)**: Removed the `InsertTime: time.Now().UTC()` field from `RollupRow` initialization. The database default now provides the value exclusively.

## Invariant

This implementation satisfies **Invariant 5** from the plan:

> All rows for a given `repo_id` share one `insert_time` — a violation means a partial write escaped the whole-slice-replace transaction.

Because all rows are inserted in one transaction with a column default drawn from `transaction_timestamp()`, this invariant holds by construction rather than by careful application bookkeeping.
