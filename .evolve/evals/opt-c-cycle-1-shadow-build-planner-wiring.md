# Eval: opt-c-cycle-1-shadow-build-planner-wiring

## Code Graders (bash commands that must exit 0)
- `[code]` `test -f agents/evolve-build-planner.md`
- `[code]` `test -f .evolve/profiles/build-planner.json`
- `[code]` `test -f docs/architecture/adr/0019-build-planner-phase.md`

## Regression Evals (full test suite)
- `[code]` `cd go && go test ./...`

## Acceptance Checks
- `[code]` `jq -e '.phases[] | select(.name == "build-planner")' docs/architecture/phase-registry.json`
- `[code]` `grep -q "gate_tdd_to_build_planner" scripts/lifecycle/phase-gate.sh`
- `[code]` `grep -q "gate_build_planner_to_build" scripts/lifecycle/phase-gate.sh`
- `[code]` `grep -q "build-planner" scripts/dispatch/list-phase-order.sh`
- `[code]` `grep -q "build-planner" scripts/dispatch/subagent-run.sh`
- `[code]` `grep -q "build-planner" scripts/guards/phase-gate-precondition.sh`

## Thresholds
- All checks: pass@1 = 1.0
