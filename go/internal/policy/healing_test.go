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
		`{"healing":{"detector":{"stage":"enforce"}}}`: "selfheal-repair-cycle-path",
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

func TestHealingOrchestrator_DefaultsToShadowWithTwoIncidentsAPass(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{}`, `{"healing":{}}`, `{"healing":{"orchestrator":{}}}`} {
		p, err := policy.Parse([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if got := p.HealingOrchestrator(); got.Stage != policy.HealingStageShadow || got.MaxPerPass != 2 || got.RetryAfterHours != 6 {
			t.Errorf("%s: %+v, want shadow, 2 incidents a pass and a retry after 6 hours", body, got)
		}
	}
	p, err := policy.Parse([]byte(`{"healing":{"orchestrator":{"stage":"off","max_per_pass":5,"retry_after_hours":1}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.HealingOrchestrator(); got.Stage != policy.HealingStageOff || got.MaxPerPass != 5 || got.RetryAfterHours != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestPolicyParse_RefusesABadHealingOrchestratorBlock(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`{"healing":{"orchestrator":{"stage":"enforce"}}}`:      "selfheal-repair-cycle-path",
		`{"healing":{"orchestrator":{"stage":"loud"}}}`:         `orchestrator.stage "loud"`,
		`{"healing":{"orchestrator":{"max_per_pass":-1}}}`:      "max_per_pass",
		`{"healing":{"orchestrator":{"retry_after_hours":-1}}}`: "retry_after_hours",
		`{"healing":{"orchestrator":{"stgae":"shadow"}}}`:       "healing.orchestrator.stgae",
	}
	for body, want := range cases {
		if _, err := policy.Parse([]byte(body)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to name %s", body, err, want)
		}
	}
}

func TestHealingPolicy_TheTypedBlockCarriesTheOrchestratorStage(t *testing.T) {
	t.Parallel()
	p := policy.Policy{Healing: &policy.HealingPolicy{Orchestrator: &policy.HealingOrchestratorPolicy{Stage: policy.HealingStageOff}}}
	if got := p.HealingOrchestrator(); got.Stage != policy.HealingStageOff || got.MaxPerPass != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestHealingValidator_DefaultsToShadowWithTheBudgetsOfTheSpec(t *testing.T) {
	t.Parallel()
	want := policy.HealingValidatorPolicy{Stage: policy.HealingStageShadow, WindowHours: 24, T1Repairs: 3, T2Repairs: 6, BreakerFailures: 2, BreakerOpenHours: 24}
	for _, body := range []string{`{}`, `{"healing":{}}`, `{"healing":{"validator":{}}}`} {
		p, err := policy.Parse([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if got := p.HealingValidator(); got != want {
			t.Errorf("%s: %+v, want %+v", body, got, want)
		}
	}
	p, err := policy.Parse([]byte(`{"healing":{"validator":{"stage":"off","window_hours":12,"t1_repairs":1,"t2_repairs":2,"breaker_failures":3,"breaker_open_hours":4}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.HealingValidator(); got != (policy.HealingValidatorPolicy{Stage: policy.HealingStageOff, WindowHours: 12, T1Repairs: 1, T2Repairs: 2, BreakerFailures: 3, BreakerOpenHours: 4}) {
		t.Fatalf("%+v", got)
	}
}

func TestPolicyParse_RefusesABadHealingValidatorBlock(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`{"healing":{"validator":{"stage":"enforce"}}}`:       "selfheal-repair-cycle-path",
		`{"healing":{"validator":{"stage":"loud"}}}`:          `validator.stage "loud"`,
		`{"healing":{"validator":{"window_hours":-1}}}`:       "window_hours",
		`{"healing":{"validator":{"t1_repairs":-1}}}`:         "t1_repairs",
		`{"healing":{"validator":{"t2_repairs":-1}}}`:         "t2_repairs",
		`{"healing":{"validator":{"breaker_failures":-1}}}`:   "breaker_failures",
		`{"healing":{"validator":{"breaker_open_hours":-1}}}`: "breaker_open_hours",
		`{"healing":{"validator":{"t3_repairs":1}}}`:          "healing.validator.t3_repairs",
	}
	for body, want := range cases {
		if _, err := policy.Parse([]byte(body)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to name %s", body, err, want)
		}
	}
}
