package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFleetConfig_TheOrchestratorDefaultsToShadow(t *testing.T) {
	want := policy.OrchestratorConfig{Stage: "shadow", ScanK: 32, HubMinImporters: 120}
	for name, p := range map[string]policy.Policy{
		"no fleet block":        {},
		"no orchestrator block": {Fleet: &policy.FleetPolicy{Count: 4}},
	} {
		if got := p.FleetConfig().Orchestrator; got != want {
			t.Errorf("%s: Orchestrator = %+v, want %+v", name, got, want)
		}
	}
}

func TestFleetConfig_TheOrchestratorKeysOverrideTheDefaults(t *testing.T) {
	p, err := policy.Parse([]byte(`{"fleet":{"count":4,"orchestrator":{"stage":"enforce","scan_k":8,"hub_min_importers":90}}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := policy.OrchestratorConfig{Stage: "enforce", ScanK: 8, HubMinImporters: 90}
	if got := p.FleetConfig().Orchestrator; got != want {
		t.Fatalf("Orchestrator = %+v, want %+v", got, want)
	}
	p, err = policy.Parse([]byte(`{"fleet":{"orchestrator":{"stage":"shadow"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.FleetConfig().Orchestrator; got.ScanK != 32 || got.HubMinImporters != 120 {
		t.Errorf("Orchestrator = %+v, want the unset keys at their defaults", got)
	}
}

func TestParse_RefusesABadOrchestratorBlock(t *testing.T) {
	cases := map[string]struct{ raw, want string }{
		"an unknown stage":     {`{"stage":"enfroce"}`, `"enfroce"`},
		"an unknown key":       {`{"stage":"shadow","window":4}`, `"window"`},
		"a negative scan_k":    {`{"scan_k":-1}`, "scan_k"},
		"a negative hub floor": {`{"hub_min_importers":-3}`, "hub_min_importers"},
		"a malformed block":    {`{"scan_k":"many"}`, "scan_k"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := policy.Parse([]byte(`{"fleet":{"orchestrator":` + tc.raw + `}}`))
			if err == nil || !strings.Contains(err.Error(), "fleet.orchestrator") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want a fleet.orchestrator refusal that names %s", err, tc.want)
			}
		})
	}
}

func TestFleetConfig_ATypedOrchestratorBlockKeepsItsStage(t *testing.T) {
	p := policy.Policy{Fleet: &policy.FleetPolicy{Orchestrator: &policy.FleetOrchestratorPolicy{Stage: "enforce"}}}
	if got := p.FleetConfig().Orchestrator; got != (policy.OrchestratorConfig{Stage: "enforce", ScanK: 32, HubMinImporters: 120}) {
		t.Errorf("Orchestrator = %+v, want enforce with the default scan_k and hub floor", got)
	}
}
