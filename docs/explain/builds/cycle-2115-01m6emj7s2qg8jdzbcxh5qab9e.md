# Build Explanation — Cycle 2115

## Build Binding
- Cycle: 2115
- Base SHA: f192ee804c3bc258972ddb14f345c5fe8f824aa1

## Summary
The command `evolve loop --dry-run` now prints a `wave_plan` key beside the resolved config. The key lists the width, the reason, the items of each lane with their weights, and the items that the wave excludes.

## Rationale
Before this change, the dry run printed the config only. The operator cannot see which items the next wave dispatches. The plan must equal the plan of the wave engine, so the dry run calls the same planning body as the dispatch. The dry run must not write state, so it uses the plan function without the stale-claim release.

## Changed Areas
- `go/internal/loopwave/preview.go` — adds `Engine.Preview` and the plan types. It builds the lanes from the shared planning body, routes width sizing through `Engine.Size`, and lists the excluded items with the reason.
- `go/internal/loopwave/plan.go` — splits `PlanFn` into `planFn(count, release)`, so a read-only variant exists. `PlanFn` keeps its behavior.
- `go/internal/loopwave/dispatch.go` — extracts `planLanes` from `dispatch`. The dispatch and the preview now share one plan and adapt step.
- `go/cmd/evolve/cmd_loop.go` — `loopDryRunPlan` adds the `wave_plan` key. It is the production caller of `Engine.Preview`.

## Design Decisions
One planning body serves both the dispatch and the preview. An alternative was a second planner in the command layer. The second planner can drift from the engine, so it was rejected. A flag on `PlanFn` was also rejected: `planFn` takes a parameter and the exported `PlanFn` keeps its signature. Width sizing routes through `Engine.Size` to evaluate budget settings without live quota probes or throughput history.

## Verification
The ACS predicates `TestC2115_001` to `TestC2115_010` run the built binary on fixture inboxes and test preview sizing against `Engine.Size`. They compare the printed lanes with the lanes that the wave engine dispatches. They compare the project tree before and after the dry run. The package `loopwave` keeps 100% statement coverage.

## Compatibility
The existing keys `dry_run` and `config` are unchanged. The key `wave_plan` is new. `PlanFn` and `Dispatch` behave as before.

## Limitations
The preview does not model the last-moment freshness gate or the backlog refill of the launcher. The preview does not model the pool scheduler; it names the pool as the reason. The preview does not release stale claims (`go/internal/loopwave/plan.go:19`), so an item that the real wave reclaims is absent from the preview. The dry run does not probe quota, so with `fleet.budget.stage=enforce` it holds the configured width.
