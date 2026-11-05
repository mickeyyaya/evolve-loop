# Build Explanation — Cycle 2176

## Build Binding
- Cycle: 2176
- Base SHA: ca70c89cc53c2c2319c532f3d1d377065d35348c

## Summary
The catalog refresh now runs at loop boot, gated by the TTL. A family that keeps its prior live pick emits one CONFIG_CATALOG_REFRESH_DEGRADED event to the Signal Center. The WARN line stays beside the event.

## Rationale
Most of C3 was in the tree. Two parts were open: the boot refresh and the degraded event. A loop boot has no cycle workspace, so the Signal Center is the one channel that can hold the event. The existing refresher already has the TTL gate, so the boot calls that refresher and adds no second gate.

## Changed Areas
- `go/internal/modelcatalog/degraded_event.go` — adds the registered code and the builder of the config.warning event.
- `go/cmd/evolve/cmd_models_live.go` — passes an emitter to the refresher, and emits one event for each kept family.
- `go/cmd/evolve/cmd_loop_catalog_boot.go` — adds the boot call. A nil refresher is skipped, and an error is one WARN line.
- `go/cmd/evolve/cmd_loop_prebatch.go` — calls the boot refresh after the CLI update and before the preflight.
- `go/cmd/evolve/cmd_cycle.go` — builds the refresher once with the Signal Center emitter, and gives it to the orchestrator and to the new deps field. Two pairs of `opts` append calls are now two single calls. One pair is the host effects and the lane menu. The other pair is the catalog refresher and its stage. Each option is still appended. This restores the size allowance.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the loop-boot test as serial because it swaps process seams.
- `docs/architecture/signal-codes.md` — regenerated for the new code.
- `docs/plans/model-currency-2026-10.md` — records the C3 landing.
- `docs/architecture/packages/internal-modelcatalog.md` — records the event and the code.
- `docs/architecture/model-discovery-and-catalog.md` — states that C3 has landed.

## Design Decisions
The refresher is one value that serves both the cycle start and the loop boot, so the TTL and the zero-live guard have one home. The event follows the routing defect event in `go/internal/phasespec/routing_signal.go`: the same module, kind and severity fields. It adds a `Fields` map (`cli`, `fallback_reason`) that the routing event does not have (`go/internal/modelcatalog/degraded_event.go:15`).

## Verification
The cmd/evolve, modelcatalog, signalcenter and core packages pass. The ACS suite for cycle 2176 reports red=0.

## Compatibility
The refresher signature gains one trailing argument. No policy key and no flag is added.

## Limitations
A refresher error at boot is a WARN and does not halt the boot. C2 is not built. `KeepLivePicks` (`go/internal/modelcatalog/refresh.go:66`) walks only the families of the fresh catalog. A family that is absent from the fresh catalog loses its prior live pick, and no event is emitted for it. Only the refresher in `go/cmd/evolve/cmd_models_live.go:90` calls `KeepLivePicks`. The operator verb `evolve models refresh` (`go/cmd/evolve/cmd_models.go:104`) commits through `modelcatalog.Commit` with no `KeepLivePicks` guard, so a failed live list there is not kept.
