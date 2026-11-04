# test/journeys

> Spec: [regression-proof-and-supervision.md §3](../regression-proof-and-supervision.md#3-the-journey-registry). The parser: [internal-journeys.md](internal-journeys.md).

## Purpose

`test/journeys` holds the journey registry `journeys.json` and its guard test. Each row names a critical user journey and the test that proves it.

## Design

- `TestRegistry_EveryActiveRowNamesATestInTheTree` loads the real registry with `journeys.Load` and calls `journeys.MissingTests` over the Go module. It fails when an active row names a test that is not in the tree. The message tells you to fix the row, or to retire it with a reason.
- The directory has test files only. It has no production code.

## Invariants

- To add a row, name a test that exists. To remove a journey, set `retired` with a reason. `evolve commit-gate run` refuses a row that goes away without `retired`.
- The directory is on the protected surface (`/go/test/journeys/` in `go/internal/guards/integrity_surface.go`), so a cycle cannot change the registry or the guard.
- Red proof (2026-11-04): a row that named `internal/orchestrator.TestNoSuchJourney` failed the guard with `rows [red-proof] name a test that is not in the tree`.
