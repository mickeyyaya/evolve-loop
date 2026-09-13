package config

// parse.go — the dial parsers: two pure, exported stage ladders (the ONE
// spelling the composition root's forwarders keep) and the warning parsers
// over them and over the mode, model-routing and enable vocabularies. An
// unknown value always resolves to the fail-safe side WITH an unknown-value
// warning — a typo must never silently enable a kill-path or a staged rollout.

import (
	"fmt"
	"strings"
)

// GateStage is the off/shadow/enforce trichotomy the gate and recovery dials
// use (no advisory middle state — these axes are compute-and-log vs act).
// Trims; "0" and "off" are off; ok=false with StageOff for anything else.
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

// RouterStage is the full off→shadow→advisory→enforce ladder dynamic routing,
// unified phase I/O, the router re-plan and the parallel-evaluate dispatcher
// use. Trims; ok=false with StageOff for anything outside it.
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

// parseStage parses a full off→shadow→advisory→enforce dial (RouterStage).
// varName names the offending key in the unknown-value warning; an unknown
// value defaults to off. Shared by dynamic routing, unified phase I/O and the
// router-ladder policy dials.
func parseStage(v, varName string, ws *[]Warning) Stage {
	s, ok := RouterStage(v)
	if !ok {
		warn(ws, codeUnknownValue, fmt.Sprintf("%s=%q unknown, defaulting to off", varName, v),
			map[string]string{"key": varName, "value": v, "default": "off"})
	}
	return s
}

// parseEvidenceStage parses an off/shadow/enforce dial (GateStage). Used by
// the environment-backed commit-evidence stage and the gate/recovery policy
// dials; varName names the offending env var or policy key in the warning.
func parseEvidenceStage(v, varName string, ws *[]Warning) Stage {
	s, ok := GateStage(v)
	if !ok {
		warn(ws, codeUnknownValue, fmt.Sprintf("%s=%q unknown (want off|shadow|enforce), defaulting to off", varName, v),
			map[string]string{"key": varName, "value": v, "default": "off"})
	}
	return s
}

// parseMode parses the routing brain; an unknown value keeps the locked
// default (llm). The message names the registry key (the env arm restamps
// fields.key with the env var — the message spelling is pinned).
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

// parseModelRouting parses the cycle-436 model-authority axis. Unknown values
// fall back to the SAFE static side (never silently enable auto) with a
// warning — mirroring parseStage/parseMode's fail-safe-with-warning contract.
// varName names the offending key in the warning.
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

// parseEnable parses a phases[].enabled word; an unknown value is content
// (trigger-decided). The message spelling is pinned; applyPhases restamps
// fields.key with the phase's own key.
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
