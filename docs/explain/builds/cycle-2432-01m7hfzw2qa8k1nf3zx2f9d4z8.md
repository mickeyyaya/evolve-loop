# Build Explanation — Cycle 2432

## Build Binding
- Cycle: 2432
- Base SHA: fd03a5b3d9f8606984db139911e9c3539b8d9896

## Summary
Documented the idle cause precedence in the `internal/orchestrator` package note so that `operator_stop` taking precedence over `run_cap` is explicitly specified and pinned by regression tests.

## Rationale
Production wiring in `go/cmd/evolve/cmd_loop_orchestrator.go` and `go/internal/orchestrator/idle_cause.go` already checks `t.Stopped` before `t.Capped` and passes `Halted: b.poolHalted`. Updating only the package note avoids unnecessary production code changes while satisfying the documentation and STE-lint acceptance criteria.

## Changed Areas
- `docs/architecture/packages/internal-orchestrator.md` — added a Design bullet stating the `idleCause` precedence order (`operator_stop`, `run_cap`, `family_held`, `supply_empty`, `bench`, `admission_refused`) in ASD-STE100 and naming the pinning tests.
- `go/internal/orchestrator/idle_cause_test.go` — added `TestPool_ABrakedPoolPastItsRunCapNamesTheOperatorStop` (TDD phase) to pin that a braked pool at `MaxLanes` reports `operator_stop` and emits no underfill.
- `go/cmd/evolve/cmd_loop_oversight_test.go` — added `TestOrchestratedRun_AWaveStopThatMeetsTheRunCapNamesTheOperatorStop` (TDD phase) to pin end-to-end `orchestratedRun` wiring when a wave stop meets `RunMaxLanes`.
- `go/acs/cycle2432/predicates_test.go` — added the cycle-2432 ACS predicate suite (`TestC2432_001` through `TestC2432_005`, TDD phase) to verify pool behavior, loop wiring, and STE package-note compliance.
- `.evolve/evals/wave-stop-is-classified-as-operator-stop.md` — added the cycle-2432 eval contract (TDD phase) binding score caps and test evidence for `operator_stop` precedence.

## Design Decisions
Keeping the existing `idleCause` switch order and `Halted: b.poolHalted` wiring unchanged preserves the minimal diff and keeps 100% statement coverage in `internal/orchestrator`.

## Verification
`evolve docs ste-lint --strict docs/architecture/packages/internal-orchestrator.md` reports 0 findings, and all five cycle-2432 ACS predicates (`TestC2432_001` through `TestC2432_005`) pass.

## Compatibility
No public API, schema, or runtime behavior changed.

## Limitations
None; the production code already satisfied the `operator_stop` precedence requirement at the base SHA.
