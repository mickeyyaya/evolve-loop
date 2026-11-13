# internal/testdup

## Purpose

`internal/testdup` counts the extra copies of equal helper bodies in the runtime test files. It is the duplicate ratchet of [test-system.md](../test-system.md) section 6.4.

## Design

- `Scan` parses each `_test.go` file and takes each function that is not a `Test`, `Benchmark`, `Fuzz` or `Example` entry and has two statements or more.
- The body is normalized from its token stream. Spacing, blank lines and comments do not change it. The name and the file do not enter it.
- `Copies` is the sum of (sites - 1) of each group of equal bodies.
- `Scan` skips `vendor`, `testdata`, hidden and underscore directories.
- An `acs/<dir>` directory is read only when it is newer than the baseline date. The age comes from the commit of its files. A fresh checkout makes each mtime new. An untracked directory uses the newest mtime.
- A directory with a change against `HEAD` (`git diff --name-only HEAD`) is new. The ship gate runs before the landing commit, while the change is only staged. This rule makes the gate count the same directories as main counts after the landing ([incident 2026-11-09](../../incidents/2026-11-09-duplicate-ratchet-misses-staged-predicates.md)).
- `GroupsUnder` keeps each group with a site under one directory. The TDD handoff floor of `internal/core` uses it for `acs/cycle<N>`.
- `go/.test-dup-baseline` holds `count=` and `date=`.
- `Check` fails when the count is above the baseline.
- `Check` also fails when the count is below the baseline. A lowered count must lower the baseline in the same commit.

## Invariants

- Two production-path callers use the package:
  - the structure test `TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline` in `internal/repocontract`, which calls the run function of the registry entry;
  - the TDD handoff floor `tddPredicateFloor` in `internal/core`.
- A change that removes a copy lowers `count=` in `go/.test-dup-baseline` in the same commit.
- `acs/cycle2250` holds two copies, `write` and `onDisk`, that landed through the staged-predicate escape. No later phase can change that directory. The repair removed two copies in `cmd/evolve`, so the count stays at the baseline 334.

## Gate contract

- The registry entry `testdup-ratchet` in [internal-gateregistry.md](internal-gateregistry.md) runs this package. [gate-contract.md](../gate-contract.md) section 6.4 has the design.
- The structure test `TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline` calls that run function from an external test package of `internal/repocontract`.
- The package is on the protected surface. A lane cannot write it.
- `go/.test-dup-baseline` is outside the package. It is an open seam until G6.
