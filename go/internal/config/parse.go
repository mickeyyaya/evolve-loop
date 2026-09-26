package config

import (
	"fmt"
	"strings"
)

// GateStage parses the off/shadow/enforce trichotomy of the gate and recovery dials; anything else is StageOff, false.
func GateStage(v string) (Stage, bool) {
	switch strings.TrimSpace(v) {
	case "0", "off":
		return StageOff, true
	case "shadow":
		return StageShadow, true
	case "enforce":
		return StageEnforce, true
	}
	return StageOff, false
}

// RouterStage parses the full off/shadow/advisory/enforce ladder; anything else is StageOff, false.
func RouterStage(v string) (Stage, bool) {
	switch strings.TrimSpace(v) {
	case "0", "off":
		return StageOff, true
	case "shadow":
		return StageShadow, true
	case "advisory":
		return StageAdvisory, true
	case "enforce":
		return StageEnforce, true
	}
	return StageOff, false
}

// parseStage and the parsers below resolve an unknown value to the fail-safe side with a
// warning: a typo must never silently enable a kill path or a staged rollout.
func parseStage(v, varName string, ws *[]Warning) Stage {
	s, ok := RouterStage(v)
	if !ok {
		warn(ws, codeUnknownValue, fmt.Sprintf("%s=%q unknown, defaulting to off", varName, v),
			map[string]string{"key": varName, "value": v, "default": "off"})
	}
	return s
}

func parseEvidenceStage(v, varName string, ws *[]Warning) Stage {
	s, ok := GateStage(v)
	if !ok {
		warn(ws, codeUnknownValue, fmt.Sprintf("%s=%q unknown (want off|shadow|enforce), defaulting to off", varName, v),
			map[string]string{"key": varName, "value": v, "default": "off"})
	}
	return s
}

// parseMode's message names the registry key even for the env dial, whose stamp rewrites only fields.key.
func parseMode(v string, ws *[]Warning) Mode {
	switch strings.TrimSpace(v) {
	case "llm", "dynamic", "dynamic-llm":
		return ModeDynamicLLM
	case "static", "static-preset", "preset":
		return ModeStaticPreset
	default:
		warn(ws, codeUnknownValue, fmt.Sprintf("routing_mode=%q unknown, defaulting to llm", v),
			map[string]string{"key": "routing_mode", "value": v, "default": "llm"})
		return ModeDynamicLLM
	}
}

func parseModelRouting(v, varName string, ws *[]Warning) ModelRouting {
	switch strings.TrimSpace(v) {
	case "static":
		return ModelRoutingStatic
	case "advisory":
		return ModelRoutingAdvisory
	case "auto":
		return ModelRoutingAuto
	default:
		warn(ws, codeUnknownValue, fmt.Sprintf("%s=%q unknown (want static|advisory|auto), defaulting to static", varName, v),
			map[string]string{"key": varName, "value": v, "default": "static"})
		return ModelRoutingStatic
	}
}

func parseEnable(v string, ws *[]Warning) Enable {
	switch strings.TrimSpace(v) {
	case "on":
		return EnableOn
	case "off":
		return EnableOff
	case "content":
		return EnableContent
	default:
		warn(ws, codeUnknownValue, fmt.Sprintf("enabled=%q unknown, defaulting to content", v),
			map[string]string{"key": "enabled", "value": v, "default": "content"})
		return EnableContent
	}
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
