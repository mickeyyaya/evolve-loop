# Comment history: `internal/log`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d8

### `go/internal/log/sanitize.go:21` — above `func SanitizeField(value string) string {`

```text
// SanitizeField renders untrusted text as one bounded, single-line, valid-UTF-8
// value WITHOUT quoting — for values that land inside a JSON string or a
// single-line record (the Signal Center's event fields and reasons, ADR-0101).
// Control characters, Cf format characters (bidi overrides, zero-width joiners,
// the BOM) and the Unicode line/paragraph separators become a space — a rendered
// line must read left-to-right as written;
// the bound is DiagnosticField's, so the two renderers can never disagree on
// how long a field may be.
```

### `go/internal/log/sanitize_test.go:10` — above `func TestSanitizeField_SharesTheBoundWithDiagnosticFieldAndStripsControls(t *testing.T) {`

```text
// SanitizeField is the unquoted twin of DiagnosticField: the same UTF-8 repair
// and rune cap (ONE rule, shared), with control characters and line separators
// replaced by a space instead of escaped — for values that land inside a JSON
// string or a single-line log record (the Signal Center's fields, ADR-0101).
```
