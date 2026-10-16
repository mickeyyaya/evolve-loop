package log

import (
	"strings"
	"unicode"
)

func boundedUTF8(value string) string {
	value = strings.ToValidUTF8(value, "�")
	runes := []rune(value)
	if len(runes) > maxDiagnosticRunes {
		value = string(runes[:maxDiagnosticRunes-1]) + "…"
	}
	return value
}

func SanitizeField(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, boundedUTF8(value))
}
