# Task cg-4q8rn: Construct SQLite dump command for email_resolution table

## Completed: SQLite dump command construction

### Database Location (from cg-jvjw0)
- **Database path:** `/data/queue.db`
- **Pod namespace:** `commitgraph`
- **Cluster:** `ord-devimprint`
- **Table to dump:** `email_resolution`

### Recommended SQLite Dump Command

**Option 1: SQLite .dump format (portable SQL - RECOMMENDED)**
```bash
kubectl exec -n commitgraph <pod-name> -c queue-api -- sqlite3 /data/queue.db ".dump email_resolution"
```

**Option 2: With local file capture**
```bash
kubectl exec -n commitgraph <pod-name> -c queue-api -- sqlite3 /data/queue.db ".dump email_resolution" > email_resolution_dump.sql
```

**Option 3: CSV format (for data analysis tools)**
```bash
kubectl exec -n commitgraph <pod-name> -c queue-api -- sqlite3 /data/queue.db ".mode csv" ".headers on" ".output /tmp/email_resolution.csv" "SELECT * FROM email_resolution;"
```

### Why .dump format is preferred

The `.dump` format produces standard SQL output that:
- Includes full schema definition (CREATE TABLE with proper column types and constraints)
- Generates INSERT statements for all data
- Is portable across any SQLite database
- Handles special characters, quotes, and binary data correctly
- Can be restored with: `sqlite3 target.db < email_resolution_dump.sql`

### Next Steps

To execute the dump:
1. Get fresh pod name: `kubectl get pods -n commitgraph -l app=queue-api`
2. Substitute `<pod-name>` in the command above
3. Run the dump command and capture output
4. Restore to target database if needed

### Acceptance Criteria Met
✅ Command uses `sqlite3` with `.dump` mode  
✅ Command specifies the `email_resolution` table explicitly  
✅ Command output format is portable (SQLite .dump format)  
✅ Command includes proper path to database file (`/data/queue.db` from cg-jvjw0)  
✅ Command recorded in bead comments ready for kubectl exec  

### Status: COMPLETE
Command construction complete. Ready for execution in next step.
