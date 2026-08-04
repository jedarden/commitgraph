# Considered: real GitLab/Bitbucket discovery parity

**Status: captured for later consideration, not adopted into the core plan.**
A genuine scope expansion, not a fix to the current redesign — recorded here
rather than folded into Architecture since it changes what gets discovered,
not how the pipeline processes what it already discovers.

## The idea

`shared/provider_adapter` already implements GitHub, GitLab, and Bitbucket
adapters (`base.py`, `github.py`, `gitlab.py`, `bitbucket.py` all exist in
the current codebase) — but `search-worker`'s live discovery footprints
(`GITHUB_FOOTPRINTS`) and the whole flywheel's proven throughput are
GitHub-only. Bring GitLab (and Bitbucket, noting it has no commit-search
API per `search-worker`'s own startup check — `PROVIDER=bitbucket` currently
just logs an error and returns) up to genuine feature parity with the
GitHub discovery path.

## Why it's worth having

Real scope expansion using infrastructure that's already been built and
sitting mostly unused — `provider_adapter`'s GitLab support hasn't been
confirmed working end-to-end in production, only assumed to exist because
the code is there. Low marginal build cost if the adapter genuinely works
as designed.

## Why it's not adopted directly into the plan

This plan's entire framing is about fixing the write-contention/freshness
architecture for the *existing* GitHub-scoped corpus — adding a second
provider's worth of discovery is orthogonal scope, not a redesign concern.
It doesn't belong in Architecture, Storage placement, or the phased rollout
of this specific plan.

## The real tension worth naming

Discovery/clone throughput are explicitly framed elsewhere in this plan as
hard, *unclosed* ceilings this redesign doesn't touch. Adding GitLab
discovery increases total clone-worker workload against that same
already-strained capacity (1,000 repos/hour/replica, tens of thousands
already pending). The mitigating factor: GitHub's ~30 req/min search budget
and GitLab's equivalent are almost certainly *separate* rate-limited
resources — so this doesn't compete with the specific ceiling already
flagged as maxed out, it adds a new, independent one. Still worth confirming
that assumption (and that the GitLab adapter actually works against a real
GitLab API) before treating this as free.

## First step if picked up later

Confirm `search-worker`'s GitLab code path actually claims and processes a
job end-to-end against a real GitLab instance — this has apparently never
been verified live, only assumed correct because the adapter code compiles.
