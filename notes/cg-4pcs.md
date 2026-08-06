# Clone-Worker Replica Count Decision Verification (cg-4pcs)

**Date:** 2026-08-06
**Bead ID:** cg-4pcs
**Status:** VERIFIED AND DOCUMENTED

## Summary

The clone-worker replica count decision (3 replicas) has been verified as already complete and properly documented.

## Decision Confirmed

**Replica Count:** 3 replicas (baseline deployment)

## Documentation Verification

### 1. Plan.md Update (Durability and Load, Point 6)
- ✅ Point 6 is marked as "resolved 2026-08-06"
- ✅ States "3 replicas is the baseline deployment"
- ✅ References `docs/notes/cg-4pcs-clone-worker-replica-count.md` for full rationale
- ✅ Explicitly ties the number to confirmed `compute1-4` headroom constraints
- ✅ Mentions Phase 2 load test targets (3 baseline, 4-5 ceiling)
- ✅ References PgBouncer pool sizing input

### 2. Detailed Rationale Documented
- ✅ Full decision rationale exists in `docs/notes/cg-4pcs-clone-worker-replica-count.md`
- ✅ Includes capacity headroom constraint details (1.20 CPU / 1.64 GiB)
- ✅ Shows clone-worker resource profile (512 MiB memory per pod)
- ✅ Provides headroom math demonstrating ~3 pods fit per slot
- ✅ Explains why 3 replicas (not 4) based on memory constraint
- ✅ Documents Phase 2 load test targets
- ✅ Explicitly lists inputs to PgBouncer sizing work

## Acceptance Criteria Status

All acceptance criteria from the bead have been met:

- [x] A specific replica count (integer) is chosen and documented → **3 replicas**
- [x] Rationale ties the number to `capacity-check.sh`'s measured headroom → Yes, uses 1.20 CPU / 1.64 GiB constraint
- [x] The number is recorded in plan.md "Durability and load" (resolving point 6) → Yes, point 6 resolved
- [x] The number is recorded in docs/notes → Yes, in `cg-4pcs-clone-worker-replica-count.md`
- [x] The number is referenced as an explicit input by the PgBouncer sizing work → Yes, explicitly referenced

## Key Rationale Points

The decision is based on:

1. **Capacity Constraint:** Memory headroom limits us to ~3 pods per largest slot (1.64 GiB / 512 MiB ≈ 3.2 pods)
2. **Throughput Adequacy:** 3 replicas = 3,000 repos/hour at measured ~1,000 repos/hour/replica ceiling
3. **Load Test Baseline:** Provides concrete starting point for Phase 2 load testing
4. **Connection Pooling:** 3 replicas with PgBouncer can safely service write workload

## Phase 2 Load Test Plan

- **Baseline:** 3 replicas at steady state
- **Headroom test:** 4-5 replicas to establish ceiling
- **Pass/fail:** Postgres p99 < 2s, PgBouncer < 80% utilization, no timeout errors

## No Additional Work Required

The decision was already complete and properly documented. This verification confirms all requirements are satisfied and the bead can be closed.
