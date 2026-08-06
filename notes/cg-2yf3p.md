# cg-2yf3p: Execute email_resolution table dump via kubectl exec

## Task Completed
Successfully dumped the email_resolution table from the queue-api SQLite database on ord-devimprint.

## Target Pod
- **Cluster:** ord-devimprint
- **Namespace:** commitgraph
- **Pod:** queue-api-c5894c469-p9rhr
- **Container:** queue-api

## Database Location
- **File:** `/data/queue.db` (primary database file, ~810MB)
- **Format:** SQLite with WAL enabled (queue.db-wal ~10MB, queue.db-shm 32KB)

## Commands Used

### 1. Schema inspection
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig exec -n commitgraph queue-api-c5894c469-p9rhr -c queue-api -- sqlite3 /data/queue.db ".schema email_resolution"
```

### 2. Table dump (read-only operation)
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig exec -n commitgraph queue-api-c5894c469-p9rhr -c queue-api -- sh -c 'sqlite3 /data/queue.db ".dump email_resolution" > /tmp/email_resolution_dump.sql'
```

### 3. Verification
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig exec -n commitgraph queue-api-c5894c469-p9rhr -c queue-api -- sh -c 'sqlite3 /data/queue.db "SELECT COUNT(*) FROM email_resolution;"'
```

## Results
- **Total records:** 966,679 email resolution entries
- **Dump file size:** 156,655,153 bytes (~150MB) at `/tmp/email_resolution_dump.sql` on pod
- **Dump format:** SQL INSERT statements with CREATE TABLE schema (portable SQLite format)
- **Operation:** Read-only filesystem operation (no mutating kubectl commands)

## Schema Columns Dumped
The dump captures all 12 columns from the email_resolution table:
1. author_email (TEXT PRIMARY KEY)
2. github_login (TEXT)
3. provider (TEXT, NOT NULL DEFAULT 'github')
4. status (TEXT, NOT NULL DEFAULT 'pending', CHECK constraint)
5. priority (INTEGER, NOT NULL DEFAULT 0)
6. is_alias_candidate (INTEGER, NOT NULL DEFAULT 0)
7. claimed_by (TEXT)
8. claimed_at (TEXT)
9. lease_expires_at (TEXT)
10. attempted_at (TEXT)
11. created_at (TEXT, NOT NULL DEFAULT datetime('now'))
12. updated_at (TEXT, NOT NULL DEFAULT datetime('now'))

## Next Step
The dump file is ready at `/tmp/email_resolution_dump.sql` on the pod for copy to local filesystem in the next bead (cg-13m18).

## Access Notes
- Read-only kubectl proxy at `http://kubectl-proxy-ord-devimprint:8001` cannot exec (Forbidden error)
- Admin kubeconfig at `/home/coding/.kube/ord-devimprint-admin.kubeconfig` required for exec operations
- OIDC token expires ~3 days; regenerate from Spot UI if needed
