---
score_cap:
  - criterion: "A merged member moves its delivered inbox item to processed and a deferred id stays open"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestPromoter_Promote_(MovesTheDeliveredItemOfAMergedMemberToProcessed|ADeferredIdThatNeverLeftTheInboxStaysOpen)$' ./internal/inboxmover"
  - criterion: "The production enforce actor holds a Promoter and Missing() names no Promote"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestProductionEnforceActor_APromoterIsWiredSoMissingNamesNoPromote$' ./cmd/evolve"
  - criterion: "The production Flush of a merged member promotes through the production port"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestProductionEnforceActor_AMergedMemberMovesItsDeliveredItemToProcessedAndAnOpenDeferredIdStays$' ./cmd/evolve"
  - criterion: "A failed move keeps the item delivered and returns ErrMvFailed"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestPromoter_Promote_AFailedMoveKeepsTheItemDeliveredAndReturnsErrMvFailed$' ./internal/inboxmover"
  - criterion: "The change adds no test helper that is an equal copy of an existing one (testdup ratchet at its baseline)"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline$' ./internal/repocontract"
---

# Eval: The enforce actor promotes the inbox items of a merged member

> Pins the delivered-to-processed move at merged for the production enforce actor. `Mover.Promote` cannot locate an item in `delivered/`, so `inboxmover.Promoter` ends that state the way `Requeuer` ends it on ejection. Source incident: operator inbox item `train-production-promoter` (card section 7, finding 3), cycle 2658.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| promote-merged | delivered item reaches processed, deferred id stays | 4/10 | `go test -run TestPromoter_Promote_ ./internal/inboxmover` |
| missing-port | `Missing()` names no `Promote` | 4/10 | `go test -run TestProductionEnforceActor_APromoterIsWired ./cmd/evolve` |
| production-caller | Flush at merged reaches the promoter | 5/10 | `go test -run TestProductionEnforceActor_AMergedMember ./cmd/evolve` |
| failed-move | item stays delivered on a failed move | 6/10 | `go test -run TestPromoter_Promote_AFailedMove ./internal/inboxmover` |
