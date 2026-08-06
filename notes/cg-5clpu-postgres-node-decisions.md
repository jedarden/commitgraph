# Postgres Node Bid Price and Fallback Class Decisions

**Decision Date:** 2026-08-06
**Bead ID:** cg-5clpu
**Status:** RESOLVED

## Summary

This document consolidates the two prerequisite decisions for Postgres node provisioning (parent bead cg-2vhj):

1. **Bid price for `mh.vs1.large-ord` node**: `$0.006/hr` (at p50 market price)
2. **Fallback node class**: `ch.vs1.large-ord` with 15-minute trigger

---

## Decision 1: Bid Price

**Value**: `$0.006/hr` (at the p50 market price)

### Rationale

The bid price decision balances cost optimization against preemption risk for critical infrastructure with no fallback system.

**Tradeoff Analysis:**

| Factor | Low bid ($0.001/hr) | Higher bid ($0.006/hr) |
|--------|---------------------|------------------------|
| **Monthly cost (3 instances)** | $2.19 | $13.14 |
| **Preemption risk** | Higher | Lower |
| **Cost difference** | — | +$11/month |

**Critical Constraint: Sole Write Target with No Fallback**

Per "Durability and load" point 4 in plan.md:

> "With no fallback system (old pipeline torn down 2026-08-05), a single-node preemption is a hard outage with hours-long RTO and up-to-24h RPO."

This Postgres instance is the **sole write target** for the entire commitgraph pipeline:
- The old pipeline was decommissioned on 2026-08-05
- There is no rollback target
- A preemption on the dedicated node means the entire rollup write path goes down
- Recovery requires manual backup/restore with hours-long RTO

**Why Not the Low Bid?**

The existing `ch.vs1.medium-ord` nodepool successfully runs at `$0.001/hr`, but **that precedent does not apply here**:

1. **Different failure impact**: The existing nodepool hosts workloads where a preemption causes a graceful reschedule. The Postgres nodepool hosts the **primary write target**—a preemption is a hard outage.

2. **Asymmetric risk**: The $11/month premium is negligible compared to:
   - Hours-long downtime for the entire commitgraph pipeline
   - Up to 24 hours of data loss (RPO)
   - Manual restore operational overhead
   - User-facing impact during recovery

3. **Existing precedent fulfilled ≠ guaranteed**: The current `ch.vs1.medium-ord` pool being fulfilled at `$0.001/hr` means the market clears at that price for **that class**, not that a **different class** (`mh.vs1.large-ord`) will also clear. The larger class may have different market dynamics.

4. **Alignment with prior HA decision**: The organization already committed to `instances: 3` with synchronous replication (~$13/month) specifically because the no-fallback risk was unacceptable. Bid pricing should follow the same logic: minimize preemption risk for critical infrastructure.

**Market Context**

From the live percentiles.json query (2026-08-04):
- Both `ch.vs1.medium-ord` and `mh.vs1.large-ord` price at p50 **$0.006/hr**
- The `market_price` field for `ch.vs1.medium-ord` is `$0.001/hr`—matching the existing bid
- The p50 represents what the market has historically cleared at; bidding at p50 materially reduces preemption probability compared to bidding at the extreme low end

### Implementation

This `$0.006/hr` value is ready to be passed directly into:
- Terraform `bid_price` field in `rackspace-spot-terraform/clusters/ord-devimprint/main.tf`
- Rackspace Spot UI node provisioning form

**Source decision:** cg-1i8t (closed 2026-08-06)
**Full documentation:** `/home/coding/commitgraph/notes/cg-1i8t-bid-price-decision.md`

---

## Decision 2: Fallback Node Class

**Value**: `ch.vs1.large-ord` (4 CPU / 30GB, compute-optimized)

**Trigger Condition**: Wait 15 minutes after nodepool provisioning before switching to fallback class.

### Rationale

If `mh.vs1.large-ord` doesn't fulfill on the Rackspace Spot bid market, `ch.vs1.large-ord` provides:

1. **Same capacity profile**: Both are "large" size class with 4 CPU / 30GB
2. **Compute-optimized**: Better for Postgres query processing and transaction management
3. **Better availability signal**: Lower market price ($0.001 vs $0.005/hr) suggests better supply

**Pricing Impact:**

| Class | p50 | Market |
|-------|-----|--------|
| `mh.vs1.large-ord` (target) | $0.006/hr | $0.005/hr |
| `ch.vs1.large-ord` (fallback) | $0.011/hr | $0.001/hr |

- **Cost impact:** ~1.8x higher p50 price ($0.011 vs $0.006/hr)
- **Monthly delta at p50:** ~$3.60 additional per instance ($7.92 vs $4.32)
- **Total monthly at p50 (instances: 3):** ~$23.76 vs ~$12.96 for target class

### Rejected Alternatives

- `mh.vs1.xlarge-ord`: Higher capacity but same price point as fallback; more expensive than necessary
- `ch.vs1.2xlarge-ord`: Excessive capacity for current sizing (~60-90MB database)
- `mh.vs1.2xlarge-ord`: Same capacity issue at 2.2x the price

### Fallback Trigger Rationale

**15-minute window** balances patience with operational urgency:
- Spot bid fulfillment can take time to stabilize as the market clears
- Too short (e.g., 5 minutes) risks switching on transient fulfillment lag
- Too long (e.g., 60+ minutes) unnecessarily delays recovery with no rollback target available
- Given the hard-outage context (old pipeline torn down), faster recovery is preferred

### Implementation Notes

- **Check via:** Rackspace Spot UI or `rackspace-spot-terraform` state inspection (`fulfilled < desired` condition persists)
- **After triggering:** Create new nodepool in Spot UI or via Terraform, update CNPG cluster's node selector, drain old nodepool
- **Do not auto-revert:** Once on fallback class, stay there; manual intervention required to retry target class

**Source decision:** cg-2ypl (closed 2026-08-06)
**Full documentation:** `/home/coding/commitgraph/docs/notes/cg-2ypl-spot-fallback-node-class.md`

---

## Combined Cost Summary

**Target configuration (3× `mh.vs1.large-ord` at $0.006/hr):**
- Monthly: ~$13.14
- Annual: ~$157.68

**Fallback configuration (3× `ch.vs1.large-ord` at $0.011/hr):**
- Monthly: ~$23.76
- Annual: ~$285.12

**Cost difference:** ~$10.62/month additional when using fallback class

---

## Related Decisions

- **Postgres replica topology (cg-25cp):** `instances: 3` with synchronous replication for zero-data-loss automatic failover (~$13/month at p50)
- **Postgres sizing:** ~60-90MB at current scale, under 1GB at 10x growth
- **Storage class:** `sata` (5-20GB range) per org-wide Rackspace Spot rule

---

## Next Steps

These decisions are now ready for use in:
1. **Parent bead cg-2vhj**: Provision the dedicated mh.vs1.large-ord Spot node for Postgres
2. **Terraform configuration**: `rackspace-spot-terraform/clusters/ord-devimprint/main.tf`
3. **CNPG cluster provisioning**: Use these values for nodepool configuration

## Monitoring Requirements

As part of Phase 0 implementation:
- Alert on nodepool `fulfilled < desired` condition persisting beyond 5 minutes
- Alert on preemption events if any occur on the dedicated node
- Surface actual Rackspace Spot bid spend against the expected $13.14/month baseline
- Track fulfillment latency to detect if 15-minute trigger needs adjustment
