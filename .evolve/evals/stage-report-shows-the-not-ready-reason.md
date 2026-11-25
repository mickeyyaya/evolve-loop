---
score_cap:
  - criterion: "evolve stage verify prints the typed not-ready reason in its JSON and text output and a measured conflict class prints none"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2648_001|TestC2648_002' ./acs/cycle2648"
  - criterion: "evolve stage report prints the reason on the row of the key that needs it and on no other row, in JSON and text"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2648_003' ./acs/cycle2648"
  - criterion: "the reason follows the stage key, not the fields of each record"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2648_004' ./acs/cycle2648"
  - criterion: "one shared predicate states which stage keys depend on the conflict class and the scorer uses it for every registry key"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2648_005' ./acs/cycle2648"
  - criterion: "a measured chain still counts a routed held item as a conflict"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2648_006' ./acs/cycle2648"
---

# Eval: Stage report and verify show the typed not-ready reason

> Pins the operator-facing contract of the typed not-ready reason: `evolve stage verify` and `evolve stage report` print `conflicts_unmeasured` in text and in JSON (`not_ready`), and the dependency of a stage key on the conflict class has one home (`stageevidence.DependsOnConflict`). Source incident: the #1690 delta review, findings M3 and M4 (cycle 2648 inbox item stage-report-shows-the-not-ready-reason).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| verify-output | verify prints the reason in JSON and text | 4/10 | `go test -tags acs -run 'TestC2648_001|TestC2648_002' ./acs/cycle2648` |
| report-output | report prints the reason on one row only | 4/10 | `go test -tags acs -run 'TestC2648_003' ./acs/cycle2648` |
| key-not-record | the reason follows the key | 5/10 | `go test -tags acs -run 'TestC2648_004' ./acs/cycle2648` |
| one-home | one shared predicate drives the scorer | 5/10 | `go test -tags acs -run 'TestC2648_005' ./acs/cycle2648` |
| measured-conflict | a measured routed item still conflicts | 6/10 | `go test -tags acs -run 'TestC2648_006' ./acs/cycle2648` |
