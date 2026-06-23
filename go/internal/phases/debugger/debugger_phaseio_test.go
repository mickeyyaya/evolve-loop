package debugger

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phaseio"
)

// ADR-0050 §3.10 Slice 2: debugger reads the ship-failure envelope from the typed
// ErrorContext at enforce (req.Input.Active()) and the legacy ship_error_* Context
// keys below it.

func TestDebugger_ComposePrompt_TypedEqualsMap(t *testing.T) {
	ctx := map[string]string{
		"ship_error_code":  "E_PUSH_NONFF",
		"ship_error_class": "transient",
		"ship_error_stage": "ship",
		"ship_error_debug": "remote moved",
	}
	mapReq := core.PhaseRequest{Context: ctx}
	typedReq := core.PhaseRequest{
		Context: ctx,
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase: "debugger",
			Error: &phaseio.ErrorContext{Code: "E_PUSH_NONFF", Class: "transient", Stage: "ship", Debug: "remote moved"},
		}),
	}
	want := hooks{}.ComposePrompt("BODY", mapReq)
	got := hooks{}.ComposePrompt("BODY", typedReq)
	if got != want {
		t.Errorf("typed envelope prompt != map prompt:\n typed=%q\n   map=%q", got, want)
	}
}

// At enforce the typed channel is consulted (no Context): a partial envelope renders
// only its present fields, matching the legacy "only render what is present".
func TestDebugger_ComposePrompt_EnforceReadsTypedPartial(t *testing.T) {
	req := core.PhaseRequest{
		Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{
			Phase: "debugger",
			Error: &phaseio.ErrorContext{Code: "E_X", Stage: "ship"},
		}),
	}
	got := hooks{}.ComposePrompt("BODY", req)
	if !strings.Contains(got, "- ship_error_code: E_X") {
		t.Errorf("code not read from typed envelope: %q", got)
	}
	if !strings.Contains(got, "- ship_error_stage: ship") {
		t.Errorf("stage not read from typed envelope: %q", got)
	}
	if strings.Contains(got, "- ship_error_class:") || strings.Contains(got, "- ship_error_debug:") {
		t.Errorf("empty typed fields must not render: %q", got)
	}
}

// An active envelope with NO upstream error renders an empty Ship Failure Envelope —
// byte-identical to a dispatch with no ship_error_* Context keys.
func TestDebugger_ComposePrompt_EnforceNoError_NoLines(t *testing.T) {
	req := core.PhaseRequest{Input: phaseio.NewPhaseInput(phaseio.PhaseInputInit{Phase: "debugger"})}
	mapReq := core.PhaseRequest{} // no ship_error_* keys
	got := hooks{}.ComposePrompt("BODY", req)
	want := hooks{}.ComposePrompt("BODY", mapReq)
	if got != want {
		t.Errorf("active-but-error-free envelope must equal the empty-map prompt:\n got=%q\nwant=%q", got, want)
	}
	if strings.Contains(got, "- ship_error_") {
		t.Errorf("empty ErrorContext must render no ship_error lines: %q", got)
	}
}
