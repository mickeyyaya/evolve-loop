# internal/textutil

## Purpose

`internal/textutil` shortens strings for human-facing diagnostics: log lines, ledger summaries and error messages. The truncation logic was duplicated across those call sites, and one leaf keeps the elision format consistent.

## Design

- **Two helpers.** `TruncateInline` caps a string to n bytes and appends an elision marker. `TruncateMiddle` keeps a head and a tail and elides the middle, for values where both ends carry signal, such as SHAs and paths.
- **Byte-based, not rune-aware.** The helpers are for diagnostics. They must not be used to render user content.
- **Pure, stdlib only.** Neither helper has a dependency beyond `fmt`.
- **Middle elision has a slack.** `TruncateMiddle` returns the string unchanged while its length is within `head + tail + middleElisionSlack` (32 bytes). Eliding a span that short saves too little to justify the marker.

## Invariants

- **Short strings pass through.** `TestTruncateInline` and `TestTruncateMiddle` pin the unchanged case.
- **The elision format is fixed.** The tests pin the exact marker text and the elided byte count.

## Findings

- None.
