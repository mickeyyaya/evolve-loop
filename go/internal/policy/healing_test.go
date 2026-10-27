package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestHealingDetector_DefaultsToShadowWithTheCompiledWindow(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{}`, `{"healing":{}}`, `{"healing":{"detector":{}}}`} {
		p, err := policy.Parse([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		got := p.HealingDetector()
		if got.Stage != policy.HealingStageShadow || got.WindowHours != 72 || got.Cycles != 60 {
			t.Errorf("%s: %+v, want shadow, 72 hours and 60 cycles", body, got)
		}
	}
}

func TestHealingDetector_ReadsTheStageTheWindowAndTheCycles(t *testing.T) {
	t.Parallel()
	p, err := policy.Parse([]byte(`{"healing":{"detector":{"stage":"off","window_hours":24,"cycles":10}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.HealingDetector(); got.Stage != policy.HealingStageOff || got.WindowHours != 24 || got.Cycles != 10 {
		t.Fatalf("%+v", got)
	}
}

func TestPolicyParse_RefusesABadHealingDetectorBlock(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`{"healing":{"detector":{"stage":"enforce"}}}`: "selfheal-action-catalog-validator-budgets",
		`{"healing":{"detector":{"stage":"loud"}}}`:    `"loud"`,
		`{"healing":{"detector":{"window_hours":-1}}}`: "window_hours",
		`{"healing":{"detector":{"cycles":-2}}}`:       "cycles",
		`{"healing":{"detector":{"stgae":"shadow"}}}`:  "healing.detector.stgae",
	}
	for body, want := range cases {
		if _, err := policy.Parse([]byte(body)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to name %s", body, err, want)
		}
	}
}

func TestHealingPolicy_TheTypedBlockCarriesTheDetectorStage(t *testing.T) {
	t.Parallel()
	p := policy.Policy{Healing: &policy.HealingPolicy{Detector: &policy.HealingDetectorPolicy{Stage: policy.HealingStageOff}}}
	if got := p.HealingDetector(); got.Stage != policy.HealingStageOff || got.WindowHours != 72 {
		t.Fatalf("%+v", got)
	}
}
