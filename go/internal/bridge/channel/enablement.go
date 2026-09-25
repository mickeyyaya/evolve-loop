package channel

// enablement.go — ADR-0045 I6: ONE rollout dial. The live bidirectional
// channel (ADR-0037) rides EVOLVE_PHASE_RECOVERY: enforce implies the channel
// on; off/shadow → byte-identical (no .live files, no per-tick delta capture).
// The deprecated explicit flag was retired in v19.x.
//
// The single source for "is the channel on" both the bridge driver and the
// observer adapter call.

import "strings"

// ResolveStage normalizes a raw recovery stage word to the canonical stage
// vocabulary, the SINGLE home of that rule for BOTH ADR-0044 recovery dials —
// the program dial (the bridge's recoveryStageFromEnv and the observer adapter)
// and, since F27, the fatal-pane dial (fatalPaneStageOf) — so the
// "unset → shadow, typo → off" policy can never drift between readers.
// Unset/empty → "shadow" (an unwired stage observes); off|shadow|enforce →
// as-is, plus "0" → "off" (config.StageOff.String(), the word the composition
// root forwards — accepted explicitly, not by the typo rule); anything else →
// "off" (a typo must never silently enable a kill-path).
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

// Enabled reports whether the live channel is on. The channel is implied by
// the EVOLVE_PHASE_RECOVERY stage: enforce → on; off/shadow → off.
func Enabled(stage string) bool {
	return stage == "enforce"
}
