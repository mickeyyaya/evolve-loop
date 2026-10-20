# Build Explanation — Cycle 2021

## Build Binding
- Cycle: 2021
- Base SHA: ba66e738f3fe3567b53fc778d1da88d9b9ee251c

## Summary
A phase spec now carries a `stage` field with the closed set `pre-build` and `post-build`. The routing splice refuses a `pre-build` phase whose slot falls after `build`, and reports it as the registered routing-defect signal. `bug-reproduction` and `fault-localization` declare `pre-build`.

## Rationale
Cycle 1550 ran a red-first Evaluate phase after build and reddened the lane. The window pin named the two phases in code, so a new phase of the same kind had no floor. A declared field lets the splice check any phase. The check runs after placement, so it covers an explicit anchor, the default pre-audit slot, and a chain behind a post-build anchor with one rule.

## Changed Areas
- `go/internal/phasespec/phasespec.go` — adds `PhaseSpec.Stage`, the `StagePreBuild` and `StagePostBuild` words, and the stage check in `Load`.
- `go/internal/phasespec/validate.go` — adds `ValidateStage` and runs it from user-spec validation, so an unknown stage leaves the spec unrouted with a warning.
- `go/internal/phasespec/routing.go` — after placement, removes each newly placed `pre-build` spec that sits after `build`, drops its triggers and content-enable, and returns a warning that names it.
- `go/internal/phasespec/routing_signal.go` — the registered description of the routing-defect code now lists the refusal.
- `docs/architecture/signal-codes.md` — regenerated from the registry for that description.
- `.evolve/phases/bug-reproduction/phase.json` — declares `stage: pre-build`.
- `.evolve/phases/fault-localization/phase.json` — declares `stage: pre-build`.
- `docs/architecture/phase-descriptor.schema.json` — adds the `stage` property with the closed enum, because `additionalProperties` is false.
- `docs/architecture/user-defined-phases.md` — adds the `stage` row to the phase.json field table.
- `docs/architecture/packages/internal-phasespec.md` — documents the field and the refusal in the routing splice note.
- `go/cmd/evolve/routing_order_realtree_test.go` — the window pin reads `Stage` and the anchor instead of the two phase names.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the two new wiring tests as serial, like the existing anchor wiring test (process-global model catalog setter).
- `go/internal/phasespec/stage_test.go` — TDD-owned tests for the closed set, the refusal and the tracked catalog.
- `go/cmd/evolve/phase_stage_signal_test.go` — TDD-owned wiring tests for the signal reaching the Signal Center.
- `go/cmd/evolve/routing_order_stage_realtree_test.go` — TDD-owned real-tree tests of the pre-build window and of the pin.
- `go/acs/cycle2021/predicates_test.go` — TDD-owned acceptance predicates.
- `.evolve/evals/pre-build-evaluate-floor-needs-a-stage-field.md` — the eval for this task.

## Design Decisions
The check is a post-placement pass, not a guard inside each splice branch. Rejected: guarding in `spliceAfter`, because the fixpoint, the forced-placement fallback and the cycle breaker each place specs and would each need the guard. Rejected: a new phase-name list, because that repeats the defect the field removes. Only specs that this call newly placed are checked, so an overlay already in the order is never removed.

## Verification
`go test -count=1 ./internal/phasespec` passes. The cycle-2021 ACS predicates pass (4/4). `evolve acs suite --cycle 2021` reports green=240 red=0.

## Compatibility
`stage` is optional. An empty or `post-build` stage is never fenced. No tracked phase other than the two declared ones changes routing.

## Limitations
A refused phase is dropped for the cycle, not moved. The operator must fix the `after` anchor. A `pre-build` phase in an order with no `build` phase is legal.
