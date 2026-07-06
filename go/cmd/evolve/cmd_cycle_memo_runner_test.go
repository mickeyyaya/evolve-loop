// cmd_cycle_memo_runner_test.go — cycle-563 fix-memo-phase-dispatch, criterion 1
// (the regression test that would have caught the silent drop). Root cause
// (fault-localization-report.md, confidence 0.97): the runner-registration
// loop in wireOrchestratorDeps (cmd_cycle.go:406) validates each discovered
// user-overlay PhaseSpec with the non-catalog-aware phasespec.ValidateUserSpec,
// which re-imposes the two-tier single-word naming floor that
// phasespec.ApplyUserRouting (three lines above, cmd_cycle.go:399) already
// exempted for "memo" via ValidateUserSpecWithCatalog (memo is a reserved
// single-word name, but the built-in registry marks it optional:true, which is
// exactly the exemption ValidateUserSpecWithCatalog grants). So the ROUTER
// legitimately plans "memo" after "ship" (cycle-561 routing-decision-12.json),
// but no PhaseRunner is ever registered for it — cyclerun_dispatch.go's
// missing-runner escape hatch then WARNs and silently advances past memo
// without ever running it or recording it in completed_phases.
//
// This test wires the ACTUAL composition root (wireOrchestratorDeps) against
// the real repo's real built-in registry (docs/architecture/phase-registry.json,
// which declares memo optional:true) and the real user overlay
// (.evolve/phases/memo/phase.json, a bare single-word name) and asserts a
// PhaseRunner was registered for "memo" — i.e. the dispatcher would actually
// attempt to launch it, not just that Route() nominates its name.
package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// repoRootForMemoRunnerTest locates the repo root from this file's location
// (go/cmd/evolve/ → three levels up) and skips if the real memo overlay
// fixture this test depends on is absent (e.g. a partial/vendored checkout).
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

// TestWireOrchestrator_MemoRunnerRegistered is the RED regression test for the
// silent drop: the composition root must register a PhaseRunner for "memo"
// whenever the built-in registry marks it optional AND a (validly-shaped,
// single-word) user overlay activates it — mirroring cycle-561's live state.
// Uses a fresh temp evolveDir (storage/ledger) so this never touches real
// .evolve state; the projectRoot stays the real repo so the real registry +
// overlay + policy pins are exercised, not synthetic fixtures.
func TestWireOrchestrator_MemoRunnerRegistered(t *testing.T) {
	root := repoRootForMemoRunnerTest(t)
	evolveDir := t.TempDir()

	d := wireOrchestratorDeps(root, evolveDir)
	if !d.Orchestrator.HasRunner(core.Phase("memo")) {
		t.Fatal(`RED (cycle-563): wireOrchestratorDeps did not register a PhaseRunner for "memo" even though the built-in registry marks it optional:true and the real .evolve/phases/memo/phase.json overlay activates it — the runner-registration loop's phasespec.ValidateUserSpec(s) call (cmd_cycle.go:406) rejects the single-word name that phasespec.ValidateUserSpecWithCatalog (used three lines above, cmd_cycle.go:399, for routing) correctly exempts. The router plans memo but nothing ever launches it.`)
	}
}
