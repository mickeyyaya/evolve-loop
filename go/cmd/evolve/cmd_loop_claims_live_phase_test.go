package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestClaimsLivePhase_DecidesFromTheSharedTerminalBelief(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cs   core.CycleState
		want bool
	}{
		{"live phase", core.CycleState{CycleID: 3, Phase: "build"}, true},
		{"aborted", core.CycleState{CycleID: 3, Phase: string(core.PhaseAborted)}, false},
		{"end", core.CycleState{CycleID: 3, Phase: string(core.PhaseEnd)}, false},
		{"empty phase", core.CycleState{CycleID: 3}, false},
		{"no cycle", core.CycleState{Phase: "build"}, false},
		{"completed phase", core.CycleState{CycleID: 3, Phase: "build", CompletedPhases: []string{"build"}}, false},
	}
	for _, tc := range cases {
		if got := claimsLivePhase(tc.cs); got != tc.want {
			t.Errorf("%s: claimsLivePhase=%v, want %v", tc.name, got, tc.want)
		}
	}
}
