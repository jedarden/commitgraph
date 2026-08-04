# The commitgraph ↔ commitgraph-v2 name swap, and what it broke

## What happened

2026-08-04: `jedarden/commitgraph` (the live predecessor pipeline) was
renamed `jedarden/commitgraph-deprecated`; `jedarden/commitgraph-v2` (this
repo, then design-only) was renamed `jedarden/commitgraph`, taking over the
canonical name — on both Forgejo (source of truth) and its GitHub push
mirror, in the same operation.

Verified unaffected: ArgoCD's `commitgraph-ns-ord-devimprint` Application
syncs from `declarative-config`, never references `jedarden/commitgraph`
directly (`k8s/ord-devimprint/commitgraph-application.yml:12`) — live
cluster sync was never at risk from this rename.

## What was found and fixed in the same pass

- **Both Forgejo push-mirror configs pointed at the pre-rename GitHub
  URLs** — left as-is, the deprecated repo's mirror would have pushed into
  the new repo's GitHub mirror, and the new repo's mirror would have failed
  outright (its target name no longer existed). Deleted and recreated both
  against the correct renamed targets.
- **A GitHub webhook survived the rename attached to the old repo object**
  (GitHub webhooks don't follow a rename to a differently-named repo) —
  meaning a future push to the now-deprecated repo would still have fired
  a CI build, and that build would have cloned whatever now owns the
  `commitgraph` name (the new repo). Deleted the stray webhook; bumped the
  `github-webhooks` EventSource's restart annotation using the exact
  remediation pattern already documented in that file for this class of
  drift ("only registers at pod startup") so it re-registered against the
  new repo correctly.
- **`devimprint-queue-api-workflowtemplate.yml`'s `git-repo` default**
  (`jedarden/commitgraph`, unqualified) would have started cloning the
  new, code-empty repo and hard-failed on a missing
  `containers/queue-api/VERSION` the next time its trigger
  (`vibe-coding-discovery-push`, unrelated to commitgraph's own webhook)
  fired. Pinned explicitly to `commitgraph-deprecated`.
- **claude-leaderboard's `blocked_repos.json`** (both cluster copies) named
  `jedarden/commitgraph` to exclude the pipeline's own commits from its
  ranking — updated to `commitgraph-deprecated` so it still excludes the
  repo it meant to.
- Three prose source-location comments across declarative-config, and both
  repos' own README cross-references, updated to match.

## What was missed in that same pass, found by a later adversarial review

**`commitgraph-build-workflowtemplate.yml`'s `git-repo` default was left as
the bare `jedarden/commitgraph`.** This was actually *seen* during the
original sweep — the grep output included it — but reasoned about
incorrectly: since the bare name now correctly resolves to the new repo,
it was judged "fine, no fix needed." That reasoning stopped one step short.
This is the **general, auto-triggered build template**, wired to the
correctly-fixed webhook above. Once that webhook started firing on pushes
to the new repo, this template started actually running against it — and
found no `containers/` directory (the new repo is docs-only), so
`detect-changes` silently emits nothing to build, while the
`run-quality-gates` step hard-fails trying to `pip install -e .[test]`
against a repo with no `pyproject.toml`. Simultaneously, the now-deprecated
repo — the one still actually live, still receiving real fixes (a NEEDLE
worker landed `fix(aggregator): force chunk_size=1...` on it hours after
this rename) — lost its own CI trigger entirely and has none as of this
writing.

**Lesson:** fixing a hardcoded repo-name reference isn't complete just
because the string now points somewhere technically valid — it has to be
checked against what that destination *actually contains* at the time the
reference will next be exercised, not just whether the name resolves.
"Correctly resolves" and "produces a sensible build" are different claims.

**Status of this gap:** flagged, not yet fixed as of this writing — restoring
a working CI trigger for `commitgraph-deprecated` (a new eventsource entry
+ sensor trigger with a `git-repo` parameter override, since simply
pointing another webhook at the same endpoint would still build the wrong
repo) is real infrastructure work, held pending an explicit go-ahead rather
than done automatically alongside a documentation/planning pass.

## Scope of the reference sweep — bounded, not exhaustive

The sweep that caught the fixed items above was `grep -rn
"jedarden/commitgraph\b"` against `declarative-config` specifically, plus a
GitHub API check of live webhooks. It was **not** a search of: other repos'
own docs/READMEs that might link to the old URL, any beads/issue-tracker
content referencing the old repo, NEEDLE worker configs or workspace paths
pointed at a `commitgraph` directory name, or anything outside
declarative-config entirely. Treat "every found reference... updated" as
"every reference this specific bounded search found," not as a completeness
guarantee.
