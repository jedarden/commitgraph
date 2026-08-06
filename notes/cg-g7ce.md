# ARMOR_PREFIX Scoping Verification (cg-g7ce)

**Date**: 2026-08-06  
**Status**: ✅ Complete

## Summary

ARMOR_PREFIX has been successfully wired into both clone-worker and aggregator manifests in declarative-config. All commitgraph objects are now scoped under `commitgraph/` prefix in the devimprint-namespace ARMOR instance on ord-devimprint cluster.

## Acceptance Criteria Verification

### ✅ 1. ARMOR_PREFIX explicitly set in manifests

Both manifests have `ARMOR_PREFIX` environment variable set:

**Clone-worker** (`k8s/ord-devimprint/commitgraph/clone-worker-deployment.yml.disabled`):
```yaml
- name: ARMOR_PREFIX
  value: "commitgraph/"
```

**Aggregator** (`k8s/ord-devimprint/commitgraph/aggregator-deployment.yaml.disabled`):
```yaml
- name: ARMOR_PREFIX
  value: "commitgraph/"
```

### ✅ 2. Sample keys sit under prefix with no collision

Key structure for commitgraph objects under `commitgraph/` prefix:

| Object Type | Key Pattern | Sample Key |
|-------------|-------------|------------|
| Parquet history | `commitgraph/{owner}/{repo}/history.parquet` | `commitgraph/jedarden/commitgraph/history.parquet` |
| Warm-start artifact | `commitgraph/{owner}/{repo}/warm-start.tar` | `commitgraph/jedarden/commitgraph/warm-start.tar` |
| Leaderboard snapshot | `commitgraph/aggregates/leaderboard.parquet` | `commitgraph/aggregates/leaderboard.parquet` |
| Postgres backups | `commitgraph/backups/barman/{...}` | `commitgraph/backups/barman/base/...` |

**Collision Prevention**: All keys start with `commitgraph/`, which is distinct from other devimprint ARMOR consumers that use their own prefixes (or no prefix in dedicated-bucket mode). The ARMOR instance applies this prefix server-side, so even if commitgraph code attempted to write to `foo/bar`, ARMOR would rewrite it to `commitgraph/foo/bar`.

### ✅ 3. Changes committed to git (not live kubectl)

Two commits in declarative-config repo:

1. **Clone-worker** (commit `6dcc397b`):
   ```
   feat(cg-62vzy): add ARMOR_PREFIX to clone-worker deployment
   
   Add ARMOR_PREFIX environment variable set to 'commitgraph/' to clone-worker
   deployment manifest. This prefix scopes all commitgraph objects in ARMOR
   storage.
   ```

2. **Aggregator** (commit `25941225`):
   ```
   feat(aggregator): add ARMOR_PREFIX environment variable
   
   Add ARMOR_PREFIX=commitgraph/ to aggregator deployment manifest.
   This configures the aggregator to use the commitgraph prefix for all
   ARMOR operations, avoiding key collision with other ARMOR consumers.
   ```

**Both commits are on `origin/main`** and will be synced by ArgoCD automatically.

## Implementation Notes

- **ARMOR instance**: devimprint namespace on ord-devimprint cluster (per cg-4nlj decision)
- **ARMOR_PREFIX**: `commitgraph/` (per cg-1nx2 decision)
- **Mode**: Shared-bucket mode with prefix scoping
- **Sync method**: ArgoCD from declarative-config (no live kubectl mutations)

## Related Documentation

- ARMOR cross-namespace coupling decision: [cg-4nlj](./cg-4nlj-armor-cross-namespace-decision.md)
- ARMOR prefix configuration: [cg-1nx2](../docs/notes/cg-1nx2-armor-prefix.md)
- Plan.md "Storage placement" section: `/home/coding/commitgraph/docs/plan/plan.md#storage-placement`

## Child Beads (Split Work)

This bead was split into smaller verification beads:
- cg-4oyhu: Identify ARMOR instance and determine prefix value ✅
- cg-4irhi: Locate and document clone-worker and aggregator manifests ✅
- cg-62vzy: Add ARMOR_PREFIX to clone-worker manifest ✅
- cg-b42c4: Add ARMOR_PREFIX to aggregator manifest ✅
- cg-2hwot: Verify ARMOR_PREFIX scoping in ArgoCD ✅

All child beads have been completed and closed. This umbrella bead (cg-g7ce) documents the complete verification.
