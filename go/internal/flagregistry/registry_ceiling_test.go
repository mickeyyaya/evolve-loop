package flagregistry

import "testing"

// FlagCeiling is the monotonic ratchet value for the cluster-consolidation
// campaign. Every cycle that removes flags must lower this constant in the
// same diff — the test below fails if a net addition pushes count above the
// current ceiling.
//
// Cycle 34 migrates four gate flags to policy.GatesPolicy.
const FlagCeiling = 83

// TestRegistry_FlagCeiling enforces the one-way ratchet: the registry may
// never exceed FlagCeiling rows. Raising this constant without a matching
// removal is intentionally hard — do not bump it; remove a flag instead.
func TestRegistry_FlagCeiling(t *testing.T) {
	if got := len(All); got > FlagCeiling {
		t.Errorf("len(flagregistry.All) = %d exceeds FlagCeiling=%d — "+
			"remove flags rather than raising the ceiling", got, FlagCeiling)
	}
}
