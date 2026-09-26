package policy

import (
	"encoding/json"
	"testing"
)

func TestGatesConfig_ManifestGate_WrongJSONTypeErrors(t *testing.T) {
	var p Policy
	err := json.Unmarshal([]byte(`{"gates":{"manifest_gate":123}}`), &p)
	if err == nil {
		t.Fatalf("manifest_gate:123 (wrong JSON type) must fail to unmarshal, got nil error, GatesConfig=%+v", p.GatesConfig())
	}
}

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
