# Build Explanation — Cycle 1735

## Build Binding
- Cycle: 1735
- Base SHA: b22dea3b419d4bbffffb4237936ca5b5d6d64a23

## Summary
The pre-batch and batch-end GC hooks no longer write their shadow/enforce manifests into
`.evolve/runs/cycle-<next>`, the run directory of a cycle that has not started yet. Each sweep
now writes into its own stage directory under a batch-owned `.evolve/gc/` root:
`.evolve/gc/pre-batch` and `.evolve/gc/batch-end`.

## Rationale
`archivePollutedWorkspace` (`go/internal/core/workspace_guard.go`) treats only
`lane-scope.json` as legitimate pre-phase content in a cycle workspace. It treats any other file
as pollution and archives the whole directory. Both GC hook call sites pointed at
`cycleWorkspace(cfg.ProjectRoot, <next-cycle-number>)` before that cycle had started. The two
manifest files (`gc-shadow-manifest.json` and `workspace-gc-manifest.json`) therefore tripped
the guard on the next cycle's fresh workspace. This happened on cycles 1721, 1723, 1725, 1727
and 1731.

Moving the write target out of `runs/` fixes the collision at its source, because the GC hook
no longer writes anywhere the guard polices. The alternative was to allowlist the two manifest
filenames in the guard. That was rejected (scout H3): it would leave every future pre-phase
writer open to the same bug class. Audit round 1 (M2) found that a single shared `.evolve/gc`
path let the batch-end sweep overwrite the pre-batch manifest in the same batch. In enforce
mode that manifest is the evidence of record, so each sweep now gets its own stage directory.

## Changed Areas
- `go/cmd/evolve/cmd_loop_outcome.go` adds the `gcManifestDir(evolveDir)` helper. It returns
  the `.evolve/gc` root and is the single place that root path is built. It also removes
  `batchEndGCCycle`, which became dead once the batch-end call site stopped needing a cycle
  number.
- `go/cmd/evolve/cmd_loop_prebatch.go`: the pre-batch `gcHookFn` call now targets
  `gcManifestDir(cfg.EvolveDir)/pre-batch` instead of `cycleWorkspace(cfg.ProjectRoot, last+1)`.
  `prepareFreshBatch` drops its `lastCycle` return value and the `readLastCycleNumber` read
  that fed it, since nothing consumes them any more (audit L2).
- `go/cmd/evolve/cmd_loop_batch.go`: the batch-end `gcHookFn` call now targets
  `gcManifestDir(cfg.EvolveDir)/batch-end` instead of `cycleWorkspace(cfg.ProjectRoot,
  batchEndGCCycle(...))`. The `lastBeforeGCHook` field and local are removed because nothing
  reads them any more.
- `go/cmd/evolve/cmd_loop.go` adapts to `prepareFreshBatch`'s two-value return and no longer
  threads a cycle number into `loopBatchCoordinator`.
- `go/cmd/evolve/cmd_loop_gc_batchend_test.go`: the batch-end integration test used to assert
  that the workspace is under `runs/`, which encoded the bug. It now asserts the workspace is
  exactly `gcManifestDir(evolveDir)/batch-end` and not under `runs/`.
- `go/cmd/evolve/cmd_loop_gc_manifest_dir_test.go` is a new tdd grader for the helper. It pins
  `gcManifestDir` to `.evolve/gc`, outside `runs/`, so the root path cannot drift back into a
  cycle run dir.
- `go/cmd/evolve/cmd_loop_prebatch_gc_test.go` holds new tdd graders for the pre-batch hook.
  They cover the `pre-batch` stage target, the absence of manifests in the next cycle's run
  dir, and the pre-batch manifest surviving the batch-end sweep. The fixture storage reports
  `LastCycleNumber: 5`, so the inspected `cycle-6` dir is the one the old code would have
  polluted.
- `go/internal/core/orchestrator_workspace_test.go` holds the guard characterization tests
  `TestC1735_004` and `TestC1735_005`. They pin the guard's current behavior and show it is
  unchanged. They live in this existing, non-gate-shaped test file on purpose.
- `docs/architecture/packages/cmd-evolve.md` records the new invariant: GC manifests stay out of
  cycle run dirs, and each sweep owns a stage dir.
- `.evolve/evals/gc-manifests-out-of-cycle-run-dirs.md` is the task's eval, with graders for
  the four criteria.

## Design Decisions
The GC hooks write into `.evolve/gc/`, a sibling of `.evolve/runs/` and `.evolve/worktrees/`.
The root path is built only in `gcManifestDir`, which follows the single-construction-site
pattern `worktreeGCOptions` uses in the same file. The stage name (`pre-batch` / `batch-end`)
is joined at each call site because each site is the only writer of its stage. The guard
(`archivePollutedWorkspace`) is intentionally unchanged. Its correctness should not depend on
knowing GC filenames.

## Verification
- The eval graders `TestC1735_002`, `003` and `006`, plus
  `TestRunLoopBatch_GCHookFiresAfterFinalizeAtBatchEnd`, failed before the stage-dir change
  (shared `.evolve/gc` target) and pass after it. `TestC1735_001` pins only the `.evolve/gc`
  root, so it passes at both targets. `TestC1735_004`/`005` pass.
- `cd go && go test -count=1 ./cmd/evolve/... ./internal/core/... ./internal/gc/...` passes
  in full.
- `evolve acs suite --cycle 1735` is run before handoff. The round-1 red in
  `protectedsurface/TestEveryGateShapedFileIsProtectedSurface` came from this cycle's own
  gate-shaped `go/internal/core/workspace_guard_test.go`. Moving its tests into
  `orchestrator_workspace_test.go` removed it; the red was not pre-existing.

## Compatibility
There are no public API, flag or config schema changes. Manifest contents and filenames are
unchanged; their directory moves from `runs/cycle-N` to `.evolve/gc/<stage>`. Anything that
read the manifests from a cycle run dir must look in `.evolve/gc/pre-batch` or
`.evolve/gc/batch-end` instead. No in-repo code reads them.

## Limitations
The stage directories are shared across batches. Each batch overwrites the previous batch's
manifests, and two concurrent loops on one `.evolve` would overwrite each other's. In enforce
mode only the latest batch's pre-batch and batch-end records are kept. The repo runs `shadow`
today, so no applied-change evidence is lost. This change also does not audit other hooks for
future-cycle writes; scout deferred that as the separate, console-owned item
`boundary-reexec-keeps-cycle-budget-and-wave-index`.
