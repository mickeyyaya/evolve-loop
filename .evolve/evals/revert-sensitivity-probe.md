---
score_cap:
  - criterion: "A hunk that no test depends on counts as insensitive, a hunk whose test fails without it counts as sensitive, and an uncompilable revert counts sensitive and is reported"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestRevertSensitivity_AHunkWhoseTestsPassWithoutItIsInsensitive|TestRevertSensitivity_AHunkWhoseTestsFailWithoutItIsSensitive|TestRevertSensitivity_AnUncompilableRevertCountsSensitiveAndIsReported|TestGitRevertRunner_TheShipFixtureSplitsIntoSensitiveInsensitiveAndUncompilableHunks' ./internal/qualitypanel"
  - criterion: "The probe runs in a scratch worktree, refuses a scratch inside the plane or main tree, and never writes in the plane tree"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestRevertSensitivity_AScratchInsideTheTreesOrNotAbsoluteIsRefusedBeforeAnyRun|TestGitRevertRunner_ProbingNeverWritesInThePlaneTreeAndLeavesNoScratchBehind' ./internal/qualitypanel"
  - criterion: "TQ2 starts at stage shadow and emits no breach there; kill-guard path hunks are excluded"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestRevertSensitivity_TQ2StartsAtStageShadowSoNoBreachEverFiresThere|TestRevertSensitivity_AKillGuardPathHunkIsExcludedAndNeverReachesTheRunner' ./internal/qualitypanel"
  - criterion: "Every function of the probe is at 100% statement coverage"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags acs ./acs/cycle2074 -run TestC2074_007"
---

# Eval: Quality index TQ2, revert sensitivity probe

> Pins the TQ2 reader of ADR-0153. It reverts each production hunk of a ship on a scratch worktree and counts the hunk sensitive when a covering test fails. The inbox item `revert-sensitivity-probe` (cycle 2074) holds the acceptance. The worked source case is the review of #1103, where the reviewer reverted the change and the test still passed.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| outcome-classes | insensitive, sensitive and uncompilable hunks are counted and reported | 4/10 | `go test -run TestRevertSensitivity_ ./internal/qualitypanel` |
| scratch-only | the plane tree is never written | 5/10 | `go test -run TestGitRevertRunner_ProbingNeverWrites... ./internal/qualitypanel` |
| shadow-stage | no breach at stage shadow, kill-guard paths excluded | 6/10 | `go test -run TestRevertSensitivity_TQ2StartsAtStageShadow... ./internal/qualitypanel` |
| full-coverage | each changed function at 100% | 6/10 | `go test -tags acs ./acs/cycle2074 -run TestC2074_007` |
