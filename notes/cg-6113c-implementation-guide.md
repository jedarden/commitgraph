# Blocklist Extraction Implementation Guide (cg-6113c)

## Prerequisites Checklist

Before starting the extraction, ensure these requirements are met:

- [ ] Admin kubeconfig access: `ord-devimprint-admin.kubeconfig` (current: 401 unauthorized)
- [ ] kubectl installed and configured
- [ ] Access to Postgres database (for loading)
- [ ] `scripts/extract-blocklist.sh` exists and is executable
- [ ] `scripts/load-blocklist-to-postgres.sh` exists and is executable
- [ ] Read access to this implementation guide

## Step-by-Step Extraction Process

### Phase 1: Pre-Extraction Verification

#### 1.1 Test Admin Access

```bash
# Verify admin kubeconfig works
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get pods -n commitgraph

# Expected output: list of pods including queue-api-c5894c469-p9rhr
# If 401 unauthorized: kubeconfig needs refresh from Rackspace Spot UI
```

#### 1.2 Verify queue-api Pod Status

```bash
# Check pod is running
POD_NAME=$(kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
  get pods -n commitgraph -l app=queue-api -o jsonpath='{.items[0].metadata.name}')

echo "Pod: ${POD_NAME}"

# Check pod phase
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
  get pod "${POD_NAME}" -n commitgraph -o jsonpath='{.status.phase}'

# Expected: Running
```

#### 1.3 Check Database File Exists

```bash
# Verify SQLite database exists on the pod
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
  exec -n commitgraph "${POD_NAME}" -c queue-api -- ls -lh /data/queue.db

# Expected: file with size > 0
```

### Phase 2: Schema Inspection (Read-Only)

#### 2.1 Inspect Blocklist Schema

```bash
# Get blocklist table schema
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
  exec -n commitgraph "${POD_NAME}" -c queue-api -- \
  sqlite3 /data/queue.db ".schema blocklist"

# Expected: CREATE TABLE statement with provider, kind, identifier, reason, created_at
```

#### 2.2 Check Blocklist Row Counts

```bash
# Count total blocklist entries
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
  exec -n commitgraph "${POD_NAME}" -c queue-api -- \
  sqlite3 /data/queue.db "SELECT COUNT(*) FROM blocklist;"

# Count by kind
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
  exec -n commitgraph "${POD_NAME}" -c queue-api -- \
  sqlite3 /data/queue.db "SELECT kind, COUNT(*) FROM blocklist GROUP BY kind;"

# Expected: Breakdown by 'repo', 'user', 'email'
```

#### 2.3 Sample Blocklist Data

```bash
# Get sample of blocklist repo entries
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
  exec -n commitgraph "${POD_NAME}" -c queue-api -- \
  sqlite3 /data/queue.db "SELECT * FROM blocklist WHERE kind = 'repo' LIMIT 5;"

# Verify: Check timestamp format and reason field presence
```

### Phase 3: Data Extraction

#### 3.1 Run Extraction Script

```bash
# Execute the blocklist extraction script
cd /home/coding/commitgraph
./scripts/extract-blocklist.sh

# Expected output:
# - "Extracting blocklist from queue-api SQLite..."
# - "Found queue-api pod: queue-api-c5894c469-p9rhr"
# - "Exporting blocklist table to CSV..."
# - "Extracted <N> blocklist records"
# - Two files created: exports/blocklist-<timestamp>.csv and .json
```

#### 3.2 Verify Extraction Success

```bash
# Check export files were created
ls -lh /home/coding/commitgraph/exports/blocklist-*.csv | tail -1
ls -lh /home/coding/commitgraph/exports/blocklist-*.json | tail -1

# Verify CSV structure
head -1 /home/coding/commitgraph/exports/blocklist-*.csv | tail -1

# Expected header: provider,kind,identifier,reason,created_at

# Count rows (excluding header)
ROW_COUNT=$(wc -l < /home/coding/commitgraph/exports/blocklist-*.csv | tail -1 | awk '{print $1 - 1}')
echo "Extracted ${ROW_COUNT} blocklist entries"

# Verify row count matches source
```

### Phase 4: Data Loading (Postgres)

#### 4.1 Check Postgres Connectivity

```bash
# Test Postgres connection (adjust connection details as needed)
export PGHOST="${PGHOST:-localhost}"
export PGPORT="${PGPORT:-5432}"
export PGDATABASE="${PGDATABASE:-commitgraph}"
export PGUSER="${PGUSER:-postgres}"

psql -h "${PGHOST}" -p "${PGPORT}" -U "${PGUSER}" -d "${PGDATABASE}" -c "SELECT 1;"

# Expected: Single row with "1"
```

#### 4.2 Run Load Script

