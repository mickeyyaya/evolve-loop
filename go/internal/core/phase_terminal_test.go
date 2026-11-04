package core

import "testing"

func TestIsTerminalPhase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		phase string
		want  bool
	}{
		{string(PhaseEnd), true},
		{string(PhaseAborted), true},
		{"aborted", true},
		{"", false},
		{string(PhaseBuild), false},
		{string(PhaseRetro), false},
	}
	for _, tc := range cases {
		if got := IsTerminalPhase(tc.phase); got != tc.want {
			t.Errorf("IsTerminalPhase(%q)=%v, want %v", tc.phase, got, tc.want)
		}
	}
}
