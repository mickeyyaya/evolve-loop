# Build Explanation — Cycle 2358

## Build Binding
- Cycle: 2358
- Base SHA: 722ecf0d20a8e61948b4a9bdc817ac90d4672b4d

## Summary
The `inboxmover` facade now exports `RefsInboxIDs`. `ClosesInboxIDs` already treats `Closes-Inbox: none` as an empty set.

## Rationale
The leaf package `closesinbox` already holds the grammar for `none` and for `Refs-Inbox:`. The facade only passed `IDs` through. Adding one pass-through for `RefIDs` gives callers one home and duplicates no logic.

## Changed Areas
- `go/internal/inboxmover/closesmarker.go` — adds `RefsInboxIDs`, which calls `closesinbox.RefIDs`.
- `go/internal/inboxmover/closesmarker_test.go` — tests the none set, the Refs-Inbox marker and the none-only case.
- `docs/architecture/packages/internal-inboxmover.md` — states the none set and `RefsInboxIDs`.
- `docs/architecture/inbox-injection-protocol.md` — states the same in the marker section.

## Design Decisions
No new parser was written. The facade stays a thin call into the leaf, so the grammar has one home.

## Verification
The package tests and the cycle 2358 ACS predicates pass.

## Compatibility
`ClosesInboxIDs` keeps its signature and its behavior.

## Limitations
No production caller reads `RefsInboxIDs` yet.
