# Queue-API Tables Extraction - Blocker Workaround (cg-5ol6)

## Current Blocker

**Issue:** `ord-devimprint-admin.kubeconfig` returns 401 unauthorized
- Blocks `kubectl exec` into queue-api pod
- Blocks `kubectl cp` for file extraction
- Read-only proxy (`http://kubectl-proxy-ord-devimprint:8001`) explicitly denies exec access

## Tables Status

### ✅ No Extraction Needed (Documented)
1. **repo_head_cursors** - Stays in queue-api PVC for warm-start incremental cloning
2. **catalog_version** - Stays in queue-api PVC for detection catalog versioning

### ⚠️ Extraction Required (Blocked on Admin Access)
1. **blocklist** - Needs extraction to Postgres `repos.excluded_at`
2. **tombstones** - Needs extraction to Postgres `tombstones` table

## Alternative Extraction Strategies

### Option 1: HTTP Endpoints (Recommended)
Queue-api may expose HTTP endpoints for data export:

```bash
# Try accessing tombstones endpoint via port-forward to a local pod
# Requires spawning a temporary pod that can reach queue-api service
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  run -n commitgraph extract-client --image=curlimages/curl:latest \
  --command -- sleep 3600

# Then use that pod to curl queue-api endpoints
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  exec -n commitgraph extract-client -- \
  curl -s http://queue-api.commitgraph.svc.cluster.local:8080/tombstones
```

### Option 2: PVC Snapshot (if available)
Check if there's a PVC snapshot or backup that can be mounted to a temporary pod:

```bash
# Check for PVC snapshots
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get volumesnapshot -n commitgraph

# If snapshot exists, create pod from it
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  run -n commitgraph extract-db --image=sqlite:sqlite3 \
  --overrides='
{
  "spec": {
    "containers": [{
      "name": "extract",
      "image": "sqlite:sqlite3",
      "volumeMounts": [{
        "name": "queue-data",
        "mountPath": "/data"
      }]
    }],
    "volumes": [{
      "name": "queue-data",
      "persistentVolumeClaim": {
        "claimName": "queue-api-data-snapshot"
      }
    }]
  }
}'
```

### Option 3: Direct Container Access (if supported)
Check if the read-only proxy supports other access methods:

```bash
# Try kubectl debug (may have different permissions)
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  debug -n commitgraph queue-api-c5894c469-p9rhr \
  --image=sqlite:sqlite3 --copy-to=extract-debug
```

## Acceptance Criteria Status

- [x] All four tables analyzed and disposition determined
- [x] `repo_head_cursors` - Documented as staying in queue-api PVC
- [x] `catalog_version` - Documented as staying in queue-api PVC  
- [x] `blocklist` - Migration scripts created (`migrations/load_blocklist.sql`)
- [x] `tombstones` - Migration scripts created (`migrations/00002_create_tombstones.sql`)
- [x] Extraction designed as read-only against queue-api's live SQLite
- [x] blocklist extraction cross-checked against repos.excluded_at mechanism
- [x] PVC retention documented in extraction plan
- [ ] **BLOCKER**: Actual extraction awaits admin kubeconfig refresh or workaround

## Migration Scripts Ready

When extraction is possible, use these scripts:

1. **Extract:** `scripts/extract-blocklist.sh` → `exports/blocklist-<timestamp>.csv`
2. **Extract:** `scripts/extract-tombstones.sh` → `exports/tombstones-<timestamp>.jsonl`
3. **Load:** `scripts/load-blocklist-to-postgres.sh` → Postgres `repos` table
4. **Load:** `scripts/load-tombstones-to-postgres.sh` → Postgres `tombstones` table

## Resolution Path

To unblock this task, one of the following is needed:

1. **Refresh admin kubeconfig:** Update `~/.kube/ord-devimprint-admin.kubeconfig` with valid credentials
2. **HTTP endpoint access:** Implement/admin HTTP export endpoints on queue-api
3. **Alternative access:** Find another method to read queue-api SQLite (PVC snapshot, etc.)
4. **Manual export:** Cluster operator manually exports the data and provides it

## Files Created (Reference)

- `scripts/extract-blocklist.sh` - Blocklist extraction script
- `scripts/extract-tombstones.sh` - Tombstones extraction script  
- `scripts/load-blocklist-to-postgres.sh` - Blocklist loading script
- `scripts/load-tombstones-to-postgres.sh` - Tombstones loading script
- `migrations/00002_create_tombstones.sql` - Tombstones table schema
- `migrations/load_blocklist.sql` - Blocklist migration SQL
- `notes/cg-5ol6-extraction-plan.md` - Full extraction plan
- `notes/cg-5ol6-summary.md` - Analysis summary
