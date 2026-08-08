# admin-alias-configmap-watcher

Watches the `admin-alias-map` ConfigMap for changes and automatically re-runs the admin alias loader whenever the ConfigMap is modified.

## Purpose

Hand-curated aliases are stored in `declarative-config/k8s/ord-devimprint/commitgraph/admin-alias-configmap.yml` as a GitOps-managed ConfigMap. Operators edit this file to merge duplicate accounts, correct mis-detected aliases, or curate canonical identity mappings.

This watcher ensures those changes are reflected in the `user_aliases` table without requiring manual database operations.

## How it works

1. **On startup**: Runs the loader once to perform initial sync
2. **Watches**: Monitors the mounted ConfigMap file for modifications (poll interval: 5s)
3. **Debounces**: Waits 5 seconds after the last change to prevent duplicate runs
4. **Re-runs**: Executes the loader to apply the new mappings
5. **Continues**: Keeps watching even after errors

## Time bound

Changes are reflected in `user_aliases` within:
- **Worst case**: poll-interval (5s) + debounce (5s) + load time (~1-2s) = **~11-12 seconds**
- **Average case**: Much faster (poll hits change mid-cycle)

## Deployment

Runs as a Kubernetes Deployment with:
- The ConfigMap mounted as a volume at `/etc/config/aliases.yml`
- Postgres credentials via the `commitgraph-app` Secret
- Resource limits appropriate for polling (50m CPU / 64Mi memory requests)
- Liveness and readiness probes

## Idempotency

The loader is idempotent: it uses `ON CONFLICT (source_login) DO UPDATE`, so rapid re-runs are safe. Debouncing avoids unnecessary work but isn't required for correctness.

## Container contents

This container includes two binaries:
- `watch-admin-alias-configmap` — The long-running watcher process
- `load-admin-aliases` — The loader that the watcher executes on changes

Both are built from the `commitgraph` repository:
- `cmd/watch-admin-alias-configmap/main.go`
- `cmd/load-admin-aliases/main.go`

## See also

- Deployment manifest: `k8s/admin-alias-configmap-watcher-deployment.yaml`
- Loader code: `cmd/load-admin-aliases/main.go`
- Watcher code: `cmd/watch-admin-alias-configmap/main.go`
- Database schema: `migrations/00001_initial_schema.sql` (user_aliases table)
