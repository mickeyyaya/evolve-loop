package textutil

import "fmt"

// TruncateInline returns s shortened to at most n bytes, appending an
// elision marker when shortened.
func TruncateInline(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("… (%d bytes elided)", len(s)-n)
}

// TruncateMiddle keeps the first head and last tail bytes of s, with an
// elision marker in the middle. Returns s unchanged when short enough.
func TruncateMiddle(s string, head, tail int) string {
	if len(s) <= head+tail+32 {
		return s
	}
	elided := len(s) - head - tail
	return s[:head] + fmt.Sprintf("… (%d bytes elided) …", elided) + s[len(s)-tail:]
}
