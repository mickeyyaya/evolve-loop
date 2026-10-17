# Build Explanation — Cycle 1931

## Build Binding
- Cycle: 1931
- Base SHA: 2056979ec95cdb16916b65583815222f7c021f60

## Summary
Cycle 1931 adds the cycle 1637 fleet-rebase replay tests, writes the incident record for cycles 1637 and 1691, and registers the replay in the regression coverage index.

## Rationale
The carry recovery ladder landed under ADR-0105, but the unmet acceptance lines from the retired `fleet-rebase-rebuild-chase` item still lacked an end-to-end replay through `recoverFromShipError` across multiple peer moves, an incident record for cycles 1637 and 1691, and an index row. Adding the replay beside the existing carry tests and recording the incident closes that traceability gap without changing production code.

## Changed Areas
- `docs/incidents/2026-09-30-cycle-1637-1691-rebase-chase.md` — documents the cycle 1637 five-round rebase chase and the cycle 1691 disjoint rebase rebuild, the root cause, the ADR-0105 fix, and the replay test.
- `docs/incidents/REGRESSION-COVERAGE-INDEX.md` — adds the covered row for `carry_chase_replay_test.go::TestCycle1637Replay_ShipsOnFirstRecovery` and `TestCycle1637Replay_ByteDivergentRebaseDeclinesCarry`, and updates the summary counts.

## Design Decisions
The replay test reuses `newShippedLane` and `carryHarness` in `go/internal/core/carry_chase_replay_test.go` to drive `recoverFromShipError` at depth 0 after two peer commits on `main`, and the incident record links to existing cycle 1691 and ADR-0105 documents rather than duplicating them.

## Verification
`go test -count=1 -run '^(TestCycle1637Replay_ShipsOnFirstRecovery|TestCycle1637Replay_ByteDivergentRebaseDeclinesCarry)$' ./internal/core` and `go test -tags acs -count=1 ./acs/cycle1931` pass, including the pre-carry overlay check in `TestC1931_002_TheReplayIsRedOnAPreCarryBase`.

## Compatibility
No production packages, CLI flags, schemas, or public APIs change.

## Limitations
The replay tests exercise `recoverFromShipError` with in-process composition hooks in `internal/core`; full `evolve ship` subprocess execution is covered by the `phases/ship` carry suite.
