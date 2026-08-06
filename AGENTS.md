# commitgraph v2 — rules for agents working this repo

Read this before writing any CI, manifest, or deployment file. These are
org-wide standing rules, not preferences. A worker violated two of them on
2026-08-06 (added a GitHub Actions workflow and a `kind: CronJob`) because this
file did not exist yet.

The implementation plan is `docs/plan/plan.md`. Work is tracked as beads in
`.beads/` — use `bf`, never `br`.

## CI/CD — Argo Workflows only. Never GitHub Actions.

**GitHub Actions are disabled across every repo in this org and must never be
re-enabled.** Do not create `.github/workflows/*`. Anything you put there will
either fail and email the operator, or run untrusted work on a runner nobody
watches.

All CI runs on **Argo Workflows in the `iad-ci` cluster**. WorkflowTemplates
live in `jedarden/declarative-config` under `k8s/iad-ci/argo-workflows/`, and
ArgoCD syncs them on push.

If a bead's acceptance criteria say "runs in CI", that means an Argo
WorkflowTemplate — write one, and put a copy under this repo's `k8s/` if it is
useful to keep alongside the code.

## `kind: Job` and `kind: CronJob` are banned

ArgoCD cannot manage them idempotently, and CronJob-spawned Job pods are **not
owned by ArgoCD** — they are never pruned, and hold CPU and memory reservations
indefinitely. This has caused real scheduling failures on these clusters.

Use a **Deployment with an internal scheduling loop** instead. See
`k8s/invariant-2-audit-deployment.yaml` for the pattern: a `while true` loop
with `sleep`, the work wrapped in `timeout` to bound each run, and a failure
that logs rather than exits (exiting would crash-loop the Deployment and lose
the schedule).

## Cluster changes go through declarative-config + ArgoCD

Never `kubectl apply/create/delete/patch/edit/scale/rollout restart` against a
managed resource — not for triage, not "temporarily". ArgoCD `selfHeal` reverts
live edits anyway, so they do not stick and they fight the controller.

The only sanctioned path: edit the manifest in `jedarden/declarative-config` →
commit → push → let ArgoCD sync.

Read-only `kubectl get/describe/logs` is fine.

## Storage on Rackspace Spot

Always `storageClassName: sata` (or `sata-large`), always set **explicitly** —
never `ssd`/`ssd-large`, never left to the cluster default (`ord-devimprint`
defaults to `ssd`, which violates the rule).

Cinder volumes **cannot be expanded or reclassed in place**. Size correctly the
first time; the migration path is backup → restore into a larger PVC.

**Do not prune `queue-api-pvc.yml`.** `sata` has `reclaimPolicy: Delete`, so
removing that PVC destroys the volume holding `email_resolution`. Measured
2026-08-06: 966,679 rows, 59,745 positive resolutions, of which **3,821 are
AI-relevant** — real spent GitHub API budget that cannot be re-earned for
free. (Earlier drafts said "365K+ resolved pairs"; that was wrong.)

## Sizing on ord-devimprint

Six `compute1-4` nodes: **1.50 CPU / 2.54 GiB allocatable each**, ~9.00 CPU /
15.22 GiB total. Per-pod ceiling is **500m CPU / 1Gi memory**; a limit above one
node's allocatable can never be reached and guarantees an OOM kill. Always set
`resources.requests` — a container without them is BestEffort and is evicted
first.

## Images

Never `:latest`, never a bare git SHA. Pin to a **semver tag** read from
`containers/<name>/VERSION`, bumped in the same commit that changes the code.
Private `ronaldraygun/*` images need `imagePullSecrets`.

## Git

`git.ardenone.com` (Forgejo) is the source of truth; GitHub is a read-only
mirror updated by a server-side push mirror. Push only to `origin`.

**Never force-push.** If histories diverge, reconcile with a merge commit.

Commit at each completion point — ideally one commit per closed bead, not one
batched commit per session.

This is a ceiling on batching, not a floor on commit count: a bead that needed no
file change must not manufacture one. Record that outcome with
`bf update <id> --notes "..."` instead. Never write `notes/<bead-id>.md`, a
summary, or a status file to satisfy a commit requirement.

## Beads

`bf` is the CLI; `br` is deprecated. SQLite (`beads.db`) is the live store and
`issues.jsonl` is a checkpoint written by `bf sync --flush-only`.

- Never hand-edit anything under `.beads/`.
- Never `bf sync --flush-only` before `git pull` — it can overwrite a richer
  checkpoint and drop beads.
- After pulling someone else's beads, run `bf sync --import-only` before
  trusting `bf show`/`ready`.
- **`bf ready` currently ignores dependencies** and will hand you blocked work.
  Check a bead's blockers with `bf show <id>` before starting it.

## Honesty about blocked work

Several beads depend on cluster access, operator decisions, or credentials that
may not currently work (the `ord-devimprint` admin kubeconfig returns 401, for
example). If you cannot complete a bead, **say so and mark it blocked** — write
a note explaining precisely what is missing. Do not stub, fake, or simulate the
result. A worker did this correctly for `cg-3i96` when a data file was missing
from the box; follow that example.
