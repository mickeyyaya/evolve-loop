# Build Explanation — Cycle 1909

## Build Binding
- Cycle: 1909
- Base SHA: de856064796ef2afb3de94abcc8d50e6f5b5e8f8

## Summary
The practice watch now removes Unicode format characters from untrusted text. It reports an unreadable inbox directory as an error before it files an item. It marks a row whose body was cut at the 8 MiB cap as truncated.

## Rationale
The three gaps are LOW findings of the practice watch review. Each fix is the smallest change in the function that owns the behavior. No new type, flag or package is necessary.

## Changed Areas
- `go/internal/practicewatch/item.go` — `sanitizeUntrusted` also drops `unicode.Cf` runes, so bidi overrides and zero-width marks cannot hide text in the filed item. `idFiled` now returns the `WalkDir` error with the inbox path, and it ignores only a missing path.
- `go/internal/practicewatch/fetch.go` — the HTTP read takes one byte past the cap to detect a cut, and `cappedWriter` records a cut. Both set `fetched.truncated`.
- `go/internal/practicewatch/watch.go` — `fileChange` stops with an error row when `idFiled` fails. `watchSource` delegates the record, unchanged and file decision to `settle`, and `markTruncated` appends the truncation note to the row detail.
- `docs/architecture/packages/internal-practicewatch.md` — the trust section states the format-character strip, the truncation marker and the unreadable-inbox error.
- `docs/explain/builds/cycle-1909-01m532nf1rwy3wczbpbeyx40d1.md` — this explanation.

## Design Decisions
The HTTP path reads `bodyLimit+1` bytes, so a body of exactly 8 MiB is not marked. A length check of `len == bodyLimit` was rejected because it marks that exact body falsely. The truncation note goes in the row detail of every post-fetch status, so an operator sees the cut also on a recorded or unchanged row. A missing inbox stays a not-filed result, because the first filing creates the inbox.

## Verification
The cycle 1909 ACS predicates drive `practicewatch.Watch` with a real HTTP server, a stand-in command runner and the real inbox mover. All six pass, and the package tests pass.

## Compatibility
No exported symbol, field or JSON key changes. A row detail can now carry the truncation note.

## Limitations
The marker reports the cut but does not raise the cap. A later cycle can make the cap a policy value if real sources exceed it.
