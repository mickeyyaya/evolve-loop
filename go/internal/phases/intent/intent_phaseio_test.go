package intent

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

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
