package log

import "strconv"

const maxDiagnosticRunes = 512

// DiagnosticField renders untrusted diagnostic text as one bounded,
// ASCII-quoted field. The returned value never contains a raw line break,
// terminal control, invalid UTF-8 byte, or Unicode formatting character.
func DiagnosticField(value string) string {
	return strconv.QuoteToASCII(boundedUTF8(value))
}
