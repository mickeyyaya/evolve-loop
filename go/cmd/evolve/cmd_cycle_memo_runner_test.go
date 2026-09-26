package main

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// repoRootForMemoRunnerTest locates the repo root three levels up and skips
// when the real memo overlay fixture is absent.
func repoRootForMemoRunnerTest(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate repo root")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	overlay := filepath.Join(root, ".evolve", "phases", "memo", "phase.json")
	if _, err := os.Stat(overlay); err != nil {
		t.Skipf("real memo overlay fixture not found at %s: %v", overlay, err)
	}
	registry := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	if _, err := os.Stat(registry); err != nil {
		t.Skipf("built-in phase registry not found at %s: %v", registry, err)
	}
	return root
}

// A temp evolveDir keeps real .evolve state untouched, while projectRoot stays
// the real repo so the real registry, overlay and policy pins are exercised.
func TestWireOrchestrator_MemoRunnerRegistered(t *testing.T) {
	root := repoRootForMemoRunnerTest(t)
	evolveDir := t.TempDir()

	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	if !d.Orchestrator.HasRunner(core.Phase("memo")) {
		t.Fatal(`RED (cycle-563): wireOrchestratorDeps did not register a PhaseRunner for "memo" even though the built-in registry marks it optional:true and the real .evolve/phases/memo/phase.json overlay activates it — the runner-registration loop's phasespec.ValidateUserSpec(s) call (cmd_cycle.go:406) rejects the single-word name that phasespec.ValidateUserSpecWithCatalog (used three lines above, cmd_cycle.go:399, for routing) correctly exempts. The router plans memo but nothing ever launches it.`)
	}
}
