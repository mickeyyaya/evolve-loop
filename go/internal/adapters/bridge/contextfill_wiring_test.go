package bridge

// contextfill_wiring_test.go — composition-root WIRING proof for cycle-1444
// task `context-fill-warn-threshold`. Engine-side behaviour is proven in
// internal/bridge/contextfill_warn_test.go; this file proves the OTHER half —
// that the operator's policy.json value actually reaches the engine. Without
// it, ContextFillWarnPct is a Deps field only tests ever set (dead config).
//
// RED: Adapter does not resolve context_fill from policy.json and
// gobridge.Deps.ContextFillWarnPct does not exist — this file fails to COMPILE
// until Builder adds them (compile-fail = RED evidence).

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProductionDepsCarryContextFillThreshold drives the real production
// composition path (NewDefault → productionEngineDeps, the same deps every
// production launch is built from) against a policy.json that raises the
// threshold, and asserts the value lands in the engine Deps.
func TestProductionDepsCarryContextFillThreshold(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	policyJSON := `{"context_fill":{"warn_threshold_pct":42}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}

	deps := NewDefault(root).productionEngineDeps(map[string]string{"HOME": root})
	if deps.ContextFillWarnPct != 42 {
		t.Errorf("ContextFillWarnPct = %d, want 42 — the operator's context_fill block never reaches the engine (dead config)", deps.ContextFillWarnPct)
	}
}

// TestProductionDepsContextFillRejectsOutOfRange proves the root resolves
// through policy's validating resolver rather than reading the raw field: a
// nonsense operator value must arrive as the built-in 60, never as 900.
func TestProductionDepsContextFillRejectsOutOfRange(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"context_fill":{"warn_threshold_pct":900}}`), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}

	deps := NewDefault(root).productionEngineDeps(map[string]string{"HOME": root})
	if deps.ContextFillWarnPct != 60 {
		t.Errorf("ContextFillWarnPct = %d, want 60 — out-of-range operator input was passed through verbatim instead of resolved", deps.ContextFillWarnPct)
	}
}
