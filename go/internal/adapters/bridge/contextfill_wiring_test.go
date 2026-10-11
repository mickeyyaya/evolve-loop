package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProductionDepsCarryContextFillThreshold(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	policyJSON := `{"context_fill":{"warn_threshold_pct":42}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}

	deps := NewDefault(root, nil).productionEngineDeps(map[string]string{"HOME": root})
	if deps.ContextFillWarnPct != 42 {
		t.Errorf("ContextFillWarnPct = %d, want 42 — the operator's context_fill block never reaches the engine (dead config)", deps.ContextFillWarnPct)
	}
}

func TestProductionDepsContextFillRejectsOutOfRange(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"context_fill":{"warn_threshold_pct":900}}`), 0o644); err != nil {
		t.Fatalf("write policy.json: %v", err)
	}

	deps := NewDefault(root, nil).productionEngineDeps(map[string]string{"HOME": root})
	if deps.ContextFillWarnPct != 60 {
		t.Errorf("ContextFillWarnPct = %d, want 60 — out-of-range operator input was passed through verbatim instead of resolved", deps.ContextFillWarnPct)
	}
}

func TestProductionDepsCarryThePaneCaptureCap(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"attempt_postmortem":{"max_pane_capture_bytes":2048}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := NewDefault(root, nil).productionEngineDeps(nil).PaneCaptureMaxBytes; got != 2048 {
		t.Fatalf("PaneCaptureMaxBytes = %d, want the policy's 2048", got)
	}
	if got := NewDefault(t.TempDir(), nil).productionEngineDeps(nil).PaneCaptureMaxBytes; got != 64<<10 {
		t.Fatalf("PaneCaptureMaxBytes with no policy = %d, want the compiled 64 KiB", got)
	}
}
