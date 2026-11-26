---
score_cap:
  - criterion: "A composed checkpoint records the pool id, the lane cycle and the lane scope, and a sequential checkpoint records none"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestLaneIdentity_' ./internal/checkpoint"
  - criterion: "The pool id of the cycle request reaches the persisted cycle state, and the resume point reads the identity back from the checkpoint"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestLaneIdentity_' ./internal/core"
  - criterion: "evolve loop --resume of a lane checkpoint at enforce wires the lane with its pool and scope, and a legacy lane checkpoint is refused before any phase runs"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestLaneIdentity_' ./cmd/evolve"
  - criterion: "An enforce lane delivers to the wave record and stops before main"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestDeliver_EnforceStopsBeforeMainAfterWritingTheRecord' ./internal/phases/ship"
  - criterion: "Every changed production line is covered and carries no code comment"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2654_006|TestC2654_007' ./acs/cycle2654"
---

# Eval: Lane identity of a resumed checkpoint

> Pins the rule of docs/architecture/integration-driver-enforce.md section 9: the checkpoint of a fleet lane
> records its lane identity, so `evolve loop --resume` at enforce keeps the lane on the wave record and never
> lands it on main. Source incident: cycle 2654 (inbox item train-checkpoint-lane-identity).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| identity-recorded | Compose fills pool, lane cycle and scope | 4/10 | `go test -run TestLaneIdentity_ ./internal/checkpoint` |
| identity-flows | request pool reaches the cycle state and the resume point | 5/10 | `go test -run TestLaneIdentity_ ./internal/core` |
| resume-wired | resumed lane wired at enforce, legacy refused | 4/10 | `go test -run TestLaneIdentity_ ./cmd/evolve` |
| delivery-stops-before-main | enforce delivery writes the record and stops | 6/10 | `go test -run TestDeliver_EnforceStopsBeforeMainAfterWritingTheRecord ./internal/phases/ship` |
| changed-lines-covered | changed lines covered, no comment | 6/10 | `go test -tags acs -run TestC2654_006 ./acs/cycle2654` |
