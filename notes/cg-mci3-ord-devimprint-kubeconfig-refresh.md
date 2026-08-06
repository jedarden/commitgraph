# ord-devimprint-admin.kubeconfig Refresh Procedure

## Issue
The `ord-devimprint-admin.kubeconfig` file uses an OIDC token that expires approximately every 3 days. When expired, kubectl commands return 401 errors.

## Background
- **Cluster**: ord-devimprint (Rackspace Spot, ORD region)
- **Auth method**: OIDC (cloudspace-admin group)
- **Token lifetime**: ~3 days
- **Required for**: Cluster-admin access to ord-devimprint
- **Current blocker**: Extraction of `email_resolution` table from queue-api (365K+ rows)

## Refresh Procedure

### Step 1: Access Rackspace Spot UI
1. Navigate to the Rackspace Spot dashboard (you'll need to log in with your Spot credentials)
2. Find the ord-devimprint cloudspace/cluster

### Step 2: Regenerate OIDC Token
1. In the Spot UI, navigate to the cluster's access/credentials section
2. Look for the "cloudspace-admin" OIDC token or kubeconfig generation
3. Generate/refresh the admin kubeconfig - this will download a new kubeconfig file

### Step 3: Update Local File
1. Replace the contents of `/home/coding/.kube/ord-devimprint-admin.kubeconfig` with the new kubeconfig
2. Set proper permissions: `chmod 600 /home/coding/.kube/ord-devimprint-admin.kubeconfig`

### Step 4: Verify Access
```bash
# Test basic access
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get nodes

# Test cluster-admin access (list CRDs)
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get crds

# Test access to queue-api pod
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-admin.kubeconfig get pods -n devimprint
```

### Step 5: Document the Refresh Date
Update this file with the refresh date so the next expiry is predictable:
```bash
echo "Last refreshed: $(date -Iseconds)" >> /home/coding/commitgraph/notes/cg-mci3-ord-devimprint-kubeconfig-refresh.md
```

## Automation Notes
- This token cannot be automated without programmatic access to Spot API/UI
- Consider setting a calendar reminder for every 2-3 days
- The extraction task (cg-mci3) is blocked on this refresh

## Verification Checklist
- [ ] kubectl get nodes succeeds without 401 error
- [ ] Can list cluster-scoped resources (CRDs, nodes)
- [ ] Can access devimprint namespace resources
- [ ] Can exec into queue-api pod (needed for email_resolution extraction)
- [ ] Refresh date documented

## Related Files
- `/home/coding/.kube/ord-devimprint-admin.kubeconfig` - admin kubeconfig (this file)
- `/home/coding/.kube/ord-devimprint-observer.kubeconfig` - read-only kubeconfig (long-lived, doesn't need refresh)
- `/home/coding/declarative-config/k8s/ord-devimprint/CLAUDE.md` - cluster documentation
- `/home/coding/declarative-config/k8s/ord-devimprint/commitgraph/TEARDOWN.md` - extraction task details

## Troubleshooting
- If the Spot UI doesn't provide the expected OIDC token option, look for "kubeconfig download" or "cluster access" sections
- If you get 401 errors immediately after refresh, the token may have been generated for the wrong cluster or user
- The observer kubeconfig (read-only) uses a long-lived SA token and doesn't expire like the admin one
