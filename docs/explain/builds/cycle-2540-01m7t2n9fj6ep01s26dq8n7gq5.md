# Build Explanation — Cycle 2540

## Build Binding
- Cycle: 2540
- Base SHA: 6e12e6032ae6f889e9a59b279593bd2d7d8478dc

## Summary
The fleet orchestrator now selects the lane lease deadline from shipped lanes or all lanes through `fleet.orchestrator.deadline_basis`.

## Rationale
Failed lanes that hit the floor or run long lift the percentile deadline for every later lane. Separating shipped lanes from failed lanes lets operators tighten the lease deadline without changing the default `all` behavior.

## Changed Areas
- `go/internal/orchestrator/runs.go` — adds `LaneRun` and `LaneRuns` to pair each cycle span with its ship outcome, and derives `LaneTimes` from `LaneRuns` to avoid duplicate stream scans.
- `go/internal/orchestrator/deadline.go` — adds `DeadlineByBasis` so `shipped` filters to shipped lanes and falls back to the floor when none shipped, while `all` matches `DeadlineOf`.
- `go/internal/policy/fleet_orchestrator.go` — adds `deadline_basis` to `FleetOrchestratorPolicy` and `DeadlineBasis` to `OrchestratorConfig`, defaults it to `all`, and rejects values other than `all` or `shipped`.
- `go/internal/policy/fleet_orchestrator_test.go` — updates the three whole-struct `OrchestratorConfig` equality expectations to include `DeadlineBasis: "all"`.
- `go/cmd/evolve/cmd_loop_orchestrator.go` — updates `newLaneLeases` to load `LaneRuns`, compute both bases, pick `LeaseConfig.Deadline` by `DeadlineBasis`, and log `deadline_all` and `deadline_shipped`.

## Design Decisions
`LaneRuns` marks a cycle as shipped when its signal stream holds a `phase.outcome` event with `phase == "ship"` and `verdict == "PASS"`. `LaneTimes` calls `LaneRuns` rather than duplicating the directory scan.

## Verification
Unit tests in `internal/orchestrator`, `internal/policy`, and `cmd/evolve` pass, and all six predicates in `acs/cycle2540` pass with `go test -tags acs`.

## Compatibility
`fleet.orchestrator.deadline_basis` defaults to `all`, and an empty `OrchestratorConfig.DeadlineBasis` behaves as `all`. The lease log line keeps its original prefix before the two new duration fields.

## Limitations
Only a `ship` phase `PASS` outcome counts as shipped; cycles without a signal stream span are omitted from both bases.
