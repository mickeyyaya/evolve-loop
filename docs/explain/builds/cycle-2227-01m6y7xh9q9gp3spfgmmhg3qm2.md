# Build Explanation — Cycle 2227

## Build Binding
- Cycle: 2227
- Base SHA: 869080a37bc30b7ccdf2af02c8a5fc7e89b20844

## Summary
The deliverable read grace window is now a value that tests can set and restore.

## Rationale
Runner timeout tests spent sixteen seconds on settle probes that waited the hardcoded grace window. A test setter lets tests use short windows while production keeps the full window.

## Changed Areas
- `go/internal/deliverable/deliverable.go` — stores the read grace window in an atomic variable and exports a test cleanup setter.
- `go/internal/deliverable/grace_test.go` — verifies the test setter, fast missing reads, and the production window restore.
- `go/internal/phases/runner/runner_optional_test.go` — uses a short test window in artifact timeout tests so they complete fast.

## Design Decisions
Atomic storage avoids data races when concurrent tests run. The setter returns a restore closure that works with test cleanup.

## Verification
Six cycle predicates, deliverable unit tests, and runner timeout unit tests verify fast test reads and 100 percent coverage.

## Compatibility
Production keeps the default 500 millisecond read grace window.

## Limitations
Tests that run in parallel must coordinate window overrides if they expect different windows.
