//go:build acs

// Package cycle4 materializes the cycle-4 acceptance criteria for the
// committed top_n task:
//
//   - retire-disable-auto-retrospective — remove EVOLVE_DISABLE_AUTO_RETROSPECTIVE
//     from the flag registry, the legacyFlags map in config.go, and the direct
//     env-check in retro.go:70–77; update the two tests that tested the retired
//     bridge behavior; regenerate control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	retire-disable-auto-retrospective:
//	  AC1   EVOLVE_DISABLE_AUTO_RETROSPECTIVE absent from flagregistry.Lookup → C4_001 (behavioral)
//	  EDGE1 registry row count == 262 (263 - 1)                               → C4_002 (behavioral, count assertion)
//	  NEG1  flag string removed from config.go                                → C4_003 (absence check, waiver)
//	  NEG2  flag string removed from retro.go                                 → C4_004 (absence check, waiver)
//	  AC6   flag absent from control-flags.md                                 → C4_005 (absence check, waiver)
//	  AC3   config test suite still green after legacyFlags entry removal     → C4_006 (subprocess, pre-existing GREEN)
//	  AC4   retro test suite still green after bridge-block removal           → C4_007 (subprocess, pre-existing GREEN)
//
// Floor binding (R9.3): predicates only for committed top_n task
// (retire-disable-auto-retrospective). Deferred tasks (pinned-deprecated-4-flags,
// docs-sweep-bypass-ship-verify-l1) get zero predicates.
package cycle4

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const targetFlag = "EVOLVE_DISABLE_AUTO_RETROSPECTIVE"

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// TestC4_001_DisableAutoRetroAbsentFromRegistry verifies that the
// EVOLVE_DISABLE_AUTO_RETROSPECTIVE flag is no longer registered after Builder
// removes its row from registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() directly — the production SSOT
// function. A source edit alone cannot satisfy this; the row must be
// physically absent for Lookup to return ok=false.
//
// RED: flagregistry.Lookup currently returns (flag, true) because the row
// exists at registry_table.go:85.
func TestC4_001_DisableAutoRetroAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup(targetFlag); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated flag still registered.\n"+
			"Builder must remove this row from go/internal/flagregistry/registry_table.go.\n"+
			"Current entry: Status=%q Cluster=%q ReplacedBy=%q",
			targetFlag, f.Status, f.Cluster, f.ReplacedBy)
	}
}

// TestC4_002_RegistryRowCountIs262 verifies that after removing the
// EVOLVE_DISABLE_AUTO_RETROSPECTIVE row, the total registry count drops
// from 263 to 262.
//
// BEHAVIORAL: asserts len(flagregistry.All) == 262. Over-removal or
// under-removal both fail this predicate.
//
// RED: len(flagregistry.All) is currently 263.
func TestC4_002_RegistryRowCountIs262(t *testing.T) {
	const want = 262
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove exactly 1 row (263 → 262). "+
			"Over-removal (< 262) or under-removal (> 262) both fail.",
			got, want)
	}
}

// TestC4_003_FlagRemovedFromConfigGo verifies that the EVOLVE_DISABLE_AUTO_RETROSPECTIVE
// string has been fully removed from config.go — both the legacyFlags map entry
// (config.go:324) and the AuditFailRoutesTo comment (config.go:247).
//
// // acs-predicate: config-check — this is a code-deletion task; the behavioral
// criterion IS the absence of the flag string. FileNotContains is the correct
// primitive: absence checks are not susceptible to magic-string gaming because
// adding text makes the string present and fails the check.
//
// RED: config.go currently contains EVOLVE_DISABLE_AUTO_RETROSPECTIVE at lines
// 247 and 324.
func TestC4_003_FlagRemovedFromConfigGo(t *testing.T) {
	root := acsassert.RepoRoot(t)
	configPath := filepath.Join(root, "go", "internal", "config", "config.go")
	if !acsassert.FileNotContains(t, configPath, targetFlag) {
		t.Errorf("RED: config.go still references %s.\n"+
			"Builder must remove the legacyFlags map entry at config.go:324 and\n"+
			"update the AuditFailRoutesTo comment at config.go:247.\n"+
			"Affected file: %s", targetFlag, configPath)
	}
}

