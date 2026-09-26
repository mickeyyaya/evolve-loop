package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWireOrchestrator_CompositionFastPathWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	if !d.Orchestrator.CompositionFastPathWired() {
		t.Fatal("RED (cycle-804): production composition root (wireOrchestratorDeps) does not wire the RUNG 0 composition-verdict fast path (WithCompositionSnapshot/WithCompositionGateRunner/WithCompositionVerdictWriter) — every clean fleet rebase falls through to a full re-audit instead of carrying the audit verdict forward")
	}
}
