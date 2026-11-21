# Build Explanation — Cycle 2585

## Build Binding
- Cycle: 2585
- Base SHA: 23c32f42926c3dcde2c2da08401c897afd66275a

## Summary
A lane that exits with the system-failure halt code now stops the pool backfill. The wave ends when the last running lane seals. The idle cause is `system_halt`, and the pool raises an INCIDENT.

## Rationale
Wave 182 backfilled lanes after `LOOP_SYSTEM_FAILURE_HALT` and then idled for hours with the cause `operator_stop`. The pool had no knowledge of the halt code. The pool now wraps its launch function and watches the exit code.

## Changed Areas
- `go/internal/orchestrator/pool.go` — adds `SystemFailureHaltExit`, the halt flag and the launch wrapper. The flag closes the fill gate and raises the incident once.
- `go/internal/orchestrator/idle_cause.go` — adds the cause `system_halt` to the closed set. The cause outranks `operator_stop`.
- `go/internal/orchestrator/orchestrator.go` — registers the signal code `LOOP_ORCH_HALT_DRAINING`.
- `go/cmd/evolve/cmd_loop_systemfailure_halt.go` — the loop halt exit code now reads the orchestrator constant, so one home holds the value.
- `docs/architecture/signal-codes.md` — regenerated from the registry; it lists the new code and the new idle cause.
- `docs/architecture/packages/internal-orchestrator.md` — documents the drain rule and the new cause order.

## Design Decisions
The watch sits in `Pool.Run`, the one entry of every pool launch, so both the loop caller and tests reach it. A new option field was rejected: the behavior has no valid off state.

## Verification
Pool tests, the idle cause test and a cmd/evolve caller test cover the halt, the cause and the incident. The ACS predicates of cycle 2585 cover the same seams.

## Compatibility
Pools with no rc 4 lane behave as before. The idle cause set grows by one member.

## Limitations
The halt applies to the pool of one wave. The batch-level halt decision is unchanged.
