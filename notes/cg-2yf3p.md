# email_resolution Table Dump Execution

## Task Completed
Successfully executed SQLite dump of email_resolution table via kubectl exec.

## Details

### Pod Information
- **Cluster:** ord-devimprint
- **Namespace:** commitgraph
- **Pod:** queue-api-c5894c469-p9rhr
- **Container:** queue-api
- **Database:** /data/queue.db

### Exact Command Executed
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig exec -n commitgraph queue-api-c5894c469-p9rhr -c queue-api -- sh -c 'sqlite3 /data/queue.db ".dump email_resolution" > /tmp/email_resolution_dump.sql'
```

### Results
- **Dump file:** /tmp/email_resolution_dump.sql (on pod filesystem)
- **File size:** 149.4 MB
- **Records:** 966,679 INSERT statements
- **Format:** SQLite .dump format (portable SQL with CREATE TABLE and INSERT statements)

### Schema Columns Captured
All columns from the email_resolution table are included in the dump:
- author_email (TEXT, PRIMARY KEY)
- github_login (TEXT)
- provider (TEXT, NOT NULL DEFAULT 'github')
- status (TEXT, NOT NULL DEFAULT 'pending')
- priority (INTEGER, NOT NULL DEFAULT 0)
- is_alias_candidate (INTEGER, NOT NULL DEFAULT 0)
- claimed_by (TEXT)
- claimed_at (TEXT)
- lease_expires_at (TEXT)
- attempted_at (TEXT)
- created_at (TEXT, NOT NULL DEFAULT datetime('now'))
- updated_at (TEXT, NOT NULL DEFAULT datetime('now'))

### Verification
- ✅ Dump executed without errors
- ✅ Output file created on pod filesystem (149.4 MB)
- ✅ All schema columns preserved
- ✅ No mutating kubectl operations used
- ✅ Format is portable (can be restored with: `sqlite3 target.db < dump.sql`)

### Next Steps
The dump file is ready on the pod at `/tmp/email_resolution_dump.sql` for the next step (copy to local filesystem).

Executed: 2026-08-06
Bead: cg-2yf3p
