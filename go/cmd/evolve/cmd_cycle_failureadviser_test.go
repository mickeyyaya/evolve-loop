package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWireOrchestrator_FailureAdviserWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	if !d.Orchestrator.FailureAdviserWired() {
		t.Fatal("RED (R8.1): production composition root does not wire the ADR-0044 failure-advisor tail — EVOLVE_PHASE_RECOVERY=enforce would silently skip advise→promote")
	}
}
