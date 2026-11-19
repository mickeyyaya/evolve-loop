# internal/evidencex

> Plan: [incident knowledge base](../../plans/incident-knowledge-base-2026-11.md), decisions D15, D16 and D17.

## Purpose

`internal/evidencex` reduces a source log to a bounded extract and stores raw full-mode copies. The incident evidence bundle needs a small, stable text that still holds every error. The digest of the whole raw source proves what the extract came from.

## Design

- **Leaf.** The package imports the standard library and `internal/redact` only.
- **`Extract`.** It takes the source bytes and `Options`. `ModePointer` returns the digest and the size only. `ModeExtract` also returns text, kept ranges and templates. An unknown mode and a budget of zero or less are errors.
- **Fixed pipeline.** The same steps run in the same order for each input. Redact each line. Find the error lines and keep five lines of context around each. Keep the last 40 lines. Merge the other repeated lines into templates.
- **Priority.** Error windows come first, then the tail from the last line backward, then templates by count. A piece that does not fit the budget is dropped. The text never exceeds `Budget` bytes.
- **Templates.** A template groups lines that differ only in digit runs. It holds the pattern, the count, the first line and the last line. A line kept in a range does not count.
- **Digest.** The digest is the SHA-256 of the raw source. Redaction changes the text, not the digest or the ranges.
- **Determinism.** No map order, clock or random value reaches the output, so equal input gives equal bytes.
- **Archive port.** `EvidenceArchive` defines `Put` and `Get` by `source_sha256`. `NewLocalArchive` builds a local directory store with a positive `maxBytes` cap.
- **Codec seam.** An unexported `archiveCodec` interface isolates compression from storage. Because the module has no zstd package, the local adapter uses standard-library `compress/gzip` as the default codec.
- **Archive safety.** Keys must be 64 lowercase hexadecimal characters. `Put` rejects raw payloads larger than `maxBytes` with `ErrArchiveTooLarge` before disk writes, skips existing keys, and writes through a temp file and rename. `Get` returns `ErrArchiveNotFound` for absent keys.

## Invariants

- The enforced apicover gate lists the package, and `apicover_named_test.go` names each export. `Extract` and `NewLocalArchive` carry `//apicover:ignore`, as `logcompress.Compress` does. The ACS predicate runs apicover without a cover profile.
- Ranges are sorted, do not overlap and stay inside `1..N` lines.
- No malformed archive key or refused write leaves a file in the archive directory.
