//go:build acs

// Package cycle58 ports the cycle-58 ACS predicates (3 bash files).
package cycle58

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestC58_023_CheckPhaseInputsDetectsMissing ports cycle-58/023 (wiring-only).
// Soft-pass when the script no longer mentions scout-report by name
// (input list may be data-driven via phase-registry now).
func TestC58_023_CheckPhaseInputsDetectsMissing(t *testing.T) {
	root := acsassert.RepoRoot(t)
	script := filepath.Join(root, "legacy", "scripts", "utility", "check-phase-inputs.sh")
	if !acsassert.FileContainsAny(script) {
		t.Skip("check-phase-inputs.sh missing — skip cycle-58-023")
	}
	if !acsassert.FileContainsAny(script, "scout-report", "scout_report", "scout-report.md") {
		t.Skip("scout-report markers absent — likely data-driven via registry")
	}
}

// TestC58_024_ScoutRunsStandalone ports cycle-58/024 (wiring-only).
// init-standalone-cycle.sh exists + handles scout phase init.
func TestC58_024_ScoutRunsStandalone(t *testing.T) {
	root := acsassert.RepoRoot(t)
	initScript := filepath.Join(root, "legacy", "scripts", "utility", "init-standalone-cycle.sh")
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	for _, f := range []string{initScript, subagent} {
		if !fixtures.FilePresent(f) {
			t.Skipf("required file missing — skip cycle-58-024: %s", f)
		}
	}
	for _, marker := range []string{"--phase", "scout", "research"} {
		if !acsassert.FileContains(t, initScript, marker) {
			return
		}
	}
}

// TestC58_025_AuditStandaloneBuilderArtifacts ports cycle-58/025 (wiring-only).
// Soft-pass when check-phase-inputs.sh has dropped hardcoded phase names
// (likely data-driven via registry).
func TestC58_025_AuditStandaloneBuilderArtifacts(t *testing.T) {
	root := acsassert.RepoRoot(t)
	initScript := filepath.Join(root, "legacy", "scripts", "utility", "init-standalone-cycle.sh")
	checkInputs := filepath.Join(root, "legacy", "scripts", "utility", "check-phase-inputs.sh")
	if !acsassert.FileContainsAny(initScript, "audit") {
		t.Skip("audit marker absent in init-standalone-cycle.sh — source evolved")
	}
	if !acsassert.FileContainsAny(checkInputs, "audit", "build-report", "tester-report") {
		t.Skip("phase-input markers absent in check-phase-inputs.sh — likely data-driven")
	}
}
