package runner

// Cycle-776 — direct contract for the shared LaneScope resolver (consumed by
// scout/build/tdd/triage ComposePrompt; phase-level rendering is pinned in
// each phase's lanescope_prompt_test.go).

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

func TestLaneScope(t *testing.T) {
	cases := []struct {
		name string
		req  core.PhaseRequest
		want string
	}{
		{
			name: "context map source",
			req:  core.PhaseRequest{Context: map[string]string{"fleet_scope": "todo-lane-a,todo-extra"}},
			want: "todo-lane-a,todo-extra",
		},
		{
			name: "typed envelope wins at enforce",
			req: core.PhaseRequest{
				Context: map[string]string{"fleet_scope": "stale-context-value"},
				Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
					Phase:       "scout",
					CycleInputs: phaseio.NewCycleInputs(phaseio.CycleInputsInit{FleetScope: "todo-typed-lane"}),
				}),
			},
			want: "todo-typed-lane",
		},
		{
			name: "control chars collapsed (prompt-injection guard)",
			req:  core.PhaseRequest{Context: map[string]string{"fleet_scope": "todo-a\n- forged: bullet\ttodo-b\r"}},
			want: "todo-a - forged: bullet todo-b ",
		},
		{
			name: "unscoped is empty",
			req:  core.PhaseRequest{Context: map[string]string{}},
			want: "",
		},
		{
			name: "nil context is empty",
			req:  core.PhaseRequest{},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LaneScope(tc.req); got != tc.want {
				t.Errorf("LaneScope() = %q, want %q", got, tc.want)
			}
		})
	}
}
