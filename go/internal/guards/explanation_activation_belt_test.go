package guards

import (
	"os"
	"strings"
	"testing"
)

// explanationActivationBeltCallee is the fixed callee every pin below must
// call. It is a package-level const (not a literal inside the test body) so
// integrity_surface_explanation_callsites_test.go can read the same belief
// when it derives the call-site scanner's vocabulary — see the file comment
// there for why a fourth, re-typed copy of this string is the belief this
// hoist exists to avoid.
const explanationActivationBeltCallee = "explanationdocs.CrossCheckActivation"

// explanationActivationBeltPins is package-level (not a local literal) so
// integrity_surface_explanation_callsites_test.go can read the same file list
// when it derives the call-site scanner's vocabulary.
var explanationActivationBeltPins = []struct{ path, fn string }{
	{"../phases/ship/native_explanation_gate.go", "verifyNativeExplanation"},
	{"../phases/audit/audit.go", "verifyExplanationDocumentation"},
}

// TestExplanationActivationBelt_SharedByShipAndAudit pins the single-sourcing
// of the activation cross-check (architecture review 2026-09-01, HIGH):
// CycleBinding.ContractVersion==0 encodes BOTH "legacy cycle" and "a caller
// dropped the field", and only the host activation marker can tell them
// apart. Ship carried the belt inline; audit had NONE — version 0 silently
// disabled its explanation gate (Verify -> !active -> nil). Both gates must
// consult the host through explanationdocs.CrossCheckActivation, and the
// field-comparison belief must not live in a phase package.
func TestExplanationActivationBelt_SharedByShipAndAudit(t *testing.T) {
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
