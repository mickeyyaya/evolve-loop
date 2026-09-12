# Fix — a resumed cycle disposes of its worktree (2026-09-12)

Closes the fourth resume-parity hypothesis from the 2026-09-11 decomposition
rescan (`docs/reports/2026-09-11-large-component-decomposition-plan.md`, item 6d).
Format per the operator directive: issue → gap → solution.

## Issue

Every resumed cycle leaked its worktree, whatever its verdict. Twenty-six stale
`cycle-*` checkouts were sitting under the live runtime's `.evolve/worktrees`
when this was written — each a full source tree.

Mechanism: `newCycleRun` (`go/internal/core/cyclerun.go`) builds a four-entry
LIFO cleanup stack — lock release, run-ID clear, worktree prune/preserve, lease
stop — and `RunCycle` defers it. `RunCycleFromPhase` (`resume.go`) registered
three of those four by hand and no worktree teardown at all. The decision was
never wrong: `completeCycle` is shared, so `finalizeCycle` computed
`preserveOnVerdict` on the resume path too and stored it on the `cycleRun`.
Nothing on the resume path ever read it.

## Gap

The prune/preserve rule lived as an anonymous closure inside `newCycleRun`'s
stack, reachable only through the fresh entrypoint. A parallel entrypoint that
re-implemented the exit actions by hand had nothing to call, and no test drove
a resume to normal completion and asked what happened to its tree.

A second gap surfaced while fixing the first, and it is the more important one.
The first version of the fix pruned on the orchestrator's exit path exactly as
the fresh path does — and an existing test
(`TestRunCycleFromPhase_ResumedBuildUsesCorrectionAndLifecycleReview`) failed
because its checkpoint names the **project root** as the cycle's worktree. That
is a real production shape: `phases/ship/gitops.go` routes "a cycle whose
worktree resolves to the project root" to `shipDirect`. The fresh path can never
hit it (its path always comes from `worktree.Create`), but resume takes
`ActiveWorktree` from a persisted checkpoint. `gitWorktree.Cleanup` then runs
`git worktree remove --force` — a harmless failure on the main tree — followed
by `os.RemoveAll(worktree)` **unconditionally**. Giving resume the fresh path's
teardown, without more, would have deleted the repository.

## Solution

- **One rule, both entrypoints.** The prune/preserve decision is extracted
  verbatim into `(*Orchestrator).teardownCycleWorktree`
  (`cycle_worktree_teardown.go`); `newCycleRun`'s stack entry delegates to it,
  and `RunCycleFromPhase` defers it in the same LIFO position. The defer reads
  the closeout `cycleRun`'s `preserveWorktree` / `cycleCompletedNormally` live
  at exit — the same fields, on the same type, the fresh closure reads. The two
  no-prune conditions are unchanged on purpose: a missed prune costs disk; a
  wrong prune is the cycle-7 incident.
- **The provisioner refuses the project root.** `gitWorktree.Cleanup`
  (`worktree.go`) returns an error, loudly, when the path denotes the project
  root — beside `deleteCycleBranch`'s existing "cycle-" gate, and for the same
  reason: never act on a path this provisioner did not create. Placing it in the
  provisioner rather than the orchestrator covers every caller of `Cleanup` and
  leaves the fresh path's behavior with injected fakes untouched. `sameDirectory`
  decides identity by `os.SameFile` (device + inode: symlinks, case-insensitive
  APFS) when both paths exist, OR by cleaned-absolute text when one does not, so
  the predicate errs toward refusing.
- **Tests.** `resume_worktree_teardown_test.go` drives `RunCycleFromPhase`
  (not the helper — a helper that exists and is never deferred was the defect):
  normal completion prunes; a FAIL verdict preserves; an abnormal exit
  preserves; a project-root checkpoint survives an end-to-end run through the
  REAL provisioner (the syscall is pinned, not a fake's request log); the
  provisioner refuses the root, its symlink alias, and its trailing-separator
  spelling; and a `sameDirectory` table. Every guard was shown to fail by
  assertion under a reverted mutation — including that removing the provisioner
  refusal makes the end-to-end test delete its own temp repository.

## Not changed

The resume-only terminal `WriteCycleState` after `completeCycle` can fail after
`cycleCompletedNormally` is latched, so a non-FAIL tree is pruned while the
checkpoint block may still be live. That direction is intended and documented
in place: ship has merged the work, the tree is spent, and a later `--resume`
then fails loudly rather than re-driving phases on a spent tree.
