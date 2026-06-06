---
score_cap:
  - criterion: "insert_when on an absent generic signal evaluates FALSE for every operator (fail-closed)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestEvalCondition_AbsentFieldIsAlwaysFalse|TestTriggerFires_AbsentFieldFailsClosed' ./internal/router/"
  - criterion: "a PRESENT empty-string generic signal keeps normal string-comparison semantics (no over-fix)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestEvalCondition_PresentEmptyString ./internal/router/"
  - criterion: "typed-field semantics unchanged: the tdd conditional-mandatory pin stays true pre-handoff"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestEvalCondition_TypedFieldAbsentKeepsLegacySemantics ./internal/router/"
---

# Eval: Missing-signal fail-closed (advisory-soak defect D2)

> Pins the fail-closed contract of the routing condition evaluator: a generic
> signal field that was never emitted makes ANY insert_when/skip_when clause
> false — for negative operators (`ne`/`!=`) too. Source incident: cycle 238
> (2026-06-06), the first advisory-default soak — two catalog phases with
> `goal_type != <other-goal>` triggers both fired because `scout.goal_type`
> was absent and `"" != value` evaluated true, contributing to an 18-phase
> pile-on against an 8-phase plan. The over-fix guards (present empty string,
> typed-field tdd-pin) are score-capped separately because breaking either
> silently weakens the trigger plane or the integrity floor's tdd pin.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| absent-fail-closed | absent field false for all ops, both trigger polarities | 4/10 | `go test -run 'TestEvalCondition_AbsentFieldIsAlwaysFalse|TestTriggerFires_AbsentFieldFailsClosed'` |
| present-empty-normal | present "" keeps string semantics | 6/10 | `go test -run TestEvalCondition_PresentEmptyString` |
| typed-pin-preserved | cycle_size ne trivial true pre-handoff (tdd pin) | 5/10 | `go test -run TestEvalCondition_TypedFieldAbsentKeepsLegacySemantics` |
