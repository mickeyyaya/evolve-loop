//go:build acs

// Package cycle3 materializes the cycle-3 acceptance criteria for the
// committed top_n task:
//
//   - retire-bypass-ship-verify — remove EVOLVE_BYPASS_SHIP_VERIFY from the
//     flag registry, delete the WARN-bridge block in ship/native.go, strip the
//     vestigial env-var from rollback.go, and regenerate control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	retire-bypass-ship-verify:
//	  AC1   EVOLVE_BYPASS_SHIP_VERIFY absent from flagregistry.Lookup     → C3_001 (behavioral)
//	  AC2   registry row count == 263 (264 - 1)                           → C3_002 (behavioral, count assertion)
//	  NEG1  bridge block removed from ship/native.go                      → C3_003 (absence check, waiver)
//	  NEG2  vestigial env-var removed from rollback.go                    → C3_004 (absence check, waiver)
//	  AC6   EVOLVE_BYPASS_SHIP_VERIFY absent from control-flags.md        → C3_005 (absence check, waiver)
//	  AC3   ship test suite still green after bridge removal + test update → C3_006 (subprocess, pre-existing GREEN)
//	  AC4   rollback test suite still green                                → C3_007 (subprocess, pre-existing GREEN)
//
// Floor binding (R9.3): predicates only for committed top_n task
// (retire-bypass-ship-verify). Deferred tasks (EVOLVE_DISABLE_AUTO_RETROSPECTIVE,
// internal classification) get zero predicates.
package cycle3

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const targetFlag = "EVOLVE_BYPASS_SHIP_VERIFY"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// TestC3_001_BypassShipVerifyAbsentFromRegistry verifies that the
// EVOLVE_BYPASS_SHIP_VERIFY flag is no longer registered after Builder
// removes its row from registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() directly — the production SSOT
// function. A source edit alone cannot satisfy this; the row must be
// physically absent for Lookup to return ok=false.
//
// RED: flagregistry.Lookup currently returns (flag, true) because the row
// exists at registry_table.go:66.
func TestC3_001_BypassShipVerifyAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup(targetFlag); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated flag still registered.\n"+
			"Builder must remove this row from go/internal/flagregistry/registry_table.go.\n"+
			"Current entry: Status=%q Cluster=%q ReplacedBy=%q",
			targetFlag, f.Status, f.Cluster, f.ReplacedBy)
	}
}

// TestC3_002_RegistryRowCountIs263 verifies that after removing the
// EVOLVE_BYPASS_SHIP_VERIFY row, the total registry count drops from 264 to 263.
//
// BEHAVIORAL: asserts len(flagregistry.All) == 263. Over-removal or
// under-removal both fail this predicate.
//
// RED: len(flagregistry.All) is currently 264.
func TestC3_002_RegistryRowCountIs263(t *testing.T) {
	const want = 263
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove exactly 1 row (264 → 263). "+
			"Over-removal (< 263) or under-removal (> 263) both fail.",
			got, want)
	}
}

// TestC3_003_BridgeBlockRemovedFromNativeGo verifies that the WARN-bridge
// block in ship/native.go — the only live production reader of the flag —
// has been deleted.
//
// // acs-predicate: config-check — this is a code-deletion task; the
// behavioral criterion IS the absence of the if-block. FileNotContains is the
// correct primitive (absence checks are not susceptible to magic-string gaming
// because adding text makes the string present and fails the check).
//
// RED: native.go currently contains `envBool("EVOLVE_BYPASS_SHIP_VERIFY")` at
// line 226 (the bridge entry point).
func TestC3_003_BridgeBlockRemovedFromNativeGo(t *testing.T) {
	root := acsassert.RepoRoot(t)
	nativePath := filepath.Join(root, "go", "internal", "phases", "ship", "native.go")
	// Assert the bridge entry-point call is gone.
	if !acsassert.FileNotContains(t, nativePath, `envBool("EVOLVE_BYPASS_SHIP_VERIFY")`) {
		t.Errorf("RED: ship/native.go still contains the WARN-bridge block.\n"+
			"Builder must delete the if-block at native.go:222-243.\n"+
			"Affected file: %s", nativePath)
	}
}

