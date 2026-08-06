# cg-mci3: MANUAL ACTION REQUIRED

## Status: ⛔ BLOCKED - Manual intervention needed

This task requires manual action to regenerate an OIDC token from the Rackspace Spot UI. Automated tools cannot access the Spot UI to perform this action.

## What You Need To Do

### 1. Access Rackspace Spot UI
- Log in to the Rackspace Spot dashboard
- Navigate to the ord-devimprint cluster/cloudspace
- Find the "Access" or "Credentials" section

### 2. Generate New Admin Kubeconfig
- Look for "cloudspace-admin" OIDC token or kubeconfig download
- Generate/download the new admin kubeconfig
- Save the contents to replace `/home/coding/.kube/ord-devimprint-admin.kubeconfig`

### 3. Set Proper Permissions
```bash
chmod 600 /home/coding/.kube/ord-devimprint-admin.kubeconfig
```

### 4. Verify It Works
```bash
# Quick test
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get nodes

# Full verification
/home/coding/commitgraph/scripts/verify-ord-devimprint-admin-kubeconfig.sh
```

### 5. Document the Refresh Date
```bash
echo "Last refreshed: $(date -Iseconds)" >> /home/coding/commitgraph/notes/cg-mci3-ord-devimprint-kubeconfig-refresh.md
```

## Why This Is Needed

The `ord-devimprint-admin.kubeconfig` uses an OIDC token that expires every ~3 days. This is blocking:

1. **Email Resolution Extraction**: We need to extract 365K+ email→login pairs from queue-api's SQLite database. This is the single highest-value inheritance in the migration - months of GitHub API budget spent.
2. **Phase 0 Work**: Any cluster-admin tasks for the new commitgraph v2 pipeline setup
3. **Queue-api Access**: The pod is running but we can't exec into it without admin access

## Current State

- ❌ `/home/coding/.kube/ord-devimprint-admin.kubeconfig` - does not exist or expired
- ✅ Observer kubeconfig (read-only) - exists and works (doesn't expire)
- ❌ Can't access queue-api pod for email_resolution extraction
- ❌ Blocked on Phase 1's `email_resolution` extraction

## Documentation Created

I've created the following files to help with this and future refreshes:

1. **Detailed refresh procedure**: `notes/cg-mci3-ord-devimprint-kubeconfig-refresh.md`
2. **Verification script**: `scripts/verify-ord-devimprint-admin-kubeconfig.sh`
3. **This summary**: `notes/cg-mci3-MANUAL-REFRESH-REQUIRED.md`

## Timeline Considerations

- Token expires every ~3 days
- Consider setting a recurring calendar reminder
- Each refresh requires manual Spot UI access
- Plan around this cadence for Phase 0 work

## Next Steps After Refresh

Once you've refreshed the kubeconfig and verified it works:

1. Run the verification script to confirm access
2. Proceed with email_resolution extraction from queue-api
3. Continue with Phase 0 CNPG/operator installation tasks
4. Update the refresh date in the documentation

## Verification

Before closing this task, ensure:

- [ ] `kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get nodes` works
- [ ] Can list CRDs (cluster-admin access confirmed)
- [ ] Can access queue-api pod (extraction access confirmed)
- [ ] Refresh date documented in notes
- [ ] Verification script passes all tests

---

**Note to agents**: This task cannot be completed automatically. It requires human access to the Rackspace Spot UI. Do not attempt to close this bead automatically.