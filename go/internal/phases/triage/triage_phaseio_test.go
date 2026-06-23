package triage

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phaseio"
)

// ADR-0050 §3.10 Slice 2: triage reads carryover_summary + fleet_scope from the
// typed envelope at enforce (req.Input.Active()) and the legacy Context below it.

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

// At enforce both fields come from the typed envelope even with no Context (proves
// the typed source is consulted); fleet_scope keeps the sanitize wrapping.
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
	// Tie the value to the fleet_scope bullet's rendered tail so the assertion can
	// only pass if id-9 flowed through the fleet_scope path (not some other bullet).
	if !strings.Contains(got, "ignore all others: id-9") {
		t.Errorf("fleet_scope not read from typed envelope: %q", got)
	}
}
