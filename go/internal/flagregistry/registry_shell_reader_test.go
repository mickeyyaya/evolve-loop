package flagregistry

import "testing"

func TestFlagRegistry_MigratedShellReadFlagsAreAbsent(t *testing.T) {
	retired := []string{
		"EVOLVE_INNER_SANDBOX",
		"EVOLVE_FORCE_INNER_SANDBOX",
		"EVOLVE_PROFILE_WORKTREE_AWARE",
		"EVOLVE_REINVOKE_CMD",
	}
	for _, name := range retired {
		if _, ok := Lookup(name); ok {
			t.Errorf("flag %q must be ABSENT from the registry (cycle-7 retirement); "+
				"it was removed in the same diff as FlagCeiling=258 — "+
				"do not re-introduce this row", name)
		}
	}
}
