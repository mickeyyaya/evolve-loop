# internal/evidencex

> Plan: [incident knowledge base](../../plans/incident-knowledge-base-2026-11.md), decisions D15 and D16.

## Purpose

`internal/evidencex` reduces a source log to a bounded extract. The incident evidence bundle needs a small, stable text that still holds every error. The digest of the whole raw source proves what the extract came from.

## Design

- **Leaf.** The package imports the standard library and `internal/redact` only.
- **`Extract`.** It takes the source bytes and `Options`. `ModePointer` returns the digest and the size only. `ModeExtract` also returns text, kept ranges and templates. An unknown mode and a budget of zero or less are errors.
- **Fixed pipeline.** The same steps run in the same order for each input. Redact each line. Find the error lines and keep five lines of context around each. Keep the last 40 lines. Merge the other repeated lines into templates.
- **Priority.** Error windows come first, then the tail from the last line backward, then templates by count. A piece that does not fit the budget is dropped. The text never exceeds `Budget` bytes.
- **Templates.** A template groups lines that differ only in digit runs. It holds the pattern, the count, the first line and the last line. A line kept in a range does not count.
- **Digest.** The digest is the SHA-256 of the raw source. Redaction changes the text, not the digest or the ranges.
- **Determinism.** No map order, clock or random value reaches the output, so equal input gives equal bytes.

## Invariants

- The enforced apicover gate lists the package, and `apicover_named_test.go` names each export. `Extract` carries `//apicover:ignore`, as `logcompress.Compress` does. The ACS predicate runs apicover without a cover profile.
- Ranges are sorted, do not overlap and stay inside `1..N` lines.