```bash
# Execute the load script with the extracted CSV
CSV_FILE=$(ls -t /home/coding/commitgraph/exports/blocklist-*.csv | head -1)

./scripts/load-blocklist-to-postgres.sh "${CSV_FILE}"

# Expected output:
# - "Loading blocklist from <file>"
# - Blocklist summary by kind
# - Loaded repos: <count>
# - Verification results
# - Migration complete!
```

#### 4.3 Verify Load Success

```bash
# Check excluded repos count
export PGHOST="${PGHOST:-localhost}"
export PGPORT="${PGPORT:-5432}"
export PGDATABASE="${PGDATABASE:-commitgraph}"
export PGUSER="${PGUSER:-postgres}"

psql -h "${PGHOST}" -p "${PGPORT}" -U "${PGUSER}" -d "${PGDATABASE}" -c \
  "SELECT COUNT(*) FROM repos WHERE excluded_at IS NOT NULL;"

# Expected: Count matching blocklist kind='repo' entries
```

### Phase 5: Cross-Check Verification

#### 5.1 Verify All Blocklist Repos Are Excluded

```bash
# Create temporary table from blocklist CSV for verification
CSV_FILE=$(ls -t /home/coding/commitgraph/exports/blocklist-*.csv | head -1)

psql -h "${PGHOST}" -p "${PGPORT}" -U "${PGUSER}" -d "${PGDATABASE}" <<EOF
BEGIN;

-- Create temp table for verification
DROP TABLE IF EXISTS blocklist_verify;
CREATE TEMP TABLE blocklist_verify (
    provider TEXT,
    kind TEXT,
    identifier TEXT,
    reason TEXT,
    created_at TEXT
);

-- Load CSV
\copy blocklist_verify FROM '${CSV_FILE}' CSV HEADER

-- Verification query: find blocklist repos that are NOT excluded
SELECT 'Missing exclusions (should be 0):' AS info;
SELECT COUNT(*) AS missing_exclusions
FROM (SELECT DISTINCT provider, identifier 
      FROM blocklist_verify WHERE kind = 'repo') AS bl
LEFT JOIN repos ON bl.provider = repos.provider 
              AND bl.identifier = repos.repo_full_name
WHERE repos.repo_id IS NULL OR repos.excluded_at IS NULL;

-- Check for timestamp conversion failures
SELECT 'Timestamp conversion failures (should be 0):' AS info;
SELECT COUNT(*) AS conversion_failures
FROM blocklist_verify bv
JOIN repos r ON bv.provider = r.provider 
            AND bv.identifier = r.repo_full_name
WHERE bv.kind = 'repo' 
  AND bv.created_at IS NOT NULL
  AND bv.created_at ~ '^\d{4}-\d{2}-\d{2}'
  AND r.excluded_at IS NULL;

COMMIT;
EOF

# Expected: Both queries return 0
```

#### 5.2 Verify Default Reason Applied

```bash
psql -h "${PGHOST}" -p "${PGPORT}" -U "${PGUSER}" -d "${PGDATABASE}" <<EOF
-- Check exclusions with default reason
SELECT 'Exclusions with default reason:' AS info;
SELECT COUNT(*) AS with_default_reason
FROM repos
WHERE excluded_reason = 'migrated from queue-api blocklist';

-- Sample a few exclusions to verify data quality
SELECT 'Sample exclusions (first 5):' AS info;
SELECT provider, repo_full_name, excluded_at, excluded_reason
FROM repos
WHERE excluded_at IS NOT NULL
ORDER BY excluded_at DESC
LIMIT 5;
EOF

# Expected: Reasonable counts and sensible data
```

#### 5.3 Summary Statistics

```bash
psql -h "${PGHOST}" -p "${PGPORT}" -U "${PGUSER}" -d "${PGDATABASE}" <<EOF
-- Overall migration statistics
SELECT 
    COUNT(*) AS total_excluded_repos,
    COUNT(DISTINCT provider) AS providers,
    COUNT(CASE WHEN excluded_reason LIKE '%migrated%' THEN 1 END) AS with_default_reason,
    MIN(excluded_at) AS earliest_exclusion,
    MAX(excluded_at) AS latest_exclusion
FROM repos
WHERE excluded_at IS NOT NULL;
EOF

# Expected: Sensible statistics (no null timestamps, reasonable date range)
```

### Phase 6: Rollback Testing (Optional)

#### 6.1 Test Migration Reversibility

