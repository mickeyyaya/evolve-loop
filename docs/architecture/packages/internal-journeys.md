# internal/journeys

> Spec: [regression-proof-and-supervision.md §3](../regression-proof-and-supervision.md#3-the-journey-registry). The guard: [test-journeys.md](test-journeys.md).

## Purpose

`internal/journeys` parses the journey registry `go/test/journeys/journeys.json`. Each row names a critical user journey and the test that proves it. The guard test, `evolve commit-gate run` and the shadow check of the `Fix-Journey` trailer read the rows through this package.

## Design

- **`RegistryPath`** is the one spelling of the path of the registry, relative to the repository root.
- **`Parse`** decodes the file strictly: an unknown key fails. Then it validates each row: the id is a slug (`ValidID`), the id is unique, and the `journey` and `issue` are not empty. The `test` is `<package dir>.Test<Name>`, and a `retired` object has a reason.
- **`Load`** reads the registry under a repository root. A missing file gives an error that wraps `os.ErrNotExist`.
- **`Active`** gives the rows that are not retired. **`Known`** is true only for the id of an active row, so a retired id is unknown.
- **`Removed`** gives the ids of a base registry that are not in the registry. A retired row is still in the registry, so it is not removed.
- **`MissingTests`** parses the `_test.go` files of the package directory of each active row. It gives the id of each row with no top-level function of that name. It does not run a test. A file that does not parse is an error.
- The package uses the standard library only.

## Invariants

- A method or a function in a non-test file does not satisfy a row (`TestMissingTests_NamesEachActiveRowWhoseTestIsNotInTheTree`).
- A retired row with an empty reason fails the parse (`TestParse_RefusesEachBadRow`).
- The package is on the protected surface (`/go/internal/journeys/` in `go/internal/guards/integrity_surface.go`).
