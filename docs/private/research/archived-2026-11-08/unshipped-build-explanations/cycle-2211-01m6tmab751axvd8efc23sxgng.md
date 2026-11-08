# Build Explanation — Cycle 2211

## Build Binding
- Cycle: 2211
- Base SHA: f2c168cac7992885a5a91681e7f7d8b967b10bb5

## Summary
A failed retry ladder now sets the redesign mark and emits LADDER_REDESIGN for the redesign rung of ADR-0126. Before this change, the end of the ladder was a silent stop.

## Rationale
ADR-0164 ruling 1 says the redesign rung runs first, and the operator gets the subject only at a second end. The change reuses the existing stop decision of the ladder, so no new retry path exists.

## Changed Areas
- `go/internal/core/retry_ladder.go` — `askLadder` calls `routeToRedesign`, which emits `LADDER_REDESIGN` and sets the redesign mark when the ladder ends at R3 or at a spec stop.
- `go/internal/cyclestate/state.go` — `LadderState` gets the `redesign` field, which records that the redesign route ran.
- `docs/architecture/adr/0146-a-defect-that-survives-two-builder-rounds-is-fixed-from-the-finders-spec-and-verified-by-another.md` — item 9 names the redesign route and the operator at a second end.
- `docs/architecture/signal-codes.md` — a new row describes `LADDER_REDESIGN`.
- `go/internal/core/retry_ladder_test.go` — tests cover the end routes, the spec stop, the registration, the early cases and the second end.
- `go/acs/cycle2211/predicates_test.go` — predicates check the tests and the ADR record.
- `.evolve/evals/ladder-end-routes-to-the-redesign-rung.md` — the eval lists the graders of the task.

## Design Decisions
The redesign mark is a state field, not a feature flag. A non-empty mark blocks a second automatic redesign. The ladder ends on a capability failure at R3 or on a spec class. Halt, infra and quota classes do not end the ladder.

## Verification
The targeted core tests and the three ACS predicates pass. The signalcenter, cyclestate and retryladder packages pass.

## Compatibility
The `redesign` field is optional in the JSON state, so old state files load unchanged. No existing signal code changes.

## Limitations
This change emits the signal and records the mark. A consumer that starts the redesign phase from the signal is outside this task.
