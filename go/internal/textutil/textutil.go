package textutil

import "fmt"

const middleElisionSlack = 32

func TruncateInline(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + fmt.Sprintf("… (%d bytes elided)", len(s)-n)
}

func TruncateMiddle(s string, head, tail int) string {
	if len(s) <= head+tail+middleElisionSlack {
		return s
	}
	elided := len(s) - head - tail
	return s[:head] + fmt.Sprintf("… (%d bytes elided) …", elided) + s[len(s)-tail:]
}
