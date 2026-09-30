package flagregistry

import "testing"

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
