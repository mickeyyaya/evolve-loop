---
score_cap:
  - criterion: "Two consecutive zero-ship waves halt the loop with the zero-ship code before a third launches"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestRunLoop_ZeroShip_TwoConsecutiveZeroShipWavesHaltBeforeTheThird|TestPrepareIteration_ZeroShip_AWaveIsTheCyclesBetweenTwoIterationBoundaries' ./cmd/evolve"
  - criterion: "A wave with one ship resets the count; SKIPPED no-work and failed waves count as zero-ship"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestRunLoop_ZeroShip_AShipResetsTheCount|TestRunLoop_ZeroShip_FailedAndSkippedNoWorkWavesCountAsZeroShip' ./cmd/evolve"
  - criterion: "The pure rule counts only the trailing zero-ship run and a non-positive ceiling disables it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestEvaluateZeroShip' ./internal/core"
  - criterion: "The threshold failure_policy.thresholds.zero_ship_halt is read from policy.json with a compiled default of 2"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestZeroShipHalt_' ./internal/policy && go test -count=1 -run 'TestRunLoop_ZeroShip_ThresholdComesFromPolicyJSON' ./cmd/evolve"
  - criterion: "operating-policy.md section 4.1 documents the compiled zero_ship_halt rule and no longer says no Go code counts zero-ship cycles"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1961_006' ./acs/cycle1961"
  - criterion: "runtime-reference.md documents failure_policy.thresholds.zero_ship_halt with its compiled default 2"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1961_007' ./acs/cycle1961"
  - criterion: "the build explanation document states the same serial test count the registry holds"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1961_008' ./acs/cycle1961"
---

# Eval: Zero-ship halt breaker (lane part)

> Pins the compiled zero-ship rule that replaces the operator-only guardrail of operating-policy.md
> section 4.1. Two consecutive waves (fleet) or cycles (sequential) that land no ship halt the loop as an
> ADR-0072 system failure with a Signal Center code and a P0 pipeline-defect item. Source incident: the
> inbox record cli-loop-zero-ship-breaker-lane (cycle 1961), which found that SKIPPED no-work lanes and mixed
> EMPTY and FAIL outcomes trip neither the consecutive-failures rule nor the goal-stall escalation.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| halt-before-third | Two zero-ship waves halt before a third launch | 4/10 | `go test -run TestRunLoop_ZeroShip_TwoConsecutive...` |
| reset-and-classify | A ship resets; no-work and failed waves count | 5/10 | `go test -run TestRunLoop_ZeroShip_AShipResetsTheCount` |
| pure-rule | Trailing run only; disabled by a non-positive ceiling | 6/10 | `go test -run TestEvaluateZeroShip ./internal/core` |
| threshold | zero_ship_halt read from policy.json, default 2 | 6/10 | `go test -run TestZeroShipHalt_ ./internal/policy` |
