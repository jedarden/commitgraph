# queue-api Pod Access Verification (cg-st350)

## Findings

### 1. Cluster Access
Successfully verified read access to ord-devimprint cluster via observer kubeconfig proxy:
- Proxy endpoint: `http://kubectl-proxy-ord-devimprint:8001`
- Access method: Observer ServiceAccount with read-only RBAC

### 2. Queue-API Pod Identity
- **Pod Name**: `queue-api-c5894c469-p9rhr`
- **Namespace**: `commitgraph`
- **Status**: Running (2/2 containers ready)
- **Image**: `ronaldraygun/commitgraph-queue-api:2.8.0`

### 3. SQLite Database Location
- **Database Path**: `/data/queue.db`
- **Storage**: Mounted from PVC `queue-api-data` (persistent volume claim)
- **Environment Variable**: `DB_PATH=/data/queue.db` (set in both init container and main queue-api container)
- **Replication**: Database file is replicated by litestream sidecar to B2

### 4. Container Architecture
The pod contains 2 containers:
1. `queue-api` - Main application container
2. `litestream` - Database replication sidecar (replicates `/data/queue.db` to B2)

## Next Steps for Extraction
To extract the `email_resolution` table, we can:
1. Use `kubectl exec` with admin kubeconfig to run `sqlite3 /data/queue.db`
2. Or copy the entire database file locally via `kubectl cp`
3. Query the `email_resolution` table schema and data

## Verification Status
✅ All acceptance criteria met:
- Successfully listed pods in queue-api namespace
- Identified exact queue-api pod name
- Located SQLite database file path
- Used only read-only kubectl operations (get/describe)
- Pod and database path recorded
