# Build Explanation — Cycle 2262

## Build Binding
- Cycle: 2262
- Base SHA: 1670401601befe15d56bd06384543835605a8baf

## Summary
The inbox item writer now writes the layout that `evolve inbox add` files: two-space indent, sorted keys, no HTML escape, a final newline.

## Rationale
The writer used `json.Marshal`. It wrote one line and escaped `<`, `>` and `&`. A weight stamp then changed the whole file in git. One encoder at the single writer seam fixes every verb that rewrites an item.

## Changed Areas
- `go/internal/inboxmover/lifecycle/item.go` — `rewriteItemJSON` encodes through `marshalFiled`, which indents and does not escape HTML.
- `go/internal/inboxmover/lifecycle/file.go` — `publishNewItem` encodes through `marshalFiled`, so `inbox add` and `inbox edit` share one home for the filed layout.
- `go/internal/inboxmover/lifecycle/item_test.go` — byte expectations now name the indented layout.
- `go/internal/inboxmover/lifecycle/fieldowner_test.go` — `keysChanged` compares compacted JSON, so a re-indent is not a change.
- `go/internal/inboxmover/lifecycle/release_test.go` — the quarantined item bytes use the indented layout.
- `go/internal/inboxmover/lifecycle/release_claim_test.go` — the stamp check names the indented key.
- `go/internal/inboxmover/lifecycle/release_redact_test.go` — the stamp checks name the indented keys.
- `go/internal/inboxmover/continuation_release_test.go` — the release record check names the indented keys.
- `go/internal/inboxmover/continuation_shed_test.go` — the preserved pointer checks name the indented keys.
- `go/internal/inboxmover/lifecycle_seam_test.go` — the drain and bump byte checks use the indented layout.
- `go/internal/inboxmover/mover_written_facade_test.go` — the facade byte check uses the indented layout.
- `go/internal/inboxmover/testdata/g2.item.golden` — the golden item uses the indented layout.
- `go/internal/inboxmover/testdata/g4.item.golden` — the golden item uses the indented layout.
- `go/cmd/evolve/cmd_loop_wavesync_test.go` — compares with whitespace removed.
- `go/cmd/evolve/cmd_inbox_curator_test.go` — the route check names the indented key.
- `go/cmd/evolve/cmd_syncmain_backfill_e3_test.go` — the route check names the indented key.

## Design Decisions
One encoder at the writer seam, with no flag. Nested objects are indented too, as `inbox add` files them.

## Verification
`go test -count=1 ./internal/inboxmover/...` passes. Predicates 001, 002 and 003 of cycle 2262 pass.

## Compatibility
Key order stays sorted. Readers parse JSON, so the layout change does not affect them.

## Limitations
`edit.go` is a protected path, so `jsonStrings` still HTML-escapes list entries on add or remove. That part is console work.
