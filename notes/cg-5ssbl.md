# Queue-API Table Schemas and Extraction Plan (cg-5ssbl)

## Overview

This document describes the four tables in queue-api's SQLite database (`/data/queue.db`) that are relevant to the commitgraph v2 migration, their current schemas, and their target destinations in the new pipeline.

**Parent bead:** cg-5ol6 (Queue-api PVC preservation & extraction)

**Database location:** `ord-devimprint` cluster, pod `queue-api-c5894c469-p9rhr` (namespace `commitgraph`), PVC `queue-api-data` (10Gi sata volume)

## Summary Table

| Table | Destination | Extraction Required | Method |
|-------|-------------|---------------------|--------|
| `repo_head_cursors` | queue-api PVC (stays) | No | N/A |
| `catalog_version` | queue-api PVC (stays) | No | N/A |
| `blocklist` | Postgres `repos` table | Yes | CSV export + SQL migration |
| `tombstones` | Postgres `tombstones` table | Yes | CSV export + SQL migration |

---

## 1. repo_head_cursors

### Status
✅ **NO EXTRACTION NEEDED** - Stays in queue-api PVC

### Purpose
Tracks the last extracted HEAD SHA for each repository to enable warm-start incremental cloning via `git fetch` instead of full `git clone` on rescan. See `docs/research/incremental-fetch-warm-start.md` for empirical validation.

### Schema
```sql
CREATE TABLE repo_head_cursors (
    provider       TEXT    NOT NULL,
    repo_full_name TEXT    NOT NULL,
    head_sha       TEXT,   -- Last extracted HEAD (40-hex); NULL until first clone
    updated_at     TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (provider, repo_full_name)
);
```

### Columns
| Column | Type | Description |
|--------|------|-------------|
| `provider` | TEXT NOT NULL | Git hosting provider (e.g., 'github', 'gitlab') |
| `repo_full_name` | TEXT NOT NULL | Repository full name (e.g., 'owner/repo') |
| `head_sha` | TEXT | Last extracted HEAD SHA; NULL until first clone completes |
| `updated_at` | TEXT NOT NULL | SQLite timestamp of last update |

### Indexes
- Primary key: `(provider, repo_full_name)`

### Target Destination
**Stays in queue-api PVC** (`/data/queue.db`)

### Reason
Required for warm-start incremental cloning. The new pipeline needs these cursors to avoid re-downloading full repository history on rescan. The queue-api PVC must be preserved permanently to avoid losing this optimization.

### Extraction Method
N/A - No extraction needed. The table remains in the SQLite database and continues to be read by the new clone-worker implementation.

---

## 2. catalog_version

### Status
✅ **NO EXTRACTION NEEDED** - Stays in queue-api PVC

### Purpose
Tracks the detection catalog version for re-detection triggers. When a new AI tool signature is added to the detection catalog (`shared/detection.py`), this version number allows the system to identify repos that need re-scanning with the updated catalog.

### Schema
```sql
CREATE TABLE catalog_version (
    id         INTEGER PRIMARY KEY CHECK (id = 1),  -- Singleton row (id = 1 only)
    version    INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT    NOT NULL DEFAULT (datetime('now'))
);
```

### Columns
| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PRIMARY KEY | Always 1 (singleton table) |
| `version` | INTEGER NOT NULL | Current catalog version number |
| `updated_at` | TEXT NOT NULL | SQLite timestamp of last update |

### Constraints
- `CHECK (id = 1)` ensures only one row exists (singleton pattern)

### Indexes
- Primary key: `id`

### Target Destination
**Stays in queue-api PVC** (`/data/queue.db`)

### Reason
The detection catalog version tracking is queue-api's responsibility. When new tool signatures are added to `shared/detection.py` (which has `CATALOG_VERSION = "2024-08-05"` as the string version), the integer version in this table is incremented to trigger re-detection across the corpus.

### Extraction Method
N/A - No extraction needed. The table remains in the SQLite database and continues to be used for re-detection triggers.

---

## 3. blocklist

### Status
⚠️ **EXTRACTION REQUIRED** - Must be migrated to Postgres

### Purpose
Manually excluded repositories and users. Seeded by operators to exclude repos that don't meet inclusion criteria (e.g., forks that don't pass filters, spam, or policy violations).

### Schema
```sql
CREATE TABLE blocklist (
    provider  TEXT    NOT NULL,
    kind      TEXT    NOT NULL CHECK (kind IN ('repo','user','email')),
    identifier TEXT   NOT NULL,  -- repo full name, login, or email
    reason    TEXT,
    created_at TEXT   NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (provider, kind, identifier)
);
```

### Columns
| Column | Type | Description |
|--------|------|-------------|
| `provider` | TEXT NOT NULL | Git hosting provider |
| `kind` | TEXT NOT NULL | Type of exclusion: 'repo', 'user', or 'email' |
| `identifier` | TEXT NOT NULL | Repo full name, login, or email being excluded |
| `reason` | TEXT | Free-form exclusion reason |
| `created_at` | TEXT NOT NULL | SQLite timestamp when exclusion was added |

