# Build Explanation — Cycle 2658

## Build Binding
- Cycle: 2658
- Base SHA: 00043bed92314a0232760adc6ad080bc088c5b1d

## Summary
The production integration enforce actor now wires `inboxmover.Promoter` into `EnforceActor.Promote`. Merged lane members move their items from `delivered/` to `processed/cycle-N/`, while deferred items stay open.

## Rationale
`Mover.Promote` searches `processing/cycle-N/` and the inbox root only, so it cannot move items waiting in `delivered/`. Adding `inboxmover.Promoter` alongside `inboxmover.Requeuer` gives `EnforceActor` a dedicated production adapter without altering `Mover.Promote` semantics.

## Changed Areas
- `go/internal/inboxmover/promote_delivered.go` — implements `Promoter` satisfying `integration.Promoter` to read `committedset` ids for `rec.Cycle` and rename each item in `delivered/` to `processed/cycle-N/`.
- `go/cmd/evolve/cmd_loop_driver.go` — wires `inboxmover.Promoter` into `productionEnforceActor` so `EnforceActor.Missing()` no longer names `Promote`.
- `docs/architecture/integration-driver-enforce.md` — updates section 7 in STE to document `inboxmover.Promoter`, `delivered/`, and `processed/`.
- `docs/architecture/packages/internal-inboxmover.md` — updates the `delivered` state notes to document `Requeuer` and `Promoter` ending `delivered/`.

## Design Decisions
`Promoter.Promote` resolves `Options`, skips non-positive cycles or missing `committedset` records, and deduplicates committed ids. It filters by `deliveredAt`, renames each delivered item into `processed/cycle-N/`, releases any continuation binding, and appends a chained lifecycle record. Because ship only moves in-scope committed ids to `delivered/`, deferred ids never enter `delivered/` and remain open.

## Verification
Unit tests in `go/internal/inboxmover` and `go/cmd/evolve`, the cycle 2658 ACS suite, and `ste-lint --strict` verify promotion, error handling, 100% coverage, zero comments, and card compliance. `gofmt -l` and `go vet` are clean.

## Compatibility
Existing `EnforceActor` callers with nil `Promote` remain unchanged; `productionEnforceActor` populates `Promote` alongside `Requeue`.

## Limitations
`Promoter` relies on `committedset` records written at ship time; records without a `committedset` file are treated as no-ops.
