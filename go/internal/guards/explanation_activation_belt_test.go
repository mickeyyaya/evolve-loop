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
}

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
