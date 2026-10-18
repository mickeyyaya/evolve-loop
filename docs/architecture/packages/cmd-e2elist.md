# cmd/e2elist

## Purpose

`cmd/e2elist` prints the `go test -run` pattern for `make test-e2e`. The e2e step then runs only the tests that the `e2e` build tag adds. It does not repeat the tests that the integration step already ran under `-race`. Plan: [ci-wall-time-2026-10](../../plans/ci-wall-time-2026-10.md), component C6.

## Design

- The `main` package holds no logic beyond the call into `internal/ciparity`. The selection lives in `ciparity.E2ETestNames`.
- The command takes the roots to scan (`cmd test/e2e`) and the e2e tags (`-tags "e2e evolve_test_phases"`). It prints one line, `^(TestA|TestB)$`.
- A test is on the list when only the `e2e` tag adds its file to the package. The list follows the build tag. A new e2e file joins it with no edit.

## Invariants

- The command fails when the list is empty or a root cannot be read. An e2e tier that runs no test must not pass.
- One pattern applies to every package. A plain test with the name of an e2e test in another package also runs. This costs only time.
- A change to the selection belongs in `internal/ciparity`, never here.