### Constraints
- `CHECK (kind IN ('repo','user','email'))` - Only these three values allowed

### Indexes
- Primary key: `(provider, kind, identifier)`

### Target Destination
**Postgres `repos` table** (for `kind='repo'` entries only)

### Transformation
```sql
-- Load blocklist repo entries as excluded repos
INSERT INTO repos (provider, repo_full_name, excluded_at, excluded_reason)
SELECT
    provider,
    identifier AS repo_full_name,
    -- Convert SQLite TEXT timestamp to TIMESTAMPTZ
    CASE
        WHEN created_at ~ '^\d{4}-\d{2}-\d{2}' THEN
            (created_at::timestamp)::timestamp with time zone
        ELSE NULL
    END AS excluded_at,
    COALESCE(reason, 'migrated from queue-api blocklist') AS excluded_reason
FROM blocklist_temp
WHERE kind = 'repo'
ON CONFLICT (provider, repo_full_name)
DO UPDATE SET
    excluded_at = EXCLUDED.excluded_at,
    excluded_reason = EXCLUDED.excluded_reason;
```

### Open Questions
1. **Where should `kind='user'` entries go?** Not currently used, but schema allows it.
2. **Where should `kind='email'` entries go?** Not currently used, but schema allows it.

### Extraction Method
**CSV Export + Postgres Migration**

#### Step 1: Export from queue-api
```bash
kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig exec \
  -n commitgraph queue-api-c5894c469-p9rhr -c queue-api \
  -- sqlite3 /data/queue.db <<'EOF'
.mode csv
.headers on
.output /tmp/blocklist.csv
SELECT * FROM blocklist;
.quit
EOF

kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig cp \
  -n commitgraph queue-api-c5894c469-p9rhr:/tmp/blocklist.csv \
  /home/coding/commitgraph/exports/blocklist.csv
```

#### Step 2: Load into Postgres
```bash
# Create temp table and load CSV
psql <database> <<EOF
CREATE TEMP TABLE blocklist_temp (
    provider TEXT, kind TEXT, identifier TEXT, reason TEXT, created_at TEXT
);
\copy blocklist_temp FROM 'exports/blocklist.csv' CSV HEADER
\i migrations/load_blocklist.sql
DROP TABLE blocklist_temp;
EOF
```

#### Step 3: Verify
```sql
-- Check row count
SELECT COUNT(*) AS blocklist_count FROM blocklist_temp;

-- Verify repos were marked excluded
SELECT COUNT(*) AS repos_excluded
FROM repos
WHERE excluded_at IS NOT NULL;
```

### Dependencies
- Requires admin kubeconfig for ord-devimprint (currently blocked on 401 unauthorized)
- Requires Postgres `repos` table to exist (migrations/00001_initial_schema.sql)

---

## 4. tombstones

### Status
⚠️ **EXTRACTION REQUIRED** - Must be migrated to Postgres

### Purpose
Row-level exclusion list for erasure enforcement. Stores commits that must be excluded from all results due to GDPR requests, leaked credentials, or takedown notices. Used by filter/compact workers for anti-join exclusion.

### Schema
```sql
CREATE TABLE tombstones (
    sha          TEXT    NOT NULL,
    author_email TEXT    NOT NULL,
    reason       TEXT,
    source       TEXT,
    created_at   TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (sha, author_email)
);
```

### Columns
| Column | Type | Description |
|--------|------|-------------|
| `sha` | TEXT NOT NULL | Commit SHA to exclude (globally unique, not provider-keyed) |
| `author_email` | TEXT NOT NULL | Author email for the excluded commit |
| `reason` | TEXT | Free-form context (GDPR, leaked credential, takedown) |
| `source` | TEXT | Actor that recorded the batch (audit provenance) |
| `created_at` | TEXT NOT NULL | SQLite timestamp when tombstone was added |

### Indexes
- Primary key: `(sha, author_email)`
- Note: Postgres migration adds secondary index on `author_email`

### Target Destination
**Postgres `tombstones` table** (new table)

### Postgres Schema
```sql
CREATE TABLE IF NOT EXISTS tombstones (
    sha          TEXT    NOT NULL,
    author_email TEXT    NOT NULL,
    reason       TEXT,
    source       TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (sha, author_email)
);

CREATE INDEX IF NOT EXISTS idx_tombstones_email
    ON tombstones (author_email);
```

### Key Differences from SQLite
- `created_at` is `TIMESTAMPTZ` (Postgres) instead of `TEXT` (SQLite)
- Additional index on `author_email` for query performance
- Table and column comments for documentation

### Extraction Method
**CSV Export + Postgres Migration**

#### Step 1: Export from queue-api
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

#### Step 2: Load into Postgres
```bash
psql <database> <<EOF
BEGIN;

-- Load CSV (handle SQLite TEXT → TIMESTAMPTZ conversion)
COPY tombstones (sha, author_email, reason, source, created_at)
FROM 'exports/tombstones.csv' WITH (FORMAT CSV, HEADER);

COMMIT;
EOF
```

