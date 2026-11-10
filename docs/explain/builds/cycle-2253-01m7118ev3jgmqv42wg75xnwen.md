# Build Explanation — Cycle 2253

## Build Binding
- Cycle: 2253
- Base SHA: 6184cfa4a88ed9140ed28f3d9bb6cce5526375b2

## Summary
A router launch that ends at a usage-limit wall now pauses the cycle for quota. Before, the cycle fell back to the static spine and sealed SKIPPED_UNKNOWN.

## Rationale
The planning branch in the protected file `go/internal/core/cyclerun.go` drops the router error. The fix therefore uses the seams that are not protected. The router launcher keeps the wall in the error. `observedPlan` records the wall in a workspace file. `selectNext` reads that file on the first transition and calls the existing `pauseForQuota`. This reuses the checkpoint, the signal and the ledger entry of a phase pause.

## Changed Areas
- `go/internal/core/phase_advisor.go` — the bridge launcher wraps a failed launch in a `WalkError` that carries the response cause code, so the wall survives the advisor error chain.
- `go/internal/core/router_observer.go` — `observedPlan` writes `router-quota-wall.json` when the plan error holds a quota wall. `pauseAtRouterWall` reads it, benches the quota families and pauses the cycle.
- `go/internal/core/cyclerun_select.go` — `selectNext` hands the start, triage and TDD early stops to the new `stopBeforeRouting`, which calls `pauseAtRouterWall` at start. This keeps `selectNext` at its base size.
- `go/internal/bridge/launchoutcome/classify.go` — `Classify` gives exit 85 with cause code `rate_limit` or `exhausted` the signal `BRIDGE_EXIT_QUOTA_WALL`, so the exit names the quota and not an unknown prompt.
- `go/internal/bridge/launchoutcome/signal.go` — registers the code `BRIDGE_EXIT_QUOTA_WALL` with its description.
- `docs/architecture/signal-codes.md` — lists the new signal code.
- `go/internal/core/router_quota_wall_repro_test.go` — the reproduction test now calls `t.Parallel`, which the parallel-default gate requires.
- `go/acs/cycle2253/predicates_test.go` — four predicates drive the real advisor, orchestrator, storage and resume listing.

## Design Decisions
A workspace artifact carries the wall because the `Orchestrator` struct is protected. A non-quota exit 85 leaves no artifact, so it still degrades to the static spine. Benching the family gives the checkpoint a reset time, so the auto-resume budget is not zero.

## Verification
The four predicates of cycle 2253 pass. The ACS suite reports green=267 red=0. The `internal/core` package passes after the `t.Parallel` correction.

## Compatibility
No exported symbol changes. A re-plan at a later transition keeps its old behavior.

## Limitations
The mid-cycle re-plan path does not pause for quota. Only the plan at cycle start does.
