package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_BindsRoutingTypes(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "phase-registry.json")
	reg := `{
      "schema_version": 3,
      "config": {
        "conditional_mandatory": {"tdd": "cycle_size != trivial"}
      },
      "phases": [
        {
          "name": "tester",
          "optional": true,
          "routing": {
            "insert_when": [{"field": "build.acs_red", "op": "gt", "value": 0}],
            "skip_when":   [{"field": "scout.goal_type", "op": "eq", "value": "growth"}]
          }
        }
      ]
    }`
	if err := os.WriteFile(regPath, []byte(reg), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}

	// Explicit declarations name RoutingConfig and Warning for apicover.
	var cfg RoutingConfig
	var warnings []Warning
	cfg, warnings = Load(regPath, map[string]string{})
	if len(warnings) > 0 {
		t.Logf("Load returned non-fatal warnings on the clean fixture: %v", warnings)
	}

	wantRule := CondRule{Field: "cycle_size", Op: "!=", Value: "trivial"}
	if got := cfg.Conditional["tdd"]; got.Field != wantRule.Field || got.Op != wantRule.Op || got.Value != wantRule.Value || len(got.And) != 0 {
		t.Errorf("Conditional[tdd] = %+v, want %+v", got, wantRule)
	}

	block, ok := cfg.Triggers["tester"]
	if !ok {
		t.Fatalf("Triggers missing tester routing block")
	}
	wantBlock := RoutingBlock{
		InsertWhen: []Condition{{Field: "build.acs_red", Op: "gt", Value: float64(0)}},
		SkipWhen:   []Condition{{Field: "scout.goal_type", Op: "eq", Value: "growth"}},
	}
	if len(block.InsertWhen) != 1 || len(block.SkipWhen) != 1 {
		t.Fatalf("RoutingBlock = %+v, want 1 insert_when + 1 skip_when", block)
	}
	if block.InsertWhen[0] != wantBlock.InsertWhen[0] {
		t.Errorf("InsertWhen[0] = %+v, want %+v", block.InsertWhen[0], wantBlock.InsertWhen[0])
	}
	if block.SkipWhen[0] != wantBlock.SkipWhen[0] {
		t.Errorf("SkipWhen[0] = %+v, want %+v", block.SkipWhen[0], wantBlock.SkipWhen[0])
	}

	if len(cfg.Order) != 1 || cfg.Order[0] != "tester" {
		t.Errorf("Order = %v, want [tester]", cfg.Order)
	}
}

func TestModelRouting_ParsesAndStringifies(t *testing.T) {
	cases := []struct {
		value string
		want  ModelRouting
		str   string
	}{
		{"static", ModelRoutingStatic, "static"},
		{"advisory", ModelRoutingAdvisory, "advisory"},
		{"auto", ModelRoutingAuto, "auto"},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		regPath := filepath.Join(dir, "phase-registry.json")
		reg := `{"schema_version":3,"config":{"model_routing":"` + tc.value + `"},"phases":[]}`
		if err := os.WriteFile(regPath, []byte(reg), 0o644); err != nil {
			t.Fatalf("write registry: %v", err)
		}
		cfg, _ := Load(regPath, map[string]string{})
		if cfg.ModelRouting != tc.want {
			t.Errorf("model_routing=%q => ModelRouting = %v, want %v", tc.value, cfg.ModelRouting, tc.want)
		}
		if got := cfg.ModelRouting.String(); got != tc.str {
			t.Errorf("ModelRouting(%v).String() = %q, want %q", cfg.ModelRouting, got, tc.str)
		}
	}
}
