package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFleetConfig_TheAdmissionRuleDefaultsToShadow(t *testing.T) {
	for name, raw := range map[string]string{
		"no fleet block":              `{}`,
		"no orchestrator block":       `{"fleet":{"count":4}}`,
		"an orchestrator without key": `{"fleet":{"orchestrator":{"stage":"enforce"}}}`,
		"an empty admission key":      `{"fleet":{"orchestrator":{"admission":""}}}`,
	} {
		p, err := policy.Parse([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := p.FleetConfig().Orchestrator.Admission; got != "shadow" {
			t.Errorf("%s: Admission = %q, want shadow", name, got)
		}
	}
}

func TestFleetConfig_TheAdmissionKeyIsIndependentOfTheOrchestratorStage(t *testing.T) {
	for _, tc := range []struct{ raw, stage, admission string }{
		{`{"fleet":{"orchestrator":{"stage":"shadow","admission":"enforce"}}}`, "shadow", "enforce"},
		{`{"fleet":{"orchestrator":{"stage":"enforce","admission":"shadow"}}}`, "enforce", "shadow"},
		{`{"fleet":{"orchestrator":{"admission":"enforce"}}}`, "shadow", "enforce"},
	} {
		p, err := policy.Parse([]byte(tc.raw))
		if err != nil {
			t.Fatalf("%s: %v", tc.raw, err)
		}
		if got := p.FleetConfig().Orchestrator; got.Stage != tc.stage || got.Admission != tc.admission {
			t.Errorf("%s: Stage = %q, Admission = %q, want %q and %q", tc.raw, got.Stage, got.Admission, tc.stage, tc.admission)
		}
	}
}

func TestParse_RefusesAnUnknownAdmissionStage(t *testing.T) {
	for name, tc := range map[string]struct{ raw, want string }{
		"a typo":          {`{"admission":"enfroce"}`, `"enfroce" is not "shadow" or "enforce"`},
		"the word on":     {`{"admission":"on"}`, `"on" is not "shadow" or "enforce"`},
		"a capital stage": {`{"admission":"Enforce"}`, `"Enforce" is not "shadow" or "enforce"`},
		"a number":        {`{"admission":1}`, "admission"},
	} {
		_, err := policy.Parse([]byte(`{"fleet":{"orchestrator":` + tc.raw + `}}`))
		if err == nil || !strings.Contains(err.Error(), "fleet.orchestrator") || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want a fleet.orchestrator refusal that says %s", name, err, tc.want)
		}
	}
}

func TestFleetConfig_ATypedOrchestratorBlockKeepsItsAdmissionStage(t *testing.T) {
	p := policy.Policy{Fleet: &policy.FleetPolicy{Orchestrator: &policy.FleetOrchestratorPolicy{Admission: "enforce"}}}
	if got := p.FleetConfig().Orchestrator.Admission; got != "enforce" {
		t.Errorf("Admission = %q, want enforce", got)
	}
}
