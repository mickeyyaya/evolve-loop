# Build Explanation — Cycle 1776

## Build Binding
- Cycle: 1776
- Base SHA: d2946e34f4ec958a9dea72c8b23d5eb681b1cc82

## Summary
`evolve inbox consume` now writes the `consumed{at, via, cycle, resolution}` stamp onto the moved item in the same invocation as the move, so console operators no longer hand-edit the consumed JSON afterwards. Two optional flags feed the stamp: `--resolution <text>` and `--cycle <n>|console`.

## Rationale
Console consumptions already carry a `consumed` object, and `inboxmover`'s `retiredAtCycle` (`go/internal/inboxmover/continuation_release.go`) reads `consumed.cycle` as a number or a string. The CLI moved the item and acked its fingerprint but never wrote that stamp, which pushed operators outside the interface. Adding the stamp to the existing command is the smallest change that closes the gap. It adds no new subcommand and no shared helper, because this is the only writer of this shape (YAGNI).

## Changed Areas
- `go/cmd/evolve/cmd_inbox_consume.go` — `runInboxConsume` parses `--resolution`/`--cycle` (the item path may come before or after the flags), decodes the item as a JSON object before the move, renames it into `consumed/`, and atomically rewrites it with the stamp before the binding release and fingerprint ack. The stamp uses `via` = `console-manual`, `at` = UTC RFC3339, `cycle` defaults to `console`, and `resolution` defaults to `""`.
- `go/cmd/evolve/cmd_inbox_consume_test.go` — adds a table-driven edge-case test (flags before the path, unrelated fields preserved, non-numeric, negative and stray-positional arguments refused before the move, malformed/array/null items left pending) and a re-consume test showing a new stamp replaces an earlier one. These sit beside the TDD-authored stamp tests.
- `docs/operations/runtime-reference.md` — the `evolve inbox consume` operator entry documents the new flags, the stamp shape and the refuse-before-move rules.

## Design Decisions
- The item is decoded as `map[string]json.RawMessage`, so every existing field value round-trips unchanged; only the `consumed` key is set or replaced.
- Validation happens before the move. A `--cycle` that is neither a non-negative integer nor `console` exits 10, and an item that is not a JSON object exits 1. In both cases the item stays pending and untouched.
- The stamp is written after the rename, never before it. A pending item never carries a consumed stamp, and a failed rename leaves the item byte-identical.
- The stamp is written before the binding release and the ack. A stamp-write failure after the move is reported non-zero, naming the moved path, the same way the existing ack-failure branch works. The breaker's consumed-corpus reconcilers repair the skipped ack and binding release on the next check.
- The cycle is stored as a string so the no-flag default `console` and a numeric cycle share one JSON type. `cycleOf` already parses numeric strings.

## Verification
The TDD-authored stamp tests `TestRunInbox_Consume_WithFlagsStampsConsumedRecord` and `TestRunInbox_Consume_NoFlagsDefaultsConsumedStamp` failed before the change and pass after it. The third TDD-authored test, `TestRunInbox_Consume_MissingItemWithFlagsStillFailsAndStampsNothing`, is a regression guard: it passed before the change and still passes after it. The pre-existing zero-flag consume tests pass without modification. The builder-added edge-case and re-consume tests pass. ACS predicates `TestC1776_001`–`004` drive these tests through the real `runInbox` dispatch.

## Compatibility
A bare `evolve inbox consume <item-path>` behaves as before, except that it now also writes a default stamp. Key order in the rewritten item follows Go's sorted map encoding, and it is re-indented with two spaces; field values are unchanged. Before this change, an item holding invalid JSON or a non-null value that is not an object (an array, string or number) was moved and then failed at the fingerprint ack with exit 1. A JSON `null` item decoded without error there, so it was moved and the command exited 0. All of these items are now refused with exit 1 while still pending.

## Limitations
`via` is fixed to `console-manual`; there is no `--via` flag because no caller needs one. A crash between the rename and the stamp write leaves a consumed item without a stamp, the same window the move-then-ack ordering already has.
