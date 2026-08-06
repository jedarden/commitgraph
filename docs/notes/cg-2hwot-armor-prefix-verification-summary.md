# ARMOR_PREFIX Scoping Verification Summary (cg-2hwot)

**Date**: 2026-08-06  
**Status**: Configuration complete, runtime verification blocked by teardown

## What Was Accomplished

### 1. Commits Pushed ✅
Both commits successfully pushed to `jedarden/declarative-config`:

- **Commit 6dcc397b**: `feat(cg-62vzy): add ARMOR_PREFIX to clone-worker deployment`
  - Added `ARMOR_PREFIX: "commitgraph/"` to clone-worker-deployment.yml.disabled
  - Correctly scopes all commitgraph clone-worker objects under the commitgraph prefix

- **Commit 25941225**: `feat(aggregator): add ARMOR_PREFIX environment variable`
  - Added `ARMOR_PREFIX: "commitgraph/"` to aggregator-deployment.yaml.disabled
  - Ensures aggregator writes leaderboards and warm-start artifacts under the commitgraph prefix

### 2. Configuration Review ✅
Both manifests now correctly specify:
```yaml
env:
  - name: ARMOR_PREFIX
    value: "commitgraph/"
```

This configuration will:
- Scope Parquet artifacts under `commitgraph/` prefix
- Scope warm-start artifacts under `commitgraph/` prefix  
- Scope leaderboard snapshots under `commitgraph/` prefix
- Prevent collision with other ARMOR consumers using the same ARMOR instance

## What Cannot Be Verified (Blocked by Teardown)

### ArgoCD Sync Status
The commitgraph old pipeline was torn down on 2026-08-05 per TEARDOWN.md:
- clone-worker-deployment.yml.disabled (not managed by ArgoCD)
- aggregator-deployment.yaml.disabled (not managed by ArgoCD)

Since these are disabled, ArgoCD does not show them as applications, and sync status cannot be verified.

### Runtime Key Placement
Since the deployments are disabled and pods are not running:
- Cannot verify actual Parquet keys under `commitgraph/` prefix
- Cannot verify actual warm-start keys under `commitgraph/` prefix  
- Cannot verify no collision with existing objects

## Next Steps

When the v2 pipeline deploys these workloads:
1. Enable the manifests (remove `.disabled` extension)
2. ArgoCD will sync the applications
3. Verify keys land under `commitgraph/` prefix in B2
4. Confirm no collisions with other ARMOR consumers

## Technical Context

**ARMOR Instance**: `devimprint` namespace on `ord-devimprint` cluster  
**ARMOR_PREFIX**: `commitgraph/` (set via cg-1nx2)  
**Bucket**: `commitgraph-corpus` (shared B2 bucket)  
**Decision Reference**: cg-g7ce (umbrella bead for ARMOR scoping)

The ARMOR instance uses dedicated-bucket mode with `commitgraph/` prefix scoping, ensuring commitgraph objects are isolated from other consumers of the same ARMOR instance.
