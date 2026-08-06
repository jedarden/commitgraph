# ARMOR Instance and Prefix Identification (cg-4oyhu)

**Date**: 2026-08-06
**Status**: Complete

## Task Summary

Identified which ARMOR instance commitgraph will use and determined the appropriate `ARMOR_PREFIX` value to avoid key collisions.

## Decisions

### ARMOR Instance
**Choice**: `devimprint` namespace ARMOR on `ord-devimprint` cluster

**Rationale**:
- Reuses existing, battle-tested infrastructure rather than provisioning a new deployment
- Cross-namespace coupling is acceptable (same cluster, not cross-cluster)
- SPOF concern is materially narrower than ADR-009's original worry:
  - ARMOR is not in the hot ranking-query path (queries go to Postgres)
  - ARMOR unavailability delays extraction/publishing but doesn't take down ranking
  - Clear unidirectional dependency pattern (commitgraph → ARMOR, not vice versa)
- Cold access pattern: ARMOR is used for artifact storage, not hot-path queries

### ARMOR_PREFIX
**Value**: `commitgraph/`

**Rationale**:
- Scopes all commitgraph objects under a single prefix to avoid key collision
- Multiple consumers can safely share the same ARMOR instance without conflict
- Prefix is applied server-side by ARMOR proxy (no client-side concatenation needed)

## Implementation Details

All commitgraph objects are stored under ARMOR with keys prefixed by `commitgraph/`:

1. **Raw per-repo commit-history artifact** (Parquet)
   - Format: `commitgraph/{repo_owner}/{repo_name}/history.parquet`

2. **Warm-start artifact** (tar'd git pack + refs + config)
   - Format: `commitgraph/{repo_owner}/{repo_name}/warm-start.tar`

3. **Leaderboard snapshot**
   - Format: `commitgraph/aggregates/leaderboard.parquet`

4. **Postgres barmanObjectStore backups**
   - Format: `commitgraph/backups/barman/{...}` (exact structure TBD by barman)

## Status

**Already implemented** via declarative-config commit `58dfb860`:
- ConfigMap: `k8s/ord-devimprint/devimprint/armor-configmap.yml`
- Deployment: `k8s/ord-devimprint/devimprint/armor-deployment.yml`
- ArgoCD app `ord-devimprint` will sync automatically

## Verification

After ArgoCD sync completes, verify:

```bash
# Check ConfigMap exists
kubectl --server=http://traefik-ord-devimprint:8001 get configmap armor-config -n devimprint -o jsonpath='{.data.ARMOR_PREFIX}'

# Check deployment has correct env var
kubectl --server=http://traefik-ord-devimprint:8001 get deployment armor -n devimprint -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ARMOR_PREFIX")].valueFrom.configMapKeyRef.key}'
```

Expected results:
- ConfigMap: `commitgraph/`
- Deployment env var key: `ARMOR_PREFIX`

## References

- ARMOR cross-namespace coupling decision: [cg-4nlj](/home/coding/commitgraph/docs/notes/cg-4nlj-armor-cross-namespace-decision.md)
- ARMOR prefix configuration: [cg-1nx2](/home/coding/commitgraph/docs/notes/cg-1nx2-armor-prefix.md)
- Plan.md "Open decisions": `/home/coding/commitgraph/docs/plan/plan.md#open-decisions`
- Plan.md "Storage placement": `/home/coding/commitgraph/docs/plan/plan.md#storage-placement`
