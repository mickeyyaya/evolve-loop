package log

import (
	"strings"
	"unicode"
)

// boundedUTF8 is the ONE bound both field renderers share: invalid bytes become
// the replacement rune and the text is capped at maxDiagnosticRunes runes with a
// trailing ellipsis. DiagnosticField quotes the result; SanitizeField keeps it
// as a single line.
func boundedUTF8(value string) string {
	value = strings.ToValidUTF8(value, "�")
	runes := []rune(value)
	if len(runes) > maxDiagnosticRunes {
		value = string(runes[:maxDiagnosticRunes-1]) + "…"
	}
	return value
}

// SanitizeField renders untrusted text as one bounded, single-line, valid-UTF-8
// value WITHOUT quoting — for values that land inside a JSON string or a
// single-line record (the Signal Center's event fields and reasons, ADR-0101).
// Control characters, Cf format characters (bidi overrides, zero-width joiners,
// the BOM) and the Unicode line/paragraph separators become a space — a rendered
// line must read left-to-right as written;
// the bound is DiagnosticField's, so the two renderers can never disagree on
// how long a field may be.
func SanitizeField(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, boundedUTF8(value))
}
