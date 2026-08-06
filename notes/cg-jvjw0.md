# SQLite Database Location Discovery (cg-jvjw0)

## Task
Locate SQLite database file path in queue-api pod

## Findings

### Database Location
**Path:** `/data/queue.db`

**Pod:** `queue-api-c5894c469-p9rhr` (commitgraph namespace, ord-devimprint cluster)

### Verification
- ✅ File exists: 810,295,296 bytes (~810 MB)
- ✅ File format: SQLite 3 (confirmed via file header "SQLite format 3")
- ✅ File permissions: `-rw-r--r--` (owner: queueapi:queueapi)
- ✅ Mounted from PVC: `queue-api-data`

### Configuration Source
Path confirmed from deployment manifest:
- File: `declarative-config/k8s/ord-devimprint/commitgraph/queue-api-deployment.yml`
- Environment variable: `DB_PATH=/data/queue.db`
- Litestream config: `path: /data/queue.db`

### Verification Commands Used (Read-only)
```bash
# Get pod name
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get pods -n commitgraph -l app=queue-api

# Verify file exists
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig exec -n commitgraph queue-api-c5894c469-p9rhr -- ls -la /data/queue.db

# Confirm SQLite format
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig exec -n commitgraph queue-api-c5894c469-p9rhr -- head -c 16 /data/queue.db
```

## Notes
The database is backed up to B2 via Litestream sidecar to `commitgraph-ops/queue-api/queue.db` (private bucket).

Discovery completed 2026-08-06.
