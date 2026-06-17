package flagregistry

import "testing"

// TestFlagRegistry_NoBudgetClusterDeadFlags is the cycle-356 regression guard.
// It asserts that none of the 12 dead Budget Cluster flags (StatusDead,
// DEPRECATED no-op since PR #96) remain in the registry after removal.
//
// This test is authored RED by TDD-engineer (flags still present) and turns
// GREEN when Builder removes all 12 rows from registry_table.go.
//
// The 12 flags were confirmed dead by cycle-356 grep sweep — zero behavioral
// readers; only help text, test setenv, and documentation references remain.
func TestFlagRegistry_NoBudgetClusterDeadFlags(t *testing.T) {
	deadBudgetFlags := []string{
		"EVOLVE_BATCH_BUDGET_CAP",
		"EVOLVE_BATCH_BUDGET_DISABLE",
		"EVOLVE_BUDGET_CAP",
		"EVOLVE_BUDGET_ENFORCE",
		"EVOLVE_BUDGET_MAX_CYCLES",
		"EVOLVE_BUILDER_COST_GUARD_STRICT",
		"EVOLVE_BUILDER_COST_THRESHOLD",
		"EVOLVE_CHECKPOINT_AT_PCT",
		"EVOLVE_CHECKPOINT_WARN_AT_PCT",
		"EVOLVE_FANOUT_PER_WORKER_BUDGET_USD",
		"EVOLVE_MAX_BUDGET_USD",
		"EVOLVE_PHASE_COST_CEILING",
	}
	for _, name := range deadBudgetFlags {
		if f, ok := Lookup(name); ok {
			t.Errorf("dead Budget Cluster flag %q still in registry: Status=%q Cluster=%q\n"+
				"Remove this row from registry_table.go (cycle-356 dead-flag removal).",
				name, f.Status, f.Cluster)
		}
	}
}