// TestC3_004_RollbackVestigialEnvVarRemoved verifies that the vestigial
// EVOLVE_BYPASS_SHIP_VERIFY=1 env-var in rollback.go has been stripped.
//
// // acs-predicate: config-check — code-deletion task; the criterion is the
// absence of the env-var append. rollback.go already uses `--class manual`
// (line 365) so the env-var is a no-op today; removing it is cleanup only.
//
// RED: rollback.go currently appends "EVOLVE_BYPASS_SHIP_VERIFY=1" to
// cmd.Env at line 374.
func TestC3_004_RollbackVestigialEnvVarRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rollbackPath := filepath.Join(root, "go", "internal", "rollback", "rollback.go")
	if !acsassert.FileNotContains(t, rollbackPath, targetFlag) {
		t.Errorf("RED: rollback.go still references EVOLVE_BYPASS_SHIP_VERIFY.\n"+
			"Builder must remove the env-var from cmd.Env at rollback.go:374 and\n"+
			"update the comments on lines 8 and 77.\n"+
			"Affected file: %s", rollbackPath)
	}
}

// TestC3_005_ControlFlagsDocEntryAbsent verifies that the generated
// docs/architecture/control-flags.md no longer lists EVOLVE_BYPASS_SHIP_VERIFY.
//
// // acs-predicate: config-check — the doc entry is generated from the
// registry; its absence follows from AC1 (row removed). This predicate ensures
// the regeneration step also ran.
//
// RED: control-flags.md currently has 2 occurrences (lines 91 and 320).
func TestC3_005_ControlFlagsDocEntryAbsent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath := filepath.Join(root, "docs", "architecture", "control-flags.md")
	if !acsassert.FileNotContains(t, docPath, targetFlag) {
		t.Errorf("RED: control-flags.md still lists EVOLVE_BYPASS_SHIP_VERIFY.\n"+
			"Builder must regenerate the doc after removing the registry row\n"+
			"(e.g. `evolve flags generate` or manual deletion of the two occurrences).\n"+
			"Affected file: %s", docPath)
	}
}

// TestC3_006_ShipTestSuiteGreenAfterBridgeRemoval verifies that the ship
// package's unit tests pass AFTER Builder removes the bridge and updates
// Test G + Test K to reflect the new behavior (flag silently ignored).
//
// PRE-EXISTING GREEN: this subprocess currently passes because the bridge
// tests (Test G, Test K) exercise the present bridge code and it works.
// It becomes a regression guard: if Builder removes the bridge but forgets
// to update the tests, this predicate catches the gap.
func TestC3_006_ShipTestSuiteGreenAfterBridgeRemoval(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/phases/ship/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("ship test suite failed (exit=%d): %v\n"+
			"Builder must update Test G and Test K in native_test.go (and related\n"+
			"bridge tests in misc_gaps_test.go, final_push_test.go) to reflect that\n"+
			"EVOLVE_BYPASS_SHIP_VERIFY is now silently ignored.\n"+
			"Output:\n%s", code, err, combined)
	}
}

// TestC3_007_RollbackTestSuiteGreen verifies that the rollback package's
// unit tests pass after the vestigial env-var is removed.
//
// PRE-EXISTING GREEN: rollback tests currently pass. This is a regression guard
// ensuring the rollback.go cleanup (env-var removal + comment updates) doesn't
// break rollback behavior.
func TestC3_007_RollbackTestSuiteGreen(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/rollback/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("rollback test suite failed (exit=%d): %v\n"+
			"Builder must not break rollback behavior when removing the vestigial env-var.\n"+
			"Output:\n%s", code, err, combined)
	}
}
