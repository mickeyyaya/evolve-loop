# Composed-gate scope: test the packages that the change affects

> [ADR-0166](adr/0166-composed-gates-test-only-the-affected-packages.md) · package page: [internal-changedpkgs.md](packages/internal-changedpkgs.md).
>
> **Terms used here:**
> - **Composed tree:** the lane tree merged with the current `main`.
> - **Main delta:** the commits that `main` gained since the lane base.
> - **Importer closure:** the changed packages plus each package that imports one of them, in its code or in its tests.

## 1. The question

The composed-tree runner ran `make test` and `make apicover-enforce` over the whole module. The runner did this for each lane. A one-package change paid the CPU of the full suite.

## 2. The three scopes

The numbers come from `go test -race -count=1` on the Go 1.x toolchain of this host, measured on 2026-11-14. The changed package for the last two rows is `internal/changedpkgs`.

| Scope | Packages | Wall | CPU | Escape risk |
|---|---|---|---|---|
| Whole module | 372 | 508 s | 2848 s | None. Each package runs. |
| Cycle-affected (changed packages only) | 1 | 15 s | 22 s | A broken importer escapes. This is the highest risk. |
| Interaction set (importer closure) | 112 | 601 s | 2797 s | A defect that shows only in a non-importer escapes. This risk is low. |

The interaction CPU is the sum of the package times, because the run reached the 600 s limit in `cmd/evolve`. The whole-module run held six failures from host conditions and one build race with a concurrent edit. The failures do not change the order of the CPU figures.

## 3. Decision

The runner uses the importer closure. It is the smallest scope that covers every package that can observe the change through the Go import graph.

- A change in a leaf package runs that package and its few importers. A change in `internal/changedpkgs` runs 112 packages, which is 30 percent of the module by count. Its CPU is close to the whole module, because `cmd/evolve` imports it and is the slowest package. The scope saves most for a leaf change and saves little for a core change.
- The closure comes from `changedpkgs.GateScope`.
- The scope is the full set when git or `go list` cannot derive it, and when a module-root file changes.
- The scope is empty when no Go package changed. The runner skips the two scoped gates, because `go test` with no argument tests the current directory.
- The compile gate and the ACS gate keep the whole module.

## 4. The main delta

The runner diffs the composed tree against `main`, so the diff holds the lane change only. The main delta is not in the scope. Each commit of the main delta passed its own gates before it landed on `main`. The composed run still catches an interaction between the lane and the main delta in two ways. The compile gate builds the whole composed tree. The closure follows the import graph of the composed tree.

## 5. Residual risk

A defect can pass through a non-import link: a file read at run time, a generated file, a CLI call or an environment effect. The scope misses it. The whole-module CI run on the merged `main` still covers it.
