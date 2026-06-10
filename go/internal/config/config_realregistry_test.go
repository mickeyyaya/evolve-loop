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

	// Default posture since 2026-06-06 (retro migration steps 1-3 landed):
	// dynamic_routing=advisory ⇒ the advisor drives the optional surface while
	// the spine stays static and ClampPlanToFloor protects the ship guarantee.
	// EVOLVE_DYNAMIC_ROUTING=off remains the operator escape hatch.
	if cfg.Stage != StageAdvisory {
		t.Errorf("Stage=%v, want StageAdvisory (registry dynamic_routing=advisory)", cfg.Stage)
	}
	if cfg.Mode != ModeDynamicLLM {
		t.Errorf("Mode=%v, want ModeDynamicLLM (registry routing_mode=llm)", cfg.Mode)
	}
	// triage joined the mandatory spine 2026-06-10 (cycles 263/264 post-mortem):
	// the advisory router skipped triage on the premise "scout picks ONE item"
	// while the scout authored THREE tasks — with the scope-clamp gone, the
	// builder under-delivered and the all-or-nothing audit failed the cycle.
	// Dispatch-level mandatory restores the pre-advisory equilibrium; triage's
	// own runner-level auto-skip (EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL) still
	// short-circuits genuinely trivial cycles, so the router may not remove
	// the clamp but the clamp stays cheap.
	wantSpine := []string{"scout", "triage", "build", "audit", "ship"}
	if len(cfg.Mandatory) != len(wantSpine) {
		t.Fatalf("Mandatory=%v, want %v", cfg.Mandatory, wantSpine)
	}
	for i, p := range wantSpine {
		if cfg.Mandatory[i] != p {
			t.Errorf("Mandatory[%d]=%s, want %s", i, cfg.Mandatory[i], p)
		}
	}
	// 6 since cycle 217 (micro-phase catalog §4.2): the refactor recipe needs
	// six optional insertions; registry config.max_optional_insertions raised 4→6.
	if cfg.MaxInsertions != 6 {
		t.Errorf("MaxInsertions=%d, want 6 (registry max_optional_insertions, raised in cycle 217)", cfg.MaxInsertions)
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
	// Phase 4b: judgment-only rubric guidance is registry data
	// (routing.rubric_hint); lines derivable from insert_when /
	// conditional_mandatory are NOT restated here — the renderer projects
	// them from the structured source.
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
	// A spine with audit+ship must not raise weak-spine.
	if hasWarning(ws, "weak-spine") {
		t.Errorf("unexpected weak-spine warning for the default spine: %v", ws)
	}
}