// TestC4_004_FlagRemovedFromRetroGo verifies that the EVOLVE_DISABLE_AUTO_RETROSPECTIVE
// string has been fully removed from retro.go — both the direct behavioral reader
// (retro.go:70) and the package-doc comment reference (retro.go:9).
//
// // acs-predicate: config-check — code-deletion task; the criterion is the
// absence of the flag string. The package-doc reference and the if-block both
// carry the string, so a single FileNotContains covers both.
//
// RED: retro.go currently contains EVOLVE_DISABLE_AUTO_RETROSPECTIVE at lines
// 9 (package comment) and 70 (if-block direct reader).
func TestC4_004_FlagRemovedFromRetroGo(t *testing.T) {
	root := acsassert.RepoRoot(t)
	retroPath := filepath.Join(root, "go", "internal", "phases", "retro", "retro.go")
	if !acsassert.FileNotContains(t, retroPath, targetFlag) {
		t.Errorf("RED: retro.go still references %s.\n"+
			"Builder must remove the if-block at retro.go:70–77 and update the\n"+
			"package-doc comment at retro.go:9.\n"+
			"Affected file: %s", targetFlag, retroPath)
	}
}

// TestC4_005_ControlFlagsDocEntryAbsent verifies that the generated
// docs/architecture/control-flags.md no longer lists EVOLVE_DISABLE_AUTO_RETROSPECTIVE.
//
// // acs-predicate: config-check — the doc entry is generated from the
// registry; its absence follows from AC1 (row removed). This predicate ensures
// the regeneration step also ran.
//
// RED: control-flags.md currently has 2 occurrences of the flag (scout finding).
func TestC4_005_ControlFlagsDocEntryAbsent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath := filepath.Join(root, "docs", "architecture", "control-flags.md")
	if !acsassert.FileNotContains(t, docPath, targetFlag) {
		t.Errorf("RED: control-flags.md still lists %s.\n"+
			"Builder must regenerate or manually update the doc after removing\n"+
			"the registry row (both entries at lines 163 and 338).\n"+
			"Affected file: %s", targetFlag, docPath)
	}
}

// TestC4_006_ConfigTestSuiteGreenAfterLegacyFlagsRemoval verifies that the
// config package's unit tests pass AFTER Builder removes the legacyFlags entry
// and converts TestDisableAutoRetro_DeprecatedButHonored to a negative assertion
// (flag silently ignored — no PhaseEnable binding, no warn emitted).
//
// PRE-EXISTING GREEN: this subprocess currently passes because the bridge
// tests exercise the present bridge code and it works. It becomes a regression
// guard: if Builder removes the legacyFlags entry but forgets to update the
// tests, this predicate catches the gap.
func TestC4_006_ConfigTestSuiteGreenAfterLegacyFlagsRemoval(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/config/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("config test suite failed (exit=%d): %v\n"+
			"Builder must update TestDisableAutoRetro_DeprecatedButHonored in\n"+
			"config_legacyflags_test.go: convert to a negative assertion that the\n"+
			"flag is now silently ignored (no PhaseEnable binding, no warn emitted).\n"+
			"Output:\n%s", code, err, combined)
	}
}

// TestC4_007_RetroTestSuiteGreenAfterBridgeRemoval verifies that the retro
// package's unit tests pass AFTER Builder removes the if-block at retro.go:70–77
// and updates retro_test.go:223 to assert retro runs normally (no bridge skip)
// when the deprecated flag is set.
//
// PRE-EXISTING GREEN: retro tests currently pass. This is a regression guard
// ensuring the bridge removal doesn't break retro behavior and that the test
// at line 223 is updated to reflect the new behavior.
func TestC4_007_RetroTestSuiteGreenAfterBridgeRemoval(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/phases/retro/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("retro test suite failed (exit=%d): %v\n"+
			"Builder must update retro_test.go:223: set EVOLVE_DISABLE_AUTO_RETROSPECTIVE\n"+
			"in the env map but assert retro still runs (the bridge skip is removed).\n"+
			"Output:\n%s", code, err, combined)
	}
}
