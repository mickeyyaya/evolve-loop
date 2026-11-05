---
score_cap:
  - criterion: "A refresh builds the gotypes index from the go module of the index worktree after the trail step"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCodeRefresh_Gotypes_BuildsTheIndexFromTheIndexWorktreeAfterTheTrailStep|TestSyncMainCodeIndex_Gotypes_TheMergeBoundaryBuildsTheIndexAfterTheTrailStep|TestDefaultCodeIndexHost_Gotypes_BuildsThroughTheRealGotypesBuild' ./cmd/evolve"
  - criterion: "A failed gotypes build prints one WARN line and the refresh keeps the exit code of the trail step"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestCodeRefresh_Gotypes_AFailedBuild|TestCodeRefresh_Gotypes_AFailedTrailStep|TestSyncMainCodeIndex_Gotypes_AFailedBuild' ./cmd/evolve"
  - criterion: "The index file lies beside the index worktree, outside every git tree"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCodeRefresh_Gotypes_WritesTheIndexBesideTheWorktreeOutsideEveryGitTree' ./cmd/evolve"
  - criterion: "No build runs when the refresh stops before the trail step or runs inside a phase"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestCodeRefresh_Gotypes_NoBuildRunsWhenTheRefreshStopsBeforeTheTrailStep|TestCodeRefresh_Gotypes_InsideAPhaseNoBuildRuns' ./cmd/evolve"
  - criterion: "code-index-cli.md section 4 names the gotypes step and the plan marks G2 built"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2173_005' ./acs/cycle2173"
  - criterion: "The gotypes build runs under the 5-minute deadline of the refresh, from both entry points, and an expired build keeps the last good index"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCodeRefresh_Gotypes_TheBuildRunsUnderTheDeadlineOfTheRefresh|TestCodeRefresh_Gotypes_ABuildThatOutlivesTheDeadline|TestSyncMainCodeIndex_Gotypes_ABuildThatOutlivesTheDeadline' ./cmd/evolve && go test -count=1 -run 'TestBuild_ACancelledContext|TestBuild_ADeadlineThatPasses' ./internal/adapters/codeindex/gotypes"
  - criterion: "A failed build is exactly one WARN line even when the cause has several lines"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'AMultiLineCause' ./cmd/evolve"
---

# Eval: The code index refresh builds the gotypes index (code index G2)

> Pins the boundary step of plan component G2: after the trail step, `evolve code refresh` and `evolve sync-main` type-check the index worktree with `gotypes.Build` and write `gotypes.IndexPath`. A failed build is one WARN line, and the exit code stays the code of the trail step. Source: inbox item `code-index-backends-g2-boundary-gotypes-build`, cycle 2173; `gotypes.Build` had no production caller on main.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wiring | The refresh reaches the build after the trail step, from both entry points | 6/10 | `go test -run TestCodeRefresh_Gotypes_Builds... ./cmd/evolve` |
| failure-isolation | A failed build is a WARN and never changes the exit code | 7/10 | `go test -run AFailedBuild ./cmd/evolve` |
| placement | The index file is outside every git tree | 6/10 | `go test -run WritesTheIndexBeside... ./cmd/evolve` |
| negative | No build before the trail step or inside a phase | 5/10 | `go test -run NoBuildRuns... ./cmd/evolve` |
| docs | Spec section 4 and plan G2 row are current | 5/10 | `go test -tags acs -run TestC2173_005 ./acs/cycle2173` |
| deadline | The build honours the refresh deadline and keeps the last good index | 6/10 | `go test -run ABuildThatOutlivesTheDeadline ./cmd/evolve` |
| one-line | A multi-line cause is one WARN line (audit round 3, M2) | 6/10 | `go test -run AMultiLineCause ./cmd/evolve` |
