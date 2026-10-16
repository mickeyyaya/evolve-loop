package log

import "strconv"

const maxDiagnosticRunes = 512

func DiagnosticField(value string) string {
	return strconv.QuoteToASCII(boundedUTF8(value))
}
