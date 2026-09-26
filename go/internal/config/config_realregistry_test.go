package config

import (
	"path/filepath"
	"testing"
)

func TestLoad_RealRegistry(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json")
	cfg, ws := Load(path, map[string]string{})

	if cfg.Stage != StageAdvisory {
		t.Errorf("Stage=%v, want StageAdvisory (registry dynamic_routing=advisory)", cfg.Stage)
	}
	if cfg.Mode != ModeDynamicLLM {
		t.Errorf("Mode=%v, want ModeDynamicLLM (registry routing_mode=llm)", cfg.Mode)
	}
	wantSpine := []string{"scout", "triage", "build", "audit", "ship"}
	if len(cfg.Mandatory) != len(wantSpine) {
		t.Fatalf("Mandatory=%v, want %v", cfg.Mandatory, wantSpine)
	}
	for i, p := range wantSpine {
		if cfg.Mandatory[i] != p {
			t.Errorf("Mandatory[%d]=%s, want %s", i, cfg.Mandatory[i], p)
		}
	}
	if cfg.MaxInsertions != 6 {
		t.Errorf("MaxInsertions=%d, want 6 (registry max_optional_insertions, raised in cycle 217)", cfg.MaxInsertions)
	}
	if r, ok := cfg.Conditional["tdd"]; !ok || r.Field != "cycle_size" || r.Op != "!=" || r.Value != "trivial" {
		t.Errorf("Conditional[tdd]=%+v (ok=%v), want cycle_size != trivial", r, ok)
	}
	tb, ok := cfg.Triggers["tester"]
	if !ok || len(tb.InsertWhen) != 2 {
		t.Fatalf("Triggers[tester]=%+v (ok=%v), want 2 insert_when clauses", tb, ok)
	}
	if tb.InsertWhen[0].Field != "build.acs_red" || tb.InsertWhen[0].Op != "gt" {
		t.Errorf("tester insert_when[0]=%+v, want build.acs_red gt", tb.InsertWhen[0])
	}
	for phase, hints := range map[string]int{"scout": 2, "plan-review": 1, "architecture-design": 1, "retrospective": 1} {
		blk, ok := cfg.Triggers[phase]
		if !ok || len(blk.RubricHint) != hints {
			t.Errorf("Triggers[%s].RubricHint=%v (ok=%v), want %d hints", phase, blk.RubricHint, ok, hints)
		}
	}
	for _, phase := range []string{"build", "audit", "tdd"} {
		if len(cfg.Triggers[phase].RubricHint) != 0 {
			t.Errorf("Triggers[%s].RubricHint=%v, want none (derivable beliefs live in structured routing data)", phase, cfg.Triggers[phase].RubricHint)
		}
	}
	if hasWarning(ws, "weak-spine") {
		t.Errorf("unexpected weak-spine warning for the default spine: %v", ws)
	}
}
