---
score_cap:
  - criterion: "The shipped basis computes the lane deadline from shipped lanes only"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2540_001' ./acs/cycle2540"
  - criterion: "The all basis stays the legacy deadline over every lane time"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2540_002' ./acs/cycle2540"
  - criterion: "The loop log line prints the deadline of both bases"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestNewLaneLeases_TheLogPrintsTheDeadlineOfBothBases' ./cmd/evolve"
  - criterion: "With no shipped lane the shipped basis takes deadline_floor_minutes"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2540_004' ./acs/cycle2540"
  - criterion: "fleet.orchestrator.deadline_basis defaults to all and refuses any value other than all or shipped"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'DeadlineBasis' ./internal/policy"
---

# Eval: Lane deadline from shipped lanes

> Pins ADR-0158 decision 17. The lane lease deadline is the p95 of recent lane times. A failed lane that ran for hours
> lifts that deadline, so the hang guard stops late. The shipped basis counts only the lanes whose ship phase passed.
> Source incident: cycle 2540 inbox item lane-deadline-from-shipped-lanes.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| shipped-only | 3 shipped lanes of 40m and 2 failed lanes of 6h give 40m | 4/10 | `TestC2540_001` |
| all-unchanged | the all basis equals the legacy deadline | 5/10 | `TestC2540_002` |
| both-bases-logged | the log line names both deadlines | 6/10 | `TestNewLaneLeases_TheLogPrintsTheDeadlineOfBothBases` |
| floor | no shipped lane gives the floor | 5/10 | `TestC2540_004` |
| policy | only all or shipped load | 5/10 | `DeadlineBasis` tests in `internal/policy` |
