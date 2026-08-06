# ARMOR Instance Scoping Decision (cg-2bdg6)

**Date**: 2026-08-06
**Bead**: cg-2bdg6
**Status**: Completed

## Decision

**Chosen ARMOR instance**: `devimprint` namespace ARMOR instance on `ord-devimprint` cluster

**Cross-namespace coupling**: Accepted. The commitgraph pipeline (namespace `commitgraph`) will depend on ARMOR running in namespace `devimprint`.

## Rationale

### Why the `devimprint` instance?

1. **Same-cluster dependency**: Both namespaces are on `ord-devimprint` (cross-namespace, not cross-cluster)
2. **Operational simplicity**: Reusing battle-tested, proven infrastructure vs. provisioning new deployment
3. **Cold access pattern**: ARMOR is cold-storage for artifacts in this design, not hot-path for queries
4. **Unidirectional dependency**: commitgraph writes to ARMOR; ARMOR requires no knowledge of commitgraph

### Why accept cross-namespace coupling?

The SPOF concern is materially narrower than ADR-009's original worry:
- ARMOR is **not** in the hot ranking-query path (all ranking queries go directly to Postgres)
- ARMOR unavailability **delays extraction/publishing, but does not take down ranking**
- The blast radius is extraction/publishing latency, not query availability

### Why not a new scoped deployment?

A new `commitgraph`-scoped ARMOR deployment would isolate the coupling ADR-009 objected to, but:
- **Costs**: New deployment to operate/monitor/debug
- **Benefits**: Namespace isolation (already cross-namespace only, not cross-cluster) and independent lifecycle (ARMOR failures already only affect extraction/publishing, not queries)
- **Tradeoff**: Not favorable given the narrowed SPOF exposure

## Implementation notes

**`ARMOR_PREFIX` scoping**: While the instance is chosen, the prefix/key structure commitgraph will use (e.g., `commitgraph/` prefix) remains for Phase 0 implementation to avoid key collision with other `devimprint` ARMOR consumers.

This is an implementation detail, not an architectural decision — the chosen ARMOR instance is the `devimprint` one regardless of the specific prefix used.

## References

- Cross-namespace coupling rationale: `docs/notes/cg-4nlj-armor-cross-namespace-decision.md`
- plan.md "Storage placement" section: `/home/coding/commitgraph/docs/plan/plan.md#storage-placement`
- plan.md "Open decisions": `/home/coding/commitgraph/docs/plan/plan.md#open-decisions` (updated)
