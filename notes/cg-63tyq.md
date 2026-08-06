# Bead cg-63tyq: Complete kubectl exec dump command

## Constructed Command

```bash
kubectl exec -n commitgraph queue-api-c5894c469-p9rhr -c queue-api -- sqlite3 /data/queue.db ".dump email_resolution" > /tmp/email_resolution.dump
```

## Command Components

- **Pod name**: `queue-api-c5894c469-p9rhr` (current live pod from cg-60e81)
- **Namespace**: `commitgraph`
- **Container**: `queue-api`
- **Database path**: `/data/queue.db` (from cg-jvjw0)
- **Dump command**: `.dump email_resolution` (from cg-4q8rn)
- **Output destination**: `/tmp/email_resolution.dump` (pod filesystem)

## Ready for Execution

This command is fully constructed and ready to run. When executed, it will:
1. Connect to the queue-api pod
2. Run sqlite3 on the database at /data/queue.db
3. Dump the email_resolution table schema and data
4. Write the output to /tmp/email_resolution.dump on the pod

**Note**: The output is written to the pod's filesystem at `/tmp/email_resolution.dump`, not the local machine.
