# Can a stored, previously-cloned repo be warm-started to avoid re-downloading full history on rescan? Empirically tested, 2026-08-04

## The question

clone-worker's rescan cycle (queue-api's default 24h resync sweeper re-pends
every repo in the corpus, whether it changed or not) currently does a fresh
`git clone --bare --filter=blob:none <url>` from scratch on every single
scan — confirmed via live clone-worker logs earlier this session
(`cloned repo=X size=Y` immediately precedes the `incremental extraction`
step; extraction is incremental, cloning is not). The question: can a
previously-cloned repo's state be materialized on a *different* worker
replica (the fleet has no per-repo pinning) well enough that `git fetch`
retrieves only the commits added since the last scan, instead of the full
history every time? This matters because clone throughput (~1,000
repos/hour/replica, independently measured, see `docs/plan/plan.md`) is a
named, unaddressed hard ceiling — and unlike discovery's GitHub search-API
budget, git fetch bandwidth is a completely different, unclaimed resource.

**Answer: yes, the underlying mechanism works and was verified end-to-end
— but the specific transport originally proposed (`git bundle create`) is
actively wrong and was rejected after testing. A raw pack-file transport is
the validated alternative.**

## Methodology

Real public repo, not a synthetic fixture: `jedarden/NEEDLE` (887 commits at
test time, a real Rust project — large enough to be representative, small
enough to iterate on quickly). git 2.47.3. All clones used
`--filter=blob:none`, matching clone-worker's actual flag exactly. Every
test that mattered was re-run from a **completely fresh clone** after an
earlier pass turned out to be contaminated by intermediate `gc`/`repack`
experimentation — noted below, because the contamination was itself a real
finding, not just a mistake to bury.

## Finding 1 — the core mechanism works, cleanly, when done right

Rewound a fresh clone's `refs/heads/main` 30 commits back (858 of 887
commits), transported *only* the raw pack files, that specific ref, and
three required config values (see Finding 3) to a brand-new empty
directory — simulating a different clone-worker replica materializing a
stored snapshot — then ran `git fetch origin main` against the real GitHub
remote. Result: **0.4–0.8s**, a **305-byte** negotiation request, correctly
resolved `FETCH_HEAD` to the exact current tip, and reached the full,
correct 887-commit history. This validates the fundamental premise: a
replica that only has an *old* snapshot of a repo's objects, once
correctly reconstructed, gets the new commits as a small, fast, genuinely
incremental fetch — not a full re-clone.

## Finding 2 — `git bundle create` is not a viable transport for this

The original proposal (see prior conversation) was to store a `git bundle`
per repo in ARMOR. Tested directly: `git bundle create` against the
rewound, blob-filtered clone produced a **105MB bundle from an 833KB source
pack** — a **~127x** size increase, for *fewer* commits than the source
(858 vs. 887) — and took **22.6 seconds / ~60 CPU-seconds** to create, for
a repo of well under 1,000 commits.

**Confirmed filter-specific, not general bundle behavior**, via a control
test: an ordinary *unfiltered* clone of a small repo (`octocat/Hello-World`,
13 objects, 2KB pack) bundled to **1,833 bytes** — smaller than the source,
created in 0.012s. The bloat only appears when bundling a `--filter=blob:none`
partial/promisor clone.

**Second, independent problem with the same root cause**: creating the
bundle left a bloated ~102MB pack behind in the *source* repo's own object
store as a side effect — this is what caused an earlier, initially
inexplicable observation (a repo that measured 988K/4,500 objects right
after cloning later measured 101M/8,447 objects with no `rm`/mutation
command run against it in between). The bundle-creation step itself was the
cause, not the `gc`/`prune` experimentation that had been the leading
suspect at the time.

**Conclusion: reject `git bundle create` as the ARMOR transport for this.**
Under the plan's whole-object-overwrite pattern (recreate the artifact
fresh on every scan, matching how the Parquet extraction is already
handled), this cost would be paid on *every single clone-worker job* — a
severe throughput regression risk, directly working against the reason this
optimization exists.

## Finding 3 — the validated alternative: raw pack files, not a bundle

