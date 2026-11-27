# Build Explanation — Cycle 2656

## Build Binding
- Cycle: 2656
- Base SHA: f8769e839b568fa19a1c774a36087035d7fdefbb

## Summary
The production enforce actor now wires `Requeue` and `Briefs` ports. An ejected lane member writes its brief and leaves `delivered/` for the inbox root. The next lane prompt then shows that brief path.

## Rationale
`EnforceActor` already calls `Briefs.Write` and `Requeue.Requeue` in `ejectMember`, while `productionEnforceActor` left both nil. Implementing `FileBriefStore` in `internal/integration` and `Requeuer` in `internal/inboxmover` avoids a package cycle (`inboxmover` already imports `integration`). This reuses `Ejection.WriteBrief`, `committedset.ReadRecord`, `deliveredAt`, and `UpdateItemJSON`.

## Changed Areas
- `go/internal/inboxbatch/fieldowner.go` — adds `BriefPathField` (`brief_path`) as a loop-owned stamp so `mentionedFiles` ignores it.
- `go/internal/inboxbatch/item.go` — adds `BriefPath` (`json:"brief_path,omitempty"`) to `Item` and decodes it with `stampText` in `UnmarshalJSON`.
- `go/internal/inboxbatch/fieldowner_test.go` — adds `brief_path` to `loopStampKeys` and `TestStampFieldConstants_SpellTheKeysTheInboxHolds`.
- `go/internal/integration/brief.go` — adds `FileBriefStore` implementing `BriefStore` by calling `Ejection.WriteBrief`.
- `go/internal/integration/brief_test.go` — adds `TestFileBriefStore_Write_WritesBriefAndReturnsErrorOnUnwritableDir` covering success and write-error branches.
- `go/internal/inboxmover/requeue.go` — adds `Requeuer` implementing `integration.Requeuer`. It reads committed item IDs with `committedset.ReadRecord`, moves delivered items back to the root with `brief_path` and bumped `failure_count`, and writes a ledger record.
- `go/internal/inboxmover/requeue_delivered_test.go` — tests `Requeuer.Requeue` across happy path, no-ops, empty brief path, ledger warning, existing root copy, unwritable directories, and invalid options.
- `go/internal/phases/triage/triage.go` — adds `itemLabels` in `selectableBatchesNote` to append `, brief <path>` on the triage menu line when `Item.BriefPath` is non-empty.
- `go/cmd/evolve/cmd_loop_driver.go` — wires `Requeue` (`inboxmover.Requeuer`) and `Briefs` (`integration.FileBriefStore` under `integration.WaveDir`) into `productionEnforceActor`.
- `go/cmd/evolve/cmd_inbox_signals_test.go` — enrolls `cmd/evolve/cmd_loop_driver.go` in `allowed` for its wired `inboxmover.Options` literal.
- `go/cmd/evolve/inbox_writers_contract_test.go` — enrolls `internal/inboxmover.requeueDeliveredFile` in `inboxWritersThatFileNothing`.
- `docs/architecture/integration-driver-enforce.md` — updates sections 2 and 4 in STE to document the kept `Requeuer` and `BriefStore` ports, `Ejection.WriteBrief`, and `BriefPath`.

## Design Decisions
- Kept `Requeuer` as a method-bearing struct in `internal/inboxmover` instead of `internal/integration`, because `inboxmover` already depends on `integration` and `committedset`.
- Rendered `BriefPath` in `triage.go` (`selectableBatchesNote`) instead of `inboxrank.Labels` so ranking label unit tests and `inboxrank` remain untouched.
- Decoded `brief_path` with `stampText` in `Item.UnmarshalJSON` (matching `Priority`) so `TestConsoleRouted_NoStampIsTheItemsText` passes without relaxing its stamp-object probe.

## Verification
All 6 cycle-2656 ACS predicates (`go test -tags acs -count=1 ./acs/cycle2656`) and `evolve docs ste-lint --strict` pass, with 100% statement coverage on added production lines outside `cmd/evolve`.

## Compatibility
`inboxbatch.Item.BriefPath` uses `omitempty` and omits the field when empty. Existing inbox items and triage menus without `brief_path` render identically.

## Limitations
`EnforceActor.Promote` remains unwired on `productionEnforceActor` and is out of scope for this task (`Missing()` still reports `Promote`).
