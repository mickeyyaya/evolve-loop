---
score_cap:
  - criterion: "trigger-class (EnableContent) phases scheduled by an advisory plan respect MaxInsertions (skip + max-insertions-cap clamp at the cap)"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestRoute_AdvisoryTriggerCapEnforced|TestRoute_AdvisoryTriggerWithinCap' ./internal/router/"
  - criterion: "operator-forced (EnableOn) plan phases and ship-floor phases remain cap-exempt (floor > cap precedence)"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run 'TestRoute_AdvisoryPlanPhaseExemptsFromCap|TestRoute_AdvisoryFloorPhaseNotCapped' ./internal/router/"
---

# Eval: Advisory trigger-insertion cap (advisory-soak defect D3)

> Pins the optional-insertion budget under advisory routing: shouldRun's plan
> path may not bypass MaxInsertions for trigger-class (EnableContent) phases.
> Source incident: cycle 238 (2026-06-06) — 9 optional inserts ran against
> `max_optional_insertions=6` because the advisory plan path skipped the cap
> check wholesale. The exemption side is score-capped at 3/10 because capping
> a ship-floor phase (audit forced Run:true under a shrunken mandatory set)
> would let the cap break the integrity floor — the single worst regression
> this eval can catch (intent.md cycle-240 constraint: "veto/cap logic must
> never drop mandatory phases").

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cap-enforced | EnableContent plan phase at cap → skip + clamp; within cap → runs | 4/10 | `go test -run 'TestRoute_AdvisoryTriggerCapEnforced|TestRoute_AdvisoryTriggerWithinCap'` |
| floor-over-cap | EnableOn + ship-floor phases never cap-skipped | 3/10 | `go test -run 'TestRoute_AdvisoryPlanPhaseExemptsFromCap|TestRoute_AdvisoryFloorPhaseNotCapped'` |
