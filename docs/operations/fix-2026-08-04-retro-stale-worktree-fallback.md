# Fix — retro stale-worktree fallback (built cycle-1278, landed cycle-1283)

Closes the verified-open half of the cycle-1255 CRITICAL recorded as Finding F1 in
[batch-integrity-review-2026-08-04.md](batch-integrity-review-2026-08-04.md).
Format per the operator directive: issue → gap → solution.

## Issue

A fleet lane whose worktree had been torn down lost its retrospective entirely —
a failure in the failure-handler, so the cycle that most needed a post-mortem was
the one that could not produce one.

Mechanism: `cs.ActiveWorktree` (`go/internal/core/cyclerun.go`) had exactly one
assignment — at worktree creation — and nothing cleared it when the teardown
callback pruned the directory. The persisted cycle state therefore kept naming a
deleted path. The next dispatch read that path, handed it to the retro phase, and
the bridge's launch guard (`driver_tmux_repl.go`, `!IsDir(workingDir)`) refused
the launch with `ExitBadFlags` — stderr only, no error return, so the refusal was
silent to the orchestrator.

## Gap

The defect was tracked through a salvage chain (1255 → 1268 → 1270 → 1272) that
progressively narrowed it to the shape it had fixed. Cycle-1270 taught
`retroWorktree` to fall back to the workspace-owned scratch cwd when
`req.Worktree == ""`, and cycle-1272's CHANGELOG entry (`68322bdf`) declared the
finding verified-closed.

`""` was never the shape a torn-down lane produces. A pruned lane leaves a
**non-empty but stale** path, which the `== ""` condition passed through verbatim
into the guard's refusal. The four existing retro worktree tests all pinned the
empty shape, so the coverage looked complete while the live failure mode was
untouched — the narrowing, not the code, is what made this look closed.

## Solution

Two changes: one containing the symptom, one removing the source.

1. **`go/internal/phases/retro/retro.go`** — `retroWorktree` now falls back when
   `fleetMode(req) && !gobridge.IsDir(req.Worktree)`, i.e. it tests the
   guard's own predicate instead of a string shape. Empty and stale are one
   contract: both are "no usable worktree". A live worktree still passes through
   verbatim (a fallback that fired unconditionally would strand every normal
   fleet retro in a repo-less scratch dir), and non-fleet dispatch is untouched —
   outside fleet mode the bridge keeps its process-cwd fallback and reports the
   bad dir to the operator, and retro must not silently rewrite an operator's
   designated worktree.

2. **`go/internal/core/cyclerun.go`** — the lane-teardown callback now calls
   `clearActiveWorktree` once `o.worktree.Cleanup` **succeeds**, so the record
   stops naming a directory that no longer exists. It is a read-modify-write
   against storage, not a rewrite of the `newCycleRun`-era snapshot: by teardown
   the run has persisted later phase progress, which writing the init snapshot
   back would discard. A path guard makes it a no-op if the record has since
   moved on. A **preserved** worktree keeps its path — `evolve loop --resume` and
   `evolve cycle reset` reclaim the lane by it, and clearing unconditionally
   would trade a stale path for permanently orphaned audited work (the cycle-7
   lost-work incident).

`isDir` was exported as `bridge.IsDir` rather than re-derived in the phase: a
launch-refusal predicate with two definitions can drift, and this defect is what
that drift costs.

## Verification

`go/acs/cycle1278` (4 predicates / 9 subtests) plus the acceptance tests in
`go/internal/phases/retro/retro_stale_worktree_test.go` and
`go/internal/core/cyclerun_worktree_teardown_test.go`, all RED before this change
for the stale and teardown cases and PASS after. The three retro input shapes —
empty, live, stale — and the non-fleet passthrough are each pinned separately.

## Follow-up

The CHANGELOG closure claim sealed by `68322bdf` was struck and corrected in the
cycle-1283 landing (see the `retro-fleet-worktree-dispatch` entry, now marked
**Corrected in cycle-1283**), together with the note on why the cycle-1272
machine guard could not catch the over-claim: it re-ran the cited proof but never
checked that the proof covered the claim.

Still open, deferred out of the cycle-1283 fleet scope rather than fixed:
1267-F2 (`DirectImporters` unbounded parse), 1267-F3 / 1270-R-1 (`ScratchCwd`
symlink hardening), the `continuation-defect-ledger` class fix, and
`acs-eval-suite-stale-shape-hardening`. Dispositions are recorded in the F1
landing record of
[batch-integrity-review-2026-08-04.md](batch-integrity-review-2026-08-04.md).
