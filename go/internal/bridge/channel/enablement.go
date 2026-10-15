package channel

import "strings"

func ResolveStage(raw string) string {
	switch s := strings.ToLower(strings.TrimSpace(raw)); s {
	case "":
		return "shadow"
	case "0":
		return "off"
	case "off", "shadow", "enforce":
		return s
	default:
		return "off"
	}
}

func Enabled(stage string) bool {
	return stage == "enforce"
}
