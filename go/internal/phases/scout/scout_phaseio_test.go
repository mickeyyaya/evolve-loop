package scout

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phaseio"
)

// ADR-0050 §3.10 Slice 3: scout reads strategy/goal/challengeToken from the typed
// envelope at enforce (req.Input.Active()) and the legacy Context map below it. The
// strategy read is verdict-affecting (convergence-confirmation → SKIPPED), so both
// ComposePrompt AND Classify must consult the typed source.

// Typed envelope and legacy map carrying the same values render the identical prompt
// — the byte-identity proof for the migrated reads.
func TestScout_ComposePrompt_TypedVsMap_Identical(t *testing.T) {
	const strategy, goal, tok = "convergence-confirmation", "ship the typed envelope", "abc123token"
	ctx := map[string]string{"strategy": strategy, "goal": goal, "challengeToken": tok}
	mapReq := core.PhaseRequest{Context: ctx}
	typedReq := core.PhaseRequest{
		Context: ctx,
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase:       "scout",
			CycleInputs: phaseio.NewCycleInputs(phaseio.CycleInputsInit{Strategy: strategy, Goal: goal, ChallengeToken: tok}),
		}),
	}
	want := hooks{}.ComposePrompt("BODY", mapReq)
	got := hooks{}.ComposePrompt("BODY", typedReq)
	if got != want {
		t.Errorf("typed envelope prompt != map prompt:\n typed=%q\n   map=%q", got, want)
	}
}

// At enforce all three fields come from the typed envelope even with NO Context
// (proves the typed source is genuinely consulted, not silently reading the map).
func TestScout_ComposePrompt_EnforceReadsTypedNotMap(t *testing.T) {
	req := core.PhaseRequest{
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase:       "scout",
			CycleInputs: phaseio.NewCycleInputs(phaseio.CycleInputsInit{Strategy: "S-typed", Goal: "G-typed", ChallengeToken: "T-typed"}),
		}),
	}
	got := hooks{}.ComposePrompt("BODY", req)
	if !strings.Contains(got, "- strategy: S-typed") {
		t.Errorf("strategy not read from typed envelope: %q", got)
	}
	if !strings.Contains(got, "- goal: G-typed") {
		t.Errorf("goal not read from typed envelope: %q", got)
	}
	if !strings.Contains(got, "- challenge_token: T-typed") {
		t.Errorf("challenge_token not read from typed envelope: %q", got)
	}
}

// Classify's strategy read is verdict-affecting: convergence-confirmation with an
// empty backlog maps to SKIPPED. The typed envelope (no Context) must produce the
// SAME verdict as the legacy map — verdict-equivalence AND proof the typed source is
// consulted (map-only-reading code would FAIL the empty-backlog artifact instead).
func TestScout_Classify_TypedStrategy_Identical(t *testing.T) {
	const artifact = "# Scout Report\n\nConvergence confirmed — no new backlog this cycle.\n"
	mapReq := core.PhaseRequest{Context: map[string]string{"strategy": "convergence-confirmation"}}
	typedReq := core.PhaseRequest{
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase:       "scout",
			CycleInputs: phaseio.NewCycleInputs(phaseio.CycleInputsInit{Strategy: "convergence-confirmation"}),
		}),
	}
	wantVerdict, _, _ := hooks{}.Classify(artifact, mapReq, core.BridgeResponse{})
	gotVerdict, _, _ := hooks{}.Classify(artifact, typedReq, core.BridgeResponse{})
	if wantVerdict != core.VerdictSKIPPED {
		t.Fatalf("precondition: map convergence+empty-backlog must be SKIPPED, got %q", wantVerdict)
	}
	if gotVerdict != wantVerdict {
		t.Errorf("typed-strategy verdict %q != map verdict %q (typed source not consulted in Classify)", gotVerdict, wantVerdict)
	}
}
