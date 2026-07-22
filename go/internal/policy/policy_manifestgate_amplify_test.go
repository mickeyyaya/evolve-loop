package policy

import (
	"encoding/json"
	"testing"
)

// TestGatesConfig_ManifestGate_WrongJSONTypeErrors is the fail-loudly axis:
// an operator hand-editing .evolve/policy.json who types a number instead of
// a string for manifest_gate must get a decode error, not a silently
// defaulted/garbage value.
func TestGatesConfig_ManifestGate_WrongJSONTypeErrors(t *testing.T) {
	var p Policy
	err := json.Unmarshal([]byte(`{"gates":{"manifest_gate":123}}`), &p)
	if err == nil {
		t.Fatalf("manifest_gate:123 (wrong JSON type) must fail to unmarshal, got nil error, GatesConfig=%+v", p.GatesConfig())
	}
}

// TestGatesConfig_ManifestGate_CaseAndWhitespacePreservedVerbatim pins that
// policy.go performs NO normalization on the raw string — a load-bearing
// assumption for reconcileManifest's strict equality check against the exact
// literal "enforce". If policy.go ever started trimming/lowercasing, this
// test documents the change is deliberate rather than an accidental drift.
func TestGatesConfig_ManifestGate_CaseAndWhitespacePreservedVerbatim(t *testing.T) {
	for _, tc := range []struct{ name, raw, want string }{
		{"wrong case Enforce", `{"gates":{"manifest_gate":"Enforce"}}`, "Enforce"},
		{"padded value", `{"gates":{"manifest_gate":" enforce "}}`, " enforce "},
		{"uppercase", `{"gates":{"manifest_gate":"ENFORCE"}}`, "ENFORCE"},
	} {
		var p Policy
		if err := json.Unmarshal([]byte(tc.raw), &p); err != nil {
			t.Fatalf("%s: unmarshal: %v", tc.name, err)
		}
		if got := p.GatesConfig().ManifestGate; got != tc.want {
			t.Errorf("%s: ManifestGate = %q, want %q (verbatim, no normalization)", tc.name, got, tc.want)
		}
	}
}

// TestGatesConfig_ManifestGate_MultiGateInteraction sets manifest_gate
// alongside several other confirmed gate keys in ONE JSON blob, each to a
// distinct non-default value. The pre-existing DoesNotDisturbOtherGates test
// only sets manifest_gate and checks the OTHER gates stay at their defaults;
// this is the inverse combination — several explicitly set together — which
// catches field-tag collisions or struct-layout drift that per-field
// isolation cannot.
func TestGatesConfig_ManifestGate_MultiGateInteraction(t *testing.T) {
	raw := `{"gates":{
		"contract_gate":"off",
		"eval_gate":"shadow",
		"report_size_gate":"enforce",
		"topn_gate":"shadow",
		"manifest_gate":"enforce"
	}}`
	var p Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := p.GatesConfig()
	for _, tc := range []struct{ name, have, want string }{
		{"ContractGate", got.ContractGate, "off"},
		{"EvalGate", got.EvalGate, "shadow"},
		{"ReportSizeGate", got.ReportSizeGate, "enforce"},
		{"TopNGate", got.TopNGate, "shadow"},
		{"ManifestGate", got.ManifestGate, "enforce"},
	} {
		if tc.have != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.have, tc.want)
		}
	}
}

// TestGatesConfig_ManifestGate_Idempotent proves GatesConfig() is a pure
// projection: calling it twice on the same Policy returns equal results, and
// the resolved GatesConfig is a value the caller owns (mutating one call's
// result must not leak into the next call's result via shared state).
func TestGatesConfig_ManifestGate_Idempotent(t *testing.T) {
	var p Policy
	if err := json.Unmarshal([]byte(`{"gates":{"manifest_gate":"enforce"}}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	first := p.GatesConfig()
	first.ManifestGate = "mutated"

	second := p.GatesConfig()
	if second.ManifestGate != "enforce" {
		t.Errorf("second GatesConfig().ManifestGate = %q, want %q (must not alias the first call's result)", second.ManifestGate, "enforce")
	}
}
