package buildplanner

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
)

// TestPhase_BaseRunnerNamedRunner names the concrete buildplanner.Phase type
// (New returns *Phase but the type is never named in a test) and exercises the
// BaseRunner() adapter — the documented seam that wraps the phase so it
// satisfies core.PhaseRunner for the orchestrator's runners map. It pins two
// contracts the existing tests do not:
//  1. BaseRunner() returns a non-nil *runner.BaseRunner whose Name() reports
//     the "build-planner" identity (proving the Phase's PhaseName flows through).
//  2. The returned runner satisfies core.PhaseRunner — the whole reason
//     BaseRunner() exists.
func TestPhase_BaseRunnerNamedRunner(t *testing.T) {
	var p *Phase = New(Config{})
	if p == nil {
		t.Fatal("New must return a non-nil *Phase")
	}
	// The Phase itself satisfies the runner.Hooks + runner.Skipper contracts it
	// documents.
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
