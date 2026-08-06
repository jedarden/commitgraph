# ARMOR Cross-Namespace Coupling Decision (cg-4nlj)

**Date**: 2026-08-06  
**Status**: Accepted

## Decision

**Reuse the existing `devimprint`-namespace ARMOR instance** for the commitgraph pipeline. Do not stand up a new `commitgraph`-scoped ARMOR deployment.

## Rationale

### The SPOF concern is materially narrower than ADR-009's original worry

ADR-009 objected to ARMOR on three grounds:
1. Whole-file encryption defeats DuckDB range-read pruning
2. Proxy-as-SPOF
3. Cross-namespace coupling

The new design materially changes (1): ARMOR is no longer in the hot ranking-query path. Every live ranking query goes directly to Postgres; ARMOR is only in the write path for:
- Clone-worker's raw-history artifact write (per-repo Parquet)
- Aggregator's periodic snapshot publish
- Warm-start artifact storage

This means ARMOR being briefly unavailable **delays extraction and publishing, but does not take down ranking**. The blast radius is extraction/publishing latency, not query availability.

### The cross-namespace coupling is acceptable as a tradeoff

**What we accept**: clone-worker (namespace `commitgraph`) depends on ARMOR running in namespace `devimprint`. This is the exact shape ADR-009 wanted to move away from.

**Why it's acceptable here**:
1. **Operational simplicity**: Reusing battle-tested, proven infrastructure is materially better than provisioning and maintaining a new ARMOR deployment. The `devimprint` instance is already operational and understood.
2. **Same-cluster dependency**: Both namespaces are on `ord-devimprint` (cross-namespace, not cross-cluster). This is a narrower coupling than cross-cluster dependencies.
3. **Clear dependency pattern**: commitgraph writes to ARMOR, but ARMOR requires no knowledge of commitgraph. This is a unidirectional dependency, not a bidirectional coupling.
4. **Cold access pattern**: ARMOR is cold-storage for artifacts in this design, not hot-path for queries. The access frequency is orders of magnitude lower than the old architecture's every-aggregator-cycle pattern.

### Why a new scoped deployment is not justified here

A new `commitgraph`-scoped ARMOR deployment would isolate the coupling ADR-009 objected to, but the costs outweigh the benefits:

**Costs**:
- New deployment to operate and maintain
- Additional ARMOR instance to monitor and debug
- Duplicate infrastructure when existing instance is operational
- No material improvement in blast radius given the narrowed SPOF exposure above

**Benefits**:
- Namespace isolation — but this is cross-namespace only, not cross-cluster
- Independent lifecycle — but ARMOR failures already only affect extraction/publishing, not queries

The tradeoff is not favorable given the SPOF concern is already materially narrowed.

## ARMOR instance details

From plan.md "Storage placement" section: four org-wide ARMOR instances exist:
- `devimprint` namespace on ord-devimprint
- `armor` namespace on iad-ci  
- `armor` namespace on iad-kalshi
- `armor` namespace on rs-manager

The `devimprint` instance is the one in scope for reuse.

**Note**: `ARMOR_PREFIX` is currently unset on this instance (dedicated-bucket mode). This needs explicit scoping decided before commitgraph writes to it — see "ARMOR instance/prefix scoping" in plan.md Open decisions.

## Open questions for follow-up

This decision addresses the cross-namespace coupling question but leaves related scope questions for Phase 0:
1. **ARMOR_PREFIX scoping**: Decide the prefix/key structure commitgraph will use (`commitgraph/` or similar) to avoid key collision with other devimprint consumers
2. **Verification**: Empirically verify ARMOR's actual range-read behavior before relying on it (see plan.md's "seekability verification" callout)

## References

- ADR-009: `/home/coding/commitgraph-deprecated/docs/adr/009-encrypted-public-b2-storage.md`
- Plan.md "Storage placement" section: `/home/coding/commitgraph/docs/plan/plan.md#storage-placement`
- Plan.md "Open decisions": `/home/coding/commitgraph/docs/plan/plan.md#open-decisions` (line 178-180)
