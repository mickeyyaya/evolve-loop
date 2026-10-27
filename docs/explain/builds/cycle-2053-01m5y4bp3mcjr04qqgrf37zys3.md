# Build Explanation — Cycle 2053

## Build Binding
- Cycle: 2053
- Base SHA: 1d361aef266b33532296574243e4b77a0e7c44aa

## Summary
The selection now skips a package whose module-local import graph holds no escape hatch. The skip is proven on a fixture module only. The real corpus skips no package yet.

## Rationale
The old walk listed the standard library closure. Every test package reached `os` through `testing`, so no package was skippable. The walk now follows module packages only.

## Changed Areas
- `go/internal/regressiontia/regressiontia.go` — removes two branches that the module-local walk cannot reach, and drops the marshal error branch.
- `go/internal/regressiontia/regressiontia_coverfix_test.go` — adds tests for the empty corpus and the root-wide dependency, which bring the package to full coverage.
- `docs/architecture/packages/internal-regressiontia.md` — names the cycle 2053 predicates.
- `docs/architecture/adr/0082-regression-test-impact-selection-shadow.md` — records the module-local rule.

## Design Decisions
A package that reaches `os` through a module package stays an escape hatch. The walk records each module package and each hatch import that it meets.

## Verification
The cycle 2053 predicates and the package tests pass. The package has 100% statement coverage.

## Compatibility
The public functions keep their signatures.

## Limitations
On the real tree, each of the 44 regression packages stays an escape hatch through `pkg/acsassert`. The real shadow soak therefore records would_skip=0. An audit probe of `Compute` gave selected=44 and wouldskip=0 for three changes. The follow-up item is regression-tia-declared-impact-surface.
