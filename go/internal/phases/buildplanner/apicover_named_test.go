package buildplanner

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
)

func TestPhase_BaseRunnerNamedRunner(t *testing.T) {
	var p *Phase = New(Config{})
	if p == nil {
		t.Fatal("New must return a non-nil *Phase")
	}
	var _ runner.Hooks = p
	var _ runner.Skipper = p

	var br *runner.BaseRunner = p.BaseRunner()
	if br == nil {
		t.Fatal("BaseRunner() must return a non-nil *runner.BaseRunner")
	}
	var _ core.PhaseRunner = br
	if got := br.Name(); got != string(core.PhaseBuildPlanner) {
		t.Errorf("BaseRunner().Name() = %q, want %q", got, core.PhaseBuildPlanner)
	}
}
