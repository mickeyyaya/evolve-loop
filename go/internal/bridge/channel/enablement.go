package channel

// enablement.go — ADR-0045 I6: ONE rollout dial. The live bidirectional
// channel (ADR-0037) used to have its own opt-in flag, EVOLVE_CHANNEL=1.
// I6 folds that into the EVOLVE_PHASE_RECOVERY stage so the whole corrective-
// interaction program — telemetry, the correction ladder, the AskBroker, the
// channel correlation those use — rides a single dial (the no-flag-sprawl
// rule). EVOLVE_CHANNEL is deprecated: honored for ONE release with a WARN,
// then removed.
//
// The single source for "is the channel on" both the bridge driver and the
// observer adapter call, so the deprecation policy lives in exactly one place.

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

// Enabled resolves whether the live channel is on. explicitChannel is the raw
// EVOLVE_CHANNEL value ("" when unset); stage is the resolved
// EVOLVE_PHASE_RECOVERY stage ("off"|"shadow"|"enforce").
//
//   - explicit EVOLVE_CHANNEL set → honored verbatim ("1" → on, anything else
//     → off) for one more release, with deprecated=true so the caller emits a
//     one-time WARN;
//   - unset → IMPLIED by the stage: enforce → on (corrective actions execute
//     and their injection correlation rides the channel); off/shadow → off, so
//     the shadow default stays byte-identical (no .live files, no per-tick
//     delta capture) exactly as before I6.
func Enabled(stage, explicitChannel string) (on, deprecated bool) {
	if explicitChannel != "" {
		return explicitChannel == "1", true
	}
	return stage == "enforce", false
}
