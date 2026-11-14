---
score_cap:
  - criterion: "A wave with a quota pause, a loop stop_reason or an operator brake is left out of the check"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2377_001|TestC2377_002' ./acs/cycle2377"
  - criterion: "A wave breaches on the live-lane rule only while the supply is not short, and on a host gate share above 25%"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2377_003|TestC2377_004' ./acs/cycle2377"
  - criterion: "Two breaching waves in a row emit LATENCY_BUDGET_EXCEEDED (WARN) and file one item; a rerun files no second item"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2377_005|TestC2377_006' ./acs/cycle2377"
  - criterion: "The live-lane and gate-share bounds come from policy.json, not from code"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2377_007' ./acs/cycle2377"
  - criterion: "The operator brake stop records a loop.halt signal with its stop_reason, so the check can read it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestPrepareIteration_ABrakeStopRecordsALoopHaltSignalWithTheStopReason' ./cmd/evolve"
---

# Eval: Latency gates T6, evolve wave latency --check

> Pins the wave gate of section 4.3 of docs/plans/latency-gates-2026-10.md (ADR-0144). The check reads the bounds from policy, leaves out a wave with a pause or a brake, and files one item after two breaching waves in a row. Source incident: cycle 2362 failed the audit of this item, and cycle 2377 continues it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exclusion | quota pause, stop_reason and brake leave a wave out | 6/10 | `go test -tags acs -run 'TestC2377_001\|TestC2377_002' ./acs/cycle2377` |
| breach rules | live-lane rule with supply guard, gate share above 25% | 7/10 | `go test -tags acs -run 'TestC2377_003\|TestC2377_004' ./acs/cycle2377` |
| streak | two breaches in a row emit the WARN signal and file one item | 8/10 | `go test -tags acs -run 'TestC2377_005\|TestC2377_006' ./acs/cycle2377` |
| policy | both bounds come from policy.json | 7/10 | `go test -tags acs -run 'TestC2377_007' ./acs/cycle2377` |
| brake producer | the brake stop records a loop.halt signal | 6/10 | `go test -run 'TestPrepareIteration_ABrakeStopRecordsALoopHaltSignalWithTheStopReason' ./cmd/evolve` |
