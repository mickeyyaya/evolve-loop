# Build Explanation — Cycle 2654

## Build Binding
- Cycle: 2654
- Base SHA: 92ab6246c0753563d4fdce0eba19c4c1ce8b6982

## Summary
Fleet lane checkpoints now record their pool identifier, lane cycle number, and scope. Resumed fleet loops read this identity back so enforce delivery writes the wave record and stops before main.

## Rationale
Without checkpoint identity, a resumed lane loop receives no pool or scope and runs as a sequential cycle. At enforce, that gap lets a resumed lane push straight to main instead of the wave record.

## Changed Areas
- `go/internal/checkpoint/checkpoint.go` — adds `PoolID`, `LaneCycle`, and `Scope` fields to `Checkpoint`, adds `BindLane`, and populates lane identity in `Compose`.
- `go/internal/core/resume.go` — adds `PoolID`, `LaneCycle`, and `Scope` to `ResumePoint` and reads them in `loadResumeStateFrom`.
- `go/cmd/evolve/routing_run.go` — adds `scope` and `integrationScope` to `routingRun` so resumed lane scope reaches ship integration wiring.
- `go/cmd/evolve/cmd_cycle.go` — binds the pool in `runCycleRun` with `checkpoint.BindLane` and passes resumed `run.scope` and `run.pool` into `shipIntegrationOf`.
- `go/cmd/evolve/cmd_loop.go` — preloads `ResumePoint` before orchestrator wiring in `runLoopBatch` and populates `routingRun` through `resumeRoutingRun`.
- `go/cmd/evolve/cmd_loop_resume.go` — loads the preloaded resume point in `runResumeBatch`, refuses identity-less legacy lane checkpoints at enforce, and binds the checkpoint pool for re-checkpoints.

## Design Decisions
- `BindLane` stores the active pool in a process-atomic variable instead of adding a pool field to `CycleState`, because `go/internal/core/orchestrator.go` and `go/internal/core/cyclerun.go` are protected files.
- Sequential checkpoints omit `poolId`, `laneCycle`, and `scope` with `omitempty`, so legacy lane checkpoints remain detectable at enforce when a lane pin exists without a pool.
- `runLoopBatch` preloads the checkpoint once before `wireOrchestratorDepsFn` so the single orchestrator wiring call receives `pool` and `scope` without a second wiring pass.

## Verification
Unit tests in `internal/checkpoint`, `internal/core`, and `cmd/evolve`, plus cycle 2654 predicates 001, 002, 003, 005, 006, and 007 pass. Integration test `TestDeliver_EnforceStopsBeforeMainAfterWritingTheRecord` passes when run with `-tags integration`.

## Compatibility
Sequential checkpoints and shadow-stage resumes preserve their previous behavior. Legacy lane checkpoints without a pool fail before any phase runs when `fleet.integration.stage` is `enforce`.

## Limitations
Stale or closed pool epoch checks require the wave-store epoch and remain outside this cycle's scope.
