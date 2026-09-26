package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWireOrchestrator_DeclaredDeliverablesGateWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	if !d.Orchestrator.DeclaredDeliverablesGateWired() {
		t.Fatal("the production composition root does not wire the declared-deliverables gate (ADR-0100) — a phase that omits a declared output would proceed exactly as before")
	}
}
