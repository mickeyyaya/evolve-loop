---
score_cap:
  - criterion: "Round 1 of cycle 2035 stops with handoff_blocker and names M1 and its open console item"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2039_001|TestC2039_002' ./acs/cycle2039"
  - criterion: "A failure block with only lane findings returns no stop and lists the lane findings"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2039_003 ./acs/cycle2039"
  - criterion: "A hand-off finding that does not block is listed and does not stop the cycle"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2039_004|TestC2039_008' ./acs/cycle2039"
  - criterion: "The stop decision carries the salvage pointer, the blocking findings and the console item"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2039_005|TestC2039_006|TestC2039_007' ./acs/cycle2039"
  - criterion: "workflow.repair.fixability.stage is a strict stage key: off, shadow, enforce, default shadow"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2039_010|TestC2039_011|TestC2039_012|TestC2039_013' ./acs/cycle2039"
  - criterion: "The fixability package is enrolled in the apicover gate and names every export"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2039_009 ./acs/cycle2039"
---

# Eval: repair-fixability-handoff

> Pins the pure classifier of ADR-0158 decisions 1 to 5 and the strict stage key `workflow.repair.fixability.stage`. The fixtures are the four audit failure blocks of cycle 2035, which spent 104.3 LLM minutes on repair rounds after round 1 had named the blocker M1. The call-site wiring is the console item `wire-cost-aware-repair-call-sites`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| blocker-stop | Round 1 of cycle 2035 stops with handoff_blocker | 4/10 | `go test -tags acs -run TestC2039_001 ./acs/cycle2039` |
| lane-only | H1 alone returns no stop | 6/10 | `go test -tags acs -run TestC2039_003 ./acs/cycle2039` |
| non-blocking | M2 is listed and does not stop | 6/10 | `go test -tags acs -run TestC2039_004 ./acs/cycle2039` |
| seal-fields | The stop carries salvage, findings and console item | 6/10 | `go test -tags acs -run TestC2039_005 ./acs/cycle2039` |
| strict-stage | The stage key is strict with default shadow | 5/10 | `go test -tags acs -run TestC2039_010 ./acs/cycle2039` |
| graduation | The new package is enrolled and fully named | 7/10 | `go test -tags acs -run TestC2039_009 ./acs/cycle2039` |
