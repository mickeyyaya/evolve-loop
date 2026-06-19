package flagregistry

import "testing"

// TestFlagRegistry_MigratedShellReadFlagsAreDeprecated is the cycle-360
// regression guard, updated for the cycle-7 retirement.
//
// Cycle 360 tried to remove four flags as "dead", trusting a stale 2026-06-11
// "no reader on any surface" inventory — but at the time they had LIVE readers
// in adapters/claude.sh, so the remediation pinned them as live.
//
// The script→Go migration (TASK A5) then DELETED adapters/claude.sh: the four
// flags lost their ONLY reader and were reclassified to StatusDeprecated.
//
// Cycle 7 completes the journey: all four rows are REMOVED from the registry.
// Operators who still set these env vars get a clean no-op (unrecognized vars
// are silently ignored by the Go runtime); the Go bridge has superseded every
// former use. This guard now asserts ABSENCE — any accidental re-introduction
// of these rows will fail CI immediately.
func TestFlagRegistry_MigratedShellReadFlagsAreDeprecated(t *testing.T) {
	retired := []string{
		"EVOLVE_INNER_SANDBOX",          // superseded by EVOLVE_SANDBOX
		"EVOLVE_FORCE_INNER_SANDBOX",    // deprecation bridge — both ends gone
		"EVOLVE_PROFILE_WORKTREE_AWARE", // superseded by BridgeRequest.Worktree
		"EVOLVE_REINVOKE_CMD",           // superseded by manifest interactive_prompts
	}
	for _, name := range retired {
		if _, ok := Lookup(name); ok {
			t.Errorf("flag %q must be ABSENT from the registry (cycle-7 retirement); "+
				"it was removed in the same diff as FlagCeiling=258 — "+
				"do not re-introduce this row", name)
		}
	}
}
