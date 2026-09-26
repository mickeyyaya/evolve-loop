package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func TestClampPlanToFloor_PreservesMintPhases(t *testing.T) {
	mint := []phaseconfig.PhaseConfig{
		{PhaseSpec: phasespec.PhaseSpec{Name: "minted-x", Optional: true}, Prompt: "p"},
	}
	plan := &PhasePlan{
		Entries:    []PhasePlanEntry{{Phase: "scout", Run: true}, {Phase: "ship", Run: true}},
		MintPhases: mint,
	}
	out, _ := ClampPlanToFloor(RouteInput{}, plan)
	if out == nil {
		t.Fatal("clamp returned nil")
	}
	if len(out.MintPhases) != 1 || out.MintPhases[0].Name != "minted-x" {
		t.Errorf("MintPhases not preserved through clamp; got %+v", out.MintPhases)
	}
}
