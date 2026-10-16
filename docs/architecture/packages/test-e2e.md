# test/e2e

## Purpose

`test/e2e` holds end-to-end tests that run a full path against the real artifact. It is the heaviest and slowest tier (`make test-e2e`) and may shell out to git. Tiers: [testing.md](../../../go/docs/testing.md).

## Design

- `TestVersionSmoke` builds the real `evolve` binary and runs a subcommand: the smallest honest end-to-end check that the shipped artifact starts and answers. Heavier full-cycle e2e tests live in `cmd/evolve` (`e2e_*_test.go`, also e2e-tagged).
- **Pattern for a new e2e test.**
  1. Skip when the toolchain needed to build is absent.
  2. Build through `gotooltest.NewCache(gotooltest.ExecRunner)` and `Build(gotooltest.Spec{Pkg, Name})`, and close the cache in `t.Cleanup`.
  3. Run it and assert on real stdout and exit status.

## Invariants

- **Assert the stable token, not the version.** The version and commit vary with ldflags (a plain `go build` prints `evolve (devel) (unknown)`), so the test pins that the output starts with the program name.
