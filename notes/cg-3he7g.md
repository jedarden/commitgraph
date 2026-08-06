# Transfer dump file from pod to local filesystem

## Task Completed

Successfully transferred the email_resolution dump file from the queue-api pod to the local filesystem on ex44.

## Execution Details

### Source Information
- **Cluster**: ord-devimprint
- **Namespace**: commitgraph  
- **Pod**: queue-api-c5894c469-p9rhr
- **Container**: queue-api (defaulted from queue-api, litestream, init-schema)

### File Details
- **Source path**: `/tmp/email_resolution_dump.sql`
- **Local path**: `/tmp/email_resolution_dump.sql`
- **File size**: 150M (156,655,153 bytes)
- **Transfer date**: 2026-08-06

### Command Executed
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig cp \
  commitgraph/queue-api-c5894c469-p9rhr:/tmp/email_resolution_dump.sql \
  /tmp/email_resolution_dump.sql
```

### Acceptance Criteria Met
- ✅ kubectl cp command executed successfully
- ✅ File exists in local filesystem at known path: `/tmp/email_resolution_dump.sql`
- ✅ File size is non-zero: 156,655,153 bytes (150M)
- ✅ File path recorded in bead comments

## Notes
The transfer was completed with no errors. The file is now available locally for further processing or analysis.
