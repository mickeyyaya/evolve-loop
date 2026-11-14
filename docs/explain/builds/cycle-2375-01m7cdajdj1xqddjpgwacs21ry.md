# Build Explanation — Cycle 2375

## Build Binding
- Cycle: 2375
- Base SHA: 5da3551a32ee8caacaa852c5a6517f98bbda7744

## Summary
The composed-tree test and apicover gates now run the importer closure of the changed packages. They no longer run the whole module for each lane.

## Rationale
The whole-module run cost 2848 s of CPU for a change in one package. The importer closure covers every package that can see the change through the import graph. A fallback to the full set keeps the gate safe when the scope cannot be derived.

## Changed Areas
- `go/internal/changedpkgs/gatescope.go` — adds `GateScope` and `Scope.Enforced`, which derive the scope and filter the enforce list.
- `go/cmd/evolve/cmd_composition_wiring.go` — passes the scope to `make` as `FAST_PKGS` and `APICOVER_PKGS`, and skips both gates for an empty scope.
- `go/internal/changedpkgs/gatescope_test.go` — tests the scope derivation and the fallbacks.
- `go/cmd/evolve/cmd_composition_scope_test.go` — tests that the runner passes the scope to `make`.
- `go/acs/cycle2375/predicates_test.go` — pins the design doc, the lint, the scope behavior, the coverage and the no-comment rule.
- `.evolve/evals/composed-gates-test-only-the-affected-packages.md` — holds the eval graders of the task.
- `docs/architecture/composed-gate-scope.md` — compares the three scopes with measured CPU and escape risk.
- `docs/architecture/adr/0166-composed-gates-test-only-the-affected-packages.md` — records the decision and the rejected options.
- `docs/architecture/packages/internal-changedpkgs.md` — adds the new invariants of the package.

## Design Decisions
The scope is data that the runner passes to the existing `make` variables. No flag selects it. The enforce list file does not widen the scope, because the list only selects entries. Compile and ACS keep the whole module.

## Verification
The ACS predicates of the cycle pass 9/9. The ACS suite passes 272/272. The package tests of `changedpkgs` and the composed-gate tests of `cmd/evolve` pass.

## Compatibility
The `make` targets and variables do not change. A lane with a module-root change or an unreadable diff runs the full set as before.

## Limitations
A change to a core package has an importer closure with a CPU cost close to the whole module. A defect that only a non-importer can show stays outside the scope. A change with no Go package skips the test gate (`go/cmd/evolve/cmd_composition_wiring.go:94-96`, `go/internal/changedpkgs/gatescope.go:22-27`). A test that reads a changed non-Go file at run time does not run in that case.
