# Bead cg-1qms — OBSOLETE by Architecture Correction

**Status:** This bead is superseded by the 2026-08-05 architecture correction and must NOT be executed.

## What this bead asked for

Disable queue-api manifests + PVC after extraction verification.

## Why this is obsolete

Both `docs/plan/plan.md` and `declarative-config/k8s/ord-devimprint/commitgraph/TEARDOWN.md` were corrected on 2026-08-05 to state:

> **queue-api is permanent infrastructure in this design** — it owns `search_queue`/`repo_queue`/`user_queue` claim-lease semantics, `repo_head_cursors`, `catalog_version`, and the email-resolution work queue. Only the resolution results move out to Postgres.

From `plan.md` Phase 6 (lines 1504-1519):

> **Corrected 2026-08-05: queue-api is NOT decommissioned.** An earlier draft of this phase said to extract the remaining tables and then `.disabled` the queue-api manifests and PVC — which directly contradicted Phase 1, where the preserved instance is *reused* as the new pipeline's job coordinator. [...] Phase 1 is the correct one: **queue-api is permanent infrastructure in this design**.

From `TEARDOWN.md` (lines 41-54):

> **queue-api is PERMANENT — corrected 2026-08-05**
>
> So this is not a temporary reprieve pending extraction. **Do not disable or prune the queue-api Deployment, Service, or PVC at any point** — the `sata` `reclaimPolicy: Delete` hazard applies indefinitely, not just until the data is copied.

## Architectural conflict resolution

The conflict flagged in this bead's description ("this appears to conflict with `docs/plan/plan.md` Architecture's 'queue-api: kept as-is'") was resolved by choosing the "kept as-is" path and correcting the Phase 6 language that suggested tearing it down.

## Correct Phase 6 work

The actual Phase 6 work is tracked by epic `cg-y4ti1`, which explicitly notes "queue-api is permanent infrastructure and is NOT decommissioned." Valid Phase 6 tasks:
- `cg-5the`: Restore CI trigger for commitgraph-deprecated
- `cg-66js`: Fix workflow template for docs-only repo
- `cg-1zwwl`: Remove dirty_partitions from queue-api (safe cleanup, not removal)

## Action taken

Closed this bead as OBSOLETE. Executing it would break the pipeline by removing queue-api, which is now designated as permanent coordination infrastructure for the v2 design.

## Acceptance criteria status

- ❌ `mig-phase6-extract-verify-counts` closed first — YES, but irrelevant now
- ❌ Explicit reconciliation with "queue-api kept as-is" — Already happened 2026-08-05; conclusion was the OPPOSITE of this bead's premise
- ❌ `.disabled` files pushed — Intentionally NOT done; would be destructive
- ❌ PVC destruction — Intentionally NOT done; queue-api PVC is permanent

**This bead was created before the architecture correction and should not have been part of the Phase 6 epic.**
