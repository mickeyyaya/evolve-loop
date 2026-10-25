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

func TestIsolationExecStage_ReadsIsolationI3StageApartFromI2(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		``:                 "",
		`{"isolation":{}}`: "",
		`{"isolation":{"i2":{"stage":"enforce"}}}`: "",
		`{"isolation":{"i3":{"stage":"enforce"}}}`: "enforce",
	}
	for body, want := range cases {
		var p policy.Policy
		if body != "" {
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
		}
		if got := p.IsolationExecStage(); got != want {
			t.Errorf("%s: IsolationExecStage = %q, want %q", body, got, want)
		}
	}
}

func TestIsolationFlowStage_ReadsIsolationI8StageAndDefaultsToEmpty(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		``:                 "",
		`{"isolation":{}}`: "",
		`{"isolation":{"i2":{"stage":"enforce"}}}`: "",
		`{"isolation":{"i8":{"stage":"enforce"}}}`: "enforce",
	}
	for body, want := range cases {
		var p policy.Policy
		if body != "" {
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
		}
		if got := p.IsolationFlowStage(); got != want {
			t.Errorf("%s: IsolationFlowStage = %q, want %q", body, got, want)
		}
	}
}

func TestIsolationReadStageAndLineageMax_ReadTheI5BlockApartFromI2AndI3(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		stage string
		max   int
	}{
		``: {},
		`{"isolation":{"i3":{"stage":"enforce"}}}`:                 {},
		`{"isolation":{"i5":{"stage":"enforce"},"lineage_max":3}}`: {stage: "enforce", max: 3},
		`{"isolation":{"i5":{"stage":"shadow"}}}`:                  {stage: "shadow"},
	}
	for body, want := range cases {
		var p policy.Policy
		if body != "" {
			if err := json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
		}
		if got := p.IsolationReadStage(); got != want.stage {
			t.Errorf("%s: IsolationReadStage = %q, want %q", body, got, want.stage)
		}
		if got := p.IsolationLineageMax(); got != want.max {
			t.Errorf("%s: IsolationLineageMax = %d, want %d", body, got, want.max)
		}
	}
}
