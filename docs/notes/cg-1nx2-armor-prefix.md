# ARMOR Instance and Prefix Configuration (cg-1nx2)

**Date**: 2026-08-06
**Status**: Implemented

## Decision

**ARMOR instance**: `devimprint` namespace on `ord-devimprint` cluster
**ARMOR_PREFIX**: `commitgraph/`

## Rationale

The ARMOR cross-namespace coupling decision (cg-4nlj) established that commitgraph reuses the existing `devimprint`-namespace ARMOR instance on the `ord-devimprint` cluster rather than standing up a new `commitgraph`-scoped deployment.

To avoid key collision with other consumers of the same ARMOR instance, all commitgraph objects are scoped under the explicit `commitgraph/` prefix.

## ARMOR Instance Details

- **Cluster**: ord-devimprint
- **Namespace**: devimprint
- **Bucket**: `devimprint` (dedicated B2 bucket)
- **Prefix**: `commitgraph/` (NEW - set via ConfigMap)
- **Mode**: Shared-bucket mode (with prefix scoping)

## Commitgraph Objects Stored

All of the following commitgraph objects are stored under ARMOR with keys prefixed by `commitgraph/`:

1. **Raw per-repo commit-history artifact** (Parquet)
   - Format: `commitgraph/{repo_owner}/{repo_name}/history.parquet`
   - Content: sha/author/email/day/message columns
   - Access pattern: Write on clone/rescan, read only for rare catalog-triggered redetect jobs

2. **Warm-start artifact** (tar'd git pack + refs + config)
   - Format: `commitgraph/{repo_owner}/{repo_name}/warm-start.tar`
   - Content: Raw pack files + loose ref + three promisor config values
   - Access pattern: Write on clone, read on subsequent clones of same repo
   - Note: Not yet validated at corpus's actual large-repo scale

3. **Leaderboard snapshot**
   - Format: `commitgraph/aggregates/leaderboard.parquet`
   - Content: Full ranked list (hundreds of thousands of rows)
   - Access pattern: Periodic publish from aggregator

4. **Postgres barmanObjectStore backups**
   - Format: `commitgraph/backups/barman/{...}` (exact structure TBD by barman)
   - Content: Postgres backup objects
   - Access pattern: Scheduled backups, restores

## Implementation

**ConfigMap**: `k8s/ord-devimprint/devimprint/armor-configmap.yml`
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: armor-config
  namespace: devimprint
data:
  ARMOR_PREFIX: "commitgraph/"
```

**Deployment update**: `k8s/ord-devimprint/devimprint/armor-deployment.yml`
- Changed `ARMOR_PREFIX` from ExternalSecret (`prefix` key, optional) to ConfigMap (`armor-config`, `ARMOR_PREFIX` key, required)
- This ensures commitgraph objects are always scoped under the prefix

**Sync**: Applied via declarative-config commit `58dfb860` on `main` branch
- ArgoCD app `ord-devimprint` will sync automatically on next reconciliation

## Usage in Phase 1 Workloads

Clone-worker and aggregator will connect to ARMOR at:
- **Service**: `http://armor.devimprint.svc.cluster.local:9000`
- **Credentials**: Read from ARMOR's readonly auth (via ExternalSecret when Phase 1 workloads are deployed)
- **Prefix**: Handled automatically by ARMOR (no client-side prefix concatenation needed)

All S3 PUT/GET operations to ARMOR will automatically be scoped under `commitgraph/` by the ARMOR proxy itself.

## Verification

After ArgoCD sync completes, verify:

```bash
# Check ConfigMap exists
kubectl --server=http://traefik-ord-devimprint:8001 get configmap armor-config -n devimprint -o jsonpath='{.data.ARMOR_PREFIX}'

# Check deployment has correct env var
kubectl --server=http://traefik-ord-devimprint:8001 get deployment armor -n devimprint -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ARMOR_PREFIX")].valueFrom.configMapKeyRef.key}'

# Check ARMOR pods have the env var set (after rollout)
kubectl --server=http://traefik-ord-devimprint:8001 get pods -n devimprint -l app=armor -o jsonpath='{.items[0].spec.containers[0].env[?(@.name=="ARMOR_PREFIX")].valueFrom}'
```

Expected results:
- ConfigMap: `commitgraph/`
- Deployment env var key: `ARMOR_PREFIX`
- Pods: ConfigMap reference (not Secret reference)

## References

- ARMOR cross-namespace coupling decision (cg-4nlj): `/home/coding/commitgraph/docs/notes/cg-4nlj-armor-cross-namespace-decision.md`
- Plan.md "Storage placement" section: `/home/coding/commitgraph/docs/plan/plan.md#storage-placement`
- Plan.md "Open decisions": `/home/coding/commitgraph/docs/plan/plan.md#open-decisions` (line 184-188)
- Declarative-config commit: `jedarden/declarative-config@58dfb860`
