package flagregistry

import "testing"

// FlagCeiling is the monotonic ratchet value for the cluster-consolidation
// campaign. Every cycle that removes flags must lower this constant in the
// same diff — the test below fails if a net addition pushes count above the
// current ceiling.
//
// Cycle 39 migrates 6 legacyFlags (REQUIRE_INTENT, TRIAGE_DISABLE, PLAN_REVIEW,
// TEST_PHASE_ENABLED, BUILD_PLANNER, SWARM_PLANNER) and CONSENSUS_AUDIT to policy structs.
// Cycle 43 migrates 5 bridge-timing flags (SCROLLBACK_LINES, BOOT_TIMEOUT_S,
// ARTIFACT_TIMEOUT_S, ARTIFACT_MAX_EXTENDS, PSMAS_SKIP) to BridgePolicy/WorkflowPolicy.
const FlagCeiling = 68

// TestRegistry_FlagCeiling enforces the one-way ratchet: the registry may
// never exceed FlagCeiling rows. Raising this constant without a matching
// removal is intentionally hard — do not bump it; remove a flag instead.
func TestRegistry_FlagCeiling(t *testing.T) {
	if got := len(All); got > FlagCeiling {
		t.Errorf("len(flagregistry.All) = %d exceeds FlagCeiling=%d — "+
			"remove flags rather than raising the ceiling", got, FlagCeiling)
	}
}
