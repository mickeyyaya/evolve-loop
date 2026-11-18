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

	var hooks runner.Hooks = p
	if hooks.PhaseName() != string(core.PhaseBuildPlanner) {
		t.Errorf("Hooks.Name() = %q, want %q", hooks.PhaseName(), core.PhaseBuildPlanner)
	}
	var skipper runner.Skipper = p
	if skipped, _, _, _ := skipper.ShouldSkip(core.PhaseRequest{ProjectRoot: t.TempDir()}); !skipped {
		t.Error("an unconfigured build-planner must skip")
	}

	var br *runner.BaseRunner = p.BaseRunner()
	if br == nil {
		t.Fatal("BaseRunner() must return a non-nil *runner.BaseRunner")
	}
	var pr core.PhaseRunner = br
	if got := pr.Name(); got != string(core.PhaseBuildPlanner) {
		t.Errorf("BaseRunner().Name() = %q, want %q", got, core.PhaseBuildPlanner)
	}
}
