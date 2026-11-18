package swarmplan

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
)

func TestPhase_NamedSatisfiesSkipperContract(t *testing.T) {
	var p *Phase = New(Config{})
	if p == nil {
		t.Fatal("New must return a non-nil *Phase")
	}

	var hooks runner.Hooks = p
	if hooks.PhaseName() == "" {
		t.Error("swarmplan hooks must name their phase")
	}
	var skipper runner.Skipper = p
	skip, verdict, next, _ := skipper.ShouldSkip(core.PhaseRequest{
		ProjectRoot: t.TempDir(),
		Env:         map[string]string{"EVOLVE_SWARM_STAGE": "shadow"},
	})
	if !skip {
		t.Fatal("ShouldSkip: swarm-plan must skip when the swarm stage is shadow")
	}
	if verdict != core.VerdictSKIPPED || next != string(core.PhaseBuild) {
		t.Errorf("ShouldSkip = (verdict=%q next=%q), want (SKIPPED, build)", verdict, next)
	}
}
