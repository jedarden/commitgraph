# Postgres Node Provisioning - cg-2vhj

**Date:** 2026-08-06
**Bead ID:** cg-2vhj
**Status:** IN PROGRESS

## Objective

Provision a dedicated `mh.vs1.large-ord` (4 CPU / 30GB) node in ord-devimprint cluster for Postgres as part of Phase 0 infrastructure setup for commitgraph v2 redesign.

## Configuration

**Target node class:** `mh.vs1.large-ord` (memory-optimized, 4 CPU / 30GB)
**Bid price:** $0.006/hr (p50 market price per cg-1i8t)
**Node count:** 1 (per cg-25cp decision: instances: 1 with synchronous replication)
**Fallback class:** `ch.vs1.large-ord` if unfulfilled after 15 minutes (per cg-2ypl decision)

## Terraform Configuration Status

✅ **Terraform configuration is already written and ready**
- Location: `/home/coding/rackspace-spot-terraform/clusters/ord-devimprint/main.tf`
- Variables defined:
  - `postgres_server_class = "mh.vs1.large-ord"`
  - `postgres_bid_price = 0.006`
  - `postgres_node_count = 1`
- Resource: `spot_spotnodepool.postgres`

❌ **Authentication blocker: API token not available**
- Terraform requires `rackspace_spot_token` variable
- Token not found in environment, credential files, or common storage locations
- Terraform v1.9.8 available via nix-shell with `NIXPKGS_ALLOW_UNFREE=1`

## Alternative Path: Spot Web UI

The task description explicitly allows Spot web UI as a fallback option:
> "Using `jedarden/rackspace-spot-terraform` (preferred) or the Spot web UI as fallback"

### Web UI Provisioning Steps

1. Access Rackspace Spot web UI for ord-devimprint cloudspace
2. Create new nodepool with:
   - Server class: `mh.vs1.large-ord`
   - Bid price: $0.006/hr
   - Desired count: 1
3. Monitor fulfillment status for 15 minutes
4. If unfulfilled after 15 minutes, create nodepool with:
   - Server class: `ch.vs1.large-ord` (fallback per cg-2ypl)
   - Bid price: $0.011/hr (p50)
   - Desired count: 1

## Next Steps

**Option A:** Obtain Rackspace Spot API token
- Source token from secure credential store
- Run: `cd /home/coding/rackspace-spot-terraform/clusters/ord-devimprint && terraform apply -var="rackspace_spot_token=<token>"`

**Option B:** Use Spot web UI (recommended given token access limitation)
- Manual provisioning through web interface
- Document nodepool creation details
- No Terraform state to manage

## Related Decisions

- **Bid price (cg-1i8t):** $0.006/hr at p50 to minimize preemption risk
- **Replica topology (cg-25cp):** instances: 1 with synchronous replication
- **Fallback class (cg-2ypl):** ch.vs1.large-ord with 15-minute trigger
- **Storage class:** sata (5-20GB range) per org-wide rule

## Acceptance Criteria

- [ ] Nodepool defined and created (Terraform or Spot UI)
- [ ] Node reaches `fulfilled == desired` OR fallback class documented
- [ ] Terraform changes committed (if Terraform path used)
- [ ] No declarative-config PR used (nodepools are exception to GitOps rule)
