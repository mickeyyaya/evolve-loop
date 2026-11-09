# internal/testdup

## Purpose

`internal/testdup` counts the extra copies of equal helper bodies in the runtime test files. It is the duplicate ratchet of [test-system.md](../test-system.md) section 6.4.

## Design

- `Scan` parses each `_test.go` file and takes each function that is not a `Test`, `Benchmark`, `Fuzz` or `Example` entry and has two statements or more.
- The body is normalized from its token stream. Spacing, blank lines and comments do not change it. The name and the file do not enter it.
- `Copies` is the sum of (sites - 1) of each group of equal bodies.
- `Scan` skips `vendor`, `testdata`, hidden and underscore directories.
- An `acs/<dir>` directory is read only when it is newer than the baseline date. The age comes from the commit of its files. A fresh checkout makes each mtime new. An untracked directory uses the newest mtime.
- `go/.test-dup-baseline` holds `count=` and `date=`.
- `Check` fails when the count is above the baseline.
- `Check` also fails when the count is below the baseline. A lowered count must lower the baseline in the same commit.

## Invariants

- The structure test `TestTestDuplicates_TheCountOfCopiesEqualsTheBaseline` in `internal/repocontract` is the only production-path caller.
- A change that removes a copy lowers `count=` in `go/.test-dup-baseline` in the same commit.
