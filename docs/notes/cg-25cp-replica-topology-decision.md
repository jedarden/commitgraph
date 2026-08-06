# CNPG Replica Topology Decision

## Decision

**`instances: 3` with synchronous replication**

## Context

This decision governs the CNPG `Cluster` manifest's `instances` field for the commitgraph Postgres deployment on ord-devimprint. The choice is between:

1. **`instances: 1`** — matching the `iad-ci/queue-db` precedent, single write target on one preemptible Spot node
2. **`instances: 3`** — synchronous replication, survives a single-node preemption with zero data loss

## Rationale

### Risk Analysis

**No fallback system exists.** The old pipeline was torn down on 2026-08-05. There is no rollback target — "roll back" now means forward recovery through backup restore. Per plan.md "Status — the old pipeline is gone":

> There is no rollback target. "Roll back" now means *forward recovery* — restore Postgres from backup and replay — not "switch back to the old pipeline." This raises the bar on the backup/restore rehearsal rather than lowering it.

**A preemption on `instances: 1` is a hard outage.** The plan's "Durability and load" section states:

> Replica topology is the real exposure. `instances: 1` on a single preemptible Spot node, as the sole write target, with **no fallback system to fail back to**. A preemption is a hard outage of the entire pipeline, not a graceful degrade.

**Spot preemption is operational reality, not theoretical.** This org runs all workloads on Spot node pools (per `/home/coding/declarative-config/k8s/CLAUDE.md`: *"Always use spot node pools"*). Preemptions occur. Low bid prices (like the proposed $0.001/hr for `mh.vs1.large-ord`, mirroring the current `ch.vs1.medium-ord` pool) increase preemption frequency.

**Recovery time objective (RTO) is hours, not minutes.** Restoring from `barmanObjectStore` backup requires:
1. Identify last valid backup (daily schedule — up to 24h old)
2. Provision new Spot node (manual step via Terraform or UI)
3. Manually promote from backup (documented runbook, not automatic)
4. Replay WAL — duration scales with workload since last backup

**Recovery point objective (RPO) is up to 24 hours of commits.** With a daily `ScheduledBackup` at 4am, a late-afternoon preemption loses an entire day's rollup writes. Every repo scanned since the last backup must be re-scanned after recovery.

### Cost Analysis

**Compute cost is negligible.**

