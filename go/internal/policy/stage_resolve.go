package policy

import "strings"

const (
	stageWordLegacy  = "legacy"
	stageWordEnforce = "enforce"
)

func ResolveIsolationStage(raw string) string {
	switch word := strings.ToLower(strings.TrimSpace(raw)); word {
	case "":
		return stageWordShadow
	case "0":
		return stageWordOff
	case stageWordOff, stageWordShadow, stageWordEnforce:
		return word
	default:
		return stageWordOff
	}
}

func ResolveFlowStage(raw string) (string, bool) {
	switch raw {
	case "", stageWordLegacy:
		return stageWordLegacy, true
	case stageWordShadow, stageWordEnforce:
		return raw, true
	}
	return stageWordLegacy, false
}

func ResolveQueueStage(raw string) string {
	if raw == stageWordEnforce {
		return stageWordEnforce
	}
	return stageWordShadow
}
