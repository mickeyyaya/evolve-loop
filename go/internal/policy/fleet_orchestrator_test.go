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
			DeadlineFloorMinutes: 60, StallMinutes: 30, RunMaxLanes: tc.runMaxLanes, WindowStart: 4, WindowMin: 4}
		if got := tc.p.FleetConfig().Orchestrator; got != want {
			t.Errorf("%s: Orchestrator = %+v, want %+v (run_max_lanes is 4 x count)", name, got, want)
		}
	}
}

func TestFleetConfig_TheOrchestratorKeysOverrideTheDefaults(t *testing.T) {
	p, err := policy.Parse([]byte(`{"fleet":{"count":8,"orchestrator":{"stage":"enforce","scan_k":8,"hub_min_importers":90,"deadline_floor_minutes":45,"stall_minutes":10,"run_max_lanes":6,"window":{"start":6,"min":3}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := policy.OrchestratorConfig{Stage: "enforce", ScanK: 8, HubMinImporters: 90, DeadlineFloorMinutes: 45, StallMinutes: 10, RunMaxLanes: 6, WindowStart: 6, WindowMin: 3}
	if got := p.FleetConfig().Orchestrator; got != want {
		t.Fatalf("Orchestrator = %+v, want %+v", got, want)
	}
	p, err = policy.Parse([]byte(`{"fleet":{"orchestrator":{"stage":"shadow"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.FleetConfig().Orchestrator; got.ScanK != 32 || got.HubMinImporters != 120 || got.DeadlineFloorMinutes != 60 || got.StallMinutes != 30 || got.RunMaxLanes != 4 || got.WindowStart != 4 || got.WindowMin != 4 {
		t.Errorf("Orchestrator = %+v, want the unset keys at their defaults", got)
	}
}

func TestParse_RefusesABadOrchestratorBlock(t *testing.T) {
	cases := map[string]struct{ raw, want string }{
		"an unknown stage":     {`{"stage":"enfroce"}`, `"enfroce"`},
		"an unknown key":       {`{"stage":"shadow","width":4}`, `"width"`},
		"a negative scan_k":    {`{"scan_k":-1}`, "scan_k"},
		"a negative hub floor": {`{"hub_min_importers":-3}`, "hub_min_importers"},
		"a malformed block":    {`{"scan_k":"many"}`, "scan_k"},
		"a negative deadline":  {`{"deadline_floor_minutes":-1}`, "deadline_floor_minutes"},
		"a negative stall":     {`{"stall_minutes":-5}`, "stall_minutes"},
		"a negative run limit": {`{"run_max_lanes":-2}`, "run_max_lanes"},

		"a zero window start":                 {`{"window":{"start":0}}`, "window.start"},
		"a negative window start":             {`{"window":{"start":-2}}`, "window.start"},
		"a zero window min":                   {`{"window":{"min":0}}`, "window.min"},
		"a negative window min":               {`{"window":{"min":-1}}`, "window.min"},
		"a window min above the start":        {`{"window":{"start":3,"min":5}}`, "window.min"},
		"a window min above the default":      {`{"window":{"min":6}}`, "window.min"},
		"an unknown window key":               {`{"window":{"wide":4}}`, `"wide"`},
		"a window that is not a block":        {`{"window":4}`, "window"},
		"a window start that is not a number": {`{"window":{"start":"four"}}`, "start"},
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
	if got := p.FleetConfig().Orchestrator; got != (policy.OrchestratorConfig{Stage: "enforce", ScanK: 32, HubMinImporters: 120, DeadlineFloorMinutes: 60, StallMinutes: 30, RunMaxLanes: 4, WindowStart: 4, WindowMin: 4}) {
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

func TestParse_RefusesAWindowStartAboveTheFleetCount(t *testing.T) {
	cases := map[string]string{
		"a start above the count":           `{"fleet":{"count":4,"orchestrator":{"window":{"start":5}}}}`,
		"a start when the count is not set": `{"fleet":{"orchestrator":{"window":{"start":2}}}}`,
		"a count that follows the block":    `{"fleet":{"orchestrator":{"window":{"start":6,"min":4}},"count":5}}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := policy.Parse([]byte(raw))
			if err == nil || !strings.Contains(err.Error(), "fleet.orchestrator") || !strings.Contains(err.Error(), "window.start") || !strings.Contains(err.Error(), "fleet.count") {
				t.Fatalf("err = %v, want a fleet.orchestrator refusal that names window.start and fleet.count", err)
			}
		})
	}
}

func TestParse_AcceptsEveryWindowThatKeepsMinUnderStartUnderCount(t *testing.T) {
	cases := map[string]struct {
		raw        string
		start, min int
	}{
		"start and min equal the count": {`{"fleet":{"count":8,"orchestrator":{"window":{"start":8,"min":8}}}}`, 8, 8},
		"one lane":                      {`{"fleet":{"count":4,"orchestrator":{"window":{"start":1,"min":1}}}}`, 1, 1},
		"the default count of 1":        {`{"fleet":{"orchestrator":{"window":{"start":1}}}}`, 1, 1},
		"a count under the default":     {`{"fleet":{"count":3}}`, 4, 4},
		"a null window":                 {`{"fleet":{"count":8,"orchestrator":{"window":null}}}`, 4, 4},
		"null keys":                     {`{"fleet":{"count":8,"orchestrator":{"window":{"start":null,"min":null}}}}`, 4, 4},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := policy.Parse([]byte(tc.raw))
			if err != nil {
				t.Fatalf("Parse refused %s: %v", tc.raw, err)
			}
			if got := p.FleetConfig().Orchestrator; got.WindowStart != tc.start || got.WindowMin != tc.min {
				t.Errorf("window = %d to %d, want %d to %d", got.WindowStart, got.WindowMin, tc.start, tc.min)
			}
		})
	}
}

func TestFleetConfig_AnUnsetWindowMinFollowsAnExplicitStartDown(t *testing.T) {
	cases := map[string]struct {
		raw        string
		start, min int
	}{
		"a start under the default min": {`{"fleet":{"count":8,"orchestrator":{"window":{"start":2}}}}`, 2, 2},
		"a start over the default min":  {`{"fleet":{"count":8,"orchestrator":{"window":{"start":6}}}}`, 6, 4},
		"a min under the default start": {`{"fleet":{"count":8,"orchestrator":{"window":{"min":3}}}}`, 4, 3},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := policy.Parse([]byte(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			if got := p.FleetConfig().Orchestrator; got.WindowStart != tc.start || got.WindowMin != tc.min {
				t.Errorf("window = %d to %d, want %d to %d", got.WindowStart, got.WindowMin, tc.start, tc.min)
			}
		})
	}
}

func TestFleetConfig_ATypedWindowBlockReachesTheConfig(t *testing.T) {
	start, floor := 5, 2
	p := policy.Policy{Fleet: &policy.FleetPolicy{Count: 8, Orchestrator: &policy.FleetOrchestratorPolicy{
		Window: &policy.FleetWindowPolicy{Start: &start, Min: &floor}}}}
	if got := p.FleetConfig().Orchestrator; got.WindowStart != 5 || got.WindowMin != 2 {
		t.Errorf("window = %d to %d, want 5 to 2", got.WindowStart, got.WindowMin)
	}
}
