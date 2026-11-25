---
score_cap:
  - criterion: "A boot refresh re-exec with ResumeWaves=k arms a handoff the next image takes as k"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestBootBinaryRefresh_ReExecArmsTheWaveHandoff|TestRunLoop_BootRefreshReExecCarriesTheResumePointTheLaunchTook)$' ./cmd/evolve"
  - criterion: "A plane HEAD ahead of origin with a go/ change is not rebuilt into the loop binary, and a shipped HEAD still is"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestBootBinaryRefresh_UnshippedPlaneHeadDoesNotRebuild|TestBootBinaryRefresh_ShippedPlaneHeadStillRebuilds)$' ./cmd/evolve"
  - criterion: "Zero ResumeWaves arms nothing and a failed arm boots as-is without a re-exec"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestBootBinaryRefresh_ZeroResumeWavesArmsNothing|TestBootBinaryRefresh_FailedArmBootsAsIsWithoutReExec)$' ./cmd/evolve"
---

# Eval: boot refresh re-arms the wave handoff

> Pins two contracts of `bootBinaryRefresh` (go/cmd/evolve/cmd_loop_boot_refresh.go). The re-exec keeps the PID, so the image it starts must find a handoff marker carrying the wave count the launch took, or it restarts at wave 0. The refresh must also never build the loop binary from a plane HEAD that origin does not hold. Source incident: cycle 2644 inbox item boot-refresh-rearms-the-wave-handoff.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| handoff-armed | next image takes ResumeWaves=k through takeReexecHandoff | 4/10 | `go test -run TestBootBinaryRefresh_ReExecArmsTheWaveHandoff ./cmd/evolve` |
| unshipped-head | unshipped HEAD is not rebuilt, shipped HEAD is | 5/10 | `go test -run TestBootBinaryRefresh_UnshippedPlaneHeadDoesNotRebuild ./cmd/evolve` |
| edge-cases | zero waves arms nothing, failed arm boots as-is | 6/10 | `go test -run TestBootBinaryRefresh_FailedArmBootsAsIsWithoutReExec ./cmd/evolve` |
