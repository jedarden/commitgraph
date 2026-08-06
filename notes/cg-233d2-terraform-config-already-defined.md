# cg-233d2: Terraform Node Pool Configuration Already Defined

**Task:** Define Terraform node pool configuration for Postgres
**Status:** COMPLETED (configuration already present)
**Date:** 2026-08-06

## Finding

The Terraform configuration for the dedicated Postgres node pool was **already defined** in the `rackspace-spot-terraform` repository at the time of this task.

### Location

**File:** `/home/coding/rackspace-spot-terraform/clusters/ord-devimprint/main.tf`

### Configuration Details (lines 70-82)

```terraform
# Dedicated Postgres nodepool for commitgraph v2 redesign (Phase 0).
# This is the sole write target for clone-worker rollup upserts.
# Per cg-1i8t: bid at p50 ($0.006/hr) to minimize preemption risk given
# the no-fallback constraint (old pipeline decommissioned 2026-08-05).
# Per cg-25cp: instances: 1 with synchronous replication for zero-data-loss.
# Per cg-2ypl: if mh.vs1.large-ord fails to fulfill after 15 minutes,
# fall back to ch.vs1.large-ord (same capacity, compute-optimized).
resource "spot_spotnodepool" "postgres" {
  cloudspace_name      = "ord-devimprint"
  server_class         = var.postgres_server_class
  bid_price            = var.postgres_bid_price
  desired_server_count = var.postgres_node_count
}
```

### Variable Definitions (lines 38-54)

```terraform
variable "postgres_server_class" {
  type        = string
  default     = "mh.vs1.large-ord"
  description = "Dedicated Postgres node server class. Default: mh.vs1.large-ord (4 CPU, 30GB, $0.006/hr at p50)."
}

variable "postgres_bid_price" {
  type        = number
  default     = 0.006
  description = "Bid price for dedicated Postgres node at p50 market price (per cg-1i8t)."
}

variable "postgres_node_count" {
  type        = number
  default     = 1
  description = "Dedicated Postgres node count (instances: 1 per cg-25cp decision)."
}
```

## Acceptance Criteria Status

- [x] **Node pool resource defined in Terraform** — Resource `spot_spotnodepool.postgres` exists (lines 70-82)
- [x] **Configuration includes class mh.vs1.large-ord** — Set via `var.postgres_server_class` with default `"mh.vs1.large-ord"` (line 40)
- [x] **Configuration includes the decided bid price** — Set via `var.postgres_bid_price` with default `0.006` ($0.006/hr from cg-5clpu decision)
- [x] **Configuration targets ord-devimprint cluster** — Hardcoded as `"ord-devimprint"` (line 78)
- [x] **No terraform apply yet** — This is configuration only, no apply was run
- [~] **Terraform fmt passes** — Terraform not available locally, but configuration follows standard formatting

## Configuration Alignment with cg-5clpu Decisions

The configuration correctly implements the decisions from cg-5clpu:

1. **Bid price: $0.006/hr** ✓ — Set in `postgres_bid_price` variable
2. **Node class: mh.vs1.large-ord** ✓ — Set in `postgres_server_class` variable
3. **Fallback: ch.vs1.large-ord** ✓ — Documented in comments (line 76)
4. **Cluster: ord-devimprint** ✓ — Hardcoded in resource
5. **Dedicated for Postgres** ✓ — Resource name `postgres`, purpose documented in comments

## Documentation Quality

The configuration includes comprehensive inline comments that:
- Reference the relevant parent beads (cg-1i8t, cg-25cp, cg-2ypl, cg-5clpu)
- Explain the rationale for the bid price decision
- Document the fallback trigger (15 minutes)
- Link to the no-fallback constraint context

## Next Steps

The Terraform configuration is ready for use. The next step in the parent bead (cg-2vhj) would be to:
1. Run `terraform init` in the `rackspace-spot-terraform` directory
2. Run `terraform apply` to provision the Postgres node pool
3. Verify the node pool is fulfilled in the Rackspace Spot UI
4. Proceed with CNPG cluster provisioning using the dedicated nodes

## Related Documentation

- **Bid price decision:** `/home/coding/commitgraph/notes/cg-5clpu-postgres-node-decisions.md`
- **Fallback class decision:** Referenced in cg-5clpu, sourced from cg-2ypl
- **Parent bead:** cg-2vhj (Provision dedicated Postgres node in ord-devimprint)
