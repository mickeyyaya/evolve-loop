---
score_cap:
  - criterion: "A failed boundary step's wave next --json Detail carries its last 40 stderr lines beside rc=N"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^(TestWaveNext_FailedStepDetailCarriesTheLast40StderrLinesBesideRC|TestWaveNext_FailedStepTailHoldsOnlyThatStepsStderr|TestWaveNext_StepStderrStillReachesTheCallersStderr)$' ./cmd/evolve"
  - criterion: "A doctor live failure names exactly one failing stage (launch, trust prompt, REPL prompt, delivery, artifact) inside the 40-line tail"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestDoctorLive_FailureNamesItsFailingStage|TestDoctorLive_EveryFailureCodeNamesExactlyOneStageInsideTheWaveTail|TestDoctorLive_HealthyProbeNamesNoFailingStage)$' ./internal/cli/opscmd"
  - criterion: "runBoundarySteps writes the failed step's stderr tail to the boundary log after the failure line"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestBoundary_StepFailureWritesTheStepsStderrTailAfterTheFailureLine|TestBoundary_PassingRunWritesNoStderrTail)$' ./cmd/evolve"
---

# Eval: the boundary keeps the failed step's stderr tail and doctor live names its failing stage

> Pins the debuggability contract of `evolve wave next` and `evolve boundary run`. At the wave 101 and
> wave 102 boundaries (2026-10-16), the first `evolve doctor live claude-tmux` step failed with rc=1, and
> the envelope kept only `rc=1`. The cause could not be read. A failed step must now keep its last 40
> stderr lines in the envelope and in the boundary log, and doctor live must name the stage that failed.
> Source incident: inbox item boundary-doctor-live-keeps-no-output, cycle 1889.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| envelope-tail | Failed step Detail = rc=N plus its own last 40 stderr lines; passing steps stay rc=0 | 8/10 | `go test -run TestWaveNext_FailedStep... ./cmd/evolve` |
| doctor-stage | Exactly one `failing stage: <name>` line, inside a 40-line tail; none on a healthy probe | 7/10 | `go test -run TestDoctorLive_... ./internal/cli/opscmd` |
| boundary-log-tail | The failure line is followed by the failed step's tail only | 6/10 | `go test -run TestBoundary_... ./cmd/evolve` |
