# Build Explanation — Cycle 2049

## Build Binding
- Cycle: 2049
- Base SHA: 0dc815909c3078ea8acb6c7f159e61ed09623053

## Summary
The regression selection now checks the module-local import graph of each regression package for an escape hatch. A package that imports only `testing` and module packages without `os` can be skipped. On the real corpus the soak result does not change. Every regression package is still an escape hatch, so the soak still records `would_skip` 0.

## Rationale
The old listing used the transitive `.Deps` of each test package. Every test package reaches `os` through `testing`, so each package looked like an escape hatch and the selection never skipped one. The new listing reads direct imports and walks only packages of the module. This keeps the real hatches: a package that reaches `os` through a module package still counts as a hatch.

## Changed Areas
- `go/internal/regressiontia/regressiontia.go` — `listCorpusDeps` lists direct imports with `-deps` and walks only module packages. It ignores the generated `X.test` main package, which imports `os` in every test binary.
- `docs/architecture/packages/internal-regressiontia.md` — replaces the known-limit note with the description of the module-local walk.
- `go/internal/regressiontia/module_local_walk_repro_test.go` — reproduces the defect with a fixture module.
- `go/acs/cycle2049/predicates_test.go` — pins the four acceptance criteria of the inbox item.
- `.evolve/evals/regression-tia-escape-hatch-module-local.md` — holds the eval graders of the task.

## Design Decisions
The listing keeps the `path|imports` record format, so `resolveDeps` and its tests do not change. The walk records each escape-hatch import that a module package or a test package imports, and `resolveDeps` still drops that package. A rejected option: filter `testing` out of the transitive `.Deps`. That option breaks when another stdlib package reaches `os`, and it is a deny-list.

## Verification
`go test -count=1 ./internal/regressiontia` passes. `go test -tags acs ./acs/cycle2049` passes for all four predicates. The ACS suite reports green=267 red=0.

## Compatibility
The `Decision` JSON and the exported API do not change. Selection can only skip more packages than before, and only for packages with no hatch in their module-local graph.

## Limitations
On the real corpus every regression package is still an escape hatch, either through a direct `os` import or through `pkg/acsassert`. The audit probe reported `corpus=44 derivable=0`, so the real shadow soak still records `would_skip` 0. Only the fixture module shows a skippable package.

The walk cannot see what a package reads at run time without importing `os`; the escape-hatch list stays the only guard for that case.
