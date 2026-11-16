# Build Explanation — Cycle 2427

## Build Binding
- Cycle: 2427
- Base SHA: 38bc28bc639269d8e738b125443e9ce24a44a869

## Summary
`loopBatchCoordinator.orchestratedRun` now constructs and runs `orchestrator.Oversight` alongside `pool.Run`, wiring `pool.Window` as `LandingObserver` and sharing `orchestrator.Intake` when cell admission loads. `docs/architecture/wave-orchestrator.md` now records that `orchestratedRun` runs the oversight loop while `Brief` stays staged.

## Rationale
Constructing `orchestrator.Oversight` inside `orchestratedRun` binds its lifecycle to `pool.Run` with a child context and waitgroup. This ensures clean shutdown when the pool exits and prevents goroutine leaks. Using an injectable `oversightSource` seam on `loopBatchCoordinator` (defaulting to `wake.New(nil)` + `orchestrator.Subscribe`) allows deterministic in-process testing while preserving graceful degradation when kernel wake refuses.

## Changed Areas
- `go/cmd/evolve/cmd_loop_batch.go` — adds the `oversightSource` field to `loopBatchCoordinator` so tests can inject a scripted `orchestrator.Source`.
- `go/cmd/evolve/cmd_loop_orchestrator.go` — wires `startOversight`, `openOversightSource`, and `oversightResize` into `orchestratedRun`. It also guards `pool.Intake` on `pool.Admit != nil` and logs a warning if the event source fails.
- `docs/architecture/wave-orchestrator.md` — updates §9 and §10 to document that `orchestratedRun` constructs and runs `orchestrator.Oversight` with `pool.Window` as `LandingObserver`, and removes stale statements that O6 is unbuilt.

## Design Decisions
- Guarded `pool.Intake = intake` on `pool.Admit != nil` because `Pool.Run` returns `errIntakeNeedsAdmit` when `Intake` is non-nil and `Admit` is nil (file-rule fallback). `Oversight` still receives the non-nil `intake` instance required by `Oversight.Run`.
- Used an `oversightSource func() (orchestrator.Source, error)` seam instead of a waiter-only factory so tests can drive `Oversight` deterministically without kernel-wake polling, while production defaults to `wake.New(nil)` and `orchestrator.Subscribe`.

## Verification
Targeted unit and race tests (`go test -race -count=1 ./cmd/evolve`), all 8 cycle-2427 ACS predicates (`go test -tags acs -count=1 ./acs/cycle2427`), and `evolve docs ste-lint --strict` pass after the final edit.

## Compatibility
Existing callers of `orchestratedRun` with `oversightSource == nil` use the default `wake.New(nil)` + `orchestrator.Subscribe` path and remain compatible. If `wake.New(nil)` returns an error, `orchestratedRun` logs a warning and runs `pool.Run` without `Oversight`.

## Limitations
`Oversight.Brief` remains `nil` (staged) until a production producer emits `SHIP_INTEGRATION_EJECTED`.
