# ADR-0166: The composed-tree test and apicover gates run the affected packages

- **Status:** Accepted (2026-11-14).
- **Design:** [composed-gate-scope.md](../composed-gate-scope.md).
- **Relates to:** ADR-0110 (whole-tree tests run before main) and ADR-0147 (the train tests the merged tree).

## Context

The composed-tree runner ran `make test` and `make apicover-enforce` over the whole module for every lane. Each run cost the CPU of the full suite, also for a change in one package. The design doc measures this cost and compares three scopes.

## Decision

1. `changedpkgs.GateScope` derives the scope from the diff of the composed tree against `main`. The scope is the importer closure of the changed packages, without the `acs` packages.
2. The runner passes the scope to `make test` as `FAST_PKGS` and to `make apicover-enforce` as `APICOVER_PKGS`. The apicover list is `go/.apicover-enforce` filtered to the scope.
3. The scope is the full set when git or `go list` cannot derive it, and when a module-root file changes.
4. The scope is empty when no Go package changed. The runner then skips both gates and logs the skip, because `go test` with no argument tests the current directory.
5. The compile gate and the ACS gate keep the whole module.

## Consequences

- A one-package change runs that package and its importers, not the module.
- A failure to derive the scope never lowers coverage. It runs the full set.
- A defect that only a non-importer can show stays outside the scope. The design doc names this residual risk and the gates that still cover it.

## Rejected

- **Whole module for every lane:** the measured CPU is the cost that this record removes.
- **Changed packages only:** it misses each importer that the change breaks.
- **A feature flag for the scope:** a flag adds a second behavior. The full-set fallback already covers each case that the scope cannot derive.