#### Step 3: Verify
```sql
-- Check row count
SELECT COUNT(*) AS total_tombstones FROM tombstones;

-- Check recent tombstones (last 30 days)
SELECT COUNT(*) AS recent_tombstones
FROM tombstones
WHERE created_at > NOW() - INTERVAL '30 days';

-- Spot-check data integrity
SELECT * FROM tombstones LIMIT 5;
```

### Dependencies
- Requires admin kubeconfig for ord-devimprint (currently blocked on 401 unauthorized)
- Requires Postgres migration 00002_create_tombstones.sql to be run first
- No dependencies on other extractions

---

## Extraction Dependencies

### Dependency Graph
```
blocklist ──┐
            ├──► no dependencies between extractions
tombstones ─┘
```

### Sequential vs Parallel
The two extractions (`blocklist` and `tombstones`) are **independent** and can be run in parallel:
- They write to different Postgres tables
- They have no foreign key relationships to each other
- They don't depend on shared migration state

### Prerequisites
Both extractions require:
1. Admin kubeconfig for ord-devimprint (blocked on 401 unauthorized)
2. Postgres database running and accessible
3. Base schema migration (00001_initial_schema.sql) applied

### Execution Order
1. **Run base schema:** `00001_initial_schema.sql`
2. **Run tombstones migration:** `00002_create_tombstones.sql` (already exists)
3. **Export & load blocklist:** (independent, can run in parallel with tombstones)
4. **Export & load tombstones:** (independent, can run in parallel with blocklist)

---

## PVC Preservation Critical Reminder

**The `queue-api-data` PVC must be preserved permanently** even after extractions complete:

### What Must Be Preserved
1. **repo_head_cursors** - Warm-start incremental cloning data
2. **catalog_version** - Detection catalog version tracking
3. **Work queues** - 98,747 discovered repos awaiting scan
4. **Other queue-api state** - Ongoing coordination state

### Why It Must Be Preserved
- The new pipeline reads `repo_head_cursors` directly from queue-api
- The new pipeline uses `catalog_version` for re-detection triggers
- The `sata` StorageClass has `reclaimPolicy: Delete`
- Deleting the PVC destroys the Cinder volume and all data permanently

### What Can Be Deleted After Migration
Nothing from the PVC itself - the extractions are **read-only** copies that seed Postgres tables. The original data in queue-api SQLite must remain accessible to the new pipeline.

---

## Verification Checklist

### Pre-Extraction
- [ ] Admin kubeconfig for ord-devimprint is available (401 resolved)
- [ ] Postgres database is running and accessible
- [ ] Base schema migration (00001_initial_schema.sql) has been applied
- [ ] Tombstones migration (00002_create_tombstones.sql) has been applied
- [ ] Export directory exists: `/home/coding/commitgraph/exports/`

### Post-Extraction
- [ ] `blocklist.csv` exported successfully with correct row count
- [ ] `tombstones.csv` exported successfully with correct row count
- [ ] CSV files verified: row counts match SQLite `COUNT(*)` results
- [ ] Blocklist loaded into Postgres `repos` table
- [ ] Tombstones loaded into Postgres `tombstones` table
- [ ] Verification queries pass (see individual table sections)
- [ ] PVC retention documented in plan/TEARDOWN.md

### Operational Validation
- [ ] New pipeline can read `repo_head_cursors` from queue-api
- [ ] New pipeline can read `catalog_version` from queue-api
- [ ] Excluded repos in Postgres match blocklist source data
- [ ] Tombstones in Postgres match SQLite source data
- [ ] No data loss in transformations (SQLite TEXT → Postgres TIMESTAMPTZ)

---

## References

### Parent Documentation
- **Parent bead:** cg-5ol6 (Queue-api PVC preservation & extraction)
- **Extraction plan:** notes/cg-5ol6-extraction-plan.md
- **Warm-start research:** docs/research/incremental-fetch-warm-start.md

### Related Migrations
- **Base schema:** migrations/00001_initial_schema.sql
- **Tombstones:** migrations/00002_create_tombstones.sql
- **Blocklist loading:** migrations/load_blocklist.sql

### Related Beads
- **cg-2v70:** email_resolution extraction (separate bead, same PVC context)
- **cg-5ol6:** Overall queue-api PVC preservation and extraction

---

## Appendix: Current Blocker Status

**Status:** BLOCKED on Admin Credentials

The ord-devimprint read-only proxy (`http://kubectl-proxy-ord-devimprint:8001`) explicitly blocks:
- `kubectl exec` - "unable to upgrade connection: Forbidden"
- `kubectl cp` - requires exec internally
- `kubectl port-forward` - "cannot create resource pods/portforward"

**Resolution path:** Need refreshed `ord-devimprint-admin.kubeconfig` (currently 401 unauthorized)

Once access is restored, the extraction commands in this document can proceed immediately.
