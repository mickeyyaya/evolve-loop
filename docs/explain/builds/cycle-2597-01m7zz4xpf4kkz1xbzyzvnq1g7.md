# Build Explanation — Cycle 2597

## Build Binding
- Cycle: 2597
- Base SHA: 268f3f430b91b473ab1670def8fb5d79fd94fef3

## Summary
`evolve sync-main` carries console edits of tracked inbox items through the merge. Both tests in `cmd_syncmain_untracked_test.go` now call `t.Parallel()` so the package parallel-default gate passes.

## Rationale
The console edits curated fields, route stamps, and consume moves by hand. Widening the stamp classifier by field owner removes the refusal, and origin wins any shared-field conflict.

## Changed Areas
- `go/internal/inboxstamps/inboxstamps.go` — treats edits to non-author-owned fields as stamps, classifies moves into `consumed/`, and records console values that origin replaced in `Plan.Dropped`.
- `go/cmd/evolve/cmd_syncmain.go` — reports each dropped console edit on stdout and removes identical untracked copies before the merge.
- `go/cmd/evolve/cmd_syncmain_untracked.go` — deletes an untracked file equal to the file origin adds, and leaves any different copy intact.
- `go/cmd/evolve/cmd_syncmain_untracked_test.go` — adds `t.Parallel()` to `TestRemoveUntrackedCopiesOfOrigin_StopsWhenTheDiffFails` and `TestRemoveUntrackedCopiesOfOrigin_KeepsATrackedFile`.

## Design Decisions
Field ownership in `inboxbatch` remains the sole table for console-editable keys. Quarantine moves keep the strict mover-written rule because a test pins rewritten quarantine copies as dirt.

## Verification
All five ACS predicates (`TestC2597_001` through `TestC2597_005`), `go/internal/inboxstamps`, and `TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial` pass after the final edit.

## Compatibility
`Partition`, `Move`, and `Stamp` keep their public fields. `Plan` adds the `Dropped` slice.

## Limitations
A console edit that collides with an origin deletion of the same file still aborts the merge.
