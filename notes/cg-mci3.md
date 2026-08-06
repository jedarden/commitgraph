# Refresh ord-devimprint-admin.kubeconfig

## Context
The `ord-devimprint-admin.kubeconfig` uses a `cloudspace-admin` OIDC token that expires every ~3 days. As of 2026-08-06, it needs to be regenerated from the Rackspace Spot UI.

## Current Status
- Read-only proxy works: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001` ✓
- Admin kubeconfig: Missing (needs creation)
- Cluster: ord-devimprint (Rackspace Spot cluster, accessible via Tailscale)

## Refresh Procedure
To regenerate the admin kubeconfig from the Rackspace Spot UI:

1. Log in to Rackspace Spot UI (cloudspace-admin access required)
2. Navigate to the ord-devimprint cloudspace
3. Generate a new OIDC token for cloudspace-admin group
4. Download/update the kubeconfig file
5. Save to `/home/coding/.kube/ord-devimprint-admin.kubeconfig`

## Access Levels
- **Observer** (`ord-devimprint-observer.kubeconfig`): Read-only RBAC (pods, events, deployments, PVCs, volumeattachments)
- **Admin** (`ord-devimprint-admin.kubeconfig`): Full cluster-admin access via cloudspace-admin OIDC token (~3 day expiry)

## What This Unblocks
- Phase 0 CNPG/operator installation tasks requiring cluster-admin access
- Phase 1 `email_resolution` extraction (365K+ pairs of rate-limited API budget)

## Last Refresh
- **Date:** 2026-08-06
- **Next refresh needed:** ~2026-08-09 (3 days from refresh)

## Verification Commands
```bash
# Test admin access
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get nodes

# Verify cluster-admin (can list CRDs)
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get crds
```
