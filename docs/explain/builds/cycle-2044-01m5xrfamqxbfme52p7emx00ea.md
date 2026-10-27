# Build Explanation — Cycle 2044

## Build Binding
- Cycle: 2044
- Base SHA: 4fc458846355c8e758bf56dd6c6770295d56b61c

## Summary
The `acsassert` package now has four predicate helpers: `ModuleDir`, `OutputTail`, `FuncCoverage` and `WalkModule`. The TDD persona and the ACS README name them.

## Rationale
Cycles wrote 597 copies of these helpers. One tested home stops the copies. The helpers use only the standard library.

## Changed Areas
- `go/pkg/acsassert/predicate_kit.go` — adds the four helpers.
- `go/pkg/acsassert/predicate_kit_test.go` — table tests and failure cases for each helper.
- `go/acs/cycle2044/predicates_test.go` — predicates that grade the acceptance criteria.
- `.evolve/evals/acsassert-predicate-kit-helpers.md` — the eval of this task.
- `go/acs/README.md` — a table of the four helpers.
- `agents/evolve-tdd-engineer.md` — names the helpers in the DSL paragraph.

## Design Decisions
`FuncCoverage` returns the lowest figure when a name occurs in two files, so a gap cannot hide. `WalkModule` skips `.git` and returns slash paths.

## Verification
`go test ./pkg/acsassert/` passes, and each kit function has 100.0% coverage. The cycle2044 predicates pass.

## Compatibility
No existing symbol changes. The change only adds exports.

## Limitations
`WalkModule` does not follow symlinks.
