---
score_cap:
  - criterion: "Two lanes that resume one paused cycle admit exactly one; the refused lane spends no attempt"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestRecordLaneResume_TwoConcurrentLaneResumesOfOnePausedCycleAdmitExactlyOne|TestRecordLaneResume_AFreshLeaseRefusesAndSpendsNoAttempt' ./internal/checkpoint"
  - criterion: "A spent auto-resume budget takes no lease, and the operator resume of evolve loop --resume takes none either"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestRecordLaneResume_ASpentBudgetTakesNoLease|TestRecordResume_TheOperatorResumeIgnoresAFreshLeaseAndTakesNone' ./internal/checkpoint"
  - criterion: "TakeResumeLease refuses a fresh holder with a typed error that names it and takes over an expired lease"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestTakeResumeLease_|TestReleaseResumeLease_' ./internal/core"
  - criterion: "The lane entry point adopts a paused cycle once under any scheduling, with one lane in front of the other or both at the lease"
    max_if_missing: 5
    evidence: "cd go && go test -count=5 -cpu 1,4 -run 'TestLaneResume_TwoLanesOfOneScopeAdoptAPausedCycleExactlyOnce|TestLaneResume_ALaneThatArrivesAfterTheWinnerLeasedTheCycleStartsFresh' ./cmd/evolve"
  - criterion: "The production entry point wires the lane to RecordLaneResume and the loop keeps RecordResume"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestLoopResume_TheOperatorResumeKeepsTheLeaseFreeRecorder|TestRunCycleRun_TheLaneRecordsItsResumeThroughRecordLaneResume' ./cmd/evolve"
---

# Eval: A paused cycle is adopted by one lane only

> Pins the behaviour of the closed PR #1256 that the lanes can deliver (F1): the lane resume counts the auto-resume and takes the resume lease of the paused cycle under one path lock, so a second lane that finds the same paused cycle is refused and spends no attempt. F2, the lane identity for the project lock, edits protected paths and is a console follow-up, so this eval does not pin it. Source incident: the stale-PR triage of 2026-11-12 and cycle 2353, whose first lane test depended on goroutine scheduling and failed under `-cpu 1`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exclusive-adoption | One winner of two concurrent lane resumes | 4/10 | `go test -run TestRecordLaneResume_TwoConcurrent... ./internal/checkpoint` |
| budget-and-loop | Spent budget and loop resume take no lease | 6/10 | `go test -run TestRecordLaneResume_ASpentBudgetTakesNoLease ./internal/checkpoint` |
| lease-seam | TakeResumeLease refuses fresh, takes expired | 5/10 | `go test -run TestTakeResumeLease_ ./internal/core` |
| scheduling-proof | The entry-point test holds under -cpu 1 and -cpu 4 | 5/10 | `go test -cpu 1,4 -run TestLaneResume_ ./cmd/evolve` |
| wiring | The real entry point uses RecordLaneResume | 5/10 | `go test -run TestRunCycleRun_TheLaneRecords... ./cmd/evolve` |
