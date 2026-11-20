package policy_test

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFleetConfig_AdaptiveAbsentMeansCapEqualsSized(t *testing.T) {
	for name, raw := range map[string]string{
		"no fleet block":         `{}`,
		"no orchestrator block":  `{"fleet":{"count":12}}`,
		"no window block":        `{"fleet":{"count":12,"orchestrator":{"stage":"enforce"}}}`,
		"a window without adapt": `{"fleet":{"count":12,"orchestrator":{"window":{"start":8,"min":4}}}}`,
	} {
		p, err := policy.Parse([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := p.FleetConfig().Orchestrator.Adaptive; got != (policy.AdaptiveConfig{}) {
			t.Errorf("%s: Adaptive = %+v, want the zero value (not enabled)", name, got)
		}
	}
}

func TestFleetConfig_AdaptiveEmptyBlockEnablesTheShadowDefaults(t *testing.T) {
	p, err := policy.Parse([]byte(`{"fleet":{"count":12,"min_lanes":4,"orchestrator":{"window":{"start":8,"min":4,"adaptive":{}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := policy.AdaptiveConfig{Enabled: true, Stage: "shadow", LoadHigh: 1.5, LoadLow: 1.0, MemMinPct: 15, SwapMaxMB: 4096,
		DecreaseFactor: 0.5, Step: 1, CooldownS: 60, IncreaseCooldownS: 180, SampleS: 15, SmoothSamples: 3}
	got := p.FleetConfig().Orchestrator
	if got.Adaptive != want {
		t.Fatalf("Adaptive = %+v, want %+v", got.Adaptive, want)
	}
	if got.WindowStart != 8 || got.WindowMin != 4 {
		t.Errorf("window = %d to %d, want 8 to 4", got.WindowStart, got.WindowMin)
	}
}

func TestFleetConfig_AdaptiveKeysOverrideTheDefaults(t *testing.T) {
	raw := `{"fleet":{"count":12,"orchestrator":{"stage":"enforce","window":{"start":8,"adaptive":{"stage":"enforce","load_per_core_high":3,"load_per_core_low":2.5,
		"mem_available_min_pct":20,"swap_used_max_mb":1000,"step":2,"decrease_factor":0.75,"cooldown_s":30,"increase_cooldown_s":90,"sample_s":5,"smooth_samples":4}}}}}`
	p, err := policy.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	want := policy.AdaptiveConfig{Enabled: true, Stage: "enforce", LoadHigh: 3, LoadLow: 2.5, MemMinPct: 20, SwapMaxMB: 1000,
		DecreaseFactor: 0.75, Step: 2, CooldownS: 30, IncreaseCooldownS: 90, SampleS: 5, SmoothSamples: 4}
	if got := p.FleetConfig().Orchestrator.Adaptive; got != want {
		t.Fatalf("Adaptive = %+v, want %+v", got, want)
	}
}

func TestParse_FleetAdaptiveStrictAndValidated(t *testing.T) {
	cases := map[string]string{
		"an unknown key":                `{"stage":"shadow","speed":1}`,
		"an unknown stage":              `{"stage":"live"}`,
		"low equal to high":             `{"load_per_core_low":1.5,"load_per_core_high":1.5}`,
		"low above the default high":    `{"load_per_core_low":2}`,
		"high under the default low":    `{"load_per_core_high":0.5}`,
		"a negative high":               `{"load_per_core_high":-1}`,
		"a negative low":                `{"load_per_core_low":-1}`,
		"a negative memory floor":       `{"mem_available_min_pct":-1}`,
		"a memory floor of 100":         `{"mem_available_min_pct":100}`,
		"a negative swap max":           `{"swap_used_max_mb":-1}`,
		"a negative step":               `{"step":-1}`,
		"a decrease factor of 1":        `{"decrease_factor":1}`,
		"a negative decrease factor":    `{"decrease_factor":-0.5}`,
		"a negative cooldown":           `{"cooldown_s":-1}`,
		"a negative increase cooldown":  `{"increase_cooldown_s":-1}`,
		"a negative sample interval":    `{"sample_s":-1}`,
		"a negative smoothing":          `{"smooth_samples":-1}`,
		"a stage that is not a string":  `{"stage":4}`,
		"a block that is not an object": `4`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := policy.Parse([]byte(`{"fleet":{"count":12,"orchestrator":{"window":{"adaptive":` + raw + `}}}}`))
			if err == nil {
				t.Fatal("want a refusal")
			}
		})
	}
}

func TestParse_FleetAdaptiveRefusalsWrapTheAdaptiveError(t *testing.T) {
	for _, raw := range []string{`{"stage":"live"}`, `{"load_per_core_low":2}`, `{"step":-1}`} {
		_, err := policy.Parse([]byte(`{"fleet":{"count":12,"orchestrator":{"window":{"adaptive":` + raw + `}}}}`))
		if !errors.Is(err, policy.ErrFleetAdaptive) {
			t.Errorf("%s: err = %v, want ErrFleetAdaptive", raw, err)
		}
	}
}

func TestParse_WindowStartAboveCountIsStillRefusedWithAdaptive(t *testing.T) {
	if _, err := policy.Parse([]byte(`{"fleet":{"count":6,"orchestrator":{"window":{"start":8,"adaptive":{}}}}}`)); err == nil {
		t.Fatal("window.start 8 above fleet.count 6: want a refusal")
	}
}

func TestFleetConfig_SizedWidthIsTheSmallerOfCountAndConcurrencyAtLeastOne(t *testing.T) {
	cases := map[string]struct {
		count, concurrency, want int
	}{
		"a cap under the count": {16, 6, 6},
		"a cap over the count":  {8, 10, 8},
		"no cap":                {8, 0, 8},
		"nothing set":           {0, 0, 1},
	}
	for name, tc := range cases {
		if got := (policy.FleetConfig{Count: tc.count, Concurrency: tc.concurrency}).SizedWidth(); got != tc.want {
			t.Errorf("%s: SizedWidth = %d, want %d", name, got, tc.want)
		}
	}
}

func TestParse_AdaptiveEnforceNeedsTheOrchestratorAtEnforce(t *testing.T) {
	for _, orch := range []string{`"stage":"shadow",`, ``} {
		raw := `{"fleet":{"count":12,"orchestrator":{` + orch + `"window":{"adaptive":{"stage":"enforce"}}}}}`
		if _, err := policy.Parse([]byte(raw)); !errors.Is(err, policy.ErrFleetAdaptive) {
			t.Errorf("%s: err = %v, want ErrFleetAdaptive: the pool reads the window only at orchestrator stage enforce", raw, err)
		}
	}
	if _, err := policy.Parse([]byte(`{"fleet":{"count":12,"orchestrator":{"stage":"enforce","window":{"adaptive":{"stage":"enforce"}}}}}`)); err != nil {
		t.Fatalf("orchestrator enforce with adaptive enforce: err = %v, want nil", err)
	}
}

func TestAdaptiveStage_ShadowAndEnforceAreTheOnlyAcceptedValues(t *testing.T) {
	if policy.AdaptiveStageShadow != "shadow" || policy.AdaptiveStageEnforce != "enforce" {
		t.Fatalf("stage constants = %q, %q, want the policy.json spellings", policy.AdaptiveStageShadow, policy.AdaptiveStageEnforce)
	}
	for stage, wantErr := range map[policy.AdaptiveStage]bool{policy.AdaptiveStageShadow: false, policy.AdaptiveStageEnforce: false, "off": true} {
		raw := `{"fleet":{"orchestrator":{"stage":"enforce","window":{"adaptive":{"stage":"` + string(stage) + `"}}}}}`
		_, err := policy.Parse([]byte(raw))
		if (err != nil) != wantErr || wantErr && !errors.Is(err, policy.ErrFleetAdaptive) {
			t.Errorf("stage %q: err = %v, want refusal %v wrapping ErrFleetAdaptive", stage, err, wantErr)
		}
	}
}

func TestFleetAdaptivePolicy_StageDefaultsToShadowWhenOmitted(t *testing.T) {
	p, err := policy.Parse([]byte(`{"fleet":{"count":12,"orchestrator":{"window":{"adaptive":{}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var block *policy.FleetAdaptivePolicy = p.Fleet.Orchestrator.Window.Adaptive
	if block == nil || block.Stage != "" {
		t.Fatalf("adaptive block = %+v, want present with the stage left empty", block)
	}
	if got := p.FleetConfig().Orchestrator.Adaptive.Stage; got != "shadow" {
		t.Errorf("resolved stage = %q, want shadow", got)
	}
}
