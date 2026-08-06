# Clone-Worker Replica Count Decision

**Decision Date:** 2026-08-06
**Bead ID:** cg-4pcs
**Status:** RESOLVED

## Decision

The clone-worker replica count for the new pipeline is **3 replicas**.

## Rationale

### Capacity Headroom Constraint

From `./k8s/capacity-check.sh ord-devimprint` (2026-08-04, live):
- The existing 6-node `compute1-4` fleet is committed **41% CPU / 50% memory**
- **Largest schedulable pod anywhere on the fleet:** 1.20 CPU / 1.64 GiB
- Clone-worker pods schedule on this existing fleet (not the dedicated Postgres node)

### Clone-Worker Pod Resource Profile

From the disabled deployment spec (`clone-worker-deployment.yml.disabled`):
- **CPU request:** 50m (0.050 CPU)
- **Memory request:** 512 MiB (0.5 GiB)
- **CPU limit:** 1000m (1.0 CPU)
- **Memory limit:** 1280 MiB (1.25 GiB)

### Headroom Math

Memory is the limiting constraint (CPU is abundant in the headroom):

```
Memory headroom per largest slot: 1.64 GiB
Clone-worker memory per pod: 0.512 GiB
Max pods per slot: 1.64 / 0.512 ≈ 3.2 pods

CPU headroom per largest slot: 1.20 CPU
Clone-worker CPU per pod: 0.050 CPU  
Max pods per slot: 1.20 / 0.050 = 24 pods
```

**Memory limits us to ~3 pods in the largest available headroom slot.**

### Comparison to Old Architecture

The old pipeline ran **4 clone-worker replicas** total:
- 1 standard clone-worker (dedicated to re-scan/freshness lane, priority ≤10)
- 2 clone-worker-parallel replicas (fresh discovery backlog, priority 50-70)
- 1 clone-worker-large replica (oversized repos >200MB)

However, the old architecture also ran filter-worker, compactor, user-enrichment-worker, aggregator, and search-worker as separate deployments all competing for queue-api's write lock. The new design consolidates filter-worker logic into clone-worker and retires compactor as a standalone service, materially reducing the per-replica processing load.

### Why 3 Replicas (Not 4)

1. **Headroom constraint:** 3 replicas × 512 MiB = 1.536 GiB < 1.64 GiB maximum schedulable slot. This fits within the confirmed headroom with a small margin.

2. **Throughput adequate:** At ~1,000 repos/hour/replica (measured ceiling from "Explicitly out of scope"), 3 replicas = 3,000 repos/hour. For 98,747 repos on a 24-hour rescan cycle, this requires ~33 hours of dedicated processing — but the system prioritizes re-scans over fresh discovery, and the 15-minute aggregator cycle means partial results publish continuously.

3. **Load-test ceiling:** Phase 2 will load-test at this baseline plus headroom (4-5 replicas) to establish the true ceiling. This decision gives Phase 2 a concrete starting point and an explicit pass/fail target.

4. **Connection pool sizing:** This count directly informs PgBouncer pool sizing (see p0-deploy-pgbouncer). 3 replicas with Postgres connection pooling can safely service the write workload without exhausting connections.

## Phase 2 Load Test Targets

**Baseline test:** 3 replicas at steady state
**Headroom test:** 4-5 replicas to establish the ceiling
**Pass/fail criteria:**
- Postgres maintains <2s p99 latency on ranking queries
- PgBouncer connection pool stays below 80% utilization
- No `context canceled` / `Read timed out` errors propagating to queue-api

## Inputs to Other Decisions

This replica count is a direct input to:
- **p0-deploy-pgbouncer:** PgBouncer pool size must accommodate 3 concurrent writers plus aggregator read traffic
- **Phase 2 load test:** Concrete pass/fail numbers now exist (3 baseline, 4-5 ceiling)

## Related Decisions

- **Postgres replica topology (cg-25cp):** `instances: 3` with synchronous replication
- **Spot fallback node class (cg-2ypl):** `ch.vs1.large-ord` with 15-minute trigger
- **Postgres sizing:** ~60-90MB at current scale, under 1GB at 10x growth

## Monitoring Requirements

As part of Phase 0 implementation:
- Alert on clone-worker `claim_latency` (time from lease claim to write completion)
- Track queue depth by priority tier (re-scan vs. fresh discovery)
- Monitor Postgres `pg_stat_database` for bloat and autovacuum effectiveness
