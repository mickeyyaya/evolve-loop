# internal/addedtests

## Purpose

`internal/addedtests` derives which test packages a tree adds and under which build tags each must run. The ship's added-test backstop and the build floor both use it. A red added test is then found while the builder still owns the tree.

## Design

- **Why the floor needs tags.** A package gated by `//go:build acs` has no Go files in the default build context. A default-context run never sees it, so the test first ran at the ship gate, where its failure cost repair rounds the builder cannot apply. Each added test therefore runs under the smallest tag selection that builds it.
- **Added test files only.** Only `go/**/_test.go` files that the change adds count. A modified test's package is the importer backstop's job.
- **Grouping.** `Groups` buckets packages by their tag set. The empty set is the default context. Each group's packages are sorted, and groups are ordered by their tag key.
- **Exclusions are named.** A test that no selection can run on this host is returned in `excluded`, not dropped silently. Two cases qualify: a `requires_tmux` constraint, and a constraint with more than `maxSearchTags` (12) tags. Past 12 tags the exhaustive search is too slow, so a constraint solver is needed.
- **The search.** `BuildTags` tries every non-empty subset of the file's tags, smallest first, and returns the first selection under which the file matches. An unsatisfiable constraint exhausts every selection and is not runnable.
- **Per-package budget.** `PackageTimeout` is 20 minutes. It sits above `go test`'s 10-minute default, so a slow added package is not cut off by the default.
- **Package wall budget.** `PackageWallBudget` is 90 seconds, at most a quarter of `PackageTimeout`. It is the alarm for a slow package. `PackageTimeout` is only the kill deadline. `testlatency -package-budget` and `make test-budget` read this one value.

## Invariants

- **A tag-gated added package runs under its own tags.** `TestGroups_TagGatedAddedPackagesRunUnderTheirOwnTags` pins this.
- **The table of constraint shapes.** `TestBuildTags_TableOfConstraintShapes` pins the default context, `||`, `&&`, negation, the unsatisfiable case, `requires_tmux` and the tag ceiling.
- **A constraint parse error or a match error is returned.** Neither is swallowed.
- **The timeout beats the default.** `TestPackageTimeout_IsAGoTestDurationWithHeadroomOverGosDefault` pins this.
- **The wall budget is well below the timeout.** `TestPackageWallBudget_IsValidDurationWellBelowPackageTimeout` pins this.

## Findings

- The 2026-09-14 incident, where an added `//go:build acs` package was invisible to the floor, is recorded in the [comment history archive](../../history/code-comments/README.md) and the CHANGELOG, not here.
