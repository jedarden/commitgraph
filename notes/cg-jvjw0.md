# queue-api SQLite Database Location

## Task cg-jvjw0: Locate SQLite database file path in queue-api pod

### Database Location
- **File path:** `/data/queue.db`
- **Pod:** `queue-api-c5894c469-p9rhr`
- **Namespace:** `commitgraph`
- **Cluster:** `ord-devimprint` (accessed via admin kubeconfig)

### Verification Details

#### 1. Path existence verified
```bash
kubectl exec -n commitgraph queue-api-c5894c469-p9rhr -- ls -la /data/
```

Output shows:
- `queue.db` - 810MB (main database file)
- `queue.db-wal` - 24MB (Write-Ahead Log)
- `queue.db-shm` - 64KB (shared memory file for WAL)
- `queue.db.backup-20260718-132909` - backup from July 18
- `email_resolution_dump.sql` - 156MB SQL dump

#### 2. SQLite format confirmed
```bash
kubectl exec -n commitgraph queue-api-c5894c469-p9rhr -- sqlite3 /data/queue.db ".tables"
```

Database contains the following tables:
- `_litestream_lock` - Litestream replication lock
- `_litestream_seq` - Litestream sequence tracking
- `audit_log` - Audit logging
- `author_login_cache` - Author login cache
- `blocklist` - Blocked items
- `catalog_version` - Schema version tracking
- `dirty_partitions` - Dirty partition tracking
- `email_resolution` - Email resolution data
- `onboard_progress` - Onboarding progress
- `rate_limit_events` - Rate limiting
- `repo_head_cursors` - Repository head cursors
- `repo_queue` - Repository queue
- `schema_meta` - Schema metadata
- `search_queue` - Search queue
- `stats` - Statistics
- `tombstones` - Deleted records
- `user_aliases` - User aliases
- `user_enrichment` - User enrichment data
- `user_queue` - User queue
- `username_revalidation` - Username revalidation

### Database Configuration

According to the deployment configuration (`/home/coding/declarative-config/k8s/ord-devimprint/commitgraph/queue-api-deployment.yml`):
- **DB_PATH environment variable:** `/data/queue.db`
- **PVC:** `queue-api-data` (10GB SATA storage)
- **Backup:** Litestream replicates WAL to Backblaze B2 `commitgraph-ops` bucket
- **Backup path in B2:** `queue-api/queue.db`

### Pod Details
- **Deployment:** `queue-api` in `commitgraph` namespace
- **Image:** `ronaldraygun/commitgraph-queue-api:2.8.0`
- **Containers:** 2/2 running (queue-api main + litestream sidecar)
- **Storage:** Mounted at `/data` from PVC `queue-api-data`

## Access Notes
- **Read-only access:** Available via kubectl-proxy (http://kubectl-proxy-ord-devimprint:8001)
- **Admin access:** Requires `/home/coding/.kube/ord-devimprint-admin.kubeconfig`
- **Observer access:** Not configured for this cluster
