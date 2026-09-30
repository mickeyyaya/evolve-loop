package triage

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

func TestTriage_ComposePrompt_TypedEqualsMap(t *testing.T) {
	const carry, scope = "carried: finish the digest fallback", "id-1,id-2"
	ctx := map[string]string{"carryover_summary": carry, "fleet_scope": scope}
	mapReq := core.PhaseRequest{Context: ctx}
	typedReq := core.PhaseRequest{
		Context: ctx,
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase:       "triage",
			CycleInputs: phaseio.NewCycleInputs(phaseio.CycleInputsInit{Carryover: carry, FleetScope: scope}),
		}),
	}
	want := hooks{}.ComposePrompt("BODY", mapReq)
	got := hooks{}.ComposePrompt("BODY", typedReq)
	if got != want {
		t.Errorf("typed envelope prompt != map prompt:\n typed=%q\n   map=%q", got, want)
	}
}

func TestTriage_ComposePrompt_EnforceReadsTyped(t *testing.T) {
	req := core.PhaseRequest{
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase:       "triage",
			CycleInputs: phaseio.NewCycleInputs(phaseio.CycleInputsInit{Carryover: "C-typed", FleetScope: "id-9"}),
		}),
	}
	got := hooks{}.ComposePrompt("BODY", req)
	if !strings.Contains(got, "- carryover_summary: C-typed") {
		t.Errorf("carryover not read from typed envelope: %q", got)
	}
	// Asserts against the fleet_scope bullet's tail so the check only passes
	// if id-9 came through fleet_scope.
	if !strings.Contains(got, "ignore all others: id-9") {
		t.Errorf("fleet_scope not read from typed envelope: %q", got)
	}
}
