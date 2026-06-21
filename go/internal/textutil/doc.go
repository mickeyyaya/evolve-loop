// Package textutil holds small, allocation-conscious string-shortening helpers
// for human-facing diagnostics (log lines, ledger summaries, error messages).
//
// HOW: [TruncateInline] caps a string to n bytes with a trailing elision
// marker; [TruncateMiddle] keeps a head and a tail and elides the middle (for
// values where both ends carry signal, e.g. SHAs and paths). Byte-based, not
// rune-aware — intended for diagnostics, not for rendering user content.
//
// WHY: the same truncation logic was being duplicated across log/ledger/error
// call sites; one leaf keeps the elision format consistent and the call sites
// terse (DRY). Pure functions, zero dependencies.
//
// Key exported symbols:
//   - [TruncateInline] — cap to n bytes with a trailing elision marker
//   - [TruncateMiddle] — keep head + tail, elide the middle
//
// Depends on: standard library only.
package textutil
