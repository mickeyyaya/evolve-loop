---
score_cap:
  - criterion: "a landing or a seal for a cycle that no live lane holds emits one WARN signal that names the cycle and the event"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestOversight_AnUnlocatedLandingOrSealEmitsOneWarnSignalNamingItsCycleAndEvent$' ./internal/orchestrator"
  - criterion: "an unlocated landing keeps the existing cleanup and a located lane gets no such signal"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestOversight_AnUnlocatedLanding(KeepsTheLandingStateCleanup|SignalIsAbsentForALocatedLane)$' ./internal/orchestrator"
---

# Eval: the oversight signals a landing for an unlocated lane

> The oversight returned silently from `landed` and `sealed` when no live lane held the cycle. The operator then saw no trace of a landing that the pool could not attribute. This eval pins one WARN signal that names the cycle and the event, with no signal for a located lane. Source: cycle 2444, inbox item oversight-landing-for-an-unlocated-lane-emits-a-signal.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| unlocated-signal | one WARN signal with fields cycle and event, at both stages | 6/10 | `go test -run TestOversight_AnUnlocatedLandingOrSealEmitsOneWarnSignalNamingItsCycleAndEvent` |
| preservation | the cleanup stays and a located lane stays quiet | 7/10 | `go test -run TestOversight_AnUnlocatedLanding(KeepsTheLandingStateCleanup\|SignalIsAbsentForALocatedLane)` |
