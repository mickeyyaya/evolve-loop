package channel

// enablement.go — ADR-0045 I6: ONE rollout dial. The live bidirectional
// channel (ADR-0037) rides EVOLVE_PHASE_RECOVERY: enforce implies the channel
// on; off/shadow → byte-identical (no .live files, no per-tick delta capture).
// The deprecated explicit flag was retired in v19.x.
//
// The single source for "is the channel on" both the bridge driver and the
// observer adapter call.

import "strings"

// ResolveStage normalizes a raw EVOLVE_PHASE_RECOVERY value to the canonical
// stage vocabulary, the SINGLE home of that rule (the bridge's
// recoveryStageFromEnv and the observer adapter both delegate here, so the
// "unset → shadow, typo → off" policy can never drift between the subprocess
// and in-process readers). Unset/empty → "shadow" (the behavior-neutral first-
// ship default); off|shadow|enforce → as-is; anything else → "off" (a typo
// must never silently enable a kill-path).
func ResolveStage(raw string) string {
	switch s := strings.ToLower(strings.TrimSpace(raw)); s {
	case "":
		return "shadow"
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
