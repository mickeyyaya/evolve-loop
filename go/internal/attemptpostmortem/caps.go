package attemptpostmortem

const ellipsis = "…"

func capHead(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit-1]) + ellipsis
}

func capTail(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return ellipsis + string(runes[len(runes)-limit+1:])
}
