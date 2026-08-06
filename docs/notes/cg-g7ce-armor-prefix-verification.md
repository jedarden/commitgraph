# ARMOR_PREFIX Scoping Verification (cg-g7ce)

**Date**: 2026-08-06
**Status**: Verified

## ARMOR Configuration Verified

✅ **ARMOR_PREFIX ConfigMap**: `commitgraph/`
```bash
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get configmap armor-config -n devimprint \
  -o jsonpath='{.data.ARMOR_PREFIX}'
commitgraph/
```

✅ **ARMOR Deployment**: Uses ConfigMap for ARMOR_PREFIX
```bash
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get deployment armor -n devimprint \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ARMOR_PREFIX")].valueFrom.configMapKeyRef.name}'
armor-config
```

## Key Structure Under commitgraph/ Prefix

All commitgraph objects are stored under ARMOR with keys prefixed by `commitgraph/`:

### 1. Raw per-repo commit-history artifact (Parquet)
- **Format**: `commitgraph/{repo_owner}/{repo_name}/history.parquet`
- **Content**: sha/author/email/day/message columns
- **Access pattern**: Write on clone/rescan, read for rare catalog-triggered redetect jobs
- **Example**: `commitgraph/jedarden/commitgraph/history.parquet`

### 2. Warm-start artifact (tar'd git pack + refs + config)
- **Format**: `commitgraph/{repo_owner}/{repo_name}/warm-start.tar`
- **Content**: Raw pack files + loose ref + three promisor config values
- **Access pattern**: Write on clone, read on subsequent clones of same repo
- **Example**: `commitgraph/jedarden/commitgraph/warm-start.tar`

### 3. Leaderboard snapshot
- **Format**: `commitgraph/aggregates/leaderboard.parquet`
- **Content**: Full ranked list (hundreds of thousands of rows)
- **Access pattern**: Periodic publish from aggregator

### 4. Postgres barmanObjectStore backups
- **Format**: `commitgraph/backups/barman/{...}`
- **Content**: Postgres backup objects
- **Access pattern**: Scheduled backups, restores

## No Overlap Verification

**Sample repo keys for jedarden/commitgraph**:
- Parquet: `commitgraph/jedarden/commitgraph/history.parquet`
- Warm-start: `commitgraph/jedarden/commitgraph/warm-start.tar`
- Leaderboard: `commitgraph/aggregates/leaderboard.parquet`

These keys are:
- ✅ All under the `commitgraph/` prefix
- ✅ No collision between Parquet and warm-start keys (different suffixes)
- ✅ No collision with other repos (repo_owner/repo_name prefix)
- ✅ No collision with aggregates or backups (different path components)

**Verification of prefix isolation**:
- Any object without the `commitgraph/` prefix is outside commitgraph's scope
- ARMOR instance handles the prefix automatically (no client-side concatenation needed)
- All S3 PUT/GET operations by clone-worker and aggregator are automatically scoped under `commitgraph/` by the ARMOR proxy

## Implementation Changes

### 1. ExternalSecret Created
**File**: `k8s/ord-devimprint/commitgraph/commitgraph-externalsecrets.yml`
- Added `armor-writer` ExternalSecret to import ARMOR writer credentials
- Sources from `rs-manager/ord-devimprint/armor-writer`
- Provides `auth-access-key` and `auth-secret-key` for S3 authentication

### 2. Clone-Worker Deployment Updated
**File**: `k8s/ord-devimprint/commitgraph/clone-worker-deployment.yml.disabled`
- Changed from direct B2 access to ARMOR access
- Endpoint: `http://armor.devimprint.svc.cluster.local:9000`
- Credentials: `armor-writer` secret (auth-access-key, auth-secret-key)
- Removed: `AWS_DEFAULT_REGION`, `S3_BUCKET` (ARMOR handles these)

### 3. Aggregator Deployment Updated
**File**: `k8s/ord-devimprint/commitgraph/aggregator-deployment.yaml.disabled`
- Changed from direct B2 access to ARMOR access
- Endpoint: `http://armor.devimprint.svc.cluster.local:9000`
- Credentials: `armor-writer` secret (auth-access-key, auth-secret-key)
- Removed: `B2_BUCKET` (ARMOR handles this)

## References

- cg-1nx2 ARMOR instance and prefix configuration: `/home/coding/commitgraph/docs/notes/cg-1nx2-armor-prefix.md`
- cg-4nlj ARMOR cross-namespace coupling decision: `/home/coding/commitgraph/docs/notes/cg-4nlj-armor-cross-namespace-decision.md`
- ARMOR ConfigMap: `/home/coding/declarative-config/k8s/ord-devimprint/devimprint/armor-configmap.yml`
- ARMOR Deployment: `/home/coding/declarative-config/k8s/ord-devimprint/devimprint/armor-deployment.yml`
