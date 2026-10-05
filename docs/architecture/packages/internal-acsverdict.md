# internal/acsverdict

## Purpose

`internal/acsverdict` holds the names both writers of `acs-verdict.json` must agree on, and nothing else: the file name (`Filename`), the path under an evolve dir (`Path`), the prefix of a harness red (`SyntheticRedPrefix`, `egps/`) and the id of the "no predicates" red (`NoPredicatesID`, `egps/no-predicates`). The writers are `internal/acssuite` (`evolve acs suite`, the audit) and `internal/acsrunner` (`evolve acs run`). The readers that open the file or pick out harness reds (`internal/core`, `internal/phases/audit`, `internal/phases/ship`) take the same names from here.

## Design

- **A leaf.** The package imports only the standard library. `acsrunner` used to import `acssuite` for these names, which pulled seventeen internal packages (policy, verifylock, changedpkgs and their graphs) into a runner that needs none of them, and put `acsrunner` in the same package graph as `internal/fleet`. It now depends on `acsverdict` and `ipcenv` only.
- **One spelling, no aliases.** `acssuite` no longer exports `VerdictFilename`, `SyntheticRedPrefix`, `VerdictPath` or `NoPredicatesRed`; every caller moved to this package (`acssuite.noPredicatesRed` builds its own `Result` from `NoPredicatesID`, and `acsrunner` builds its `Predicate` from the same id).
- **`Path` refuses a relative evolve dir.** A relative dir would resolve against the process cwd; the 2026-10-01 phantom `go/.evolve/runs/cycle-104/` was that failure. Both `WriteVerdict`s write through `Path`, so neither can land a verdict there ([ADR-0114](../adr/0114-the-acs-verdict-is-always-written-and-complete.md) decisions 6 and 7).

## Invariants

- Both writers name a run with no predicates by `NoPredicatesID` (`acsrunner.TestNoPredicates_BothVerdictWritersNameTheSameHarnessRed`), under `SyntheticRedPrefix` (`TestNoPredicatesID_IsAHarnessRedID`).
- `Path` returns `<evolveDir>/runs/cycle-<N>/acs-verdict.json` for an absolute dir and an error, creating nothing, for a relative one (`TestPath_IsTheCycleVerdictFileUnderAnAbsoluteEvolveDir`, `TestPath_RefusesARelativeEvolveDirAndCreatesNothing`).
- Every export is named by a test and the package is enrolled in `go/.apicover-enforce`.

## Findings

- **2026-10-01, architecture re-review of ADR-0114**: the first fix had `acsrunner` import `acssuite` for three names, and its new imports broke three `internal/fleet` partition tests that used `acsrunner` as the package unrelated to fleet (see the fleet notes). The names moved here instead.
