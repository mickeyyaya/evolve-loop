---
score_cap:
  - criterion: "The SIGNAL_EXPECTED_ABSENT reason and fields name the cycle, the phase, the run id and the start time"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2443_00[1-2]|TestC2443_004|TestC2443_005' ./acs/cycle2443"
  - criterion: "The warning names the open start and an end in another cycle does not close it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2443_006|TestC2443_007' ./acs/cycle2443"
  - criterion: "Each function of expected.go keeps 100% coverage and the changed Go files carry no comment"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2443_008|TestC2443_009' ./acs/cycle2443"
  - criterion: "Design section 8.1 and the signalcenter package note state the identity of the warning in STE"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2443_01' ./acs/cycle2443"
---

# Eval: SIGNAL_EXPECTED_ABSENT names its cycle, phase, run id and start time

> Pins the identity of the expected-signal warning. In wave 171 the warning fired three times with the reason `phase: phase.dispatched was not followed by phase.outcome,phase.aborted within 1h30m0s`. The reason and the fields did not name the cycle, the phase or the run id, so the operator could not tell which phase had stopped. Source incident: cycle 2392 inbox item `expected-signal-warning-names-its-cycle-and-phase`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| identity-in-reason-and-fields | Reason and fields name cycle, phase, run id, start time | 7/10 | `go test -tags acs -run TestC2443_001,002,004,005 ./acs/cycle2443` |
| right-start-named | An end in another cycle does not close the start | 6/10 | `go test -tags acs -run TestC2443_006,007 ./acs/cycle2443` |
| coverage-and-no-comment | 100% function coverage, zero comments | 6/10 | `go test -tags acs -run TestC2443_008,009 ./acs/cycle2443` |
| docs-in-ste | Design 8.1 and package note updated | 5/10 | `go test -tags acs -run TestC2443_01 ./acs/cycle2443` |
