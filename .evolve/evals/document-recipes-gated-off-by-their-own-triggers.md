---
score_cap:
  - criterion: "On the real registry a strategy-options document lane's plan runs premise-challenge, forces-analysis, market-sizing and scope-baseline"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1941_001 ./acs/cycle1941"
  - criterion: "No phase named in goal_recipes[g] has a goal-type-only insert_when that cannot fire for goal type g"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1941_002 ./acs/cycle1941"
  - criterion: "A router registry test (TestGoalRecipe*) pins recipe phases to triggers that admit the recipe goal type"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run GoalRecipe ./internal/router | grep -q -- '--- PASS: TestGoalRecipe'"
---

# Eval: Goal-recipe phases are admitted by their own insert_when triggers

> Pins the fix for inbox item document-recipes-gated-off-by-their-own-triggers. The goal recipes
> strategy-options, business-plan and partnership-deal name premise-challenge, forces-analysis,
> market-sizing and scope-baseline. Their `routing.insert_when` admitted only other goal types,
> so the insert-when-gates-plan clamp skipped every one on such a lane. Source incident: the
> 2026-10-01 console probe (ORCHESTRATOR_PLAN_PHASE_GATED), materialized in cycle 1941.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| strategy-options-plan | The four recipe phases run on a strategy-options document lane | 8/10 | `go test -tags acs -run TestC1941_001 ./acs/cycle1941` |
| registry-sweep | No recipe phase is gated off by a goal-type-only trigger | 8/10 | `go test -tags acs -run TestC1941_002 ./acs/cycle1941` |
| router-pin | A router test keeps recipe and trigger as one declaration | 6/10 | `go test -run GoalRecipe ./internal/router` |
