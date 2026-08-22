---
score_cap:
  - criterion: "A non-authoritative judgment FAIL reaches the NEXT cycle's planner surface from PERSISTED state, not merely the in-memory todo array"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestJudgmentLessonEndToEnd_ReachesNextCyclePlannerPrompt$' ./internal/core"
  - criterion: "The P1 lesson survives the advisor prompt's carryover cap when the array is crowded with lower-priority entries"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestJudgmentLessonEndToEnd_SurvivesPlannerPromptCapUnderCrowdedCarryover$' ./internal/core"
  - criterion: "A CONTROL phase (retro/debugger) FAIL contributes nothing to the planner surface"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestJudgmentLessonEndToEnd_ControlPhaseFAILReachesNoPlannerPrompt$' ./internal/core"
  - criterion: "An artifact-timeout diagnostic composed with a judgment FAIL leaves exit 81 non-retryable and state.FailedAt unmutated"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestArtifactTimeoutEndToEnd_DiagnosticNeverMutatesRetryOrFailureLearning$' ./internal/core"
---

# Eval: cross-phase timeout/judgment-lesson contract

> Pins the boundary where #478 (artifact-timeout transient disclosure) and #479
> (judgment-FAIL lesson recording) meet. Each landing is green inside its own
> package and silent about the other, so the pipeline-facing claim — a diagnosed
> infrastructure timeout stays NON-retryable while a judgment verdict stays a
> planner-visible lesson that creates no failure-adapter halt vector — was never
> asserted anywhere. This eval makes that composition a permanent regression
> entry rather than a cycle-scoped predicate. Source incident: cycle 1532
> (verification lane for #478/#479); ADR-0090 decisions 1, 2 and 6.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| persistence-to-planner | Lesson reaches the planner render from persisted state | 8/10 | `go test -run TestJudgmentLessonEndToEnd_ReachesNextCyclePlannerPrompt` |
| prompt-cap-survival | P1 lesson survives the carryover prompt cap | 6/10 | `go test -run TestJudgmentLessonEndToEnd_SurvivesPlannerPromptCapUnderCrowdedCarryover` |
| control-phase-silence | Retro/debugger FAIL teaches nothing | 7/10 | `go test -run TestJudgmentLessonEndToEnd_ControlPhaseFAILReachesNoPlannerPrompt` |
| timeout-isolation | Exit 81 stays non-transient; FailedAt unmutated | 9/10 | `go test -run TestArtifactTimeoutEndToEnd_DiagnosticNeverMutatesRetryOrFailureLearning` |
