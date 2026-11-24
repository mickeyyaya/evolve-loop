# internal/emptytests

## Purpose

`internal/emptytests` refuses a vacuous Go test. A function is vacuous when it is a test, a fuzz target or a benchmark, and its body has no statement. Such a function always passes, and it proves nothing. The finding is in [cost-aware-repair-2026-10.md](../../research/cost-aware-repair-2026-10.md).

## Design

- **The rule.** A function offends when all of these hold:
  - It is a top-level function with a body and no receiver.
  - Its name starts with `Test`, `Fuzz` or `Benchmark`. The next rune is not a lowercase letter, as `go test` requires.
  - It has one parameter of type `*testing.T`, `*testing.F` or `*testing.B` that matches the prefix.
  - Its body holds no statement. A body of only comments offends.
- **The `testing` import.** The scan reads the local name of the `testing` import in each file. An aliased import is covered.
- **The permitted placeholder.** `t.Skip("reason")` is a statement, so it passes. Write the reason in the string.
- **Skipped directories.** The scan skips `vendor`, `testdata`, and each directory that starts with `.` or `_`. `go test` never builds them, so an empty function there cannot pass as a test.
- **No baseline.** The rule is absolute. `Check` returns nil for no offender. Otherwise it returns one line per offender, `<file>:<line>: <Func> has an empty body`, in sorted order.
- **A parse error fails the scan.** A guard that cannot read a file must not pass.

## Where the guard runs

- `./internal/emptytests/...` is in `repocontract.Packages()`. The external package test `TestTree_HasNoNewEmptyTest` runs the registry entry `empty-tests` on the whole module, so the pack and the registry judge with one composition. The repo-contract pack runs it at the build handoff floor and at ship.
- The guard is in the repo-contract pack, not in `acs/regression`. The build handoff floor runs the pack, and it does not run `acs/regression`. A guard under `acs/regression` reaches only the durable ACS tier.
- The registry entry `empty-tests` in [internal-gateregistry.md](internal-gateregistry.md) calls the same `Scan` and `Check`.
- `TestRepoContractPack_RejectsALaneThatAddsAnEmptyTest` in `internal/phases/ship` is the wiring proof. It runs the real pack on a scratch module with an empty test, and expects a red that names `emptytests`.

## Not covered

The guard checks for no statement only. A body such as `_ = t` passes. A check for a test with no assertion is out of scope.
