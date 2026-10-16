# internal/guards/treediff

## Purpose

`internal/guards/treediff` is the post-phase main-tree diff guard. The bridge's OS sandbox confines source-writing phases to their worktree at run time. This guard is the second line: it proves after the phase that nothing escaped.

## Design

- **Dirty set before and after.** A git worktree is a separate working directory. `git diff --name-only HEAD` in the main repo root shows only main-tree changes, never worktree changes. `Snapshot` records the dirty set before the phase. `Check` compares it with the set after the phase. A path that is dirty after and was not dirty before is a leak.
- **A seam, not a global.** `New` takes a `GitDirtyFn` (git-exec function value), so the orchestrator injects the production git call and tests inject a fake. The package has no globals. A nil seam builds a disabled guard: `Snapshot` yields no paths and `Check` returns a zero `CheckResult`, so a disabled guard never blocks.

## Invariants

- **The guard never causes a false abort.** It backs up the sandbox. An error from the git seam, before or after the phase, degrades to `SnapshotMissed`, and `Error` then says that leak detection was skipped. It is not a leak message. Pinned by `TestGuard_PostPhaseGitFails_DegradesNotAborts` and `TestCheckResult_Error_SnapshotMissed_DegradeMessage`.
- **Only new dirtiness is a leak.** A path that was already dirty before the phase (the operator's uncommitted work) is not one. Pinned by `TestGuard_PreviouslyDirty_NotALeak`.
- **A clean result gives a nil error.** The orchestrator reads nil as "the phase respected its worktree". Pinned by `TestCheckResult_Error_NoLeak_ReturnsNil`.
- **The leak error names the phase, the worktree and every leaked path**, so the operator audits the bypass at a glance. Pinned by `TestGuard_NewDirtyPath_FlagsLeak`.

## Findings

- A read-only Gemini scout wrote into the main tree during a read-only phase (the cross-CLI trust bypass of cycle 119). The sandbox was the only guard then. This package is the answer.
