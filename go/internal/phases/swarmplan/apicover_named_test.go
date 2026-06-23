package swarmplan

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phases/runner"
)

// TestPhase_NamedSatisfiesSkipperContract names the swarmplan.Phase type (New
// returns *Phase but the type is never named in a test) and pins the contract
// TestPhase_Identity does NOT: the concrete *Phase satisfies runner.Hooks +
// runner.Skipper (documented at swarmplan.go:34), and its Skipper behavior —
// with the swarm stage in shadow (the default), the planner skips straight to
// build so the next phase runs single-writer.
func TestPhase_NamedSatisfiesSkipperContract(t *testing.T) {
	var p *Phase = New(Config{})
	if p == nil {
		t.Fatal("New must return a non-nil *Phase")
	}
	// Documented interface contract — never asserted elsewhere.
	var _ runner.Hooks = p
	var _ runner.Skipper = p

	skip, verdict, next, _ := p.ShouldSkip(core.PhaseRequest{
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
