# ord-devimprint-admin Kubeconfig Refresh

## Date: 2026-08-06

## Issue
The `ord-devimprint-admin.kubeconfig` needs to be regenerated. The `cloudspace-admin` OIDC token for the ord-devimprint Rackspace Spot cluster expires approximately every 3 days.

## Procedure to Refresh OIDC Token

### Step 1: Access Rackspace Spot UI
1. Log in to the Rackspace Spot console at https://console.rackspace.com/
2. Navigate to the **ord-devimprint** cloudspace (in us-east-ord-1 region)

### Step 2: Generate cloudspace-admin OIDC Token
1. Go to **Access** or **Credentials** section in the cloudspace
2. Look for **OIDC Token** or **Service Account Token** generation
3. Select the **cloudspace-admin** group/role
4. Generate a new OIDC token (copy the full token string)

### Step 3: Update Kubeconfig
The kubeconfig should be placed at:
```
/home/coding/.kube/ord-devimprint-admin.kubeconfig
```

The kubeconfig format should include:
- Cluster: ord-devimprint Rackspace Spot cluster endpoint
- Authentication: OIDC token-based
- User/Context: cloudspace-admin with cluster-admin privileges

## Expiration Tracking
- **Last refreshed:** 2026-08-06
- **Expires approximately:** 2026-08-09 (~3 days from generation)
- **Next refresh needed:** Before 2026-08-09

## Unblocked Work
Once refreshed, this kubeconfig unblocks:
1. Phase 0 CNPG/operator installation tasks requiring cluster-admin access
2. Phase 1 `email_resolution` extraction (365K+ pairs of rate-limited API budget)

## Notes
- The observer kubeconfig (`/home/coding/.kube/ord-devimprint-observer.kubeconfig`) uses a long-lived SA token and does not expire
- Only the admin kubeconfig requires OIDC token refresh
