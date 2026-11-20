---
score_cap:
  - criterion: "A judgment todo with no document on main stays in the carryover log"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestJudgmentRetire_ATodoWithNoDocumentStays$' ./internal/core/carryover"
  - criterion: "A judgment todo with its document on main leaves, reached from the closeout at enforce"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRunCycle_AtEnforceAJudgmentTodoWithItsDocumentOnMainLeavesAndTheRestStay$' ./internal/core"
  - criterion: "A second closeout at enforce changes nothing in the carryover log"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRunCycle_ASecondEnforceCloseoutChangesNothingInTheCarryoverLog$' ./internal/core"
  - criterion: "After one boundary publish at enforce, the next closeout leaves no published judgment todo in state"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRunCycle_AtEnforceTheJudgmentTodoLeavesOnlyAfterTheBoundaryPublishedItsDocument$' ./internal/core"
  - criterion: "Below enforce no judgment todo retires"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRunCycle_BelowEnforceAJudgmentTodoWithItsDocumentOnMainStays$' ./internal/core"
---

# Eval: A judgment todo leaves the carryover log only when its document is on main

> Pins recovery retrospective C5 (plan D10, goal G2, ADR-0151). Each closeout at stage
> enforce retires a `cycle-<N>-judgment-<phase>` todo only when
> `docs/retrospectives/<id>.md` exists in the published tree. Source: inbox item
> `recovery-retrospective-c5-judgment-todo-retire`, first dropped by lane triage in cycle 2269.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| stay | no document keeps the todo | 6/10 | `TestJudgmentRetire_ATodoWithNoDocumentStays` |
| leave via production caller | RunCycle at enforce retires the documented todo | 7/10 | `TestRunCycle_AtEnforceAJudgmentTodoWithItsDocumentOnMainLeavesAndTheRestStay` |
| idempotent | second closeout changes nothing | 6/10 | `TestRunCycle_ASecondEnforceCloseoutChangesNothingInTheCarryoverLog` |
| boundary | publish then closeout leaves 0 published todos | 7/10 | `TestRunCycle_AtEnforceTheJudgmentTodoLeavesOnlyAfterTheBoundaryPublishedItsDocument` |
| stage | shadow and off retire nothing | 6/10 | `TestRunCycle_BelowEnforceAJudgmentTodoWithItsDocumentOnMainStays` |
