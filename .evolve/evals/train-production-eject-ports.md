---
score_cap:
  - criterion: "The production enforce actor names no Requeue or Briefs port as missing"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestProductionDriver_CheckRefusesEnforceOnTheProductionCompositionWithoutTheLedgerPorts$' ./cmd/evolve"
  - criterion: "A conflicting member leaves delivered for the queue with its brief path through the production ports"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestProductionEnforceActor_AnEjectedMemberLeavesDeliveredForTheQueueWithItsBrief$' ./cmd/evolve"
  - criterion: "A brief that cannot be written keeps the item delivered and reports the brief step"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestProductionEnforceActor_ABriefThatCannotBeWrittenKeepsTheItemDelivered$' ./cmd/evolve"
  - criterion: "The next lane prompt shows the brief path of a requeued item"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestInboxBatchesSection_AnItemWithABriefPathShowsThePathToTheNextLane$' ./internal/phases/triage"
---

# Eval: The enforce actor gets production Requeue and Briefs ports

> Pins the production wiring of `EnforceActor.Requeue` and `EnforceActor.Briefs` in `productionEnforceActor`. Before this work, `Missing()` named both ports, so an ejected lane could not return to the queue with its brief. Source: inbox item `train-production-eject-ports`, cycle 2656.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| missing-ports | Missing() names only Promote | 4/10 | `TestProductionDriver_CheckRefusesEnforceOnTheProductionCompositionWithoutTheLedgerPorts` |
| eject-journey | Ejected item returns to the queue with `brief_path` and one more failure | 4/10 | `TestProductionEnforceActor_AnEjectedMemberLeavesDeliveredForTheQueueWithItsBrief` |
| brief-failure | No requeue without a written brief | 6/10 | `TestProductionEnforceActor_ABriefThatCannotBeWrittenKeepsTheItemDelivered` |
| prompt | The next lane prompt carries the brief path | 5/10 | `TestInboxBatchesSection_AnItemWithABriefPathShowsThePathToTheNextLane` |
