# Build Explanation — Cycle 2169

## Build Binding
- Cycle: 2169
- Base SHA: 09bb331d490eed147e09f5f52c8964ad715e969d

## Summary
The command sync-main now accepts the loop's own quarantine move of a tracked inbox item. It records the move in history, then merges origin.

## Rationale
The loop moves a tracked item to the quarantine directory after repeated failures. The old classifier saw a deleted tracked file and refused. A new Move class lets the classifier separate this loop-owned write from real dirt.

## Changed Areas
- `go/internal/inboxstamps/inboxstamps.go` — adds the Move type, the quarantine-move test, CommitMoves, and a shared stampBetween function.
- `go/cmd/evolve/cmd_syncmain.go` — commits the moves before the fetch and the merge.
- `go/internal/inboxstamps/moves_test.go` — covers each new branch of the package.

## Design Decisions
A deletion counts as a move only when a quarantine copy exists and differs from the HEAD item in loop-written fields alone. The same field rule decides a stamp, so one function now serves both.

## Verification
Predicates TestC2169_001 to TestC2169_009 pass. The inboxstamps package keeps 100% statement coverage. Predicate 007 checks that a sync-main run outside a git tree fails loudly. Predicate 008 checks that `refuseDirtBeyondInboxStamps` at `go/cmd/evolve/cmd_syncmain.go:191` has 100.0% coverage from the sync-main tests. Predicate 009 checks that an origin edit of the moved item aborts the merge and keeps the move.

## Compatibility
Every refusal path stays. A source edit, a plain deletion and a rewritten copy still exit with the refused code. One order effect exists. The move commit happens in `refuseDirtBeyondInboxStamps` at `go/cmd/evolve/cmd_syncmain.go:202`, before the detached-HEAD check. A detached-HEAD refusal therefore commits the move first.

## Limitations
If origin edits the moved item, the merge conflicts and sync-main aborts cleanly. The move commit stays in history. A detached-HEAD refusal also leaves the move commit in history.
