# Build Explanation — Cycle 1864

## Build Binding
- Cycle: 1864
- Base SHA: 56cdf89abf503eef9daae72beec003fcfa8d9357

## Summary
This cycle retires RecordRootTaskFailure, drops the unused error return from ClaimLaneScope, and un-skips the amplified processing directory test.

## Rationale
RecordRootTaskFailure had zero production callers and acted as a second quarantine writer with an unneeded lock. Dropping the invariant nil error from ClaimLaneScope simplifies callers and eliminates fake error paths.

## Changed Areas
- `go/internal/inboxmover/outcome.go` — drops the unused error return from ClaimLaneScope and removes error handling at its ApplyCycleOutcome caller.
- `go/internal/inboxmover/root_failure.go` — deletes the dead root task failure writer and its bump lock.
- `docs/architecture/decomposition/06-inboxmover.md` — records the 06-F2 retirement of root_failure.go and names TestFindFileByTaskID_SkipsUnreadableAndMalformed.

## Design Decisions
The lifecycle Mover Promote method remains the single authority for task quarantine. ClaimLaneScope returns only the slice of claimed task identifiers.

## Verification
ACS suite for cycle 1864 passed with 174 green checks and 0 failures. Unit tests for internal/inboxmover passed completely.

## Compatibility
RecordRootTaskFailure had zero production callers. The ClaimLaneScope signature change updates existing call sites to receive a single slice return value.

## Limitations
This cycle addresses only the inboxmover dead quarantine writer and does not change other lifecycle transitions.
