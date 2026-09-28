# Build Explanation — Cycle 1737

## Build Binding
- Cycle: 1737
- Base SHA: cfb159f1c379a0a591b264af69a8647844683106

## Summary
The pre-batch GC sweep no longer publishes `gc-shadow-manifest.json` and
`workspace-gc-manifest.json` into `.evolve/runs/cycle-(last+1)`, the run dir of a cycle that
has not started yet. The batch-end sweep no longer publishes the same two files into the run
dir that `batchEndGCCycle` picked: the batch's own last-run cycle, or the next cycle when the
batch ran none. Each sweep now writes into its own stage dir under a batch-owned root:
`.evolve/gc/pre-batch` for the pre-batch sweep and `.evolve/gc/batch-end` for the batch-end
sweep. This cycle continues cycle 1735 and carries that cycle's production diff unchanged
(salvage snapshot `6547048a`).

## Rationale
`archivePollutedWorkspace` (`go/internal/core/workspace_guard.go`) accepts only
`lane-scope.json` in a cycle workspace before its first phase. Any other file makes it archive
the whole directory as `<workspace>.polluted-<timestamp>`. The pre-batch call site passed
`cycleWorkspace(cfg.ProjectRoot, last+1)` as the manifest sink. As a result, the next cycle
found two GC manifests in its own fresh run dir and archived that dir. This is the recurrence
seen on cycles 1721, 1723, 1725, 1727 and 1731.

The batch-end call site passed `cycleWorkspace(cfg.ProjectRoot, batchEndGCCycle(*lr,
lastBeforeGCHook+1))`. `batchEndGCCycle` returned the batch's own last-run cycle, a run dir
the guard never re-inspects. It fell back to the next cycle only when the batch ran no
cycles, which is the same pollution as the pre-batch site. It still wrote into a cycle run
dir. When a batch ran no cycles, the batch-end sweep fell back to `runs/cycle-(last+1)` and
overwrote the pre-batch manifests there. When a batch ran cycles, the workspace guard had
already archived the pre-batch manifests to `runs/cycle-N.polluted-<ts>` before the batch-end
sweep wrote, so the shared-dir overwrite shows only in the guard-less test fixture described
under Verification.

The fix is at the producer. The GC hooks now write outside `runs/`, where the guard does not
look. The alternative was to teach the guard to skip the two manifest filenames. It was
rejected because it grows a filename allowlist that the next pre-phase writer would have to
remember to extend. The guard is unchanged, and a test pins that it still archives a
GC-manifest-only dir.

The two sweeps use separate stage dirs. A single shared `.evolve/gc` dir would let the
batch-end sweep overwrite the pre-batch manifest within the same batch, and in `enforce` mode
that manifest is the record of what was applied.

## Changed Areas
- `go/cmd/evolve/cmd_loop_outcome.go`: adds `gcManifestDir(evolveDir)`, which returns
  `<evolveDir>/gc` and is the only place that root is built. Removes `batchEndGCCycle`, which
  has no caller once the batch-end site stops needing a cycle number.
- `go/cmd/evolve/cmd_loop_prebatch.go`: the pre-batch `gcHookFn` call targets
  `gcManifestDir(cfg.EvolveDir)/pre-batch` instead of `cycleWorkspace(cfg.ProjectRoot, last+1)`.
  `prepareFreshBatch` drops its `lastCycle` return and the `readLastCycleNumber` read that fed
  it, because nothing else consumed either.
- `go/cmd/evolve/cmd_loop_batch.go`: the batch-end `gcHookFn` call targets
  `gcManifestDir(cfg.EvolveDir)/batch-end` instead of
  `cycleWorkspace(cfg.ProjectRoot, batchEndGCCycle(...))`. The `lastBeforeGCHook` field and
  local go away, since no remaining code reads them.
- `go/cmd/evolve/cmd_loop.go`: `runLoopBatch` adapts to `prepareFreshBatch`'s two-value return
  and no longer passes a cycle number into `loopBatchCoordinator`.
- `go/cmd/evolve/cmd_loop_gc_batchend_test.go`: the batch-end integration test used to assert
  the sink was under `runs/`, which encoded the bug. It now asserts the sink is exactly
  `gcManifestDir(evolveDir)/batch-end` and not under `runs/`.
- `go/cmd/evolve/cmd_loop_gc_manifest_dir_test.go`: pins `gcManifestDir` to `<evolveDir>/gc`,
  outside `runs/`.
