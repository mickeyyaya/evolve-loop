---
score_cap:
  - criterion: "SpineSatisfiedUpTo rejects a ship plan whose build artifact is absent"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestSpineSatisfiedUpTo_ShipRequiresBuild' ./internal/core/... 2>&1 | grep -qE '^ok|PASS'"
  - criterion: "At least 4 direct SpineSatisfiedUpTo tests pass (floor regression guard)"
    max_if_missing: 5
    evidence: "cd go && test $(go test -json -count=1 -run 'TestSpineSatisfiedUpTo' ./internal/core/... 2>/dev/null | sed -n 's/.*\"Action\":\"pass\".*\"Test\":\"\\([A-Za-z0-9_]*\\)\".*/\\1/p' | grep -E 'TestSpineSatisfiedUpTo' | sort -u | grep -c '') -ge 4"
---

# Eval: Phase-gate floor — SpineSatisfiedUpTo ship-without-build negative test

> Pins cycle-192 intent AC6: a floor-violating plan that reaches ship WITHOUT a
> real build (and without audit) must be rejected by the runtime phase gate
> `core.StateMachine.SpineSatisfiedUpTo`, not merely by the plan-level
> `ClampPlanToFloor` prefilter. Scout finding #3 claimed this had "zero direct
> tests"; that was inaccurate — `internal/core/statemachine_spine_test.go` already
> covered the audit-absent and scout-absent negatives. The genuine residual gap
> this eval closes is the SHIP-WITHOUT-BUILD case: every prior ship scenario set
> `Build.Present=true`, so the build-absent floor (which the SpineSatisfiedUpTo
> loop already enforces) had no direct regression guard. Source incident:
> cycle 192 (defense-in-depth on the non-gameable ship floor).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ship-requires-build | build-absent → ship blocked | 7/10 | `go test -run TestSpineSatisfiedUpTo_ShipRequiresBuild ./internal/core/...` |
| spine-floor-regression | >=4 SpineSatisfiedUpTo direct tests pass | 5/10 | `go test -json -run TestSpineSatisfiedUpTo` distinct-pass count >= 4 |
