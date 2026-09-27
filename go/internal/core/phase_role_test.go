package core

import "testing"

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
