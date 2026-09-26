package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWireOrchestrator_ThroughputRecorderWired(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}

	d := wireOrchestratorDeps(root, evolveDir, io.Discard)
	if !d.Orchestrator.ThroughputRecorderWired() {
		t.Fatal("RED (R9.1): production composition root does not wire the triage-throughput recorder — the R9.2 capacity clamp would never see observed throughput")
	}
}
