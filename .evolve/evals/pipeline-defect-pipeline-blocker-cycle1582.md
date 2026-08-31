---
score_cap:
  - criterion: "recordFailureLearning does not fire on the allFamiliesQuotaExhausted (DEFERRED) dispatch arm"
    max_if_missing: 8
    evidence: "cd go && go test -run 'TestDispatch_AllFamiliesExhausted_NoFailureLearning/no_FailedRecord_appended|TestDispatch_AllFamiliesExhausted_NoFailureLearning/no_P0_carryover_todo_queued|TestDispatch_AllFamiliesExhausted_NoFailureLearning/retro_runner_never_invoked_for_learning' -count=1 -v ./internal/core"
  - criterion: "the top-level RunCycle counting-fake and the multiply-wrapped ErrAllFamiliesExhausted sentinel both confirm no bookkeeping fires on the DEFERRED arm"
    max_if_missing: 6
    evidence: "cd go && go test -run 'TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro|TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro' -count=1 -v ./internal/core"
  - criterion: "single-family exit=85 with a non-85 sibling attempt (NOT the all-families-exhausted signature) still routes through normal failure-learning — the guard is scoped, not a blanket learning suppression"
    max_if_missing: 7
    evidence: "cd go && go test -run TestDispatch_SingleFamily85WithSibling_FailureLearningUnchanged -count=1 -v ./internal/core"
---

# Eval: Guard recordFailureLearning off the all-families-quota-exhausted (DEFERRED) dispatch arm

> Pins the fix for `pipeline-defect-pipeline-blocker-cycle1582`
> (`go/internal/core/cyclerun_dispatch.go:279`): the `allFamiliesQuotaExhausted`
> arm of `dispatch()`'s attempt loop is a resource-exhaustion checkpoint meant
> to resume (`cyclehealth.ClassifyOutcome` maps its abort reason to DEFERRED,
> rc=5), not a diagnosed phase failure — but it called `recordFailureLearning`
> unconditionally, appending a `FailedRecord`, queuing a P0 carryover todo, and
> force-running the retro runner to capture a lesson on every quota wall.
> Source incident: cycle-1582 triage FAILed with `all CLI families
> quota-exhausted (exit=85)`, producing the carryover todo this cycle
> (`todo-all-families-defer-before-retro`) fixes. The fix must be scoped: a
> single exit=85 attempt with a differently-shaped sibling failure (not the
> all-85 signature) must still learn normally.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exhaustion-arm-silent | No FailedRecord/carryover-todo/retro-force-run on the all-85 path | 8/10 | `go test -run '.../no_FailedRecord_appended\|.../no_P0_carryover_todo_queued\|.../retro_runner_never_invoked_for_learning' ./internal/core` |
| classification-unchanged | RunCycle counting-fake + multiply-wrapped sentinel both see no bookkeeping | 6/10 | `go test -run 'TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro\|TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro' ./internal/core` |
| guard-scoped-not-blanket | Non-all-85 failures still learn normally | 7/10 | `go test -run TestDispatch_SingleFamily85WithSibling_FailureLearningUnchanged ./internal/core` |
