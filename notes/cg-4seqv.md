# queue-api email_resolution Schema Inspection

## Task Completion Note
**Cannot complete full task:** The admin kubeconfig for ord-devimprint (`/home/coding/.kube/ord-devimprint-admin.kubeconfig`) does not exist on this machine. The read-only proxy at `http://kubectl-proxy-ord-devimprint:8001` does not support `kubectl exec` operations required to run `PRAGMA` queries directly against the running database.

However, the **authoritative schema** is documented in the migration files and Go code, which are the source of truth for the database structure.

## email_resolution Table Schema

### Source
- Migration file: `/home/coding/commitgraph/migrations/00001_initial_schema.sql` (lines 31-38)
- Ingest implementation: `/home/coding/commitgraph/pkg/pg/identity.go`

### Schema Definition

```sql
CREATE TABLE IF NOT EXISTS email_resolution (
  email       TEXT PRIMARY KEY,
  login       TEXT NOT NULL,
  source      TEXT NOT NULL,          -- 'live' | 'seed' | 'manual'
  resolved_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS email_resolution_login_idx ON email_resolution (login);
```

### Column Details

| Column | Data Type | Constraints | Description |
|--------|-----------|-------------|-------------|
| `email` | TEXT | PRIMARY KEY, NOT NULL | Email address being resolved (unique identifier) |
| `login` | TEXT | NOT NULL | Resolved GitHub login |
| `source` | TEXT | NOT NULL | Resolution source: `'live'`, `'seed'`, or `'manual'` |
| `resolved_at` | TIMESTAMPTZ | NOT NULL | Timestamp of when this resolution was made |

### Indexes
- **Primary key:** `email` (clustered index)
- **Secondary index:** `email_resolution_login_idx` on `login` column

### Constraints
- `email` is the PRIMARY KEY (unique, NOT NULL)
- `login`, `source`, and `resolved_at` are all NOT NULL
- No foreign key constraints
- No default values

### Conflict Resolution Rule
From the ingest code (`pkg/pg/identity.go` lines 97-103):

```sql
ON CONFLICT (email) DO UPDATE
  SET login = excluded.login,
      source = excluded.source,
      resolved_at = excluded.resolved_at
  WHERE excluded.source = 'manual'
     OR (email_resolution.source <> 'manual'
         AND excluded.resolved_at > email_resolution.resolved_at)
```

This implements:
- **Manual source always wins** (overwrites any existing row)
- **Non-manual sources win only if:** existing row is also non-manual AND the new `resolved_at` is newer
- **Otherwise existing row is preserved**

## Row Count
**Not obtainable** without database access (kubectl exec). The admin kubeconfig needs to be regenerated from the Rackspace Spot UI (expires every ~3 days).

## Related Code
- Ingest endpoint: `/home/coding/commitgraph/pkg/identity/ingest.go`
- Seed command: `/home/coding/commitgraph/cmd/seed-email-resolution/main.go`
- Test data: `/home/coding/commitgraph/pkg/pg/identity_test.go`

## To Complete Original Task Requirements
To run `PRAGMA table_info(email_resolution);` and `SELECT COUNT(*) FROM email_resolution;` directly:
1. Regenerate admin kubeconfig from Rackspace Spot UI for ord-devimprint cluster
2. Save to `/home/coding/.kube/ord-devimprint-admin.kubeconfig`
3. Run:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
     exec -n commitgraph queue-api-<pod-name> -- \
     sqlite3 /path/to/database.db "PRAGMA table_info(email_resolution);"
   kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig \
     exec -n commitgraph queue-api-<pod-name> -- \
     sqlite3 /path/to/database.db "SELECT COUNT(*) FROM email_resolution;"
   ```

## Summary
✅ Schema fully documented from migration source of truth  
✅ All column names, types, and constraints identified  
✅ Conflict resolution rule documented  
❌ Row count not obtainable (requires admin kubeconfig)  
❌ Direct PRAGMA query not runnable (requires admin kubeconfig)
