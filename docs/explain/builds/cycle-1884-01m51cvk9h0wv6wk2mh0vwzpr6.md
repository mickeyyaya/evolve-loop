# Build Explanation — Cycle 1884

## Build Binding
- Cycle: 1884
- Base SHA: 517de13ad14e39e958c383e797849904fa55fdb3

## Summary
The read-only phase fence (`internal/treefence`, ADR-0097) now writes every snapshot tree through `gitexec.Git.TreeOfIndexCopy`. The fence no longer has its own copy of the scratch-index steps. The four ambient git variable names have one list, `gitexec.AmbientGitVars`, and treefence filters its own git calls by it. A full snapshot of a worktree without an index is now refused, the same as a staged snapshot and the same as `TreeOfIndexCopy`.

## Rationale
Two packages had the same index-copy routine, which keeps the index mtime so that git does not read a racily clean entry as clean. They also had the same list of ambient variables. A fix in one copy did not reach the other. `TreeOfIndexCopy` is the newer and stricter copy: it reads and stats from one open file handle, and it isolates every git call that it makes. The rejected option was to keep the private copy in treefence and only share the list. That option keeps two index writers, and the architecture re-review of 2026-10-06 named those writers as the defect.

## Changed Areas
- `go/internal/treefence/fence.go` — `indexCopyTree` delegates to `gitexec.Isolated(worktree).TreeOfIndexCopy` with the same `git add` argument lists; the private `copyIndex`, the temp-index setup and the private `ambientGitVars` map are deleted; `gitEnv` filters by `gitexec.AmbientGitVars`.
- `go/internal/gitexec/scratchindex.go` — the ambient list is exported as `AmbientGitVars`, so treefence uses the same names.
- `go/internal/gitexec/scratchindex_test.go` — names `AmbientGitVars` in a real assertion (apicover floor for a new export).
- `go/internal/treefence/fence_backfill_f4_test.go` — the copy-index failure-stage test is removed with the function (gitexec's `scratchindex_backfill_f4_test.go` covers the same stages); the write-tree and temp-dir failure tests now assert the gitexec error text that the fence wraps.
- `go/internal/core/ship_recovery_debugger_test.go` — the git shim matches the new scratch-index temp directory `gitexec-index-*`, so the forced take-time and restore-time write-tree failures still hit the fence.
- `go/internal/phases/audit/predicate_authority_backfill_f4_test.go` — a repository with no index is now refused at the predicate-tree stage, one stage before the ship tree; the subtest asserts that earlier stage.
- `docs/architecture/packages/internal-gitexec.md` — names treefence as a caller of `TreeOfIndexCopy` and `AmbientGitVars` as the one list.
- `go/acs/cycle1884/predicates_test.go` — the TDD phase's five predicates for this task.
- `.evolve/evals/treefence-writes-its-snapshot-through-gitexec-tree-of-index-copy.md` — the eval for this task.

## Design Decisions
The missing-index behaviour of a full snapshot is a refusal, not a tree built from an empty index. Evidence: `TestC1884_003` pins the refusal, and `Begin` reports it through `TakeErr`, so a read-only phase on such a worktree runs with a visible warning. treefence keeps its own `git` helper for `diff-tree`, `ls-tree` and `cat-file`, because `gitexec.Output` trims stdout and would change blob bytes on a restore. The error from the shared writer is wrapped as `treefence: snapshot tree: …`, so the fence's diagnostics still name the fence.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1884`: 5/5 predicates PASS.
- `go test -count=1` on treefence, gitexec, core, phases/audit, phases/runner, phases/retro, wtcheckpoint, recoveryguard, shipmanifest, rawgitratchet and prompts: all PASS.
- `internal/phases/ship` has one red, `TestWorktreeShipLand_AMainThatMovedPastTheLaneNeverPushesNorPrepares`. The same red occurs on an export of the base tree, so this change did not cause it.

## Compatibility
The `Take`, `TakeStaged`, `Begin` and `End` signatures are unchanged. The snapshot tree id for a worktree with an index is unchanged: `TestC1884_002` compares it with `TreeOfIndexCopy`. The temp directory prefix changes from `treefence-index-` to `gitexec-index-`.

## Limitations
The audit predicate-tree stage that reports a ship-tree capture failure no longer has a no-index test case, because the no-index case now stops one stage earlier.
