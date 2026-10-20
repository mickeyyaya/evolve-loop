package guards

import (
	"os"
	"strings"
	"testing"
)

const explanationActivationBeltCallee = "explanationdocs.CrossCheckActivation"

var explanationActivationBeltPins = []struct{ path, fn string }{
	{"../phases/ship/native_explanation_gate.go", "verifyNativeExplanation"},
	{"../phases/audit/audit.go", "verifyExplanationDocumentation"},
	{"../phases/retro/retro.go", "refreshExplanationHandoff"},
	{"../core/build_explanation_handoff.go", "requireResumeExplanationIdentity"},
}

var sameWorkspacePathPins = []struct{ path, fn string }{
	{"../explanationdocs/contract_state.go", "ActivationForCycle"},
	{"../explanationdocs/contract_state.go", "RecoverRebaseSplit"},
	{"../explanationdocs/explanationdocs.go", "resolveActivation"},
}

func TestExplanationActivationBelt_SharedByEveryHostConsult(t *testing.T) {
	for _, pin := range explanationActivationBeltPins {
		t.Run(pin.path, func(t *testing.T) {
			if !functionCalls(t, pin.path, pin.fn, explanationActivationBeltCallee) {
				t.Fatalf("%s must consult the host via explanationdocs.CrossCheckActivation", pin.fn)
			}
			body, err := os.ReadFile(pin.path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), "does not match host activation") {
				t.Fatalf("%s restates the identity-comparison belief; it must live only in explanationdocs", pin.path)
			}
		})
	}
}

func TestSameWorkspacePath_IsTheOneWorkspaceRuleInsideExplanationdocs(t *testing.T) {
	for _, pin := range sameWorkspacePathPins {
		t.Run(pin.fn, func(t *testing.T) {
			if !functionCalls(t, pin.path, pin.fn, "sameWorkspacePath") {
				t.Fatalf("%s must compare workspaces through sameWorkspacePath", pin.fn)
			}
		})
	}
}
