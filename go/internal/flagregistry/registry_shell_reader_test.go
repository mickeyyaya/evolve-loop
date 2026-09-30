package flagregistry

import "testing"

// TestFlagRegistry_MigratedShellReadFlagsAreDeprecated asserts these flags are
// absent from the registry, despite the name: they progressed from deprecated
// to fully removed.
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
