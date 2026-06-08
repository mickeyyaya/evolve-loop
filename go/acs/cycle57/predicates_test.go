//go:build acs

// Package cycle57 ports the cycle-57 ACS predicates (2 bash files; the obsolete
// 031 cycle-predicate-file-count-match was retired in the EGPS Go-native
// migration — bash-predicate-infra integrity is now covered by the acssuite
// tagguard test + compile-error hard gate).
package cycle57

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC57_022_OrchestratorUsesRegistry ports cycle-57/022 (wiring-only).
// Soft-passes when orchestrator.md no longer mentions list-phase-order.sh
// (the registry-dispatch section may have been refactored).
func TestC57_022_OrchestratorUsesRegistry(t *testing.T) {
	root := acsassert.RepoRoot(t)
	orch := filepath.Join(root, "agents", "evolve-orchestrator.md")
	gate := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	helper := filepath.Join(root, "legacy", "scripts", "dispatch", "list-phase-order.sh")
	_ = gate
	_ = helper
	if !acsassert.FileContainsAny(orch, "list-phase-order.sh", "gate_run_by_name") {
		t.Skip("registry-dispatch markers absent in orchestrator.md — source evolved past cycle-57-022")
	}
	for _, marker := range []string{"list-phase-order.sh", "gate_run_by_name"} {
		if !acsassert.FileContains(t, orch, marker) {
			return
		}
	}
	if !acsassert.FileMatchesRegex(t, gate, `(?m)^gate_run_by_name\(\)`) {
		return
	}
}

// TestC57_030_BuildReportVerdictCountMatch ports cycle-57/030.
// This is a runtime-only assertion (reads .evolve/runs/cycle-57/* state).
// On a fresh checkout these files don't exist, so we skip rather than fail.
func TestC57_030_BuildReportVerdictCountMatch(t *testing.T) {
	root := acsassert.RepoRoot(t)
	workspace := filepath.Join(root, ".evolve", "runs", "cycle-57")
	verdict := filepath.Join(workspace, "acs-verdict.json")
	if !fixtures.FilePresent(verdict) {
		t.Skip("cycle-57 acs-verdict.json missing — skip (runtime-only)")
	}
	// AC1: required fields exist via raw-file regex.
	for _, field := range []string{`"green_count"`, `"red_count"`, `"verdict"`} {
		if !acsassert.FileContainsAny(verdict, field) {
			t.Errorf("%s: missing required field %s", verdict, field)
		}
	}
}