- `go/cmd/evolve/cmd_loop_prebatch_gc_test.go`: drives `runLoop` with a fixture storage at
  `LastCycleNumber: 5`, so the inspected `runs/cycle-6` is the dir the old code polluted. It
  checks that the pre-batch sink is `.evolve/gc/pre-batch`, that `runs/cycle-6` holds no GC
  manifest while `.evolve/gc/pre-batch` holds one, and that the pre-batch manifest survives the
  batch-end sweep unchanged.
- `go/internal/core/orchestrator_workspace_test.go`: characterization tests that the guard still
  archives a GC-manifest-only workspace (`TestC1735_004`) and keeps a lane-scope-only one
  (`TestC1735_005`). They are in this existing test file rather than a new gate-shaped
  `workspace_guard_test.go`, which would sit outside the protected-surface manifest.
- `docs/architecture/packages/cmd-evolve.md`: records the invariant that GC manifests never
  land in a cycle run dir and that each sweep owns a stage dir.
- `.evolve/evals/gc-manifests-out-of-cycle-run-dirs.md`: the task eval and its graders.
- `docs/private/research/archived-2026-09-28/unshipped-build-explanations/cycle-1735-01m3k2r1z13znw2sze4j96xx4k.md`: archives the cycle-1735 explanation, which never shipped,
  so this cycle's document is the one of record.

## Design Decisions
The manifest root is a sibling of `.evolve/runs/` and `.evolve/worktrees/`. It is built only in
`gcManifestDir`, following the single-construction-site pattern of `worktreeGCOptions` in the
same file. Each call site joins its own stage name, because each site is the only writer of that
stage. `runGCHook` itself is unchanged: it still creates and writes to the directory it is
given, so it needed no new provisioning code.

`go/internal/sizeratchet/offenders.json` is deliberately left untouched. Cycle 1735's audit
failed because the three shrunken functions (`loopBatchCoordinator.run`, `prepareFreshBatch`,
`runLoopBatch`) fell below their exact allowances under the old ratchet `Check`. Main's PR #695
later made an allowance a ceiling, so a shrink no longer fails the ratchet. Only the
wave-boundary tighten step writes the allowances file.

## Verification
- `cd go && go test -count=1 ./cmd/evolve/... ./internal/core/... ./internal/sizeratchet/... ./internal/gc/...`
  passes in full. `gofmt -l` prints nothing, and `go vet ./cmd/evolve/ ./internal/core/` is
  clean.
- `evolve acs suite --cycle 1737` gives `verdict=PASS red=0`, which covers the six
  `go/acs/cycle1737` predicates and the `protectedsurface` regression predicate.
- Faithful pre-fix base: a `go test -overlay` restores the `c9093d5c` versions of the four
  production files, adding only a `gcManifestDir` stub so the tests compile. On it,
  `TestC1735_002_PreBatchGCHookTargetsBatchOwnedDir`,
  `TestC1735_003_NextCycleWorkspaceHasNoGCManifestsAfterPreBatchGC`,
  `TestC1735_006_PreBatchGCManifestSurvivesBatchEndGC` and
  `TestRunLoopBatch_GCHookFiresAfterFinalizeAtBatchEnd` FAIL, while `TestC1735_001` passes.
  In that fixture the batch-end sink is `runs/cycle-6`, the cycle the batch just ran, which is
  also where the pre-batch sweep wrote; that shared dir is why `TestC1735_006` fails there.
  Predicate `TestC1737_005` re-runs this overlay and checks this document against it.
- Synthetic mutant (TDD run R1): the pre-batch site was put back to
  `cycleWorkspace(cfg.ProjectRoot, last+1)` and the batch-end site was pointed at a synthetic
  `lastCycleIn(*lr)+1` target, which is not the removed `batchEndGCCycle` code. That mutant
  kills the pre-batch and batch-end site graders, but `TestC1735_006` stays green under the
  synthetic target because the two sinks then differ.
- Guard allowlist (TDD run R2): with the rejected design, a guard that skips the GC filenames,
  `TestC1735_004_ArchivePollutedWorkspaceStillArchivesGCManifestOnlyDir` reports `--- FAIL`.

## Compatibility
No public API, CLI flag or config schema changes. Manifest filenames and contents are unchanged.
Only their directory moves, from `runs/cycle-N` to `.evolve/gc/pre-batch` or
`.evolve/gc/batch-end`. No in-repo code reads the manifests from a cycle run dir.

## Limitations
The stage dirs are not per-batch: each batch overwrites the previous batch's manifests, and two
concurrent loops sharing one `.evolve` would overwrite each other's. The repo runs GC in
`shadow` mode today, so no applied-change record is lost. Retention and rotation of
`.evolve/gc/` are not addressed; scout deferred them as a beyond-the-ask follow-up.
