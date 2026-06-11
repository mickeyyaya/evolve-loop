package swarmplan

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestPhase_CoverageRunnerAndPrompt(t *testing.T) {
	p := New(Config{})

	if br := p.BaseRunner(); br == nil {
		t.Fatal("BaseRunner returned nil")
	}

	req := core.PhaseRequest{
		Cycle:       282,
		ProjectRoot: "/repo",
		Workspace:   "/repo/.evolve/runs/cycle-282",
		Env:         map[string]string{"EVOLVE_SWARM_STAGE": "advisory"},
	}
	got := p.ComposePrompt("planner body", req)
	for _, want := range []string{"planner body", "cycle: 282", "workspace:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ComposePrompt missing %q:\n%s", want, got)
		}
	}
}
