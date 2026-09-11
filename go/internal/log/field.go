package log

import (
	"strconv"
	"strings"
)

const maxDiagnosticRunes = 512

// DiagnosticField renders untrusted diagnostic text as one bounded,
// ASCII-quoted field. The returned value never contains a raw line break,
// terminal control, invalid UTF-8 byte, or Unicode formatting character.
func DiagnosticField(value string) string {
	value = strings.ToValidUTF8(value, "\uFFFD")
	runes := []rune(value)
	if len(runes) > maxDiagnosticRunes {
		value = string(runes[:maxDiagnosticRunes-1]) + "…"
	}
	return strconv.QuoteToASCII(value)
}