Packaging the *original* pack files directly (`objects/pack/*.pack`,
`.idx`, `.promisor`, `.rev` — no repacking, no `git bundle`) plus the
specific rewound ref, tarred: **796KB** — matching the source almost
exactly, no bloat, created near-instantly.

This is not quite drop-in, though — a naive version of this **failed** on
the first attempt (`fatal: pack has 49 unresolved deltas` /
`fatal: fetch-pack: invalid index-pack output`), which led to two more real
findings, both confirmed by fixing them one at a time and re-testing:

1. **`packed-refs` is a stale snapshot, not the live ref state.** The
   first raw-pack attempt tarred `packed-refs` instead of the loose ref
   file — `packed-refs` reflects clone-time state and does **not** update
   when a loose ref override (e.g., `git update-ref`) is written elsewhere;
   the loose ref file (`refs/heads/<branch>`) must be the thing
   transported, or the "rewind" is silently lost and the fetch has nothing
   new to retrieve (confirmed: that specific run showed a 305-byte
   negotiation and zero object growth, because the target was already at
   the current tip).
2. **A partial/promisor clone's pack is unusable without matching repo
   config, even though the pack file itself is complete and correct.**
   The original clone's `.git/config` carries `core.repositoryformatversion
   = 1`, `remote.origin.promisor = true`, and
   `remote.origin.partialclonefilter = blob:none`. Without these three
   values, git has no way to know that trees referencing missing blob
   objects are *expected* (by design, under `blob:none`) rather than
   corruption — hence "unresolved deltas." Adding exactly those three
   config values to the target repo, with no other change, made the
   identical pack fetch cleanly and correctly. This was verified as the
   complete fix, not a guess: same tarball, same pack, only the config
   changed, fetch succeeded.

**So the artifact that would actually need to travel through ARMOR for
this to work is: the raw pack directory + the specific ref + these three
config values** — not a `git bundle`, and not just "the pack files."

## Practical gotchas found along the way (secondary, but real if bundles are ever revisited)

- `git clone <bundle-file> <dir>` auto-creates `origin` pointing at the
  *bundle's file path*, not a real URL — `git remote add origin <url>`
  fails ("already exists"); must be `git remote set-url origin <url>`.
- Cloning from a bundle doesn't reliably attach `HEAD` to the bundle's
  actual branch if the local `init.defaultBranch` differs from the
  bundle's ref name (`main` vs. `master`) — needs an explicit
  `git symbolic-ref HEAD refs/heads/<actual-branch>` fix-up.
- Both gotchas are moot if bundles are rejected in favor of the raw-pack
  approach (Finding 3), which sidesteps both by construction — recorded
  here in case a future implementer reconsiders bundles for some other
  reason.

## What this means for the plan

The underlying optimization is real and empirically validated — this
should be pursued, but as a **raw-pack-tarball artifact** (objects/pack/*
+ ref + the three promisor config values), stored in ARMOR alongside (not
replacing) the existing Parquet extraction, **not** as a `git bundle`. This
corrects the mechanism sketched in prior conversation before this research
was done. **Reflected 2026-08-04 (gap-review round 5): plan.md's
Architecture section ("The warm-start artifact is a raw pack-file
transport, not a `git bundle`") now fully incorporates this research's
findings almost verbatim** — the 127x bundle-bloat rejection, the three
required promisor config values, and the loose-ref-vs-packed-refs gotcha
are all carried over. This note is no longer a live gap.

## Not tested — open questions for a future pass

- Behavior at real corpus scale: NEEDLE is a few hundred commits; the
  corpus includes far larger repos. Pack size, tarball transfer cost, and
  whether the config-based approach holds at that scale are unverified.
- Whether ARMOR's own write/read path handles a multi-file artifact
  (pack + idx + promisor + rev + ref + config, several files) as cleanly
  as the single-file Parquet artifact — this doc assumes a tarball
  wrapping them into one object, not verified against ARMOR directly.
- Concurrent access: this was tested as a clean sequential handoff between
  two directories, not under the fleet's actual multi-replica
  claim/lease/re-queue conditions.
- Whether `--filter=blob:none`'s specific promisor-pack behavior is
  consistent across git versions other than 2.47.3.
