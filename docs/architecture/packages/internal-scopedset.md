# internal/scopedset

## Purpose

`internal/scopedset` finds the packages that both the lane change and the new commits of `main` can affect. It then judges each red of a re-check against that set. It is component S8 of the gate-ship retry-waste plan, at the shadow stage.

## Design

- `Compute` takes the repository root, the lane files and the `main` files. It builds the reverse import closure of each side through `changedpkgs.ImporterClosureChecked`, and keeps the packages where the two closures overlap. A pattern that covers another pattern counts as an overlap.
- A test that reads the tree joins the set. It is a `_test.go` file with a string literal that starts with `../`. The literal must name a changed file, or a directory with a changed file. The file can be in the lane list or in the `main` list.
- `Escapes` returns each red key whose package is outside the set. A red that names no package is an escape with an empty `Package`, because the set cannot prove that it covered the red. A full set has no escape.
- `Record` is one line of `.evolve/scoped-recheck-shadow.ndjson`. `Append` writes one line per call under the plane directory and refuses an empty plane.

## Invariants

- A global input selects the whole module (`Set.All` with a reason).
  - A file input is `go.mod`, `go.sum`, `vendor/` or any file at the module root.
  - A read input is an unreadable root, module path or import graph.
- An empty overlap is an empty set, never `All`.
- The package imports `changedpkgs` and `baseattr` only. It imports no pipeline package.
- The stage is shadow. The composed-gate runner still runs the full re-check, and a record that cannot be written changes no gate result.

## Callers

- `recordScopedShadow` in `cmd/evolve/cmd_composition_shadow.go` runs at the end of `composedGatesTo`. It reads the `core.RecheckSite` of the context.
- `identityCarryForward` and `scopedMergeCarryForward` in `core` stamp the site: the project root, the cycle and the audited base.

## Evidence

Stage 2 (enforce) needs 2 waves of records with 0 escapes. Count the waves with `jq -s 'group_by(.cycle)|length' .evolve/scoped-recheck-shadow.ndjson`.
