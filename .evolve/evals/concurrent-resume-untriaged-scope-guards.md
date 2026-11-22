---
score_cap:
  - criterion: "A granted paused scope whose inbox item is gone is not put first"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestPausedPlan_AGrantedScopeWhoseItemLeftTheInboxIsNotPutFirstEvenWithARecordedSpec' ./cmd/evolve"
  - criterion: "An untriaged paused scope leads with its recorded deps, priority and output contract"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestPausedPlan_AnUntriagedScopeLeadsWithItsRecordedDepsPriorityAndContract' ./cmd/evolve"
  - criterion: "The paused scope scan carries the recorded lane spec"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestPausedLaneScopes_CarriesTheLaneSpecOfTheLineageRecord' ./internal/core"
---

# Eval: Resume-first pool keeps the deps and contract of an untriaged scope

> Pins the guards on the resume-first backlog (`pausedPlan.order`). A granted paused scope whose inbox item is gone must not lead. An untriaged scope must lead with the dependencies, priority and output contract that its lane recorded at claim time. Source incident: console round-2 adversarial seat on concurrent-resume (F1, F2), cycle 2603.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| gone-item-guard | scope with no inbox item is not put first | 7/10 | `go test -run TestPausedPlan_AGrantedScopeWhoseItemLeftTheInbox... ./cmd/evolve` |
| lead-spec | untriaged lead keeps deps and contract | 6/10 | `go test -run TestPausedPlan_AnUntriagedScopeLeadsWith... ./cmd/evolve` |
| scan-carry | PausedLaneScopes returns the spec | 6/10 | `go test -run TestPausedLaneScopes_CarriesTheLaneSpec... ./internal/core` |
