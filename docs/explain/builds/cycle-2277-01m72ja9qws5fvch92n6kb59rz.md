# Build Explanation — Cycle 2277

## Build Binding
- Cycle: 2277
- Base SHA: e1c54e038a160586406b4d8d44160f0fb473a306

## Summary
The findings block of a continuation now carries no control byte. Each reason stays on one line. The size cut never splits a multibyte rune.

## Rationale
The prior-try findings come from a failure artifact and enter the build prompt and the console brief. `readContinuationFindings` serves both the build request and the console brief, so one sanitizer in that function closes both paths. `renderFailReasons` also serves the repair brief (`go/internal/core/repair_brief.go:48`), so the repair brief gets the same one-line reasons. A control byte or an embedded newline in a reason can forge a line of the block.

Rejected option: reuse `inboxbatch.StripControl` (`go/internal/inboxbatch/item.go:385`). It replaces every byte below 0x20 with a space, so it cannot keep the newline and tab of the raw body. It also ignores the C1 controls and the U+2028 and U+2029 line separators. The private `stripControl` at `go/internal/core/continuation_stamp.go:158` takes a per-rune policy, so the raw body keeps its line breaks and a reason loses them.

## Changed Areas
- `go/internal/core/continuation_stamp.go` — adds a control-byte stripper for the raw body. It flattens each phase and reason to one line. It moves the size cut back to a rune start.

## Design Decisions
The raw body keeps its newline and tab, because it is free text. A reason and the phase lose every line break, because the renderer owns the line structure. Clean input renders byte for byte as before.

## Verification
The six predicates of cycle 2277 pass. The core package tests pass with the new unit tests for strip, forgery, rune cut and empty body.

## Compatibility
The output for clean reasons does not change. The truncation marker format does not change.

## Limitations
The stripper removes the escape byte but keeps the printable rest of an escape sequence.

The invariants of `FuzzReadContinuationFindings_IsBoundedAndRendersEveryReason` (`go/internal/core/continuation_backfill_e4_test.go:74` and `:79`) no longer hold for an input with a control byte. That test expects the output to start with the raw phase and to equal the trimmed raw body. A phase or a body with a control byte now renders sanitized, so the output differs. The seed corpus has no such input, so `go test` does not show the gap. A fuzz run can find it.
