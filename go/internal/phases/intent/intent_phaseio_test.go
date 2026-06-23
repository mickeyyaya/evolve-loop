package intent

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phaseio"
)

// ADR-0050 §3.10 Slice 2: intent reads the goal from the typed envelope at enforce
// (req.Input.Active()) and from the legacy Context["goal"] below it.

// Typed envelope and legacy map carrying the same goal render the identical prompt
// — the byte-identity proof for the migrated read.
func TestIntent_ComposePrompt_TypedGoalEqualsMap(t *testing.T) {
	const goal = "ship a typed phase envelope"
	mapReq := core.PhaseRequest{Context: map[string]string{"goal": goal}}
	typedReq := core.PhaseRequest{
		Context: map[string]string{"goal": goal},
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase:       "intent",
			CycleInputs: phaseio.NewCycleInputs(phaseio.CycleInputsInit{Goal: goal}),
		}),
	}
	want := hooks{}.ComposePrompt("BODY", mapReq)
	got := hooks{}.ComposePrompt("BODY", typedReq)
	if got != want {
		t.Errorf("typed envelope prompt != map prompt:\n typed=%q\n   map=%q", got, want)
	}
	if !strings.Contains(got, "- goal: "+goal) {
		t.Errorf("goal not rendered: %q", got)
	}
}

// At enforce the typed source must actually be consulted: with NO Context["goal"]
// but a populated envelope, the goal still renders (would fail if the gate read the
// map).
func TestIntent_ComposePrompt_EnforceReadsTypedNotMap(t *testing.T) {
	const goal = "from the envelope only"
	req := core.PhaseRequest{
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase:       "intent",
			CycleInputs: phaseio.NewCycleInputs(phaseio.CycleInputsInit{Goal: goal}),
		}),
	}
	got := hooks{}.ComposePrompt("BODY", req)
	if !strings.Contains(got, "- goal: "+goal) {
		t.Errorf("enforce must read goal from the typed envelope even with empty Context: %q", got)
	}
}
