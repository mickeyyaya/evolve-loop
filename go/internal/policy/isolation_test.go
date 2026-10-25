package policy_test

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestIsolationEnvStage_ReadsIsolationI2StageAndDefaultsToEmpty(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		``:                 "",
		`{"isolation":{}}`: "",
		`{"isolation":{"i2":{"stage":"enforce"}}}`: "enforce",
		`{"isolation":{"i2":{"stage":"off"}}}`:     "off",
	}
	for body, want := range cases {
		var p policy.Policy
		if body != "" {
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
		}
		if got := p.IsolationEnvStage(); got != want {
			t.Errorf("%s: IsolationEnvStage = %q, want %q", body, got, want)
		}
	}
}

func TestIsolationPolicy_TheTypedBlockCarriesTheI2Stage(t *testing.T) {
	t.Parallel()
	p := policy.Policy{Isolation: &policy.IsolationPolicy{I2: policy.IsolationInvariantPolicy{Stage: "shadow"}}}
	if got := p.IsolationEnvStage(); got != "shadow" {
		t.Fatalf("IsolationEnvStage = %q, want shadow", got)
	}
}
