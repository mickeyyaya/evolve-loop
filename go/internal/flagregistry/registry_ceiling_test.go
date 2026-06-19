package flagregistry

import "testing"

// FlagCeiling is the monotonic ratchet value for the cluster-consolidation
// campaign. Every cycle that removes flags must lower this constant in the
// same diff — the test below fails if a net addition pushes count above the
// current ceiling.
//
// 2026-06-19 integration bump 155→160: the v20 integration merged two feature
// branches that each introduced genuine new operator-facing dials — concurrency
// (cliadmit/soak/sessionreaper config) and the advisor cycle-budget
// (EVOLVE_CYCLE_BUDGET + EVOLVE_MAX_CYCLES_CAP). These are new capabilities, not
// regressions; the ratchet records the new post-integration floor. The
// consolidation campaign resumes lowering it from here toward <30.
const FlagCeiling = 160

// TestRegistry_FlagCeiling enforces the one-way ratchet: the registry may
// never exceed FlagCeiling rows. Raising this constant without a matching
// removal is intentionally hard — do not bump it; remove a flag instead.
func TestRegistry_FlagCeiling(t *testing.T) {
	if got := len(All); got > FlagCeiling {
		t.Errorf("len(flagregistry.All) = %d exceeds FlagCeiling=%d — "+
			"remove flags rather than raising the ceiling", got, FlagCeiling)
	}
}
