---
score_cap:
  - criterion: "every moved cmd/evolve test is parallel in the AST guard and absent from serialTests"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2411_001_' ./acs/cycle2411"
  - criterion: "every moved test pauses at run time and passes, so it leaves the serial chain"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2411_002_|TestC2411_006_' ./acs/cycle2411"
  - criterion: "no moved test swaps a seam or writes the process environment, and the router tests read only the environment they inject"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2411_003_|TestC2411_004_' ./acs/cycle2411"
  - criterion: "each moved test keeps its assertions"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2411_005_' ./acs/cycle2411"
  - criterion: "every seam function keeps a production caller and the production defaults stay wired"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2411_007_|TestC2411_008_|TestC2411_009_' ./acs/cycle2411"
---

# Eval: CI wall time C4, cmd/evolve serial chain

> Pins the move of 33 cmd/evolve tests off the serial chain: the router dispatch golden and refused-route tests (env writes), the bridge sessions and seat health tests (bridgeSessionsTmux and bridgeLiveSockets swaps), and the root signal center tests (attachRootEventsFn swaps). Source: plan docs/plans/ci-wall-time-2026-10.md component C4, decision D8, research F3.29. Cycle 2411 materialized the predicates in go/acs/cycle2411.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| parallel-guard | moved tests are parallel and unlisted | 5/10 | `TestC2411_001_` |
| runtime-pause | moved tests emit pause and pass | 5/10 | `TestC2411_002_`, `TestC2411_006_` |
| injected-env | no seam swap, no env write, no ambient env read | 6/10 | `TestC2411_003_`, `TestC2411_004_` |
| assertions-kept | assertion count per moved test never drops | 6/10 | `TestC2411_005_` |
| production-wiring | seam files stay reachable from main and keep their defaults | 7/10 | `TestC2411_007_`, `TestC2411_008_`, `TestC2411_009_` |
