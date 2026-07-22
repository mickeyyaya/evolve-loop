package policy

import (
	"encoding/json"
	"testing"
)

// TestGatesConfig_ManifestGateDefaultsToShadow pins the behavior-preserving
// default for the new dial (cycle-1064, task manifest-gate-policy-wiring). The
// manifest gate is shadow-first, exactly like ReportSizeGate: observed before it
// can block a cycle. An absent `gates` block and a present-but-silent one must
// both resolve to "shadow".
func TestGatesConfig_ManifestGateDefaultsToShadow(t *testing.T) {
	for name, p := range map[string]Policy{
		"no gates block":    {},
		"empty gates block": {Gates: &GatesPolicy{}},
		"other gates set":   {Gates: &GatesPolicy{EvalGate: "off"}},
	} {
		if got := p.GatesConfig().ManifestGate; got != "shadow" {
			t.Errorf("%s: ManifestGate = %q, want %q", name, got, "shadow")
		}
	}
}

// TestGatesConfig_ManifestGateFromJSON is the crux: an operator writing
// `gates.manifest_gate` in .evolve/policy.json must reach the resolved config.
// Parsed from JSON (not a struct literal) so the `manifest_gate` TAG is exercised
// — a field with the wrong/missing tag is unreachable from policy.json, which is
// exactly today's inert state.
func TestGatesConfig_ManifestGateFromJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"enforce", `{"gates":{"manifest_gate":"enforce"}}`, "enforce"},
		{"explicit shadow", `{"gates":{"manifest_gate":"shadow"}}`, "shadow"},
		{"off", `{"gates":{"manifest_gate":"off"}}`, "off"},
		{"empty string falls back to default", `{"gates":{"manifest_gate":""}}`, "shadow"},
		{"unrelated gate only", `{"gates":{"topn_gate":"off"}}`, "shadow"},
	} {
		var p Policy
		if err := json.Unmarshal([]byte(tc.raw), &p); err != nil {
			t.Fatalf("%s: unmarshal: %v", tc.name, err)
		}
		if got := p.GatesConfig().ManifestGate; got != tc.want {
			t.Errorf("%s: ManifestGate = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestGatesConfig_ManifestGateDoesNotDisturbOtherGates is the regression axis:
// adding a seventh gate must leave the six existing defaults and their
// resolution logic byte-identical.
func TestGatesConfig_ManifestGateDoesNotDisturbOtherGates(t *testing.T) {
	var p Policy
	if err := json.Unmarshal([]byte(`{"gates":{"manifest_gate":"enforce"}}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := p.GatesConfig()
	for _, tc := range []struct{ name, have, want string }{
		{"ContractGate", got.ContractGate, "enforce"},
		{"EvalGate", got.EvalGate, "enforce"},
		{"TriageCapGate", got.TriageCapGate, "enforce"},
		{"ReviewGate", got.ReviewGate, "off"},
		{"ReportSizeGate", got.ReportSizeGate, "shadow"},
		{"TopNGate", got.TopNGate, "enforce"},
	} {
		if tc.have != tc.want {
			t.Errorf("%s = %q, want %q (unchanged by the new gate)", tc.name, tc.have, tc.want)
		}
	}
}
