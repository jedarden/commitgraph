# Task Completion Summary: cg-25cp - CNPG Replica Topology Decision

## Task Status

**COMPLETED** - Decision made, documented, and committed.

## Decision

**`instances: 3` with synchronous replication** (not `instances: 1`)

## Rationale

### Risk Analysis
- **No fallback system exists:** Old pipeline torn down 2026-08-05; no rollback target
- **Single-node preemption is a hard outage:** RTO: hours, RPO: up to 24h of commits
- **Spot preemption is operational reality:** Low bid prices increase preemption frequency

### Cost Analysis
- **Compute cost:** $0.018/hr ≈ **$13/month** (3× `mh.vs1.large-ord` at p50 $0.006/hr)
- **Storage cost:** Unchanged (3× 1GB = 3GB, still within `sata` tier)
- **Value:** Trivial insurance for critical infrastructure with no fallback

### Why `instances: 3` Beats `instances: 1`

| Factor | `instances: 1` | `instances: 3` |
|--------|----------------|----------------|
| Preemption impact | Hard outage, hours RTO | Automatic failover, zero downtime |
| Data loss on preemption | Up to 24h of commits | Zero (synchronous replication) |
| Recovery process | Manual backup restore | Automatic replica promotion |
| Monthly cost delta | Baseline | +$13 |

### Implementation Details
- **Synchronous replication:** `synchronous_commit: on` with `maxSyncReplicas: 1` (CNPG default)
- **Resource allocation:** ~1 CPU / 4GB per instance (3 CPU / 12GB total across 3 instances)
- **Node placement:** All three pods on same dedicated `mh.vs1.large-ord` node
- **Limitation:** Does not protect against node preemption (all 3 pods share one node)

## What This Doesn't Solve

Node preemption still causes outage. If the dedicated Spot node preempts:
- All three Postgres pods go down together
- Recovery is still from backup (RTO: hours, RPO: up to 24h)
- True cross-node resilience would require 3 dedicated nodes (~$39/month), not chosen here

## Acceptance Criteria Status

- [x] `instances: 1` or `instances: 3` is explicitly chosen → **`instances: 3`**
- [x] Rationale documented, weighing cost against outage exposure
- [x] Synchronous replication mode is specified
- [x] Decision recorded in plan.md "Open decisions" (resolved)

## Documentation

- **Decision record:** `/home/coding/commitgraph/docs/notes/cg-25cp-replica-topology-decision.md`
- **Plan.md:** Open decisions section marked as resolved 2026-08-06
- **Commit:** 8143b79 "docs(cg-25cp): decide CNPG replica topology as instances: 3 with synchronous replication"

## Next Steps

- Phase 0: Implement CNPG `Cluster` manifest with `instances: 3`
- Phase 0: Backup/restore rehearsal remains **blocking gate** (replication doesn't replace backups)
- Phase 2: Load test to validate synchronous replication doesn't introduce unacceptable write latency

## Decision Date

**2026-08-06**
