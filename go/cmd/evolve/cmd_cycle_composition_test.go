// cmd_cycle_composition_test.go — cycle-804 TDD contract (inbox weight
// 0.98, wire-rung0-composition-writer-into-fleet-rebase).
//
// Mirrors cmd_cycle_failureadviser_test.go's pattern exactly: the RUNG 0
// composition-verdict writer (cycle-786) and the core seam
// (composition_carryforward.go, cycle-801) are fully built, but
// grepping go/cmd/evolve/*.go for WithCompositionSnapshot/
// WithCompositionGateRunner/WithCompositionVerdictWriter finds zero call
// sites — the composition root never binds them, so
// compositionCarryForward's nil-guard always trips and every clean fleet
// rebase falls through to a full re-audit, the exact behavior cycle-786+801
// were built to eliminate (scout-report.md cycle 804).
//
// This is a REAL (non-fake) test: it drives the actual production
// composition root (wireOrchestratorDeps, cmd_cycle.go) with a real
// temp-dir project root, not an injected fake — the same pattern
// TestWireOrchestrator_FailureAdviserWired already uses to pin the failure-
// advisor tail's wiring.
//
// RED today: wireOrchestratorDeps binds none of the three composition
// closures, so d.Orchestrator.CompositionFastPathWired() returns false.
package main

import (
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

	d := wireOrchestratorDeps(root, evolveDir)
	if !d.Orchestrator.CompositionFastPathWired() {
		t.Fatal("RED (cycle-804): production composition root (wireOrchestratorDeps) does not wire the RUNG 0 composition-verdict fast path (WithCompositionSnapshot/WithCompositionGateRunner/WithCompositionVerdictWriter) — every clean fleet rebase falls through to a full re-audit instead of carrying the audit verdict forward")
	}
}
