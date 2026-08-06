# Tombstones Extraction Status (cg-56tnu)

## Blocker Status

**BLOCKED on Admin Credentials for ord-devimprint**

This extraction requires admin access to the ord-devimprint cluster, which is currently unavailable.

## Current Situation

As of 2026-08-06, the following access methods are blocked by the read-only proxy (`http://kubectl-proxy-ord-devimprint:8001`):

1. **`kubectl exec` - Forbidden**: Cannot execute commands in the queue-api pod to export SQLite data
2. **`kubectl cp` - Blocked**: Requires exec internally, cannot copy files from pod
3. **`kubectl port-forward` - Forbidden**: Cannot create resource pods/portforward
4. **Pod creation - Forbidden**: Cannot create temporary pods to access queue-api HTTP endpoint

## Extraction Plan (When Access is Restored)

Once admin credentials are available, the extraction will proceed as follows:

### Step 1: Export from queue-api SQLite

```bash
kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig exec \
  -n commitgraph queue-api-c5894c469-p9rhr -c queue-api \
  -- sqlite3 /data/queue.db <<'EOF'
.mode csv
.headers on
.output /tmp/tombstones.csv
SELECT * FROM tombstones;
.quit
EOF

kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig cp \
  -n commitgraph queue-api-c5894c469-p9rhr:/tmp/tombstones.csv \
  /home/coding/commitgraph/exports/tombstones.csv
```

### Step 2: Verify Export

```bash
# Check row count matches source
wc -l /home/coding/commitgraph/exports/tombstones.csv
# Should match: SELECT COUNT(*) FROM tombstones
```

### Step 3: Load into Postgres

The Postgres schema is already created by `migrations/00002_create_tombstones.sql`:

```sql
CREATE TABLE IF NOT EXISTS tombstones (
    sha          TEXT    NOT NULL,
    author_email TEXT    NOT NULL,
    reason       TEXT,
    source       TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (sha, author_email)
);
CREATE INDEX IF NOT EXISTS idx_tombstones_email ON tombstones (author_email);
```

Load script: `scripts/load-tombstones-to-postgres.sh`

### Step 4: Verification

```sql
-- Check row count
SELECT COUNT(*) AS total_tombstones FROM tombstones;

-- Spot-check data integrity
SELECT * FROM tombstones LIMIT 5;
```

## Schema Information

**Source (queue-api SQLite):**
- Table: `tombstones`
- Primary key: `(sha, author_email)`
- Columns: `sha`, `author_email`, `reason`, `source`, `created_at` (TEXT)

**Destination (Postgres):**
- Table: `tombstones`
- Primary key: `(sha, author_email)`
- Columns: `sha`, `author_email`, `reason`, `source`, `created_at` (TIMESTAMPTZ)
- Index: `idx_tombstones_email` on `author_email`

## Key Transformation

- SQLite TEXT timestamp → Postgres TIMESTAMPTZ
- Additional index on `author_email` for query performance

## Dependencies

- Requires admin kubeconfig for ord-devimprint
- Requires Postgres `tombstones` table (migration 00002 already exists)
- No dependencies on other extractions (can run in parallel with blocklist)

## References

- Documentation: `notes/cg-5ssbl.md` (Queue-API Table Schemas and Extraction Plan)
- Schema migration: `migrations/00002_create_tombstones.sql`
- Load script: `scripts/load-tombstones-to-postgres.sh`
- Parent bead: cg-5ol6 (Queue-api PVC preservation & extraction)

## Next Steps

Once the ord-devimprint-admin.kubeconfig access is restored (401 resolved), the extraction can proceed immediately using the documented steps.
