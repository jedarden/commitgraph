# Bead cg-30z4i: Execute SQLite dump via kubectl exec

## Executed Command

The original command from cg-63tyq had the redirect outside kubectl exec, which would write to the local filesystem. To write to the **pod filesystem** as required by the acceptance criteria, the redirect needed to be inside the exec:

```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig exec -n commitgraph queue-api-c5894c469-p9rhr -c queue-api -- sh -c 'sqlite3 /data/queue.db ".dump email_resolution" > /tmp/email_resolution.dump'
```

## Results

- **Exit code**: 0 (success)
- **Output location**: `/tmp/email_resolution.dump` in pod filesystem
- **File size**: 149.4MB
- **Content verification**: Valid SQLite dump with:
  - PRAGMA foreign_keys=OFF;
  - BEGIN TRANSACTION;
  - CREATE TABLE email_resolution schema
  - INSERT statements with data

## Cluster

- **Cluster**: ord-devimprint
- **Namespace**: commitgraph
- **Pod**: queue-api-c5894c469-p9rhr
- **Container**: queue-api
- **Database path**: /data/queue.db

## stderr Output

None - command completed cleanly with no errors.
