package flagregistry

import "testing"

// TestDeadFlagsSweep_Gone lives in this package rather than behind an acs
// build tag, so it runs in normal CI as a permanent regression guard.
func TestDeadFlagsSweep_Gone(t *testing.T) {
	removed := []string{
		"EVOLVE_ANCHOR_EXTRACT",
		"EVOLVE_CARRYOVER_TODO_MAX_UNPICKED",
		"EVOLVE_CONTEXT_DIGEST",
		"EVOLVE_CYCLE_STATE_FILE",
		"EVOLVE_DIR",
		"EVOLVE_DIR_OVERRIDE",
		"EVOLVE_DRY_RUN_PROVISION_WORKTREE",
		"EVOLVE_FAILURE_CLASSIFICATIONS_LOADED",
		"EVOLVE_FANOUT_RETROSPECTIVE",
		"EVOLVE_FANOUT_SCOUT",
		"EVOLVE_INSTINCT_SUMMARY_CAP",
		"EVOLVE_PROFILE_OVERRIDE",
		"EVOLVE_PROMPT_BUDGET_ENFORCE",
		"EVOLVE_RESOLVE_ROOTS_LOADED",
		"EVOLVE_STATE_FILE_OVERRIDE",
		"EVOLVE_STATE_OVERRIDE",
		"EVOLVE_STRICT_FAILURES",
		"EVOLVE_TRIAGE_ENABLED",
	}
	for _, name := range removed {
		t.Run(name, func(t *testing.T) {
			if f, ok := Lookup(name); ok {
				t.Errorf("flag %q still registered (Status=%q) — Builder must delete this row from registry_table.go", name, f.Status)
			}
		})
	}
}