```bash
# Test that we can rollback if needed (in a transaction)
psql -h "${PGHOST}" -p "${PGPORT}" -U "${PGUSER}" -d "${PGDATABASE}" <<EOF
BEGIN;

-- Save current state
CREATE TEMP TABLE repos_before AS SELECT * FROM repos WHERE excluded_at IS NOT NULL;
SELECT COUNT(*) AS repos_before_exclusion FROM repos_before;

-- Simulate rollback by clearing exclusions
UPDATE repos SET excluded_at = NULL, excluded_reason = NULL WHERE excluded_at IS NOT NULL;

-- Verify rollback (should be 0 excluded repos)
SELECT COUNT(*) AS after_rollback FROM repos WHERE excluded_at IS NOT NULL;

-- Rollback the transaction
ROLLBACK;

-- Verify state is restored
SELECT COUNT(*) AS after_rollback_restore FROM repos WHERE excluded_at IS NOT NULL;

-- Expected: after_rollback = 0, after_rollback_restore = original count
EOF
```

## Troubleshooting

### Issue: Admin kubeconfig 401 Unauthorized

**Solution:** Refresh the admin kubeconfig from Rackspace Spot UI:
1. Log in to Rackspace Spot console
2. Navigate to the ord-devimprint cloudspace
3. Generate new cloudspace-admin OIDC token
4. Update kubeconfig at `/home/coding/.kube/ord-devimprint-admin.kubeconfig`

### Issue: Pod Not Found

**Solution:** Check pod deployment:
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
  get pods -n commitgraph -l app=queue-api

# If no pods found: queue-api may be scaled down or redeployed
# Check deployment status and scale up if needed
```

### Issue: CSV File Empty

**Solution:** Verify blocklist table has data:
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
  exec -n commitgraph "${POD_NAME}" -c queue-api -- \
  sqlite3 /data/queue.db "SELECT COUNT(*) FROM blocklist;"

# If 0: blocklist table is empty - nothing to migrate
# If > 0: Check extraction script logs for errors
```

### Issue: Postgres Connection Failed

**Solution:** Verify Postgres is accessible:
```bash
# Check Postgres pod is running
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get pods -n commitgraph -l app=postgres

# Check connection string details
# Verify network policies allow access
# Check credentials are correct
```

### Issue: Verification Fails (Missing Exclusions)

**Solution:** Investigate data mismatch:
```bash
# Check blocklist CSV for repo entries
grep ",repo," /home/coding/commitgraph/exports/blocklist-*.csv | head -5

# Check if repos exist in Postgres
psql -h "${PGHOST}" -p "${PGPORT}" -U "${PGUSER}" -d "${PGDATABASE}" <<EOF
SELECT 'Sample blocklist repos:' AS info;
SELECT DISTINCT provider, identifier
FROM blocklist_verify 
WHERE kind = 'repo'
LIMIT 5;

SELECT 'Check if these repos exist in repos table:' AS info;
SELECT provider, repo_full_name, excluded_at
FROM repos
WHERE (provider, repo_full_name) IN (
    SELECT DISTINCT provider, identifier
    FROM blocklist_verify
    WHERE kind = 'repo'
    LIMIT 5
);
EOF

# Common cause: repos don't exist in repos table (need to be created first)
# Resolution: Ensure repos are seeded before exclusion migration
```

## Post-Migration Tasks

### 1. Document User/Email Exclusions

The blocklist contains `kind='user'` and `kind='email'` entries that are NOT migrated. Document this:

```bash
# Count user/email exclusions
grep ",user," /home/coding/commitgraph/exports/blocklist-*.csv | wc -l
grep ",email," /home/coding/commitgraph/exports/blocklist-*.csv | wc -l

# Add to documentation: these remain in queue-api until future user/email exclusion mechanism
```

### 2. Update Runbooks

Update `/home/coding/commitgraph/docs/runbooks/repo-exclusion.md` to reference migrated exclusions.

### 3. Set Up Monitoring

Add monitoring for excluded repos count to detect unexpected changes.

## Success Criteria

The extraction is complete when:

- [ ] Blocklist CSV extracted successfully (>0 rows for kind='repo')
- [ ] All `kind='repo'` blocklist entries loaded into Postgres
- [ ] Verification query returns 0 missing exclusions
- [ ] No timestamp conversion failures (all valid timestamps converted)
- [ ] All excluded repos have non-NULL excluded_reason
- [ ] Statistics match expectations (counts, providers, date ranges)
- [ ] User/email exclusions documented as out-of-scope
- [ ] Rollback tested (optional but recommended)

## Blocker Status

**CURRENT BLOCKER:** Admin kubeconfig unavailable (`ord-devimprint-admin.kubeconfig` missing)

**What's needed:**
1. Refresh admin kubeconfig from Rackspace Spot UI (OIDC token refresh)
2. Verify admin access works with test kubectl command
3. Run extraction script

**Alternative approaches (if admin access unavailable):**
1. **Direct PVC access:** Copy PVC content to a location with admin access
2. **API endpoint:** Check if queue-api has HTTP endpoint for blocklist (TBD)
3. **Postgres-only migration:** Skip extraction and manually enter exclusions (not recommended)

---

**Next immediate action:** Resolve admin kubeconfig access, then proceed with Phase 1.
