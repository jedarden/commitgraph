# Spot Fallback Node Class Decision

**Decision Date:** 2026-08-06
**Bead ID:** cg-2ypl
**Status:** RESOLVED

## Decision

If `mh.vs1.large-ord` doesn't fulfill on the Rackspace Spot bid market, the fallback node class is **`ch.vs1.large-ord`**.

## Rationale

### Capacity Profile
- **Target (`mh.vs1.large-ord`):** 4 CPU / 30GB, memory-optimized
- **Fallback (`ch.vs1.large-ord`):** Same "large" size class, compute-optimized
- **Fit:** Postgres benefits from compute-optimized resources for query processing and transaction management; the fallback maintains similar CPU/memory capacity while trading some memory for potentially better compute performance

### Pricing (from Rackspace Spot percentile-pricing endpoint)

| Class | p50 | Market |
|-------|-----|--------|
| `mh.vs1.large-ord` (target) | $0.006/hr | $0.005/hr |
| `ch.vs1.large-ord` (fallback) | $0.011/hr | $0.001/hr |

- **Cost impact:** ~1.8x higher p50 price ($0.011 vs $0.006/hr)
- **Monthly delta at p50:** ~$3.60 additional per instance ($7.92 vs $4.32)
- **Availability signal:** Lower market price ($0.001 vs $0.005) suggests better availability
- **Total monthly at p50 (instances: 3):** ~$23.76 vs ~$12.96 for target class

### Rejected Alternatives

- `mh.vs1.xlarge-ord`: Higher capacity but same price point as fallback; more expensive than necessary
- `ch.vs1.2xlarge-ord`: Excessive capacity for current sizing (~60-90MB database)
- `mh.vs1.2xlarge-ord`: Same capacity issue at 2.2x the price

## Fallback Trigger Condition

**Wait 15 minutes after nodepool provisioning before switching to fallback class.**

### Rationale for 15-minute window
- Spot bid fulfillment can take time to stabilize as the market clears
- Too short (e.g., 5 minutes) risks switching on transient fulfillment lag
- Too long (e.g., 60+ minutes) unnecessarily delays recovery with no rollback target available
- 15 minutes balances patience with operational urgency given the hard-outage context (old pipeline torn down)

### Implementation Notes
- **Check via:** Rackspace Spot UI or `rackspace-spot-terraform` state inspection (`fulfilled < desired` condition persists)
- **After triggering:** Create new nodepool in Spot UI or via Terraform, update CNPG cluster's node selector, drain old nodepool
- **Do not auto-revert:** Once on fallback class, stay there; manual intervention required to retry target class

## Context

This decision is part of Phase 0 Postgres provisioning for the commitgraph v2 redesign. The plan provisions a **single dedicated node** (`instances: 1` matching the `iad-ci/queue-db` CNPG precedent) as the sole write target for clone-worker rollup upserts. A preemption event on this node is a hard outage for the entire rollup path, making a fallback class essential for operational resilience.

With the old pipeline torn down (2026-08-05), there is no rollback target—this decision must be made before attempting provisioning, not discovered during a live outage.

## Related Decisions

- **Postgres replica topology (cg-25cp):** `instances: 3` with synchronous replication for zero-data-loss automatic failover (~$13/month at p50)
- **Postgres sizing:** ~60-90MB at current scale, under 1GB at 10x growth
- **Storage class:** `sata` (5-20GB range) per org-wide Rackspace Spot rule

## Monitoring Requirements

As part of Phase 0 implementation:
- Alert on nodepool `fulfilled < desired` condition persisting beyond 5 minutes
- Surface actual Rackspace Spot bid spend against percentile pricing
- Track preemption events if any occur on the dedicated node
