# Build Explanation — Cycle 1750

## Build Binding
- Cycle: 1750
- Base SHA: d44676280fb88a3ac1aa5dbc8f17d53da64d53c6

## Summary
`dag.Levels` moves its input-validation and edge-building loop into a new unexported helper, `dependencyEdges`. That brings `Levels` from 57 to 37 lines, under the 50-line size-ratchet limit, with no behavior change. New characterization tests pin the exact error text, nil-on-error levels, duplicate-node and empty-graph behavior that the baseline suite left unpinned. `cyclehealth.Check` (33 lines) and `naminguard.Fix` (37 lines) already met the limit at base, so this cycle changes no code in those packages.

## Rationale
`Levels` has two phases with one hand-off: build the indegree and dependents maps from validated input, then run Kahn's algorithm level by level. Cutting at that seam gives the smallest extraction that also reads naturally. The helper returns the two maps or the first validation error. I rejected moving the Kahn loop out instead: the loop shares `levels` and `placed` with the cycle check, so the helper would need four return values and would be harder to read. `offenders.json` is deliberately left unchanged. A lane never edits it, and the boundary `evolve sizeratchet tighten` removes the slack.

## Changed Areas
- `go/internal/dag/dag.go` — the known-node set, indegree/dependents maps and the dependency-validation loop of `Levels` move into `dependencyEdges(nodes, deps) (map[string]int, map[string][]string, error)`. The "isolated nodes" comment moves with its line. Error messages, map-iteration order handling and the sorted-within-level output stay the same.
- `go/internal/dag/levels_characterization_test.go` — new tests call `Levels` and pin: the exact text of all four error classes (self-dependency, dangling reference, unknown key, cycle with its placed/total counts), nil levels on every error, a duplicate node name leveled once, and a nil leveling for an empty graph.
- `go/acs/cycle1750/predicates_test.go` — the cycle's ACS predicates (TDD-authored). They check the size limit, the frozen `offenders.json`, mutation kill of 9 baseline `Levels` mutants, and exhaustive equivalence to the baseline.
- `.evolve/evals/sizeratchet-dag-levels-decomposition.md` — the TDD-authored eval for the `Levels` shrink.
- `.evolve/evals/sizeratchet-cyclehealth-check-decomposition.md` — the TDD-authored eval pinning the cyclehealth no-op.
- `.evolve/evals/sizeratchet-naminguard-fix-decomposition.md` — the TDD-authored eval pinning the naminguard no-op.
- `docs/explain/builds/cycle-1750-01m3mw2nnkhmb0ewzew4hmhhyk.md` — this explanation record.

## Design Decisions
The helper is unexported and has one caller, `Levels`, which keeps its name and signature, so the mutation predicate can substitute the baseline body back beside it. The helper takes the caller's slices and map read-only; nothing is sorted or appended in place on caller data. The new tests live in a new `_test.go` file, so `dag_test.go` and `apicover_named_test.go` are unchanged, and the file adds no comment lines.

## Verification
All 12 cycle-1750 ACS predicates pass (`go test -tags acs -count=1 ./acs/cycle1750`). They include mutation kill of all 9 baseline `Levels` mutants and the differential equivalence harness over every 4-node relation plus reordered, duplicate-node, empty-list, duplicate-edge and invalid-reference variants. `go vet` and `gofmt -l` are clean on `internal/dag`.

## Compatibility
Nothing public changes: no exported symbol, flag, schema or file format was added or altered, and `Levels` keeps its signature and error text.

## Limitations
The `offenders.json` allowances for `internal/dag.Levels` (57), `internal/cyclehealth.Check` (51) and `pkg/naminguard.Fix` (51) stay until a boundary tighten removes them. Other oversized functions are out of scope.
