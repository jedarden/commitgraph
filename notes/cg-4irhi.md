# cg-4irhi: Clone-Worker and Aggregator Manifest Documentation

## Overview

This document locates and documents the clone-worker and aggregator Deployment manifests in the declarative-config repository, including their current ARMOR-related environment variable configuration.

## Repository Paths

### Primary Repository
- **Repository**: `jedarden/declarative-config`
- **Target Branch**: `HEAD`
- **ArgoCD Application**: `commitgraph-ns-ord-devimprint`
- **Manifest Directory**: `k8s/ord-devimprint/commitgraph/`

### Manifest Files

| Component | Manifest Path | Status |
|-----------|--------------|--------|
| **aggregator** | `/home/coding/declarative-config/k8s/ord-devimprint/commitgraph/aggregator-deployment.yaml.disabled` | Disabled (`.disabled` extension) |
| **clone-worker** | `/home/coding/declarative-config/k8s/ord-devimprint/commitgraph/clone-worker-deployment.yml.disabled` | Disabled (`.disabled` extension) |
| **clone-worker-large** | `/home/coding/declarative-config/k8s/ord-devimprint/commitgraph/clone-worker-large-deployment.yml.disabled` | Disabled (`.disabled` extension) |
| **clone-worker-parallel** | `/home/coding/declarative-config/k8s/ord-devimprint/commitgraph/clone-worker-parallel-deployment.yml.disabled` | Disabled (`.disabled` extension) |

## ArgoCD Management Status

**CONFIRMED**: These manifests are managed by ArgoCD (not orphaned).

- **ArgoCD Application**: `commitgraph-ns-ord-devimprint`
- **Application Manifest**: `/home/coding/declarative-config/k8s/ord-devimprint/commitgraph-application.yml`
- **Sync Policy**: Automated with `prune: true` and `selfHeal: true`
- **Source Path**: `k8s/ord-devimprint/commitgraph`
- **Sync Pattern**: `{*.yaml,*.yml}` (includes all YAML files, but `.disabled` files are ignored by the pattern match)

**NOTE**: All four manifests are currently **disabled** (`.disabled` extension). ArgoCD's sync pattern (`{*.yaml,*.yml}`) will not match files with `.disabled` extension, so these deployments are **not actively deployed**.

## Current ARMOR-Related Environment Variables

### aggregator-deployment.yaml.disabled

**Current ARMOR-related env vars**: **NONE**

The aggregator manifest does not contain any ARMOR-related environment variables. It uses direct B2 configuration:

```yaml
# B2 Configuration (Direct - NOT ARMOR)
- name: B2_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: commitgraph-b2-workers
      key: key-id
- name: B2_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: commitgraph-b2-workers
      key: application-key
- name: B2_BUCKET
  value: "commitgraph-corpus"
- name: B2_ENDPOINT
  value: "https://s3.us-west-002.backblazeb2.com"
```

**Image**: `ronaldraygun/commitgraph-aggregator:1.9.0`

### clone-worker-deployment.yml.disabled

**Current ARMOR-related env vars**: **NONE**

The clone-worker manifest explicitly documents ARMOR retirement in comments (lines 69-78):

```yaml
# Direct B2 staging (ADR-009 retires ARMOR). The previous config
# pointed at ARMOR + bucket "commitgraph-workers", which never
# existed anywhere (ARMOR's B2 key is scoped to `devimprint` only),
# so every staging upload was doomed. Staging Parquet belongs in
# the PRIVATE commitgraph-ops bucket (see
# scripts/provision_b2_ops_bucket.py in jedarden/commitgraph-deprecated,
# renamed 2026-08-04),
# which exists and is reachable by the commitgraph-b2-workers key
# (verified 2026-07-20 via HeadBucket).
```

Current B2/S3 configuration:

```yaml
- name: S3_ENDPOINT
  value: "https://s3.us-west-002.backblazeb2.com"
- name: S3_BUCKET
  value: "commitgraph-ops"
- name: S3_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: commitgraph-b2-workers
      key: key-id
- name: S3_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: commitgraph-b2-workers
      key: application-key
```

**Image**: `ronaldraygun/commitgraph-clone-worker:1.3.0`

### clone-worker-large-deployment.yml.disabled

**Current ARMOR-related env vars**: **NONE**

Similar to clone-worker, this variant uses direct B2/S3 configuration with identical comments referencing ADR-009 ARMOR retirement:

```yaml
# Direct B2 staging (ADR-009 retires ARMOR). Staging Parquet belongs in
# the PRIVATE commitgraph-ops bucket under staging/ prefix.
# Uses clone-worker-specific staging credentials with proper scoping.
```

**Image**: `ronaldraygun/commitgraph-clone-worker:1.3.0`

### clone-worker-parallel-deployment.yml.disabled

**Current ARMOR-related env vars**: **NONE**

Same B2/S3 configuration as other clone-worker variants with identical ADR-009 comments.

**Image**: `ronaldraygun/commitgraph-clone-worker:1.3.0`

## Summary

- All four manifests are **currently disabled** (`.disabled` extension) and **not actively deployed** by ArgoCD
- **NO ARMOR-related environment variables** exist in any of the four manifests
- All manifests use **direct B2/S3 configuration** via `commitgraph-b2-workers` secret
- ARMOR was explicitly retired per **ADR-009** (documented in clone-worker manifest comments)
- Aggregator uses `B2_*` environment variables, while clone-worker variants use `S3_*` environment variables (both point to Backblaze B2)

## Active Commitgraph Deployments

The following deployments are currently **active** (no `.disabled` extension):

1. **queue-api** - `queue-api-deployment.yml`
2. **login-revalidation-worker** - `login-revalidation-worker-deployment.yml`

The clone-worker and aggregator deployments will need to be re-enabled (remove `.disabled` extension) before they can be deployed and managed by ArgoCD.
