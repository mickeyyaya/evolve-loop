# internal/docstests

## Purpose

`internal/docstests` maps changed repository paths to the Go tests that read them.
The `evolve ci docs-tests` verb calls `Select` and `Format`.
The `docs-tests` job in `.github/workflows/required.yml` runs those tests on docs-only pull requests.

## Design

- **Two sources, one selector.** For the `evolve-loop` module, `Select` reads `repocontract.DataFileReads()` and excludes `WholeRepository` and `./acs/` rows. It also scans untagged `go/**/*_test.go` files with `go/ast`.
- **Exact path match.** The AST scan resolves package constants, package variables, and string literals across test files of the same package. It follows helper calls inside the package.
- **No build-tagged tests.** Files with `//go:build` or `// +build` lines are skipped, so `go test` only gets runnable tests.
- **Why not `repocontract.ByNameTestsFor`.** That function returns the whole pack for any non-Go path, which runs tests unrelated to the changed doc.

## Invariants

- An unread doc or a path only in production code selects nothing (`TestSelect_MatchesLiteralsConstsHelpersAndSkipsTaggedAndProd`).
- Every export is named by `apicover_named_test.go`, and the package is in `go/.apicover-enforce`.
