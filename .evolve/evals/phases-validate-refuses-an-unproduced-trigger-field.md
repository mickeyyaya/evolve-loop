---
score_cap:
  - criterion: "evolve phases validate exits non-zero naming the phase and the field for an insert_when or skip_when field nothing produces"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2487_001|TestC2487_002' ./acs/cycle2487"
  - criterion: "a trigger field produced only by a later phase is refused, and a typed or earlier-phase signal is accepted"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2487_003|TestC2487_004|TestC2487_005' ./acs/cycle2487"
  - criterion: "evolve phases create refuses such a phase with an envelope naming the phase and the field and writes no phase.json"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2487_006|TestC2487_007' ./acs/cycle2487"
---

# Eval: phases validate and create refuse an unproduced trigger field

> Pins that `phasespec.UnproducedTriggerFields` has a production caller in `evolve phases validate` and `evolve phases create`. Before cycle 2487 the function existed (commit f83b40a3d) but no command called it, so a user phase whose `routing` read a signal nothing emits passed both commands and stayed unreachable at runtime.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| validate-refuses | insert_when and skip_when ghost field exits non-zero naming phase and field | 4/10 | `go test -tags acs -run 'TestC2487_001\|TestC2487_002' ./acs/cycle2487` |
| producer-order | later-phase producer refused; typed and earlier-phase producers accepted | 6/10 | `go test -tags acs -run 'TestC2487_003\|TestC2487_004\|TestC2487_005' ./acs/cycle2487` |
| create-refuses | create refuses with a named envelope and writes nothing | 4/10 | `go test -tags acs -run 'TestC2487_006\|TestC2487_007' ./acs/cycle2487` |
