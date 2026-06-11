// cmd_cycle_failureadviser_test.go — R8.1 (concurrency-factory plan): the
// ADR-0044 Slice-6 "post-soak step" — wiring the LLM failure-advisor tail
// at the production composition root. The hook itself shipped enforce-gated
// and best-effort (core/failure_hook.go), so wiring is safe at any dial:
// below enforce it never dispatches. Without this wiring, flipping
// EVOLVE_PHASE_RECOVERY=enforce would silently skip the advise→promote path
// the flip exists to activate (cycle-270 forensics: the named post-soak
// step from the ADR-0044 implementation record).
package main

import (
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

	d := wireOrchestratorDeps(root, evolveDir)
	if !d.Orchestrator.FailureAdviserWired() {
		t.Fatal("RED (R8.1): production composition root does not wire the ADR-0044 failure-advisor tail — EVOLVE_PHASE_RECOVERY=enforce would silently skip advise→promote")
	}
}
