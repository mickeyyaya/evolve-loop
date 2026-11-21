# Build Explanation — Cycle 2594

## Build Binding
- Cycle: 2594
- Base SHA: a1d94de45f161e7196b00f806015f048d8447aba

## Summary
A lane that exits with the system-failure halt code stops the pool backfill and drains the wave. The pool counts live lanes in flight and emits `live` on the `LOOP_ORCH_HALT_DRAINING` incident. The idle cause is `system_halt`, and the wave ends when the last lane seals.

## Rationale
Wave 182 continued to backfill lanes after `LOOP_SYSTEM_FAILURE_HALT` and idled with `operator_stop`. Cycle 2585 built the halt drain and `system_halt` idle cause, but its incident omitted the registered `live` field. Tracking live lanes across `Prepare` and `watchSystemHalt` aligns the emitted incident with the registry.

## Changed Areas
- `go/internal/orchestrator/pool.go` — adds `SystemFailureHaltExit`, the `sysHalt` flag, live-lane tracking across `Prepare` and `watchSystemHalt`, and `LOOP_ORCH_HALT_DRAINING` emission with `todo`, `rc`, `cause`, and `live`.
- `go/internal/orchestrator/idle_cause.go` — adds `CauseSystemHalt` (`system_halt`) to `IdleCauses` with higher priority than `operator_stop`.
- `go/internal/orchestrator/orchestrator.go` — registers `CodeHaltDraining` (`LOOP_ORCH_HALT_DRAINING`) with fields `todo`, `rc`, `cause`, and `live`.
- `go/cmd/evolve/cmd_loop_systemfailure_halt.go` — points `systemFailureHaltExitCode` to `orchestrator.SystemFailureHaltExit` so one constant defines exit code 4.
- `docs/architecture/signal-codes.md` — records `LOOP_ORCH_HALT_DRAINING` and the `system_halt` idle cause in the generated signal table.
- `docs/architecture/packages/internal-orchestrator.md` — documents the drain rule and the updated idle cause order.

## Design Decisions
Tracking `live` with an atomic counter inside `Pool.Run` rather than reading `fleet.RunPool` internals keeps the `fleet` package unchanged. Emitting `live` rather than deleting it from `orchestrator.go` preserves operator visibility into how many lanes remain active during a drain. Adding a `Pool` option field was rejected because halt drain has no valid off state.

## Verification
Unit tests in `internal/orchestrator` (`-race -cover`, 100.0% statement coverage), `TestOrchestratedRun_ASystemFailureHaltLaneStopsTheBackfill` in `cmd/evolve`, and all 11 predicates in `go/acs/cycle2594` pass.

## Compatibility
Pools without a rc 4 lane exit behave as before. The closed `IdleCauses` set includes one more value (`system_halt`).

## Limitations
The drain applies to the active wave pool; batch-level halt logic after the wave finishes is unchanged.