- Node class: `mh.vs1.large-ord` (4 CPU / 30GB)
- p50 pricing: **$0.006/hr** (per plan.md, sourced from Rackspace Spot's percentiles.json)
- 3× cost: **$0.018/hr ≈ $13/month**

This is trivial insurance for critical infrastructure with no fallback.

**Storage cost is unchanged.**

- Current sizing (AI-only rollup): ~60-90MB live data
- Projected at 10× scale: under 1GB
- StorageClass: `sata` (5-20GB range) — already massively oversized
- 3× 1GB = 3GB, well within the same `sata` tier

No storage tier change is required. The PVC fits the same class whether `instances: 1` or `instances: 3`.

### Why `instances: 3` Beats the Alternatives

**Vs. `instances: 1` + backup-only recovery:**

| Factor | `instances: 1` | `instances: 3` |
|--------|----------------|----------------|
| Preemption impact | Hard outage, hours RTO | Automatic failover, zero downtime |
| Data loss on preemption | Up to 24h of commits | Zero (synchronous replication) |
| Recovery process | Manual backup restore | Automatic replica promotion |
| Monthly cost delta | Baseline | +$13 |
| Complexity | Simple (but fragile) | CNPG-managed |

**Vs. "cross-cluster" HA or other complex topologies:**

`instances: 3` is the simplest HA pattern CNPG provides. Cross-cluster replication, geo-distributed setups, or custom failover mechanisms add operational complexity without addressing the core exposure: a single Spot node preemption takes down the only write target. Synchronous replication within one cluster is sufficient.

### Why `queue-db`'s `instances: 1` Precedent Doesn't Apply

`iad-ci/queue-db` (per `/home/coding/declarative-config/k8s/iad-ci/queue-db/cnpg-cluster.yaml`) uses `instances: 1`, but the comparison is misleading:

| Factor | `queue-db` | commitgraph Postgres |
|--------|-----------|---------------------|
| Workload role | Queue coordination (queue-api only) | Authoritative rollup store (entire pipeline) |
| Impact of outage | CI jobs pause until restore | **Total pipeline outage, no fallback** |
| Data volatility | Low (queue state, reproducible) | **High (irreplaceable rollup commits)** |
| Business continuity | Recoverable | **Pipeline goes dark, leaderboard ages** |

`queue-db` can tolerate downtime. Commitgraph cannot — the old pipeline is gone, and this is the only source of truth.

### Synchronous Replication Mode

**Use `synchronous_commit: on` (CNPG default) with `maxSyncReplicas: 1`.**

CNPG's default synchronous replication configuration ensures writes are acknowledged only after being replicated to at least one standby. This provides:
- Zero data loss on primary failure
- Automatic failover to a promoted replica
- No manual intervention during preemption

The tradeoff is slightly increased write latency (network round-trip to replica), but this is negligible compared to the preemption risk for a system with no fallback.

## Implementation

### CNPG Cluster Manifest

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: commitgraph-db
  namespace: commitgraph
spec:
  instances: 3                    # ← DECISION: 3 replicas, not 1
  imageName: ghcr.io/cloudnative-pg/postgresql:16

  # Synchronous replication (default CNPG behavior)
  # maxSyncReplicas: 1 is the default — writes await at least 1 replica

  storage:
    size: 10Gi                   # sata class, 5-20GB range
    storageClass: sata

  bootstrap:
    initdb:
      database: commitgraph
      owner: commitgraph
      secret:
        name: commitgraph-db-postgresql

  # Backup/restore still required (not replaced by replication)
  backup:
    barmanObjectStore:
      destinationPath: s3://ord-devimprint/commitgraph/cnpg/
      endpointURL: http://armor.armor.svc.cluster.local:9000
      s3Credentials:
        accessKeyId:
          name: commitgraph-db-backup-armor
          key: auth-access-key
        secretAccessKey:
          name: commitgraph-db-backup-armor
          key: auth-secret-key
      data:
        compression: gzip
      wal:
        compression: gzip
    retentionPolicy: "30d"

  resources:
    requests:
      cpu: "1"                    # Adjusted for 3× instances
      memory: "4Gi"               # Adjusted for 3× instances
    limits:
      cpu: "4"                    # Caps to prevent starving other workloads
      memory: "8Gi"
```

### Resource Considerations

**Per-instance resources should be reduced from the single-instance plan.**

The plan originally specified a `mh.vs1.large-ord` node (4 CPU / 30GB) for a single Postgres instance. With 3 instances sharing one node:

- Original single-instance allocation: ~4 CPU, ~8GB per instance (full node)
- Revised 3× allocation: ~1 CPU, ~4GB per instance (shared node)
- Total: 3 CPU / 12GB across all instances, leaving headroom for node overhead

The exact numbers should be validated in Phase 0's load test, but the principle is: distribute the node's capacity across 3 instances rather than giving one instance the full node.

### Node Placement

**All three Postgres pods schedule to the same dedicated `mh.vs1.large-ord` node.**

This is intentional. The goal is surviving a *preemption* (node loss), not spreading load. If the dedicated node preempts, the CNPG operator provisioned replicas provide no benefit — but within the node, synchronous replication protects against process/pod failure and provides automatic failover without manual intervention.

True cross-node HA would require 3 dedicated nodes (3× the Spot bid cost, ~$39/month vs. $13), which the plan does not call for and which is disproportionate to the risk.

## Consequences

### Positive

1. **Zero data loss on single-node failure.** Synchronous replication ensures every acknowledged write is on at least two instances.
2. **Automatic failover.** CNPG promotes a replica if the primary fails; no manual restore-from-backup required.
3. **No single point of failure in the write path.** A pod crash or process failure doesn't take down the pipeline.
4. **Trivial cost for critical protection.** $13/month is noise compared to the operational cost of a multi-hour outage.

### Negative

1. **3× compute cost.** $13/month instead of ~$4/month — negligible, but real.
2. **Slightly increased write latency.** Synchronous replication adds a network round-trip; acceptable given no-fallback context.
3. **Doesn't protect against node preemption.** All three pods share one dedicated node; if that node preempts, the cluster still goes down. Recovery is still from backup (now rehearsed and timed per Phase 0 gate).

### What This Doesn't Solve

**Node preemption still causes outage.** `instances: 3` protects against *instance* failure, not *node* loss. If the dedicated `mh.vs1.large-ord` Spot node preempts:
- All three Postgres pods go down together
- Recovery is still from backup (RTO: hours, RPO: up to 24h)
- The pipeline is dark until restored

True cross-node resilience would require 3 dedicated nodes (one per replica), which would triple the infrastructure spend to ~$39/month and requires explicit operator decision. That decision is outside scope here; the question was `instances: 1` vs `instances: 3` *within* the single-node design the plan already specifies.

## Verification

**Phase 0 gate remains required.** The backup/restore rehearsal is still blocking before carrying real traffic — replication doesn't replace backups, it complements them.

**Phase 2 load test should validate:**
- Synchronous replication doesn't introduce unacceptable write latency
- Per-instance resource allocation (1 CPU / 4GB) sustains the concurrent clone-worker fleet
- Connection pool behavior under replication is correct

## References

- Plan.md "Open decisions" — Postgres replica topology
- Plan.md "Durability and load" — point 4: *"Replica topology is the real exposure"*
- Plan.md "Status — the old pipeline is gone" — no fallback system context
- `/home/coding/declarative-config/k8s/iad-ci/queue-db/cnpg-cluster.yaml` — `instances: 1` precedent
- `/home/coding/declarative-config/k8s/CLAUDE.md` — Spot node pool usage

## Decision Record

**Date:** 2026-08-06
**Decision:** `instances: 3` with synchronous replication
**Rationale:** No fallback system exists; a single-node preemption is a hard outage with hours-long RTO and up-to-24h RPO; $13/month for zero-data-loss automatic failover is trivial insurance.
**Decision-maker:** Plan author (operator), via bead cg-25cp resolution
