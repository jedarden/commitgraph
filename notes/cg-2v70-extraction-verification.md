# email_resolution Extraction Verification

**Task:** cg-2v70  
**Date:** 2026-08-06  
**Extracted by:** NEEDLE worker

## Extraction Summary

Successfully extracted the full `email_resolution` table from queue-api's live SQLite database on ord-devimprint cluster.

### Source
- **Cluster:** ord-devimprint
- **Pod:** queue-api-c5894c469-p9rhr
- **Database:** `/data/queue.db` (SQLite)
- **Command:** `kubectl exec ... sqlite3 /data/queue.db ".dump email_resolution"`

### Dump Statistics
- **File:** `exports/email_resolution_fresh_20260806_161432.sql`
- **Size:** 150M
- **Row count:** 966,679 rows
- **Format:** SQLite SQL dump (CREATE TABLE + INSERT statements)

### Schema (11 columns)
| Column | Type | Notes |
|--------|------|-------|
| author_email | TEXT | PRIMARY KEY |
| github_login | TEXT | Resolved login; NULL = negative cache |
| provider | TEXT | Identity provider (github/gitlab/bitbucket) |
| status | TEXT | pending/claimed/resolved/unresolvable |
| priority | INTEGER | AI-tool commit count |
| is_alias_candidate | INTEGER | 1 = flagged for alias-map review |
| claimed_by | TEXT | Worker holding lease |
| claimed_at | TEXT | Lease claim timestamp |
| lease_expires_at | TEXT | Lease expiration |
| attempted_at | TEXT | Resolve attempt timestamp |
| created_at | TEXT | Record creation time |
| updated_at | TEXT | Record update time |

### Verification

✅ **Row count verified:** 966,679 rows match `SELECT COUNT(*) FROM email_resolution` run live against queue-api at extraction time

✅ **All columns captured:** All 11 columns from the schema are present in the dump

✅ **queue-api untouched:** No kubectl delete/patch/scale commands executed. The Service, Deployment, and PVC remain intact.

✅ **PVC status confirmed:** `queue-api-data` PVC is still `Bound` (verified at extraction time)

✅ **Durable storage:** Dump committed to git repo (pushed to Forgejo origin, mirrored to GitHub)

### PVC Status

**IMPORTANT:** The `declarative-config/k8s/ord-devimprint/commitgraph/queue-api-pvc.yml` manifest has NOT been removed and MUST NOT be removed until a downstream bead verifies the load into Postgres succeeded.

The `sata` StorageClass has `reclaimPolicy: Delete`, so pruning that PVC destroys the Cinder volume and every row in it.

### Next Steps

This dump must be loaded into the new Postgres database as part of the commitgraph pipeline migration. The PVC should remain in place until that load is verified successful.
