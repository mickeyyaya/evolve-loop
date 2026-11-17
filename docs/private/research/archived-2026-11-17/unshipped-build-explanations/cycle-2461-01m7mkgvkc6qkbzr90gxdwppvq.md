# Build Explanation — Cycle 2461

## Build Binding
- Cycle: 2461
- Base SHA: c73010a5f801e27a855c9e6f21b43e7c9c05e916

## Summary
Admission can key a todo by file for the leaf packages that the policy key `fleet.orchestrator.file_cells` lists. Two todos that edit two files of `cmd/evolve` now run in two lanes.

## Rationale
Plan K1 of admission-cmd-evolve-cell splits the one cell of `cmd/evolve` by file. A footprint gets a file-key set beside its package-key set, so the existing conflict rule stays one function.

## Changed Areas
- `go/internal/admission/admission.go` — adds `Admitter.FileCells`, the file-key sets of a footprint and the whole-package against file-key conflict.
- `go/internal/policy/fleet_orchestrator.go` — adds the `file_cells` key, its validation and `OrchestratorConfig.FileCells()`.
- `go/internal/orchestrator/supply.go` — adds `Census.FileCells` and passes it to the admitter.
- `go/cmd/evolve/cmd_loop_admission.go` — the wave shadow reads the file cells from the policy.
- `go/cmd/evolve/cmd_loop_orchestrator.go` — the pool admission reads the file cells from the policy.
- `go/cmd/evolve/cmd_supply_wiring.go` — the census reads the file cells from the policy.
- `docs/architecture/packages/internal-admission.md` — describes the file keys.
- `docs/architecture/packages/internal-policy.md` — describes the policy key.

## Design Decisions
A file key exists only for a leaf package in the list; a leaf has no importers, so a file key adds no reach. The list is stored as a newline-joined string in `OrchestratorConfig` so that the struct stays comparable.

## Verification
The admission, orchestrator and cmd/evolve tests of the cycle pass. One policy test fails for a reason that no production edit can fix (see the build report).

## Compatibility
An empty or unset `file_cells` list keeps package keys, as before.

## Limitations
Only leaf packages get file keys. Non-Go files of a listed leaf stay whole-package edits.
