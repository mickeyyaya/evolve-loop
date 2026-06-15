package core

import "testing"

// TestPhaseRole pins the phase→role vocabulary the provenance ledger binding
// records. audit→auditor and build→builder are the exact role strings ship's
// audit-binding (findLatestAudit) and the rt-001-ledger-role-completeness
// red-team predicate depend on; every other phase binds under its own name
// (identity), so a user-defined phase gets a stable, predictable role. The
// identity fallback must never rename a known agent role.
func TestPhaseRole(t *testing.T) {
	for _, c := range []struct {
		phase Phase
		want  string
	}{
		{PhaseAudit, "auditor"},
		{PhaseBuild, "builder"},
		{PhaseScout, "scout"},
		{PhaseTriage, "triage"},
		{Phase("custom-user-phase"), "custom-user-phase"},
	} {
		if got := phaseRole(c.phase); got != c.want {
			t.Errorf("phaseRole(%q) = %q, want %q", c.phase, got, c.want)
		}
	}
}
