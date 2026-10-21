# Build Explanation — Cycle 2007

## Build Binding
- Cycle: 2007
- Base SHA: b0e9de33f076718ea6a16e61910d23d6b36b02f3

## Summary
This build adds the eval and the TDD predicates for `evolve ship --class manual --paths`. It does not ship the feature: the feature needs two protected files, so it is console work.

## Rationale
The scoped stage needs a new `Options.Paths` field in `go/internal/phases/ship/native.go` and a path filter in `stageExplicitPaths` in `go/internal/phases/ship/gitops.go`. ADR-0064 forbids a cycle from editing both paths. The restage in `stageExplicitPaths` runs after the manual confirm step and adds every changed file, untracked siblings included, so no edit outside those files can keep a sibling out of the commit.

## Changed Areas
- `.evolve/evals/cli-ship-manual-paths-lane.md` — the eval graders for the task.
- `go/acs/cycle2007/predicates_test.go` — the TDD predicates for the three acceptance points; they stay red until the console applies the change.

## Design Decisions
Wrapping `Options.Runner` to rewrite git arguments was rejected: it hides the scope in a test seam instead of a typed field.

## Verification
`go build ./...` passes with the protected files restored. The six predicates `TestC2007_001` to `TestC2007_006` are expected red until the console lands the change.

## Compatibility
No production code changes, so the behaviour of `evolve ship` is unchanged.

## Limitations
The feature is not delivered. The console must add `Options.Paths`, the `--paths` flag, `manualAddArgs`, `withinPaths` and the dry-run staged-set log.
