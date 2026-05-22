// Package cycle48 ports the cycle-48 ACS predicates (4 bash files).
package cycle48

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC48_001_ExpectedShipShaAutoUpdate ports cycle-48/001.
func TestC48_001_ExpectedShipShaAutoUpdate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	ship := filepath.Join(root, "scripts", "lifecycle", "ship.sh")
	if !acsassert.FileExists(t, ship) {
		t.Skip("ship.sh missing — skip cycle-48-001")
	}
	if !acsassert.FileMatchesRegex(t, ship, `_repin_ship_sha.*post-cycle self-update`) {
		return
	}
}

// TestC48_002_MemoMaxTurns15 ports cycle-48/002.
func TestC48_002_MemoMaxTurns15(t *testing.T) {
	root := acsassert.RepoRoot(t)
	memo := filepath.Join(root, ".evolve", "profiles", "memo.json")
	if !acsassert.FileExists(t, memo) {
		t.Skip("memo.json missing — skip cycle-48-002")
	}
	if !acsassert.JSONFieldEquals(t, memo, "max_turns", float64(15)) {
		return
	}
}

// TestC48_003_AuditorStopCriterionHardGate ports cycle-48/003.
func TestC48_003_AuditorStopCriterionHardGate(t *testing.T) {
	root := acsassert.RepoRoot(t)
	auditor := filepath.Join(root, "agents", "evolve-auditor.md")
	if !acsassert.FileExists(t, auditor) {
		t.Skip("evolve-auditor.md missing — skip cycle-48-003")
	}
	if !acsassert.FileContains(t, auditor, "turn count > 30") {
		return
	}
}

// TestC48_004_IsolationBreachWiresJsonl ports cycle-48/004.
func TestC48_004_IsolationBreachWiresJsonl(t *testing.T) {
	root := acsassert.RepoRoot(t)
	gate := filepath.Join(root, "scripts", "lifecycle", "phase-gate.sh")
	if !acsassert.FileExists(t, gate) {
		t.Skip("phase-gate.sh missing — skip cycle-48-004")
	}
	if !acsassert.FileMatchesRegex(t, gate, `_append_abnormal_event.*builder-isolation-breach`) {
		return
	}
}
