# Build Explanation — Cycle 2544

## Build Binding
- Cycle: 2544
- Base SHA: 48a892d1d1b5cc5c1985b88ed476309d2080896f

## Summary
`ScopeDeltaFloorChecks` now emits one `ORCHESTRATOR_SCOPE_DELTA` summary event per changed out-of-scope set. It writes the per-path list to `scope-delta.json` and skips repeat passes.

## Rationale
Emitting one event per path on every floor pass produced 804 events in cycle 2523. Moving the path list into the workspace artifact keeps the detail while limiting signals.

## Changed Areas
- `go/internal/core/scope_delta_floor.go` — aggregates out-of-scope entries into one `ORCHESTRATOR_SCOPE_DELTA` event with disposition counts. It writes `scope-delta.json` in `in.Workspace` and skips duplicate sets.
- `docs/architecture/signal-codes.md` — updates the generated `ORCHESTRATOR_SCOPE_DELTA` row to match the summary field schema and `scope-delta.json` artifact reference.

## Design Decisions
The floor stores sorted `{path, class, disposition}` entries in `<in.Workspace>/scope-delta.json`. Repeat passes in one workspace skip duplicates, while separate workspaces report independently. When `in.Workspace` is empty, the floor still emits the summary event. When no out-of-scope path exists, it removes any stale `scope-delta.json`.

## Verification
All `TestC2544_*` predicates, `TestScopeDeltaFloorChecks_*` unit tests, and `TestProductionBuildFloorChecks_ReportsAScopeDeltaThroughTheComposedFloor` pass. `TestSignalsCodes_*`, `TestC2536_*` STE-lint checks, and `evolve acs suite --cycle 2544` also pass.

## Compatibility
`CodeScopeDelta` keeps its module, origin, and `gate.passed` kind while replacing `fields.path` with summary counts (`total`, `disposition_keep`, `disposition_carve`, `disposition_refuse`). `go/cmd/evolve/cmd_cycle_config_scopedelta_test.go` now asserts the summary schema and `scope-delta.json`.

## Limitations
Per-entry classifier reasons are not stored in `scope-delta.json`. The artifact records `path`, `class`, and `disposition` for each out-of-scope entry.
