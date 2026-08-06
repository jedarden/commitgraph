# cg-mci3: READY TO COMPLETE - Manual Action Required

## Status: 🟡 READY FOR MANUAL COMPLETION

This bead is ready to complete once the admin kubeconfig is manually obtained from the Rackspace Spot UI.

## What Has Been Done

✅ **Documentation created:**
- Detailed refresh procedure in `notes/cg-mci3-ord-devimprint-kubeconfig-refresh.md`
- Manual action summary in `notes/cg-mci3-MANUAL-REFRESH-REQUIRED.md`
- Verification script available at `scripts/verify-ord-devimprint-admin-kubeconfig.sh`

✅ **Infrastructure verified:**
- Cluster exists and is accessible via observer kubeconfig (read-only)
- Terraform configuration confirms cloudspace name: `ord-devimprint`
- Target kubeconfig path: `/home/coding/.kube/ord-devimprint-admin.kubeconfig`

## Remaining Manual Steps

### Step 1: Access Rackspace Spot UI
1. Log in to https://spot.rackspace.com with your credentials
2. Navigate to the `ord-devimprint` cloudspace/cluster
3. Find the "Access", "Credentials", or "Download Kubeconfig" section

### Step 2: Download Admin Kubeconfig
1. Look for "cloudspace-admin" OIDC token or admin kubeconfig option
2. Generate/download the new admin kubeconfig file
3. Copy the contents to clipboard or save to file

### Step 3: Update Local File
```bash
# Create/update the admin kubeconfig file
cat > /home/coding/.kube/ord-devimprint-admin.kubeconfig << 'EOF'
<PASTE CONTENTS HERE>
EOF

# Set proper permissions
chmod 600 /home/coding/.kube/ord-devimprint-admin.kubeconfig
```

### Step 4: Verify Access
```bash
# Run the verification script
/home/coding/commitgraph/scripts/verify-ord-devimprint-admin-kubeconfig.sh
```

### Step 5: Document the Refresh Date
```bash
echo "Last refreshed: $(date -Iseconds)" >> /home/coding/commitgraph/notes/cg-mci3-ord-devimprint-kubeconfig-refresh.md
```

## Expected Results

After following these steps, you should see:

✅ `kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get nodes` succeeds  
✅ Can list CRDs (cluster-admin access confirmed)  
✅ Can access queue-api pod in devimprint namespace  
✅ Ready to proceed with email_resolution extraction (365K+ rows)  

## Why This Matters

The `ord-devimprint-admin.kubeconfig` with OIDC token unlocks:
- **Phase 1 email_resolution extraction**: 365K+ already-spent GitHub API pairs
- **Phase 0 cluster-admin tasks**: CNPG/operator installation work
- **queue-api pod access**: SQLite database extraction capability

## Token Lifetime Reminder

⏰ **The OIDC token expires every ~3 days**

Consider setting a calendar reminder to refresh it. Each refresh requires manual Spot UI access.

## Verification Checklist

Before closing this bead:
- [ ] Admin kubeconfig obtained from Spot UI and saved to `/home/coding/.kube/ord-devimprint-admin.kubeconfig`
- [ ] `kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get nodes` works (no 401)
- [ ] Can list CRDs (cluster-admin access confirmed)
- [ ] Can access queue-api pod for extraction
- [ ] Refresh date documented in `notes/cg-mci3-ord-devimprint-kubeconfig-refresh.md`
- [ ] Verification script passes all tests

## Next Steps After Completion

1. Proceed with email_resolution extraction from queue-api
2. Continue with Phase 0 CNPG/operator installation tasks
3. Unblocks the single highest-value inheritance in the migration

---

**Note**: Automated tools cannot access the Rackspace Spot UI due to authentication requirements. This manual step is unavoidable and will be needed every ~3 days as the OIDC token expires.
