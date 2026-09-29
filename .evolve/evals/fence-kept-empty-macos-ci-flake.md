---
score_cap:
  - criterion: "With the fence forced to fail (Take or Restore), TestRecoverFromShipError_TheFenceKeepsTheDebuggersResolutionOfTheConflictedFile still fails, and its own failure message names TakeErr/RestoreErr with the error text instead of an unexplained kept = []"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestRecoverFromShipError_TheFenceKeepsTestNamesTheFenceErrorWhenKeptIsEmpty ./internal/core/"
  - criterion: "fencedDebugger keeps the whole treefence.Outcome, not just Kept"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestFencedDebugger_ExposesTakeErrWhenFenceGoesInert ./internal/core/"
  - criterion: "initConflictRebaseRepoT sets gittest.MaintenanceConfig (maintenance.auto=false, gc.auto=0), so background git maintenance cannot touch .git/index during the fence's snapshot"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestInitConflictRebaseRepoT_DisablesBackgroundMaintenance ./internal/core/"
  - criterion: "treefence.Fence.Begin/End tell an inert-fence TakeErr apart from a verified empty restore (Outcome.TakeErr vs Outcome.Verified), not just by Outcome.Kept"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestFenceEnd_DistinguishesInertTakeErrFromGenuineEmptyRestore ./internal/treefence/"
  - criterion: "The originally flaking test stays stable under repeated race-mode runs (regression guard; it passed locally before the fix too, so this does not confirm the root cause)"
    max_if_missing: 5
    evidence: "cd go && go test -run 'TestRecoverFromShipError_TheFenceKeepsTheDebuggersResolutionOfTheConflictedFile$' -race -count=50 ./internal/core/"
---

# Eval: fence-kept-empty-macos-ci-flake

> Pins the macOS CI flake in
> `TestRecoverFromShipError_TheFenceKeepsTheDebuggersResolutionOfTheConflictedFile`
> (train #721 run 36550234640, train #725 run 36565868172). The failure
> `debugger writable = [shared.md] kept = [], want both [shared.md]` names no
> cause, because the test discards the fence's `treefence.Outcome.TakeErr` and
> `RestoreErr`. The main criterion is that the test's own failure message names
> that error. It is graded by re-running the test in a child process with a
> `git` shim that forces the fence's Take, or its Restore, to fail. The fixture's
> missing `gittest.MaintenanceConfig` is the leading root-cause hypothesis: a
> detached `git maintenance run --auto` racing `treefence.writeTreeMode`'s read
> of `.git/index`. It is **not confirmed**. The test passed 30/30 locally before
> the fix, so no local `-race -count=N` run tells fixed from unfixed. The next CI
> occurrence, if any, will name its cause through criterion 1. Sources: inbox
> `.evolve/inbox/2026-09-29T21-05-00Z-fence-kept-empty-macos-ci-flake.json`;
> cycle 1766 `fault-localization-report.md` and `bug-reproduction-report.md`;
> cycle 1766 audit round 1 (H1: the round-1 grader tested a proxy field, not the
> failure message).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| failure names its cause | the flaking test's failure message names TakeErr/RestoreErr (child re-run under a git shim) | 8/10 | `go test -run TestRecoverFromShipError_TheFenceKeepsTestNamesTheFenceErrorWhenKeptIsEmpty ./internal/core/` |
| outcome captured | fencedDebugger keeps the full Outcome | 5/10 | `go test -run TestFencedDebugger_ExposesTakeErrWhenFenceGoesInert ./internal/core/` |
| fixture hygiene (hypothesis) | the fixture disables background git maintenance | 6/10 | `go test -run TestInitConflictRebaseRepoT_DisablesBackgroundMaintenance ./internal/core/` |
| fence coverage | Begin/End TakeErr vs Verified has direct coverage | 5/10 | `go test -run TestFenceEnd_DistinguishesInertTakeErrFromGenuineEmptyRestore ./internal/treefence/` |
| stability (non-discriminating) | the flaking test survives repeated race runs | 5/10 | `go test -run '…ConflictedFile$' -race -count=50 ./internal/core/` |
