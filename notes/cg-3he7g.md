# Task cg-3he7g: Transfer dump file from queue-api pod to local filesystem

## Status: BLOCKED - Cannot complete

## Findings

### Queue-api Pod Location
- **Cluster**: iad-options (Rackspace Spot)
- **Namespace**: options
- **Pod**: queue-api-6449cffd4d-tcnzz
- **Status**: Running (1/1)

### Blocker: Insufficient Access Permissions

The iad-options read-only proxy explicitly denies exec/cp access:

```
kubectl --server=http://traefik-iad-options:8001 exec -n options queue-api-6449cffd4d-tcnzz -- pwd
error: unable to upgrade connection: Forbidden
```

Similarly, `kubectl cp` fails:
```
kubectl --server=http://traefik-iad-options:8001 cp options/queue-api-6449cffd4d-tcnzz:/tmp/email_resolution.dump /tmp/email_resolution.dump
error: unable to upgrade connection: Forbidden
```

### Expected Access Path

Per CLAUDE.md documentation for iad-options:
- **Read-only proxy**: Explicitly denies access to secrets (stricter RBAC)
- **Admin kubeconfig**: `/home/coding/.kube/iad-options.kubeconfig` uses OIDC token (~3 day expiry)
- **Issue**: Admin kubeconfig does not exist on this system (expired, needs regeneration from Spot UI)

### Attempts Made

1. ✅ Located queue-api pod on iad-options cluster
2. ❌ Cannot exec into pod (Forbidden - read-only proxy)
3. ❌ Cannot kubectl cp (Forbidden - requires exec)
4. ❌ Admin kubeconfig does not exist at `/home/coding/.kube/iad-options.kubeconfig`
5. ❌ No PVCs in options namespace that might contain the dump file
6. ❌ No dump file references found in recent pod logs

## Resolution Required

This task requires one of the following:

1. **Regenerate iad-options admin kubeconfig** from Spot UI (expires every ~3 days)
2. **Alternative access method** if the dump file was stored elsewhere (PVC, S3, etc.)
3. **Manual file transfer** by someone with admin access to iad-options cluster

## Next Steps for Human Operator

The admin kubeconfig can be regenerated from the Rackspace Spot UI:
- Navigate to iad-options cloudspace
- Generate new cloudspace-admin OIDC token
- Update `/home/coding/.kube/iad-options.kubeconfig` with new token
- Retry kubectl cp command

Once access is restored, the command would be:
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-options.kubeconfig cp \
  options/queue-api-6449cffd4d-tcnzz:<path-to-dump-file> \
  /tmp/email_resolution.dump
```

## Metadata

- Task ID: cg-3he7g
- Date attempted: 2026-08-06
- Blocker type: Access permissions (read-only proxy RBAC)
