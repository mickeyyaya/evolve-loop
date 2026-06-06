---
score_cap:
  - criterion: "plan run:false vetoes a firing insert_when trigger at the router kernel (walk/shouldRun)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestRoute_AdvisoryPlanRunFalse|TestRoute_AdvisoryPlanVetoUserPhaseAbsentSignal' ./internal/router/"
  - criterion: "the plan veto survives the orchestrator's enforceNext decline-fallback (vetoed phases never run via nextInOrder when the spine gate declines the router's skip)"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run TestOrchestrator_AdvisoryPlanVetoSurvivesSpineDecline ./internal/core/"
  - criterion: "EVOLVE_REQUIRE_INTENT=1 forces an intent Run:true entry into the clamped advisory plan (require-intent clamp)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestClampPlanToFloor_Intent ./internal/router/"
  - criterion: "the orchestrator threads IntentRequired into the plan-clamp RouteInput"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestOrchestrator_ThreadsIntentRequiredToPlannerInput ./internal/core/"
---

# Eval: Advisory plan veto + require-intent floor (advisory-soak defects D1 + D4)

> Pins the precedence lattice settled in cycle 240: floor/required > plan veto
> > trigger. D1: the advisor's run:false (or omission) must beat a firing
> insert_when trigger at BOTH layers — the pure router walk AND the
> orchestrator's enforceNext decline-fallback, which in cycle 238 (2026-06-06)
> chained 10 unplanned catalog phases through nextInOrder while the router
> proposed "audit" nine times in a row (routing-decision-{8..16}.json). D4:
> the operator's EVOLVE_REQUIRE_INTENT=1 outranks the plan — the floor clamp
> forces intent Run:true (rule "require-intent"), independent of planRuns(ship),
> mirroring NextFromStart on the static path. Source incident: cycle 238, first
> advisory-default soak; inbox 2026-06-07T00-20-00Z-advisory-soak-defects.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| kernel-veto | plan run:false beats firing trigger in walk | 4/10 | `go test -run 'TestRoute_AdvisoryPlanRunFalse|TestRoute_AdvisoryPlanVetoUserPhaseAbsentSignal'` |
| decline-fallback-veto | veto survives enforceNext decline (cycle-238 pile-on mechanism) | 3/10 | `go test -run TestOrchestrator_AdvisoryPlanVetoSurvivesSpineDecline` |
| require-intent-clamp | intent forced Run:true when required | 4/10 | `go test -run TestClampPlanToFloor_Intent` |
| intent-threading | orchestrator passes IntentRequired to the clamp input | 5/10 | `go test -run TestOrchestrator_ThreadsIntentRequiredToPlannerInput` |
