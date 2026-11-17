# Build Explanation — Cycle 2450

## Build Binding
- Cycle: 2450
- Base SHA: 8fd4bfba2de1a6addd8504244879620a014579ad

## Summary
After main moves, a lane re-runs the composed gates in full. This build adds a shadow stage. It computes the scoped set of packages, still runs the full re-check, and records each red outside the set as an escape.

## Rationale
Stage 2 (enforce) of the scoped re-check needs evidence of 0 escapes in 2 waves. A shadow record per re-check gives that evidence without a risk to any gate.

## Changed Areas
- `go/internal/scopedset/scopedset.go` — new `Compute` and `Escapes`. The set is the overlap of the reverse closures of the lane files and the main files, plus the packages of tree-reading tests.
- `go/internal/scopedset/record.go` — new `Record` and `Append`. One JSON line per re-check goes to `.evolve/scoped-recheck-shadow.ndjson`.
- `go/internal/core/recheck_site.go` — new `RecheckSite` context value. It carries the project root, the cycle and the audited base to the gate runner.
- `go/internal/core/identity_carry_forward.go` — the identity carry stamps the site before the gates run.
- `go/internal/core/composition_carryforward.go` — the scoped merge stamps the site before the gates run.
- `go/cmd/evolve/cmd_composition_shadow.go` — new `recordScopedShadow`. It reads the lane and main file lists and writes the record.
- `go/cmd/evolve/cmd_composition_wiring.go` — `composedGatesTo` calls `recordScopedShadow` after the full re-check.
- `go/.apicover-enforce` — enrolls `./internal/scopedset` in the apicover gate.
- `docs/architecture/packages/internal-scopedset.md` — the package note.

## Design Decisions
A global input (`go.mod`, `go.sum`, `vendor/`, a module-root file, an unreadable graph) selects the whole module. A red that names no package is an escape. A failed gate with no parsed key counts as unnamed. A record that cannot be written logs and changes no gate.

## Verification
The scopedset, core and cmd/evolve tests pass, and so does the ACS suite for cycle 2450 (8 predicates). The reachability tests start at `composedGatesTo`, `identityCarryForward` and `scopedMergeCarryForward`.

## Compatibility
The four gates run and report as before. Without a site on the context, no record is written.

## Limitations
This stage only records. The scoped set does not yet select which packages run. The `main` range uses the local `main` ref.
