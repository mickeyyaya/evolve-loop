---
score_cap:
  - criterion: "With a running lane that edits internal/core, the optimistic rule admits a candidate that edits internal/router"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestAdmit_OptimisticAdmitsACandidateThatOnlyTheRunningLaneImports|TestAdmitByCell_UnderTheOptimisticRuleACoreLaneDoesNotHoldARouterTodo' ./internal/admission ./internal/orchestrator && go test -tags acs -count=1 -run TestC1995_001 ./acs/cycle1995"
  - criterion: "Two candidates that edit one package never run at once and the hub rule is unchanged"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestAdmit_OptimisticStillHoldsTwoTodosThatEditOnePackage|TestAdmit_OptimisticKeepsTheHubRule|TestAdmit_OptimisticAdmitsExactlyTheTodosThatShareNoPackageAndAreNotBothHubs' ./internal/admission && go test -tags acs -count=1 -run TestC1995_002 ./acs/cycle1995"
  - criterion: "At shadow the admitted set is byte-identical to today and each new admission is recorded with its holder"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestAdmit_Shadow|TestAdmit_TheShadowChangesNoDecisionOverRandomBacklogs|TestRecordShadow_|TestAdmitByCell_AtShadow' ./internal/admission ./internal/orchestrator && go test -tags acs -count=1 -run TestC1995_003 ./acs/cycle1995"
  - criterion: "The production pool admission reaches the shadow record and the policy admission key defaults to shadow"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCellAdmission_|TestOrchestratorPool_ThePoolAdmitReachesTheAdmissionShadow' ./cmd/evolve && go test -count=1 -run 'Admission' ./internal/policy"
---

# Eval: Wave latency W1, optimistic admission at stage shadow

> Pins ADR-0143 decision 1 to 4 at stage shadow. A lane that edits internal/core must stop holding a
> candidate that edits internal/router. The present reach rule keeps governing the admitted set until
> the admission key moves to enforce, and each candidate that only the reach rule holds is recorded
> with its holder. Source incident: waves 130 to 133, a core lane held 13 ready items for 71 minutes.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| core-beside-router | the new rule admits router beside core | 4/10 | `TestAdmit_OptimisticAdmitsACandidateThatOnlyTheRunningLaneImports` |
| same-package-and-hub | shared edits and two hubs still hold | 5/10 | `TestAdmit_OptimisticStillHoldsTwoTodosThatEditOnePackage` |
| shadow-identical | the shadow sink changes no decision | 5/10 | `TestAdmit_TheShadowChangesNoDecisionOverRandomBacklogs` |
| production-caller | cellAdmission records through the pool admit | 6/10 | `TestCellAdmission_AtShadowTheReachRuleHoldsAndThePoolAdmissionRecordsTheHolder` |
