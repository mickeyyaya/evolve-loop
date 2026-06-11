// cmd_cycle_triagecap_test.go — R9.1 (concurrency-factory plan): the
// production composition root must wire the triage-throughput recorder so
// shipped coverage cycles feed the rolling window the R9.2 capacity clamp
// reads. Without this wiring the clamp would run forever on the cycle-281
// seed instead of the observed throughput.
package main

import (
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

	d := wireOrchestratorDeps(root, evolveDir)
	if !d.Orchestrator.ThroughputRecorderWired() {
		t.Fatal("RED (R9.1): production composition root does not wire the triage-throughput recorder — the R9.2 capacity clamp would never see observed throughput")
	}
}
