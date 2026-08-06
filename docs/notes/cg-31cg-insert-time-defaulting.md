# `insert_time` Defaulting Strategy — cg-31cg

## Decision

**Chosen approach:** Column `DEFAULT transaction_timestamp()`

## Implementation

1. **Schema change:** `repo_user_daily_tool.insert_time` now has `DEFAULT transaction_timestamp()`
2. **Write-path change:** Bulk INSERT statements omit `insert_time` from the column list, allowing the DEFAULT to apply automatically

## Why This Approach

### Transaction Consistency Guarantee
`transaction_timestamp()` (PostgreSQL equivalent to SQL standard `CURRENT_TIMESTAMP`) returns the **same value for all statements within a single transaction**. This guarantees that every row inserted in one repo's DELETE+INSERT operation shares exactly one `insert_time` value, regardless of:
- How many rows are inserted
- How long the transaction takes
- When individual statements execute within the transaction

### Eliminates Application Bugs
With a column DEFAULT, there is no application code that can "forget" to reuse the same timestamp. The database enforces the invariant automatically. Contrast this with an explicit parameter approach, where:
- Every caller must remember to compute the timestamp once per job
- Every caller must remember to pass that same value to every row
- Bugs like "recompute per row" or "forget to pass entirely" are possible

### UNNEST Compatibility
The bulk insert mechanism (`INSERT INTO ... SELECT * FROM UNNEST(...)`) works seamlessly with column defaults:
- Columns omitted from the INSERT column list automatically receive their DEFAULT values
- All rows in the same INSERT receive the same DEFAULT value (within one transaction)
- No application changes needed beyond omitting the column

### Why NOT `clock_timestamp()`
PostgreSQL also provides `clock_timestamp()`, which returns the **actual current time** and can change within a transaction. This would NOT satisfy the requirement and must NOT be used. `transaction_timestamp()` is specifically chosen because it is constant within a transaction.

## Alternative Considered: Explicit Parameter

**Explicit parameter approach:** Compute timestamp once per job, pass as explicit parameter to every row in bulk INSERT.

**Rejected because:**
- Places burden on application code to remember the invariant
- "Compute once, reuse everywhere" is easy to forget when refactoring
- Column DEFAULT achieves the same result with stronger guarantees
- UNNEST has no compatibility issues with DEFAULT values

## Verification

To verify the implementation works correctly:

```sql
BEGIN;
-- Verify DEFAULT exists
\d repo_user_daily_tool
-- Should show: insert_time TIMESTAMPTZ NOT NULL DEFAULT transaction_timestamp()

-- Verify transaction consistency
INSERT INTO repo_user_daily_tool (repo_id, user_id, tool, day, commits)
VALUES (1, 1, 'test-tool', '2026-08-05', 1),
       (1, 1, 'test-tool', '2026-08-06', 1);
  
SELECT insert_time, count(*) FROM repo_user_daily_tool 
WHERE repo_id = 1 GROUP BY insert_time;
-- Should show exactly 1 row (all same insert_time)
ROLLBACK;
```

## References

- Task: cg-31cg
- Plan section: "`insert_time` — scan recency, not commit recency" (docs/plan/plan.md lines 776-793)
- PostgreSQL docs: https://www.postgresql.org/docs/current/functions-datetime.html#FUNCTIONS-DATETIME-CURRENT
