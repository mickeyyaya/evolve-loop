# Build Explanation — Cycle 1795

## Build Binding
- Cycle: 1795
- Base SHA: 0f072732bb7a4493220ded7239c3a20758687d94

## Summary
Release preflight now gates the profiles, phasecoherence and phasespec test suites in addition to guards and phases/ship.

## Rationale
DefaultGateTestSuites listed only two suites, so a release could be cut with a red repo-contract suite. Extending the existing list reuses the runner and error path with no new mechanism.

## Changed Areas
- `go/internal/releasepreflight/releasepreflight.go` — adds three suite patterns to DefaultGateTestSuites; Run already iterates the list and wraps failures in ErrCheckFailed naming the suite.

## Design Decisions
The list stays a plain slice so the count, dry-run log and GateTestsPassed all follow it automatically.

## Verification
ACS predicates TestC1795_001 to 004 cover suites run, red suites failing with ErrCheckFailed, existing suites still gating, and count consistency. The releasepreflight package tests pass.

## Compatibility
No API change; preflight simply runs three more suites.

## Limitations
Preflight runtime grows by the time of those three suites.
