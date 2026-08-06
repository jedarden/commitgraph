# `mh.vs1.large-ord` Bid Price Decision

## Decision

**Bid price: `$0.006/hr` (at the p50 market price)**

## Rationale

### Tradeoff Analysis

| Factor | Low bid ($0.001/hr) | Higher bid ($0.006/hr) |
|--------|---------------------|------------------------|
| **Monthly cost (3 instances)** | $2.19 | $13.14 |
| **Preemption risk** | Higher | Lower |
| **Cost difference** | — | +$11/month |

### Critical Constraint: Sole Write Target with No Fallback

Per "Durability and load" point 4 in plan.md:

> "With no fallback system (old pipeline torn down 2026-08-05), a single-node preemption is a hard outage with hours-long RTO and up-to-24h RPO."

This Postgres instance is the **sole write target** for the entire commitgraph pipeline:
- The old pipeline was decommissioned on 2026-08-05
- There is no rollback target
- A preemption on the dedicated node means the entire rollup write path goes down
- Recovery requires manual backup/restore with hours-long RTO

### Why Not the Low Bid?

The existing `ch.vs1.medium-ord` nodepool successfully runs at `$0.001/hr`, but **that precedent does not apply here**:

1. **Different failure impact**: The existing nodepool hosts workloads where a preemption causes a graceful reschedule. The Postgres nodepool hosts the **primary write target**—a preemption is a hard outage.

2. **Asymmetric risk**: The $11/month premium is negligible compared to:
   - Hours-long downtime for the entire commitgraph pipeline
   - Up to 24 hours of data loss (RPO)
   - Manual restore operational overhead
   - User-facing impact during recovery

3. **Existing precedent fulfilled ≠ guaranteed**: The current `ch.vs1.medium-ord` pool being fulfilled at `$0.001/hr` means the market clears at that price for **that class**, not that a **different class** (`mh.vs1.large-ord`) will also clear. The larger class may have different market dynamics.

4. **Alignment with prior HA decision**: The organization already committed to `instances: 3` with synchronous replication (~$13/month) specifically because the no-fallback risk was unacceptable. Bid pricing should follow the same logic: minimize preemption risk for critical infrastructure.

### Why Not Bid Between Low and P50?

A bid like `$0.003/hr` might seem like a middle ground, but it introduces complexity without benefit:
- Still vulnerable to preemption during demand spikes
- No clear optimization target or basis for the number
- Better to either optimize cost (low bid, accept higher risk) or optimize reliability (p50 bid, minimize risk)

### Market Context

From the live percentiles.json query (2026-08-04):
- Both `ch.vs1.medium-ord` and `mh.vs1.large-ord` price at p50 **$0.006/hr**
- The `market_price` field for `ch.vs1.medium-ord` is `$0.001/hr`—matching the existing bid
- The p50 represents what the market has historically cleared at; bidding at p50 materially reduces preemption probability compared to bidding at the extreme low end

## Implementation

This `$0.006/hr` value is ready to be passed directly into:
- Terraform `bid_price` field in `rackspace-spot-terraform/clusters/ord-devimprint/main.tf`
- Rackspace Spot UI node provisioning form

## Monitoring

Post-provisioning, verify:
- Nodepool fulfillment status (`desired=3, fulfilled=3`)
- No repeated preemption cycles in the first 30 days
- Actual Spot spend against the expected $13.14/month baseline

If preemption proves to be a non-issue in practice, the bid could be revisited—but starting at p50 is the correct default for critical infrastructure with no fallback.
