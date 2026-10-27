# Build Explanation — Cycle 2037

## Build Binding
- Cycle: 2037
- Base SHA: 2831f561e598d137f4068abd56eb6a81fec12f5e

## Summary
This cycle implements the wave integration batch flush policy and adaptive cap pure functions in package `go/internal/integration`.
The implementation provides pure decision logic for batch flushing under D7 triggers.
It isolates risky deliverables per D9 and scales the cap within bounds 1 to 4 per D8.

## Rationale
Component I4 of ADR-0147 requires pure functions that decide when to flush a batch and adapt batch capacity without side effects.
Implementing these pure functions in an isolated leaf package ensures deterministic decision evaluation and enables future integration engine orchestration without circular dependencies.

## Changed Areas
- `go/.apicover-enforce` — enrolls the new `internal/integration` package in repository-wide API coverage enforcement.
- `go/internal/integration/cap.go` — implements `HalveCap` and `GrowCap` adaptive cap functions and capacity constants.
- `go/internal/integration/doc.go` — documents the package purpose for docgo regression compliance.
- `go/internal/integration/flush.go` — implements `DecideFlush` pure flush policy decider and related deliverable types.

## Design Decisions
The flush decider processes batch items as immutable inputs and returns fresh slices for flushed and held deliverables.
Risky deliverables take strict precedence over standard D7 triggers and flush in isolation one by one.
The adaptive cap strictly clamps inputs and outputs within bounds [1, 4] with a default capacity of 4 and default maximum age of 45 minutes.

## Verification
Unit test suites in `go/internal/integration` execute table-driven scenarios with full coverage.
All 6 cycle-2037 ACS predicates pass cleanly.
All 8 eval graders pass with zero errors.

## Compatibility
This change introduces a new internal package and does not alter existing public APIs or runtime behaviors.
All exports are verified through apicover named assertions.

## Limitations
Production callers in the wave integration engine (component I5) will wire this package in subsequent cycles.
