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

## Blocking Issue
This task requires **interactive web access to the Rackspace Spot UI** to download a fresh kubeconfig with a new OIDC token. Several approaches were attempted:

1. **ADB phone access** - Failed (connection timeout, device likely disconnected)
2. **Web reader to Spot UI** - Failed (network errors on login page)
3. **API/CLI access** - No spotctl CLI installed, no Spot API token available

## What's Needed
To complete this task, one of the following is required:

1. **User provides the fresh kubeconfig content** from the Spot UI
2. **User provides Rackspace Spot API token** so I can extract via Terraform
3. **Alternative web access method** to reach the Spot console

## Manual Refresh Instructions
If the user can access the Spot UI:

1. Log in to https://spot.rackspace.com/
2. Navigate to the ord-devimprint cloudspace
3. Find the kubeconfig download option (usually under "Access" or "Credentials")
4. Download the kubeconfig and provide the content

## Last Refresh Attempt
- **Date:** 2026-08-06
- **Status:** BLOCKED - needs web UI access
- **Next refresh needed:** ~2026-08-09 (3 days from refresh)

## Verification Commands
```bash
# Test admin access
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get nodes

# Verify cluster-admin (can list CRDs)
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get crds
```
