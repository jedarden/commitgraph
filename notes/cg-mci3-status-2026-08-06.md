# ord-devimprint-admin.kubeconfig Status - 2026-08-06

## Current State
- **Date**: 2026-08-06
- **Admin kubeconfig**: ❌ Does NOT exist at `/home/coding/.kube/ord-devimprint-admin.kubeconfig`
- **Observer kubeconfig**: ❌ Does NOT exist at `/home/coding/.kube/ord-devimprint-observer.kubeconfig`
- **Tailscale proxy**: ✅ Working at `http://kubectl-proxy-ord-devimprint:8001` (read-only)

## What's Running in devimprint Namespace
Via the read-only proxy, I can see:
- `armor-b476789dd-*` pods (3 replicas, Running)
- `restore-verifier-6fc6b6b64d-*` pod (Running)
- **No queue-api pod currently running** (email_resolution extraction target)

## What You Need To Do (Manual Action Required)

### Step 1: Access Rackspace Spot UI
1. Log in to Rackspace Spot dashboard
2. Navigate to the ord-devimprint cloudspace/cluster

### Step 2: Generate Admin Kubeconfig
1. Find "Access" or "Credentials" section
2. Look for "cloudspace-admin" OIDC token or kubeconfig download
3. Generate/download the admin kubeconfig

### Step 3: Save and Set Permissions
```bash
# Save the downloaded kubeconfig to this location:
cat > /home/coding/.kube/ord-devimprint-admin.kubeconfig << 'EOF'
# PASTE DOWNLOADED KUBECONFIG HERE
EOF

chmod 600 /home/coding/.kube/ord-devimprint-admin.kubeconfig
```

### Step 4: Verify
```bash
# Quick test
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get nodes

# Full verification
/home/coding/commitgraph/scripts/verify-ord-devimprint-admin-kubeconfig.sh
```

## Why This Matters
1. **Email Resolution Extraction**: Primary blocker for extracting 365K+ email→login pairs from queue-api
2. **Phase 0 Work**: Cluster-admin access needed for CNPG/operator installation tasks
3. **Token Expiry**: Expires every ~3 days - this is a recurring manual task

## Next Steps After Refresh
1. Verify queue-api pod is running (currently not visible)
2. Extract email_resolution table from queue-api's SQLite database
3. Continue with Phase 0 CNPG/operator installation
4. Set a calendar reminder for next refresh (~3 days)

## Documentation
- Detailed refresh procedure: `notes/cg-mci3-ord-devimprint-kubeconfig-refresh.md`
- Verification script: `scripts/verify-ord-devimprint-admin-kubeconfig.sh`
- Manual intervention notice: `notes/cg-mci3-MANUAL-REFRESH-REQUIRED.md`
- This status summary: `notes/cg-mci3-status-2026-08-06.md`

---
**Note**: This task requires manual Spot UI access and cannot be completed automatically by agents.
