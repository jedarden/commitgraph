# CNPG Operator Verification on ord-devimprint

**Date:** 2026-08-06
**Task:** cg-ppsq - Verify CNPG operator is running and CRDs are registered on ord-devimprint

## Findings

### Current State: NOT INSTALLED

The CNPG operator is **NOT** currently running on ord-devimprint.

### Evidence

1. **Namespace does not exist**
   ```bash
   $ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n cnpg-system
   No resources found in cnpg-system namespace.
   ```

2. **No CNPG CRDs registered**
   ```bash
   $ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get crds | grep cnpg
   # (no output)
   
   $ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get crds -o name | grep -E 'postgresql\.cnpg\.io|cnpg\.io'
   # (no output)
   ```

3. **No CNPG operator pods found**
   - Searched across all namespaces: no postgresql or cnpg related pods
   - No related deployments found

### Declarative Config EXISTS

The declarative-config for the CNPG operator installation **exists** in `~/declarative-config/k8s/ord-devimprint/cnpg-system/`:

- `cnpg-application.yml` - ArgoCD Application resource
  - Name: `cnpg-ord-devimprint`
  - Chart: `cloudnative-pg` v0.27.1
  - Destination: `cnpg-system` namespace on ord-devimprint
  - Sync policy: Automated with prune, self-heal, CreateNamespace=true
  
- `namespace.yml` - Namespace definition

**Status:** This is Phase 0 prerequisite work that has not been completed yet.

### Plan.md Context

From `/home/coding/commitgraph/docs/plan/plan.md`:

> "Two known costs, both one-time and still unstarted: (1) node provisioning is a manual, out-of-band step — Rackspace Spot node pools are provisioned via the Spot web UI or a locally-run Terraform apply from the separate jedarden/rackspace-spot-terraform repo, **not** a declarative-config PR (in-cluster Terraform automation for this was retired org-wide 2026-04-22 after a reliability incident); (2) **CNPG operator does not exist on ord-devimprint yet and needs installing fresh** (it already runs on ardenone-cluster, apexalgo-iad, iad-ci, and rs-manager — this would be a 5th install, not a reuse)."

This confirms the CNPG operator installation is a known **Phase 0** prerequisite that hasn't been started.

## Acceptance Criteria Status

- [ ] Operator pod(s) `Running`, stable (no restarts in a reasonable observation window)
  - **NOT MET**: No operator pods exist

- [ ] `kubectl get crds` shows the CNPG CRD set registered
  - **NOT MET**: No CNPG CRDs registered

- [ ] ArgoCD Application for the operator shows `Healthy`
  - **CANNOT VERIFY**: ArgoCD read-only proxy not accessible from this box (hostname resolution failure)
  - Application definition exists in declarative-config but sync status cannot be verified

- [ ] A trivial `Cluster` dry-run/validate confirms the CRD schema is usable
  - **NOT MET**: Cannot validate without CRDs installed

## Conclusion

**Phase 0 has not started.** The CNPG operator installation is a blocking prerequisite for:
1. Creating Postgres `Cluster` resources
2. Running the corpus migration
3. All subsequent phases

The declarative-config is ready, but the operator installation needs to be completed before any Postgres provisioning can proceed.

## Next Steps

1. The ArgoCD Application `cnpg-ord-devimprint` should be synced to install the operator
2. Verify the operator pods start successfully
3. Verify CNPG CRDs are registered
4. Then proceed with Postgres cluster provisioning (the dedicated Spot node, storage, etc.)

---

**Task Status:** INCOMPLETE - CNPG operator is not installed (this is expected Phase 0 work, not a failure)
