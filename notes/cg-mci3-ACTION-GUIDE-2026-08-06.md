# ord-devimprint-admin.kubeconfig Refresh - ACTION REQUIRED

## Status: BLOCKED on Manual Web UI Access

**Date:** 2026-08-06  
**Task:** Refresh cloudspace-admin OIDC token for ord-devimprint cluster  
**Current State:** Admin kubeconfig file does not exist - needs creation from Rackspace Spot UI

## What This Unblocks
- **Phase 0:** CNPG/operator installation tasks requiring cluster-admin access
- **Phase 1:** `email_resolution` extraction from queue-api (365K+ pairs, highest-value migration target)

## The Problem
The `ord-devimprint-admin.kubeconfig` file:
- Uses a `cloudspace-admin` OIDC token that expires every ~3 days
- Currently does not exist at all (file is missing)
- Must be regenerated from the Rackspace Spot UI (requires web browser authentication)

## Verification That Proxy Works
✅ Read-only proxy is working:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get nodes
# Returns: 6 worker nodes (prod-instance-17854394915530225, etc.)
```

## What You Need To Do (Manual Steps)

### Option 1: Provide Kubeconfig Directly (Fastest)
1. Log in to Rackspace Spot UI: https://spot.rackspace.com/
2. Navigate to ord-devimprint cloudspace
3. Find "Access" or "Credentials" section
4. Download the cloudspace-admin kubeconfig
5. Share the content (or paste it into the terminal for me to save)

### Option 2: Provide API Token
1. Generate a Rackspace Spot API token from the UI
2. Share the token so I can use Terraform/spotctl to extract the kubeconfig

### Option 3: Web Browser on This Machine
If you have browser access:
1. Open Firefox/Chrome on this machine
2. Navigate to Spot UI and download the kubeconfig
3. Save to `/home/coding/.kube/ord-devimprint-admin.kubeconfig`

## Once Kubeconfig Is Available
```bash
# Save with correct permissions
cat > /home/coding/.kube/ord-devimprint-admin.kubeconfig << 'EOF'
# PASTE KUBECONFIG CONTENT HERE
EOF

chmod 600 /home/coding/.kube/ord-devimprint-admin.kubeconfig

# Run verification
/home/coding/commitgraph/scripts/verify-ord-devimprint-admin-kubeconfig.sh
```

## Acceptance Criteria
- [ ] `kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get nodes` succeeds (no 401)
- [ ] Cluster-admin access confirmed (can list CRDs)
- [ ] Refresh procedure/date noted for next expiry cycle (~3 days)

## Next Steps After Success
1. Verify queue-api pod is running in devimprint namespace
2. Extract email_resolution table from queue-api's SQLite database
3. Complete Phase 0 CNPG/operator installation tasks
4. Set calendar reminder for next refresh (~2026-08-09)

## Documentation Files
- Status summary: `notes/cg-mci3-status-2026-08-06.md`
- Verification script: `scripts/verify-ord-devimprint-admin-kubeconfig.sh`
- This action guide: `notes/cg-mci3-ACTION-GUIDE-2026-08-06.md`

---
**NOTE:** This task cannot be completed without access to the Rackspace Spot UI. 
The read-only proxy works, but admin kubeconfig generation requires web authentication.
