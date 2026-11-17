# Build Explanation — Cycle 2452

## Build Binding
- Cycle: 2452
- Base SHA: f66b7c7b7f7243e042543ed468c72e897bb43058

## Summary
Production now wires `orchestrator.RunsLocator` as the `Locate` port. A new life of a todo can no longer take the cycle of its earlier life.

## Rationale
`laneLocator` wrapped `LaneCycle`, which has a floor of zero. `RunsLocator` already has the earlier-life floor, but only tests called it. Wiring the existing function is the smallest change. It adds no second locator.

## Changed Areas
- `go/cmd/evolve/cmd_loop_orchestrator.go` — `laneLocator` returns `RunsLocator` over the runs directory, so `Pool.recover` and the oversight get the floor.
- `go/internal/orchestrator/runs.go` — `RunsLocator` guards its two maps with a mutex, because the seal and the oversight now call it at the same time.
- `docs/architecture/packages/internal-orchestrator.md` — the package note names `RunsLocator` as the `Locate` port that production wires, and drops the single-caller claim.

## Design Decisions
One mutex covers the whole locate call. The scan is short and the callers are few, so a finer lock adds risk without gain.

## Verification
`TestLaneLocator_ANewLifeOfATodoNeverTakesTheCycleOfItsEarlierLife` reaches the port through `orchestratorPool`. `TestRunsLocator_ASealAndTheOversightMayLocateAtOnce` runs under `-race`.

## Compatibility
The `Locate` signature does not change. A first life finds its cycle as before.

## Limitations
The cache lives for one loop run only. It is not persisted.
