# Build Explanation — Cycle 2459

## Build Binding
- Cycle: 2459
- Base SHA: cfceadb83e445144b0ff05b7fa4e2ca5b3a6dccf

## Summary
The TIA split now treats a corpus package with a go list error as impure. The split never marks such a package as skippable.

## Rationale
`PurePackages` already treated an errored package as impure. The corpus walk discarded the error, so the two paths disagreed. The corpus walk now uses the same parser, `parseDepGraph`, and the same error rule as `PurePackages`.

## Changed Areas
- `go/internal/regressiontia/regressiontia.go` — `listCorpusDeps` uses the shared graph format and parser, and omits a package whose closure holds an error. The new helper `reachesErrored` serves both paths.

## Design Decisions
A missing key in the dependency map makes `Select` choose the predicate. The fix omits the key instead of adding a new flag.

## Verification
The predicates TestC2459_001 to TestC2459_004 pass. The package suite of `internal/regressiontia` passes.

## Compatibility
The exported API does not change. An error-free package stays skippable.

## Limitations
The fix does not change the escape-hatch rule of the corpus walk.
