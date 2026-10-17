package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFleetConfig_TheOrchestratorDefaultsToShadow(t *testing.T) {
	for name, tc := range map[string]struct {
		p           policy.Policy
		runMaxLanes int
	}{
		"no fleet block":        {policy.Policy{}, 4},
		"no orchestrator block": {policy.Policy{Fleet: &policy.FleetPolicy{Count: 4}}, 16},
	} {
		want := policy.OrchestratorConfig{Stage: "shadow", ScanK: 32, HubMinImporters: 120,
			DeadlineFloorMinutes: 60, StallMinutes: 30, RunMaxLanes: tc.runMaxLanes}
		if got := tc.p.FleetConfig().Orchestrator; got != want {
			t.Errorf("%s: Orchestrator = %+v, want %+v (run_max_lanes is 4 x count)", name, got, want)
		}
	}
}

func TestFleetConfig_TheOrchestratorKeysOverrideTheDefaults(t *testing.T) {
	p, err := policy.Parse([]byte(`{"fleet":{"count":4,"orchestrator":{"stage":"enforce","scan_k":8,"hub_min_importers":90,"deadline_floor_minutes":45,"stall_minutes":10,"run_max_lanes":6}}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := policy.OrchestratorConfig{Stage: "enforce", ScanK: 8, HubMinImporters: 90, DeadlineFloorMinutes: 45, StallMinutes: 10, RunMaxLanes: 6}
	if got := p.FleetConfig().Orchestrator; got != want {
		t.Fatalf("Orchestrator = %+v, want %+v", got, want)
	}
	p, err = policy.Parse([]byte(`{"fleet":{"orchestrator":{"stage":"shadow"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.FleetConfig().Orchestrator; got.ScanK != 32 || got.HubMinImporters != 120 || got.DeadlineFloorMinutes != 60 || got.StallMinutes != 30 || got.RunMaxLanes != 4 {
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
		"a negative deadline":  {`{"deadline_floor_minutes":-1}`, "deadline_floor_minutes"},
		"a negative stall":     {`{"stall_minutes":-5}`, "stall_minutes"},
		"a negative run limit": {`{"run_max_lanes":-2}`, "run_max_lanes"},
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
	if got := p.FleetConfig().Orchestrator; got != (policy.OrchestratorConfig{Stage: "enforce", ScanK: 32, HubMinImporters: 120, DeadlineFloorMinutes: 60, StallMinutes: 30, RunMaxLanes: 4}) {
		t.Errorf("Orchestrator = %+v, want enforce with the default scan_k and hub floor", got)
	}
}

func TestFleetConfig_PoolSchedulingAtEnforceWarnsThatTheOrchestratorAlwaysPools(t *testing.T) {
	p, err := policy.Parse([]byte(`{"fleet":{"count":4,"scheduling":"pool","orchestrator":{"stage":"enforce"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	warned := false
	for _, w := range p.FleetConfig().Warnings {
		warned = warned || (strings.Contains(w, "fleet.scheduling") && strings.Contains(w, "enforce"))
	}
	if !warned {
		t.Errorf("warnings = %v, want one that says scheduling is moot at enforce", p.FleetConfig().Warnings)
	}
	shadow, err := policy.Parse([]byte(`{"fleet":{"count":4,"scheduling":"pool"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if w := shadow.FleetConfig().Warnings; len(w) != 0 {
		t.Errorf("warnings at shadow = %v, want none: scheduling still selects the path there", w)
	}
}
