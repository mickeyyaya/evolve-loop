---
score_cap:
  - criterion: "a higher-scored lane-eligible item takes the lowest unstarted planned slot, and a lead below preempt_margin does not"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/loopwave -run 'TestPreempt_AHigherScoredItemTakesTheLowestUnstartedSlot|TestPreempt_ALeadBelowThePreemptMarginDoesNotPreempt|TestPreempt_TheMarginComesFromThePolicyNotAConstant'"
  - criterion: "a running lane and an adopted continuation are never displaced"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/loopwave -run 'TestPreempt_ARunningLaneIsNeverDisplaced|TestPreempt_AnAdoptedContinuationIsNeverDisplaced|TestPreempt_APlanOfOnlyAdoptedContinuationsIsNeverPreempted'"
  - criterion: "the ledger records preempted <old> for <new> (score a to b) through the chained writer, and a preview writes no line"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/loopwave -run 'TestPreempt_TheLedgerRecordsTheOldAndTheNewItemWithBothScores|TestPreempt_APreviewPlansTheSwapButWritesNoLedgerLine'"
  - criterion: "the production plan path (productionWavePlanFn) reaches the preemption"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./cmd/evolve -run 'TestPreempt_ProductionWavePlanFnSwapsTheLowestSlotAndLedgersIt'"
---

# Eval: Boundary preemption in the wave plan (inbox plan P4)

> Pins the P4 component of docs/plans/inbox-prioritization-2026-10.md. When the wave plan reuses the previous triage decision, a lane-eligible item that beats the lowest uncommitted planned slot by `inbox_priority.preempt_margin` takes that slot. A running lane and an adopted continuation keep their slot. The ledger records the swap. Source incident: cycle 2464 (inbox item inbox-prioritization-p4-boundary-preemption, `preempt_margin` read by no code at HEAD 9199077a4).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| swap-and-margin | higher score takes the slot; lead below the margin does not; margin read from policy | 6/10 | `go test ./internal/loopwave -run TestPreempt_AHigherScored...` |
| committed-slots | running lane and adopted continuation never displaced | 7/10 | `go test ./internal/loopwave -run TestPreempt_ARunningLane...` |
| ledger | `preempted <old> for <new> (score a to b)` on the chained ledger; preview writes none | 7/10 | `go test ./internal/loopwave -run TestPreempt_TheLedgerRecords...` |
| caller-proof | the production wave plan path reaches the preemption | 7/10 | `go test ./cmd/evolve -run TestPreempt_ProductionWavePlanFn...` |
