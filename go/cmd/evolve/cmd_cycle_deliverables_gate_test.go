// cmd_cycle_deliverables_gate_test.go — ADR-0100 wiring proof: the production
// composition root installs a reviewer that verifies every agent-owed
// declared output. The gate lives inside the contract gate, which cmd_cycle
// mounts only when cfg.ContractGate != off — the compiled default is enforce,
// so a default-policy project must have it. Without this proof a future
// reordering of the reviewer chain (or a policy default flip) could drop the
// gate silently while every unit test stayed green.
package main

import (
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
	d := wireOrchestratorDeps(root, evolveDir)
	if !d.Orchestrator.DeclaredDeliverablesGateWired() {
		t.Fatal("the production composition root does not wire the declared-deliverables gate (ADR-0100) — a phase that omits a declared output would proceed exactly as before")
	}
}
