package config

import (
	"path/filepath"
	"testing"
)

// TestLoad_RealRegistry parses the actual repo phase-registry.json (the file
// the composition root loads at runtime). It guards against schema typos in
// the config{} block and the tester routing trigger added for dynamic routing.
func TestLoad_RealRegistry(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json")
	cfg, ws := Load(path, map[string]string{})

	// Default posture: dynamic_routing=0 ⇒ Stage:Off (static SM drives).
	if cfg.Stage != StageOff {
		t.Errorf("Stage=%v, want StageOff (registry dynamic_routing=0)", cfg.Stage)
	}
	if cfg.Mode != ModeDynamicLLM {
		t.Errorf("Mode=%v, want ModeDynamicLLM (registry routing_mode=llm)", cfg.Mode)
	}
	wantSpine := []string{"scout", "build", "audit", "ship"}
	if len(cfg.Mandatory) != len(wantSpine) {
		t.Fatalf("Mandatory=%v, want %v", cfg.Mandatory, wantSpine)
	}
	for i, p := range wantSpine {
		if cfg.Mandatory[i] != p {
			t.Errorf("Mandatory[%d]=%s, want %s", i, cfg.Mandatory[i], p)
		}
	}
	if cfg.MaxInsertions != 4 {
		t.Errorf("MaxInsertions=%d, want 4", cfg.MaxInsertions)
	}
	if r, ok := cfg.Conditional["tdd"]; !ok || r.Field != "cycle_size" || r.Op != "!=" || r.Value != "trivial" {
		t.Errorf("Conditional[tdd]=%+v (ok=%v), want cycle_size != trivial", r, ok)
	}
	// The tester routing trigger must parse into Triggers.
	tb, ok := cfg.Triggers["tester"]
	if !ok || len(tb.InsertWhen) != 2 {
		t.Fatalf("Triggers[tester]=%+v (ok=%v), want 2 insert_when clauses", tb, ok)
	}
	if tb.InsertWhen[0].Field != "build.acs_red" || tb.InsertWhen[0].Op != "gt" {
		t.Errorf("tester insert_when[0]=%+v, want build.acs_red gt", tb.InsertWhen[0])
	}
	// A spine with audit+ship must not raise weak-spine.
	if hasWarning(ws, "weak-spine") {
		t.Errorf("unexpected weak-spine warning for the default spine: %v", ws)
	}
}
