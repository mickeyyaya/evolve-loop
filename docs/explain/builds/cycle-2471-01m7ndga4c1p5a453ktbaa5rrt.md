# Build Explanation — Cycle 2471

## Build Binding
- Cycle: 2471
- Base SHA: de22fd9eeaecac66f9b14ffb7631521b3ac0c4d5

## Summary
Admission can now give each `.go` file of a listed leaf package its own key. Two todos that edit two different files of `cmd/evolve` run in two lanes.

## Rationale
The policy key `fleet.orchestrator.file_cells` selects the packages. An empty list keeps the old behavior, so no feature flag is needed. A package with importers keeps its package key, because other packages compile against each of its files.

## Changed Areas
- `go/internal/admission/admission.go` — adds the file keys and the conflict rules (same file, whole package against file, non-leaf package).
- `go/internal/policy/fleet_orchestrator.go` — parses and validates the `file_cells` key.
- `go/internal/orchestrator/supply.go` — passes the file cells to the census only.
- `go/cmd/evolve/cmd_loop_admission.go` — reads `file_cells` from the policy for the loop admission.
- `go/cmd/evolve/cmd_loop_orchestrator.go` — passes the file cells to the pool admission.
- `go/cmd/evolve/cmd_supply_wiring.go` — reads `file_cells` from the policy for the supply wiring.
- `go/internal/admission/file_cells_test.go` — tests the file keys and the guard cases.
- `go/internal/policy/fleet_orchestrator_file_cells_test.go` — tests the parse, the refusals and the unset defaults.
- `go/internal/orchestrator/census_file_cells_test.go` — tests the census with file cells.
- `go/cmd/evolve/cmd_loop_admission_file_cells_test.go` — tests that the loop callers read the policy.
- `go/acs/cycle2471/predicates_test.go` — acceptance predicates of the cycle.
- `docs/architecture/packages/internal-admission.md` — describes the file keys.
- `docs/architecture/packages/internal-policy.md` — describes the new key.
- `.evolve/evals/admission-file-cells-for-leaf-packages.md` — eval graders of the task.
- `docs/private/research/archived-2026-11-17/superseded-predicate-packages/cycle2461/predicates_test.go` — archived predicate of the failed prior try.
- `docs/private/research/archived-2026-11-17/unshipped-build-explanations/cycle-2461-01m7mkgvkc6qkbzr90gxdwppvq.md` — archived explanation of the failed prior try.

## Design Decisions
A single policy list selects the leaf packages. The decision stays in configuration data, not in a flag.

## Verification
The admission, policy, orchestrator and cmd/evolve tests pass. The cycle ACS suite covers each acceptance criterion.

## Compatibility
Without `file_cells`, admission behaves as before.

## Limitations
Only leaf packages get file keys. A non-Go file is a whole package edit.
