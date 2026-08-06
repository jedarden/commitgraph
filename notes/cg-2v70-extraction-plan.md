# email_resolution Extraction Plan (cg-2v70)

## Current Status: BLOCKED on Admin Credentials

The ord-devimprint read-only proxy (`http://kubectl-proxy-ord-devimprint:8001`) explicitly blocks:
- `kubectl exec` - "unable to upgrade connection: Forbidden"
- `kubectl cp` - requires exec internally
- `kubectl port-forward` - "cannot create resource pods/portforward"

Per plan.md line 133-135: "Extraction is blocked on a refreshed `ord-devimprint-admin.kubeconfig` (currently 401)."

## Target Information

### Pod Details
- **Pod:** `queue-api-c5894c469-p9rhr` in namespace `commitgraph`
- **Database Path:** `/data/queue.db` (not `/data/db.db`)
- **PVC:** `queue-api-data` (bound to 10Gi sata volume, **DO NOT DELETE**)
- **Container:** `queue-api` (running `ronaldraygun/commitgraph-queue-api:2.8.0`)

### Service Details
- **Service:** `queue-api` (ClusterIP `10.21.91.206:8080`)
- **Endpoints:** `10.20.218.77:8080` (pod IP)

### Table Schema (to be verified)
- From plan.md: `email_resolution` table contains 365K+ email→login pairs
- Columns likely include: `email`, `login`, and timestamp/status columns
- Need to verify exact schema by checking `.schema email_resolution`

## Extraction Commands (Once Admin Access is Available)

### Step 1: Get Schema Verification
```bash
# Using admin kubeconfig
kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig exec \
  -n commitgraph queue-api-c5894c469-p9rhr -c queue-api \
  -- sqlite3 /data/queue.db ".schema email_resolution"
```

### Step 2: Get Row Count
```bash
kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig exec \
  -n commitgraph queue-api-c5894c469-p9rhr -c queue-api \
  -- sqlite3 /data/queue.db "SELECT COUNT(*) FROM email_resolution;"
```

### Step 3: Export Data (Multiple Options)

#### Option A: SQLite Dump (Preserves Schema & Data)
```bash
kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig exec \
  -n commitgraph queue-api-c5894c469-p9rhr -c queue-api \
  -- sqlite3 /data/queue.db ".dump email_resolution" > \
  /home/coding/commitgraph/exports/email_resolution.sql
```

#### Option B: CSV Export (Better for Postgres import)
```bash
# First, enable CSV mode in sqlite3
kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig exec \
  -n commitgraph queue-api-c5894c469-p9rhr -c queue-api \
  -- sqlite3 /data/queue.db <<'EOF'
.mode csv
.headers on
.output /tmp/email_resolution.csv
SELECT * FROM email_resolution;
.quit
EOF

# Then copy the file out
kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig cp \
  -n commitgraph queue-api-c5894c469-p9rhr:/tmp/email_resolution.csv \
  /home/coding/commitgraph/exports/email_resolution.csv
```

#### Option C: Parquet Export (Best for Large Datasets)
```bash
# Requires Python with pandas/pyarrow in the pod - may not be available
# Alternative: Dump to CSV first, then convert locally
```

### Step 4: Verify Export
```bash
# Compare row counts
wc -l /home/coding/commitgraph/exports/email_resolution.csv
# Should match the COUNT(*) from Step 2 (plus 1 for header if CSV)

# Verify schema
head -1 /home/coding/commitgraph/exports/email_resolution.csv
```

### Step 5: Store in Durable Location
```bash
# Options:
# - Upload to B2 bucket (commitgraph-b2-workers)
# - Copy to ~/backups/commitgraph-cutover/
# - Commit to repo (if small enough and acceptable)

# Ensure file is not left only on ex44 local disk
```

## Acceptance Criteria Verification

- [ ] Dump captures every column in email_resolution schema
- [ ] Row count matches `SELECT COUNT(*) FROM email_resolution` (recorded, not trusted)
- [ ] queue-api, Service, and PVC completely untouched (no kubectl delete/patch/scale)
- [ ] Written confirmation that `queue-api-pvc.yml` NOT removed
- [ ] Dump file stored somewhere durable

## PVC Protection Reminder

Per plan.md line 131-133:
> `queue-api-data` holds `email_resolution` — 365K+ resolved email→login pairs
> representing months of spent GitHub API budget against a shared ~30 req/min
> ceiling — which this pipeline inherits rather than re-earns. The `sata`
> StorageClass has `reclaimPolicy: Delete`, so pruning that PVC destroys the
> Cinder volume and every row in it.

**Do NOT delete the PVC before extraction is verified.**

## Notes

- Once admin credentials are available, the extraction itself should take <5 minutes
- The SQLite dump format is self-contained and portable
- CSV format may be better for importing into Postgres
- Consider using litestream replication status as backup extraction method if direct access fails
