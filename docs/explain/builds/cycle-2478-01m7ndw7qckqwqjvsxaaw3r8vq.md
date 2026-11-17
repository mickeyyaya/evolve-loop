# Build Explanation — Cycle 2478

## Build Binding
- Cycle: 2478
- Base SHA: de22fd9eeaecac66f9b14ffb7631521b3ac0c4d5

## Summary
The inbox item writer now keeps text values literal. A curated value that holds angle brackets or an ampersand is no longer HTML-escaped in the filed item.

## Rationale
The file encoder already disabled HTML escaping. The values passed through `json.Marshal` first, which escaped them before the encoder saw them. The writer is the one place every path shares, so it restores the three escaped characters there. The route and edit helpers sit on protected paths and stay unchanged.

## Changed Areas
- `go/internal/inboxmover/lifecycle/item.go` — `marshalFiled` passes each value through `literalValue`, which turns `\u003c`, `\u003e` and `\u0026` back into literal characters and keeps key order.
- `docs/operations/runtime-reference.md` — the `evolve inbox edit` entry describes the filed-format rewrite and is split into short bullets.
- `go/internal/inboxmover/lifecycle/item_doc_repro_test.go` — guards the doc wording against the stale claim.
- `go/acs/cycle2478/predicates_test.go` — predicates for the one-line weight diff, literal id tokens and no escaping.
- `.evolve/evals/inbox-item-writer-keeps-the-filed-format-lane.md` — eval graders for the task.

## Design Decisions
One normalization step in the writer covers every value path. Two options were rejected. One was a patch at each call site. The other was a decode and re-encode of each value, which sorted nested keys and broke a golden test.

## Verification
The cycle 2478 predicates and the inboxmover package tests pass, and `go vet` reports nothing.

## Compatibility
The file layout is unchanged. Only values with `<`, `>` or `&` change: they are now literal.

## Limitations
Items filed earlier with escaped sequences keep them until the next rewrite of that item. Each rewrite passes every value of the item through `literalValue`, so an edit of any one field unescapes all values of that item.
