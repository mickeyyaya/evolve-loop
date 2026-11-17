---
score_cap:
  - criterion: "The batch test fails the A+B+C prefix when the third deliverable breaks a test and lands only A and B"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestBatchPrefix_ThreeDeliverablesWhereTheLastBreaksATestFailTheCombinedPrefix$' ./internal/integration"
  - criterion: "The ejection of the third deliverable holds the diff of A+B in its brief"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestBatchPrefix_TheEjectionOfTheThirdHoldsTheDiffOfTheFirstTwoInItsBrief$' ./internal/integration"
  - criterion: "A textual conflict ejects with no test, predicate or gate run"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestBatchPrefix_ATextualConflictOfTheThirdEjectsBeforeAnyTestOrGateRuns$' ./internal/integration"
  - criterion: "An enforce-stage ejection publishes the ejected event with the brief file of its lane"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestEnforceActor_AnEjectionPublishesTheEjectedEventWithTheBriefFileOfItsLane$' ./internal/integration"
  - criterion: "The orchestrator holds the cells of an enforce-ejected lane until the landing frees them"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestOversight_AnEnforceEjectionBriefsItsLaneAndKeepsItsCellsHeldUntilTheLandingFreesThem$' ./internal/orchestrator"
---

# Eval: Wave integration I3, the prefix check, the ejection and the brief to the lane

> Pins the four acceptance lines of the inbox item wave-integration-i3-prefix-check-ejection (plan docs/plans/wave-integration-2026-10.md, decisions D5, D6 and D20). The prefix check, the ejection and the brief existed at HEAD for two deliverables. Cycle 2467 adds the three-deliverable A+B+C cases, and closes one gap: EnforceActor wrote a brief but never published the SHIP_INTEGRATION_EJECTED event that the orchestrator reads, so the lane got no brief and its cells were not held.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| prefix-fails | A+B+C prefix red, only C ejected | 6/10 | `go test -run TestBatchPrefix_ThreeDeliverablesWhereTheLastBreaksATestFailTheCombinedPrefix` |
| diff-of-prefix | C ejected with the diff of A+B | 6/10 | `go test -run TestBatchPrefix_TheEjectionOfTheThirdHoldsTheDiffOfTheFirstTwoInItsBrief` |
| conflict-no-gate | conflict ejects with no run | 6/10 | `go test -run TestBatchPrefix_ATextualConflictOfTheThirdEjectsBeforeAnyTestOrGateRuns` |
| event-published | enforce ejection reaches the event channel | 7/10 | `go test -run TestEnforceActor_AnEjectionPublishesTheEjectedEventWithTheBriefFileOfItsLane` |
| cells-held | cells stay held until landing | 7/10 | `go test -run TestOversight_AnEnforceEjectionBriefsItsLaneAndKeepsItsCellsHeldUntilTheLandingFreesThem` |
