---
score_cap:
  - criterion: "At enforce a recovery or judgment pair is written into .evolve/retrospectives-pending and the publish commits it into docs/retrospectives"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2266_00[1-2] ./acs/cycle2266"
  - criterion: "At shadow the pair goes into the workspace only"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2266_006 ./acs/cycle2266"
  - criterion: "A document that exists, pending or published, is never replaced"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2266_00[89] ./acs/cycle2266"
  - criterion: "A write or publish failure is a WARN line and never fails the closeout"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestRunCycle_AnEnforceWriteFailureIsAWarnLineAndTheCloseoutStillSucceeds ./internal/core"
  - criterion: "The boundary publish in cmd/evolve reaches the retrospective publish"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestPublishPendingDossiers_PublishesThePendingRetrospectives|TestPublishDossierPairs_PublishesThePendingRetrospectives' ./cmd/evolve"
---

# Eval: retrospectives.stage enforce routes to the pending dir and the boundary publishes it

> Pins recovery retrospective C4 (cycles 2152 and 2266). At enforce the closeout writes into
> .evolve/retrospectives-pending and the boundary publish commits into docs/retrospectives.
> Shadow stays workspace-only, writes are create-only (D13) and a failure is a WARN (D12).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| enforce-route | pending then published | 4/10 | `go test -tags acs -run TestC2266_00[1-2] ./acs/cycle2266` |
| shadow-unchanged | workspace only | 6/10 | `go test -tags acs -run TestC2266_006 ./acs/cycle2266` |
| create-only | no replace | 5/10 | `go test -tags acs -run TestC2266_00[89] ./acs/cycle2266` |
| warn-only | closeout survives failure | 5/10 | `go test -run TestRunCycle_AnEnforceWriteFailure ./internal/core` |
| wired | boundary reaches publish | 4/10 | `go test -run TestPublishPendingDossiers_PublishesThePendingRetrospectives ./cmd/evolve` |
