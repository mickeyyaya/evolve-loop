# Build Explanation — Cycle 2653

## Build Binding
- Cycle: 2653
- Base SHA: 92ab6246c0753563d4fdce0eba19c4c1ce8b6982

## Summary
Implemented write-once duplicate-patch idempotency and typed conflict refusal in `WriteRecordWith`, enforce-stage re-admission blocking in `Driver.admits`, and integrity-class handling on `ErrRecordExists` in `worktreeShip.deliver`.

## Rationale
`recordFile.existingOutcome` compares the ordered patch IDs of the existing record file on `os.ErrExist`. Same-patch retries succeed as no-ops, while differing patch IDs refuse without rewriting the record. Returning `core.ShipClassIntegrity` on `ErrRecordExists` stops retry loops permanently before `main` moves.

## Changed Areas
- `go/internal/integration/deliverable.go` — added `RecordRefusedError` (`Member`, `Path`, `Error`, `Unwrap`) and `recordFile.existingOutcome`. `WriteOnce` returns the path for identical patch IDs and refuses differing patch IDs after removing the staged temp file.
- `go/internal/integration/driver.go` — guarded `Driver.admits` so known members are never re-admitted when `Stage` is not `FleetIntegrationShadow`.
- `go/internal/phases/ship/deliverable.go` — mapped `ErrRecordExists` errors in `worktreeShip.deliver` to `core.ShipClassIntegrity` instead of `core.ShipClassTransient`.

## Design Decisions
`recordFile.existingOutcome` checks `slices.Equal(patchIDsOf(existing), f.Patches)` inside the single writer rather than replacing records in `Driver`. This keeps exclusive-create race safety (`os.Link`) and leaves `ReplaceExisting` untouched for shadow lanes and console PR records.

## Verification
`go test -count=1 ./internal/integration/...` passes with 100.0% function coverage on `deliverable.go` and `driver.go`. The passing tests in `./internal/phases/ship` reach 100.0% function coverage on `ship/deliverable.go`. Two TDD-authored tests (`TestDeliverWriteOnce_AShipRetryUnderANewSHAWithTheSamePatchesReadsAsDelivered` and `TestDeliverWriteOnce_AResumedShipOfTheSameCommitReadsAsDelivered`) remain red because their fixture reads `l.Tip(t)` before `ship.Run` commits the lane work.

## Compatibility
`RecordRefusedError` unwraps to `ErrRecordExists` for existing `errors.Is` callers, and `ReplaceExisting` retains `os.Rename` semantics.

## Limitations
Row 10c (lane `Heads` ejection) is outside this lane's scope. The two `deliverable_write_once_integration_test.go` retry tests require a TDD-phase fixture fix (`deliverable_write_once_integration_test.go` is TDD-owned).
