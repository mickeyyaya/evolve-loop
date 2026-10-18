# Build Explanation — Cycle 1941

## Build Binding
- Cycle: 1941
- Base SHA: df4ae9c1e839dd1060d41d1a4248fc6743bfab3a

## Summary
Each phase that a goal recipe names now has an `insert_when` trigger that admits the goal type of that recipe. Before this change, the `insert-when-gates-plan` clamp skipped premise-challenge, forces-analysis, market-sizing and scope-baseline on a strategy-options document lane. The clamp skipped other recipe phases on their lanes for the same reason.

## Rationale
The router skips a planned, content-enabled phase when its `insert_when` does not fire (`go/internal/router/router.go:403`). The recipes in `goal_recipes` (`docs/architecture/phase-registry.json`) named phases whose triggers admitted only one other goal type. Thus the router clamped the recipe that it advertised. The smallest correct fix adds the recipe goal type to the trigger of each phase, with one `scout.goal_type ==` condition per goal type. The router ORs `insert_when` conditions (`triggerFires`), and the condition grammar has no `in` operator (`go/internal/router/condition.go:16-40`).

Rejected option: exempt recipe phases from the clamp in Go. That change adds a second routing rule beside the phase.json declaration, and the declaration must stay the single source (see `TestDocumentLane_TheGateIsThePhaseJSONDeclarationNotAGoList`).

## Changed Areas
- `.evolve/phases/forces-analysis/phase.json` — admits strategy-options, business-plan and partnership-deal, which name the phase in their recipes.
- `.evolve/phases/market-sizing/phase.json` — admits strategy-options, business-plan and partnership-deal.
- `.evolve/phases/scope-baseline/phase.json` — admits accessibility, i18n, data-pipeline, strategy-options, business-plan and partnership-deal.
- `.evolve/phases/okr-draft/phase.json` — admits business-plan.
- `.evolve/phases/risk-register/phase.json` — admits business-plan and partnership-deal.
- `.evolve/phases/api-contract-design/phase.json` — admits api-design.
- `.evolve/phases/prd-draft/phase.json` — admits frontend-ui.
- `.evolve/phases/premise-challenge/phase.json` — admits strategy-options in addition to the existing large-cycle trigger. Thus a small strategy-options document lane runs it.
- `.evolve/phases/migration-safety-check/phase.json` — admits data-pipeline and database (a post-build recipe phase with the same defect).
- `.evolve/phases/accessibility-audit/phase.json` — admits frontend-ui (same defect, post-build).
- `.evolve/phases/contract-fuzz-probe/phase.json` — admits messaging (same defect, post-build).
- `go/internal/router/recipes_drift_test.go` — adds the `TestGoalRecipe*` tests. One test sweeps the real registry for a recipe phase that a goal-type-only trigger gates off. One test routes a strategy-options document lane. One table test pins the classifier of the sweep.
- `go/internal/router/document_lane_routing_test.go` — the cycle-1692 plan test now routes past premise-challenge, which the strategy-options lane runs now, before it asserts that the bugfix phases are skipped. The assertions are unchanged.

## Design Decisions
The router test checks the trigger statically with `triggerFires` for every recipe phase whose trigger is goal-type-only. Thus it also catches post-build phases that a route evaluated at triage does not reach. That is how it found migration-safety-check, accessibility-audit and contract-fuzz-probe. A trigger with a size or kind condition is out of the sweep. This is because such a trigger can admit a lane on purpose for reasons other than the goal type.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1941` — 4/4 predicates pass.
- `go test -count=1 -v -run GoalRecipe ./internal/router` — 3/3 pass.
- `go test -count=1 ./internal/router ./internal/phasespec ./internal/config` — pass.

## Compatibility
The changes only add conditions. No condition was removed, so every lane that ran these phases before still runs them.

## Limitations
The bugfix, feature and agent-instruction recipes also name premise-challenge, and its large-cycle trigger still skips it on their small lanes. The sweep does not flag a size trigger, so this cycle leaves that behavior unchanged on purpose.
