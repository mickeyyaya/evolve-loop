package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_BindsRoutingTypes binds all four EVOLVE dynamic-routing config
// structs to their real producer, Load, which parses a phase-registry.json.
// Each type is asserted through the field mapping Load is responsible for:
//
//   - RoutingConfig — the immutable typed object Load returns (var cfg ... = Load).
//   - CondRule      — parsed from the registry's conditional_mandatory expr
//     "field<op>value"; Load splits it into {Field, Op, Value}.
//   - RoutingBlock  — the per-phase "routing" block; InsertWhen/SkipWhen carry
//     []Condition and are copied verbatim into cfg.Triggers[phase].
//   - Condition     — one insert_when/skip_when clause; Field/Op/Value must
//     survive the JSON round-trip intact (the router later evaluates them).
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

	// Load returns (RoutingConfig, []Warning) — warnings are non-fatal, not an
	// error; capture them rather than silently discarding, and surface any.
	var cfg RoutingConfig
	var warnings []Warning
	cfg, warnings = Load(regPath, map[string]string{})
	if len(warnings) > 0 {
		t.Logf("Load returned non-fatal warnings on the clean fixture: %v", warnings)
	}

	// CondRule: the conditional_mandatory expr must parse into the exact triple.
	wantRule := CondRule{Field: "cycle_size", Op: "!=", Value: "trivial"}
	if got := cfg.Conditional["tdd"]; got != wantRule {
		t.Errorf("Conditional[tdd] = %+v, want %+v", got, wantRule)
	}

	// RoutingBlock + Condition: the per-phase routing block must round-trip
	// field-for-field from the registry JSON into cfg.Triggers["tester"].
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

	// RoutingConfig: the loaded object must reflect the registry's phase order.
	if len(cfg.Order) != 1 || cfg.Order[0] != "tester" {
		t.Errorf("Order = %v, want [tester]", cfg.Order)
	}
}
