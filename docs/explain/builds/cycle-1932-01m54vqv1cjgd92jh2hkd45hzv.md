# Build Explanation — Cycle 1932

## Build Binding
- Cycle: 1932
- Base SHA: 51f50c55ff597baa4b97169fda1bc26eff239351

## Summary
The recurrence boundary reweight no longer writes the inbox item itself. `recurrence.ApplyOptions` carries a `Reweight` seam, and the loop boundary wires that seam to `inboxmover.Edit`. The mover applies the curator weight rule and appends the `inbox-lifecycle` `edit` ledger line.

## Rationale
ADR-0116 makes `internal/inboxmover` the one item writer. The old `setItemWeight` rewrote the item with `atomicwrite.JSON`, skipped the edit validation and wrote no ledger line. `internal/recurrence` cannot import `inboxmover`, because `inboxmover` imports `core` and `core` imports `recurrence`. An injected function is the smallest seam that breaks the cycle.

## Changed Areas
- `go/internal/recurrence/apply.go` — adds `ApplyOptions.Reweight`, calls it for an escalation and deletes `setItemWeight`; a nil seam skips the item, and a seam error is wrapped and records no bump.
- `go/cmd/evolve/cmd_loop_escalate.go` — `moverReweight` wires the seam to `inboxmover.Edit`, rooted at the boundary's `.evolve` and given its signal Center.
- `go/cmd/evolve/inbox_writers_contract_test.go` — removes the `internal/recurrence.applyIntent` enrollment, because recurrence no longer writes an item.
- `go/cmd/evolve/cmd_inbox_signals_test.go` — pins the new `inboxmover.Options` literal as wired to the Center.
- `go/internal/recurrence/apply_test.go` — gives the test options a fake item writer and adds a table test of the nil, refusing and accepting seams.
- `go/internal/recurrence/apply_backfill_f4_test.go` — drives the write fault through a refusing seam, because recurrence no longer owns the write.
- `go/cmd/evolve/cmd_loop_escalate_reweight_test.go` — the TDD phase's caller proof: the real boundary raises the weight and the mover writes one edit line.

## Design Decisions
The seam is a plain `func(id string, weight float64) error` field, not an interface, because it has one production implementation. A nil seam is a skip, not an error, so that a caller without a writer changes nothing. A seam error aborts the boundary like the old write error did, so the stamp keeps the intent for the next run. Rejected: a second ledger writer inside recurrence (audit H1: it duplicates the chain writer and hides from the write-site scan).

## Verification
`go test -tags acs ./acs/cycle1932` passes 8/8. `go test ./internal/recurrence/... ./internal/inboxmover/... ./internal/core/... ./cmd/evolve/...` passes.

## Compatibility
The JSON schemas stay unchanged. A caller that does not set `Reweight` no longer bumps a weight; the loop boundary is the only production caller and sets it.

## Limitations
The seam error does not separate a curator refusal from an I/O fault; both abort the boundary.
