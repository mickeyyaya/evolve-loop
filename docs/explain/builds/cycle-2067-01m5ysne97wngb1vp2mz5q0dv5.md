# Build Explanation — Cycle 2067

## Build Binding
- Cycle: 2067
- Base SHA: 7635ab3bb717069091a31f1a47cbd9be32cc60b5

## Summary
The quality panel now has a mutation reader. It sorts changed functions into measurable, excluded and refused. Only the measurable ones go to an injected runner. The mutants are scored as TQ1 and TQ3 at stage shadow.

## Rationale
ADR-0153 says that a kill-guard mutant once killed every user process. The reader therefore refuses at the seam before any mutation tool can start. The runner is an interface, so no test starts the real gremlins.

## Changed Areas
- `go/internal/qualitypanel/mutation.go` — adds the types, `MeasureMutation` and the scratch-path check.
- `go/internal/qualitypanel/mutation_plan.go` — adds the exclusion list, the signal-call scan and the line-range lookup.
- `go/internal/qualitypanel/mutation_score.go` — adds the TQ1 and TQ3 scoring.
- `go/internal/qualitypanel/reading.go` — adds `Reading.Extra`, so a reading can carry the `not_covered` field.
- `go/internal/qualitypanel/mutation_test.go` — tests every branch with a fake runner.
- `go/internal/qualitypanel/apicover_named_test.go` — names `MutationResult` and `MutationSkip` by full equality.
- `docs/architecture/packages/internal-qualitypanel.md` — describes the reader.
- `docs/architecture/llm-quality-indexes.md` — marks the kill-path guard as built.

## Design Decisions
A whole file is excluded when any call in it is named `Kill`, `Killpg` or `Signal`. This is broader than a per-body check and cannot miss a helper in the same file. The scan reads only that one file, so a kill call in a helper in another file of the package is not seen. The cost is that a method called `Kill` on a non-process type also excludes its file. TQ3 with no measured seconds reports n = 0 instead of dividing by zero.

## Verification
`go test -count=1 -cover ./internal/qualitypanel` reports 100% of statements. The cycle 2067 acs predicates pass.

## Compatibility
`Reading` gains one optional field. Existing readings are unchanged.

## Limitations
No production code calls `MeasureMutation` yet. The real gremlins runner and the boundary wiring are a follow-up item.
