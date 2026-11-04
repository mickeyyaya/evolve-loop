---
score_cap:
  - criterion: "A launch whose served model differs from the catalog pick emits CodeDispatchModelMismatch and exits 87"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestLaunchModelVerification_AgyClaudeBootingAnotherClaudeModelFailsOverWithTheMismatchSignal|TestLaunchModelVerification_AgyClaudeBootingAnOlderVersionOfThePickedModelIsAMismatch|TestLaunchModelVerification_ThePickedModelAtAnotherEffortStillVerifies' ./internal/bridge"
  - criterion: "claude-tmux compares the transcript message.model with the catalog pick and signals a mismatch"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestServedModel_AClaudeTmux' ./internal/bridge"
  - criterion: "agy-tmux declares the gemini launch_model_verification rule and its driver runs it"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestLaunchModelCheckFor_AgyTmuxDeclaresTheGeminiRule|TestLaunchModelVerification_AgyTmux|TestVerifiesLaunchModel_IsTheTargetDriversOwnCheck' ./internal/bridge"
  - criterion: "The boundary update stall is bounded by a budget that cuts to a skip, never a failure"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestDefaultBoundaryBudget_|TestUpdate_AStalledFamilyIsCutAtTheBudgetAndNoLaterFamilyStarts|TestUpdate_ABudgetCutAfterTheUpdaterIsNeverASmokeFailure|TestUpdate_EveryStepRunsUnderTheDefaultBoundaryDeadlineWhenTheCallerSetsNoBudget' ./internal/cliupdate"
  - criterion: "A non-positive Seams.Budget falls back to the default deadline and never expires a healthy family (cycle 2145 audit H1)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestUpdate_ANonPositiveBudgetIsTheDefaultDeadlineNotAnExpiredOne|TestUpdateAmplified_ANegativeBudgetFallsBackToTheDefaultDeadline' ./internal/cliupdate"
---

# Eval: Model currency C5 remainder, verify the launched model

> Pins the remainder of C5 in docs/plans/model-currency-2026-10.md: the launch check compares the model
> of the catalog pick, not only the family; claude-tmux reads message.model of the transcript; agy-tmux
> declares a rule; the boundary stall has a bound (N6). Source incident: cycle 2145 inbox item
> model-currency-c5-launch-model-verification-remainder.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| model-not-family | A wrong model of the right family fails the launch | 4/10 | `go test -run TestLaunchModelVerification_AgyClaudeBootingAnotherClaudeModelFailsOverWithTheMismatchSignal ./internal/bridge` |
| transcript-model | claude-tmux signals a transcript model that is not the pick | 5/10 | `go test -run TestServedModel_AClaudeTmux ./internal/bridge` |
| agy-tmux-rule | agy-tmux declares and runs the gemini rule | 5/10 | `go test -run TestLaunchModelCheckFor_AgyTmuxDeclaresTheGeminiRule ./internal/bridge` |
| n6-bound | A stalled boundary is cut at the budget to a skip | 5/10 | `go test -run TestUpdate_AStalledFamilyIsCutAtTheBudgetAndNoLaterFamilyStarts ./internal/cliupdate` |
| non-positive-budget | A negative Budget uses the default deadline | 6/10 | `go test -run TestUpdate_ANonPositiveBudgetIsTheDefaultDeadlineNotAnExpiredOne ./internal/cliupdate` |
