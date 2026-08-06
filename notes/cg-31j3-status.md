# cg-31j3 Status Report

## Task
Load extracted email_resolution dump into Postgres via ingest path (source='live')

## Completed Work

### 1. Dump Analysis ✓
- **Total rows in dump**: 966,679 INSERT statements
- **Resolved entries** (status='resolved' with non-NULL login): 59,745 rows
- **Unresolvable entries** (status='unresolvable'): 11,763 rows  
- **Pending/claimed entries**: 895,171 rows (not yet resolved, not loaded)

### 2. Schema Verification ✓
Queue-api has **12 columns** in `email_resolution`:
- author_email, github_login, provider, status
- priority, is_alias_candidate, claimed_by
- claimed_at, lease_expires_at, **attempted_at** ✓
- created_at, updated_at

The **`attempted_at`** column contains the resolution timestamp for resolved entries. Sample data shows `attempted_at` matches `updated_at` for resolved rows, confirming it's the true resolution time.

### 3. Ingest Script Created ✓
Created `cmd/load-email-resolution-from-queue-api/main.go` that:
- Parses SQLite dump format (quote-aware CSV parsing)
- Filters to only resolved entries (59,745 rows)
- Sets `source='live'` on every row
- Uses `attempted_at` as `resolved_at` (preserving original resolution time)
- Batches ingest calls (10,000 rows per batch)
- Calls identity ingest endpoint (exercises ON CONFLICT rule)

### 4. Verification ✓
Created `cmd/verify-email-resolution-dump/main.go` for dry-run analysis.

## Blocking Issue

**Postgres CNPG cluster not yet deployed.**

The ord-devimprint cluster has no CNPG cluster in the `commitgraph` namespace. This task depends on:

1. **Phase 0 infrastructure** (epic cg-55o4w):
   - Postgres node provisioned in Spot (bead cg-4qzrd - open)
   - CNPG cluster deployed (not yet assigned a bead?)
   - Schema migration applied (bead cg-62ln - open)

2. **Schema migration** must create `email_resolution` table first:
   - Requires migration 0001 to be applied
   - Table must exist before ingest can run

## What Happens When Postgres is Available

Once the CNPG cluster is running and schema is applied, execution is:

```bash
# Set DATABASE_URL to point to the CNPG cluster
export DATABASE_URL="postgres://user:pass@hostname:5432/commitgraph"

# Run the ingest
go run cmd/load-email-resolution-from-queue-api/main.go exports/email_resolution_fresh_20260806_161432.sql
```

Expected result:
- 59,745 resolved rows submitted through ingest path
- source='live' on all rows
- resolved_at preserved from queue-api's attempted_at
- Final verification: Compare `SELECT COUNT(*) FROM email_resolution WHERE source='live'` against 59,745

## Reconciliation Behavior

The ON CONFLICT rule will handle any concurrent live enrichment worker activity:
- Live worker with newer resolved_at WINS (overwrites stale data)
- Live worker with older resolved_at LOSES (preserves fresher dump data)
- Manual source ALWAYS WINS (never overwrites hand-curations)

This is the first real exercise of the conflict rule and validates the architecture works as designed.

## Files Created

1. `cmd/load-email-resolution-from-queue-api/main.go` - Main ingest script
2. `cmd/verify-email-resolution-dump/main.go` - Verification/dry-run tool

## Recommendation

**Mark this bead as blocked on Phase 0 completion**, or alternatively:
- Execute it once cg-4qzrd (Spot node) and cg-62ln (schema migration) are closed
- The script is ready and tested for parsing logic; only runtime execution awaits database availability
