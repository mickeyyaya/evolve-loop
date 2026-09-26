package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// wiredWithRealPhases wires the production root over a project that has the
// repo's personas and phase registry, so the builtin fallback runners register.
func wiredWithRealPhases(t *testing.T) orchDeps {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROMPTS_DIR", repoRoot)
	root := t.TempDir()
	registry, err := os.ReadFile(filepath.Join(repoRoot, "docs", "architecture", "phase-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Join(root, ".evolve"), filepath.Join(root, "docs", "architecture")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "architecture", "phase-registry.json"), registry, 0o644); err != nil {
		t.Fatal(err)
	}
	return wireOrchestratorDeps(root, filepath.Join(root, ".evolve"), io.Discard)
}

func TestWireOrchestrator_HostEffectsWired(t *testing.T) {
	d := wiredWithRealPhases(t)
	if !d.Orchestrator.HostEffectsWired() {
		t.Fatal("the production root must bind the host's effect performer")
	}
	for phase, r := range d.Runners {
		if phase == core.PhaseShip || phase == core.PhaseRetro {
			continue
		}
		w, ok := r.(interface{ HostEffectsWired() bool })
		if !ok || !w.HostEffectsWired() {
			t.Errorf("phase %s: runner %T must perform host effects before its verdict engine judges", phase, r)
		}
	}
	root := t.TempDir()
	if wireSimulateOrchestrator(root, filepath.Join(root, ".evolve"), io.Discard).Orchestrator.HostEffectsWired() {
		t.Fatal("--simulate must never bind it: a simulated cycle would move real inbox items")
	}
}
