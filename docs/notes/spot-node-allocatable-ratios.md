# Spot Node Allocatable Ratio Measurements — ord-devimprint

**Date:** 2026-08-08

## Purpose

Verify the allocatable-versus-capacity ratio for the newly provisioned dedicated Spot node (`mh.vs1.large-ord`), as the plan.md architecture section noted that only `compute1-4`'s ~75% CPU / ~70% memory ratio had been empirically confirmed.

## Node Verified

- **Name:** `prod-instance-17862171401240367`
- **Status:** Ready (schedulable)
- **Age:** ~56 minutes at time of verification
- **Provisioned class:** `mh.vs1.large-ord` (confirmed via `servers.ngpc.rxt.io/class` label)
- **Kubernetes instance type:** `memory1-30` (via `node.kubernetes.io/instance-type` label)

## Measurements

### mh.vs1.large-ord (new Spot node)

| Resource | Capacity | Allocatable | Ratio |
|----------|----------|------------|-------|
| CPU | 4000m (4 vCPU) | 3500m | **87.5%** |
| Memory | 30795724Ki (~29.4 GiB) | 29644748Ki (~28.3 GiB) | **96.3%** |

### ch.vs1.medium-ord / compute1-4 (baseline nodes)

| Resource | Capacity | Allocatable | Ratio |
|----------|----------|------------|-------|
| CPU | 2000m (2 vCPU) | 1500m | **75.0%** |
| Memory | 3810024Ki (~3.6 GiB) | 2659048Ki (~2.5 GiB) | **69.8%** |

## Key Findings

1. **The `mh.vs1.large-ord` class has materially better allocatable ratios than `compute1-4`:**
   - CPU: **87.5% vs 75.0%** (+12.5 percentage points)
   - Memory: **96.3% vs 69.8%** (+26.5 percentage points)

2. **The `compute1-4` Kubernetes label corresponds to `ch.vs1.medium-ord` in the Rackspace Spot pricing API** — confirmed via the `servers.ngpc.rxt.io/class` label.

3. **Do not assume allocatable ratios are consistent across Spot classes** — the `mh.vs1.large-ord` class retains significantly more of its advertised capacity for pods than `compute1-4` does.

## Implications

- The `mh.vs1.large-ord` class is more efficient than `compute1-4` — less capacity is lost to system overhead.
- Cost modeling should use measured ratios per node class, not a generic assumption.
- When provisioning Postgres on dedicated nodes, the `mh.vs1.large-ord` class's higher allocatable ratios mean better utilization of the paid capacity.

## Raw Data

**New node (mh.vs1.large-ord):**
```bash
kubectl get node prod-instance-17862171401240367 -o json | jq '{capacity: .status.capacity, allocatable: .status.allocatable}'
```

**Baseline node (ch.vs1.medium-ord / compute1-4):**
```bash
kubectl get node prod-instance-17854686092870239 -o json | jq '{capacity: .status.capacity, allocatable: .status.allocatable}'
```
