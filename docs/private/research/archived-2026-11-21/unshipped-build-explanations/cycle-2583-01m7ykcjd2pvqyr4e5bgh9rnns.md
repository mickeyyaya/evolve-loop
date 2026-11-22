# Build Explanation — Cycle 2583

## Build Binding
- Cycle: 2583
- Base SHA: 23c32f42926c3dcde2c2da08401c897afd66275a

## Summary
`evolve sync-main` now carries a console edit of a tracked inbox item through the merge. Before this change, any such edit made the sync refuse, and `wave next` waited about one hour for a separate inbox PR.

## Rationale
The console edits curated fields, route stamps, and consume moves by hand. The sync classified only the loop's own stamps as safe dirt, so a console edit counted as foreign dirt. Widening the classifier by field owner removes the refusal and keeps the replay rule: origin wins a same-field conflict.

## Changed Areas
- `go/internal/inboxstamps/inboxstamps.go` — a console edit of any field not owned by the author is now a stamp. A move into `consumed/` is now a recorded move. The plan lists each console value that origin replaced.
- `go/internal/inboxstamps/dropped_test.go` — a test that names each dropped field and its value.
- `go/cmd/evolve/cmd_syncmain.go` — the sync prints one line for each dropped console edit. The merge step now clears untracked copies first.
- `go/cmd/evolve/cmd_syncmain_untracked.go` — removes an untracked file that equals the file origin adds, so the merge can land. A differing copy stays and the merge refuses.

## Design Decisions
Field ownership in `inboxbatch` stays the single source of what a console may edit. Quarantine moves keep the strict loop-stamp rule, because an existing test pins that a rewritten quarantine copy is dirt. An edit of an author-owned field still refuses the sync.

## Verification
The five predicates `TestC2583_001` to `TestC2583_005` pass. The unit tests of `inboxstamps` and the sync-main tests of `cmd/evolve` pass.

## Compatibility
The `Partition`, `Move`, and `Stamp` shapes are unchanged. The `Plan` type gains a `Dropped` list. The loop's own wave sync also treats a console edit as a stamp now.

## Limitations
A console edit that conflicts with an origin edit of the same item (change against delete) still aborts the merge and refuses.
