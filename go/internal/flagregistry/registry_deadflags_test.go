package flagregistry

import "testing"

// TestDeadFlagsSweep_Gone is the regression test for cycle-2 dead-flag-sweep.
// It asserts that every flag retired from registry_table.go in this cycle is
// truly absent from the registry — Lookup must return ok=false for each name.
//
// RED: all 18 entries are still registered in registry_table.go (Lookup returns
// ok=true), so each sub-test fails until Builder removes the rows.
//
// This test lives in the flagregistry package (not go/acs/cycle2/) so it runs
// under `go test ./internal/flagregistry/...` without the `acs` build tag —
// providing a fast, permanent regression guard in normal CI.
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
