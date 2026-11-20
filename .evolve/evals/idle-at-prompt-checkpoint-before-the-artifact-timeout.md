---
score_cap:
  - criterion: "An idle pane with no deliverable gets its checkpoint and nudge after the idle grace, not after ArtifactTimeoutS"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestIdleCheckpoint_IdlePaneIsNudgedAfterTheGraceNotTheArtifactTimeout|TestIdleCheckpoint_DefaultGraceStillBeatsTheArtifactTimeout' ./internal/bridge"
  - criterion: "A busy pane with no deliverable still waits the full interval, and the idle clock restarts after busy work"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestIdleCheckpoint_BusyPaneWaitsTheFullInterval|TestIdleCheckpoint_IdleClockRestartsAfterBusyWork' ./internal/bridge"
  - criterion: "A pane that writes the deliverable inside the grace completes with no nudge"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestIdleCheckpoint_ArtifactInsideTheGraceCompletesWithoutANudge' ./internal/bridge"
  - criterion: "The policy value bridge.idle_checkpoint_grace_s loads into the bridge policy block"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestIdleCheckpointGrace_' ./internal/policy"
---

# Eval: Idle-pane checkpoint before the artifact timeout

> Pins the early checkpoint for a pane that sits idle at its prompt with no deliverable. At HEAD the wait loop gates the checkpoint on the artifact interval only (`go/internal/bridge/driver_tmux_wait.go`), so an idle seat waits the whole interval (300 s default, 1200 s for build) before the one-shot nudge. Source incident: the nudge data of cycles 2200-2355, inbox item `idle-at-prompt-checkpoint-before-the-artifact-timeout`, cycle 2568.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| journey | Idle pane nudged after the grace | 4/10 | `go test -run TestIdleCheckpoint_IdlePaneIsNudgedAfterTheGraceNotTheArtifactTimeout ./internal/bridge` |
| preservation | Busy pane waits the full interval | 5/10 | `go test -run TestIdleCheckpoint_BusyPaneWaitsTheFullInterval ./internal/bridge` |
| preservation | Deliverable inside the grace needs no nudge | 5/10 | `go test -run TestIdleCheckpoint_ArtifactInsideTheGraceCompletesWithoutANudge ./internal/bridge` |
| policy | Policy value loads from the bridge block | 6/10 | `go test -run TestIdleCheckpointGrace_ ./internal/policy` |
