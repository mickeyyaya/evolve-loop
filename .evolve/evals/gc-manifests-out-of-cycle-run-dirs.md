---
score_cap:
  - criterion: "GC manifests are written under a batch-owned directory, never under runs/cycle-N"
    max_if_missing: 8
    evidence: "cd go && go test -run TestC1735_001_GCManifestDirIsBatchOwnedNotCycleWorkspace ./cmd/evolve/... && go test -run TestC1735_002_PreBatchGCHookTargetsBatchOwnedDir ./cmd/evolve/... && go test -run TestRunLoopBatch_GCHookFiresAfterFinalizeAtBatchEnd ./cmd/evolve/..."
  - criterion: "A cycle that starts after a pre-batch GC archives nothing (no GC manifests reach the next cycle's run dir)"
    max_if_missing: 7
    evidence: "cd go && go test -run TestC1735_003_NextCycleWorkspaceHasNoGCManifestsAfterPreBatchGC ./cmd/evolve/..."
  - criterion: "The pre-batch GC manifest survives the batch-end GC (each sweep publishes into its own stage dir under .evolve/gc)"
    max_if_missing: 6
    evidence: "cd go && go test -run TestC1735_006_PreBatchGCManifestSurvivesBatchEndGC ./cmd/evolve/..."
  - criterion: "archivePollutedWorkspace itself is unchanged (guard correctness preserved, not weakened to accommodate GC filenames)"
    max_if_missing: 5
    evidence: "cd go && go test -run 'TestC1735_004_ArchivePollutedWorkspaceStillArchivesGCManifestOnlyDir|TestC1735_005_ArchivePollutedWorkspaceIgnoresLaneScopeOnly' ./internal/core/..."
  - criterion: "The cycle-1737 explanation document reports the pre-fix behavior a faithful restore of the pre-fix code shows (batch-end targeted its own last-run cycle; TestC1735_006 fails pre-fix)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1737_005_ExplanationDocMatchesFaithfulPreFixBase$' ./acs/cycle1737/"
  - criterion: "The cycle-1737 explanation document scopes the pre-fix overwrite to where it happened (a batch that ran no cycles, or the guard-less test fixture), never to a batch that ran cycles, whose run dir the workspace guard archived first"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC1737_006_ExplanationDocScopesPreFixOverwriteToGuardlessPaths$' ./acs/cycle1737/"
---

# Eval: GC manifests must not land in a future cycle's run dir

> The pre-batch GC hook (`cmd_loop_prebatch.go:66`) wrote
> `gc-shadow-manifest.json` and `workspace-gc-manifest.json` into
> `cycleWorkspace(cfg.ProjectRoot, last+1)` — a future cycle's own
> `.evolve/runs/cycle-N` directory, before that cycle starts. The batch-end
> hook (`cmd_loop_batch.go:132`) wrote them into
> `cycleWorkspace(cfg.ProjectRoot, batchEndGCCycle(...))` — the batch's own
> last-run cycle dir, or the next cycle's only when the batch ran none. In a
> batch that ran cycles the two sweeps named the same `runs/cycle-N` path, but
> the cycle's workspace guard archived the pre-batch manifests before the
> batch-end sweep wrote there; only a batch that ran no cycles let the
> batch-end manifest overwrite the pre-batch one. `archivePollutedWorkspace` (`internal/core/workspace_guard.go`)
> allowlists only `lane-scope.json` as pre-phase-legitimate; seeing the two GC
> manifest files instead, it counted them as pollution and renamed the fresh
> workspace to `<workspace>.polluted-<stamp>` — losing the GC's own shadow
> evidence and logging a pollution event that never happened. Observed on
> cycles 1721, 1723, 1725, 1727, 1731 (inbox record
> `gc-manifests-out-of-cycle-run-dirs`, triaged in cycle 1735). The fix moves
> both call sites onto a single batch-owned `filepath.Join(evolveDir, "gc")`
> directory via a new `gcManifestDir` helper; the guard itself does not
> change (H3 — teaching it to allowlist GC filenames was rejected as it would
> let any future writer race the same bug class). Cycle 1735 audit round 1
> (M2) found the single shared `.evolve/gc` path let the batch-end sweep
> overwrite the pre-batch manifest — the evidence of record in enforce mode —
> so each sweep publishes into its own stage dir (`.evolve/gc/pre-batch`,
> `.evolve/gc/batch-end`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| helper-and-call-sites | GC manifests write to a batch-owned dir, not runs/cycle-N | 8/10 | `go test -run TestC1735_001_GCManifestDirIsBatchOwnedNotCycleWorkspace\|TestC1735_002_PreBatchGCHookTargetsBatchOwnedDir\|TestRunLoopBatch_GCHookFiresAfterFinalizeAtBatchEnd ./cmd/evolve/...` |
| no-next-cycle-pollution | Next cycle's run dir has zero GC manifests after a pre-batch GC | 7/10 | `go test -run TestC1735_003_NextCycleWorkspaceHasNoGCManifestsAfterPreBatchGC ./cmd/evolve/...` |
| stage-owned-evidence | Pre-batch manifest survives the batch-end sweep | 6/10 | `go test -run TestC1735_006_PreBatchGCManifestSurvivesBatchEndGC ./cmd/evolve/...` |
| guard-unchanged | archivePollutedWorkspace behavior on GC-manifest-only / lane-scope-only dirs is unchanged | 5/10 | `go test -run 'TestC1735_004_ArchivePollutedWorkspaceStillArchivesGCManifestOnlyDir\|TestC1735_005_ArchivePollutedWorkspaceIgnoresLaneScopeOnly' ./internal/core/...` |
| explanation-matches-pre-fix-base | The explanation document's pre-fix claims match the graders' run on the restored `c9093d5c` production files (cycle-1737 audit round 1, H1) | 6/10 | `go test -tags acs -run TestC1737_005_ExplanationDocMatchesFaithfulPreFixBase ./acs/cycle1737/` |
| explanation-scopes-overwrite | The explanation document confines the pre-fix overwrite to a batch that ran no cycles or the guard-less fixture; a batch that ran cycles had its run dir archived by the guard first (cycle-1737 audit round 2, H1) | 6/10 | `go test -tags acs -run TestC1737_006_ExplanationDocScopesPreFixOverwriteToGuardlessPaths ./acs/cycle1737/` |
