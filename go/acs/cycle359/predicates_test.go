//go:build acs

// Package cycle359 materializes the cycle-359 acceptance criteria for the
// committed top_n task:
//
//   - remove-dead-platform-cli-hybrid-cluster — remove 5 dead Platform/CLI Hybrid
//     cluster flags (GEMINI_CLAUDE_PATH, GEMINI_REQUIRE_FULL, CODEX_CLAUDE_PATH,
//     ALLOW_INTERACTIVE_FALLBACK, FORCE_BARE) from flagregistry, update
//     cycle354/amplified_test.go, and regenerate control-flags.md (285 → 280 flags).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	remove-dead-platform-cli-hybrid-cluster:
//	  AC1  5 flags absent from flagregistry.Lookup                  → C359_001
//	  AC2  evolve flags check exits 0 (Generated Index in sync)     → C359_002 (pre-existing GREEN)
//	  AC3  cycle354 acs tests all pass (Amp_003 updated)            → C359_003 (pre-existing GREEN)
//	  AC4  5 flags absent from control-flags.md                     → C359_004
//	  AC5  0 production readers outside acs/ remain                 → C359_005
//	  [adversarial] live Platform/CLI Hybrid flags not over-removed → C359_006 (pre-existing GREEN)
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (BYPASS_SHIP_VERIFY, DISABLE_AUTO_RETROSPECTIVE, sandbox cluster) get zero predicates.
package cycle359

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the go module directory for subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// controlFlagsPath returns the absolute path to control-flags.md.
func controlFlagsPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "control-flags.md")
}

// TestC359_001_PlatformHybridFlagsAbsentFromRegistry verifies that all 5 dead
// Platform/CLI Hybrid cluster flags are no longer registered after Builder removes
// their rows from go/internal/flagregistry/registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() directly — the production SSOT
// binary-search function. A source edit alone cannot satisfy this; the flag
// rows must be physically absent for Lookup to return ok=false.
//
// NEGATIVE (AC1): each flag currently has StatusDead and Lookup returns ok=true,
// so the assert-!ok fails.
//
// RED: flagregistry.Lookup returns (flag, true) for all 5 flags — the dead rows
// are still registered in registry_table.go.
func TestC359_001_PlatformHybridFlagsAbsentFromRegistry(t *testing.T) {
	deadFlags := []string{
		"EVOLVE_GEMINI_CLAUDE_PATH",
		"EVOLVE_GEMINI_REQUIRE_FULL",
		"EVOLVE_CODEX_CLAUDE_PATH",
		"EVOLVE_ALLOW_INTERACTIVE_FALLBACK",
		"EVOLVE_FORCE_BARE",
	}
	for _, name := range deadFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — dead Platform/CLI Hybrid flag still registered.\n"+
				"Builder must remove this row from go/internal/flagregistry/registry_table.go.\n"+
				"Current entry: Status=%q Cluster=%q Doc=%q",
				name, f.Status, f.Cluster, f.Doc)
		}
	}
}

// TestC359_002_FlagsCheckExitsZero verifies that `evolve flags check` exits 0,
// confirming the Generated Flag Index in control-flags.md is in sync with the
// flagregistry after Builder removes the 5 dead rows and runs `evolve flags generate`.
//
// NOTE: pre-existing GREEN (registry and doc are currently in sync with 285 flags).
// Will temporarily become RED after Builder edits registry_table.go without
// regenerating the index, then GREEN again after `evolve flags generate`.
//
// BEHAVIORAL: runs the real evolve binary; source edits alone cannot satisfy it.
func TestC359_002_FlagsCheckExitsZero(t *testing.T) {
	root := acsassert.RepoRoot(t)
	binPath := filepath.Join(root, "go", "bin", "evolve")
	out, errOut, code, err := acsassert.SubprocessOutput(
		"bash", "-c", "cd "+root+" && "+binPath+" flags check",
	)
	combined := strings.TrimSpace(out + "\n" + errOut)
	if code != 0 || err != nil {
		t.Errorf("evolve flags check exited %d: %v\nOutput:\n%s\n"+
			"Builder must run `evolve flags generate` after removing registry_table.go rows.",
			code, err, combined)
	}
}

// TestC359_004_PlatformHybridFlagsAbsentFromControlFlagsDoc verifies that all
// 5 dead Platform/CLI Hybrid cluster flags have been removed from
// docs/architecture/control-flags.md after Builder removes registry rows and
// runs `evolve flags generate`.
//
// NEGATIVE (AC4): the flags currently appear as table rows in both the hand-maintained
// cluster section (uppercase "| DEAD |") and the Generated Flag Index (lowercase
// "| dead |"). After removal and regen both sections must have no rows for these flags.
//
// acs-predicate: config-check
//
// Pattern "| `EVOLVE_X` |" uniquely identifies a table row in the doc regardless
// of section; FileNotContains fires when any matching row remains.
//
// RED: all 5 flags appear in control-flags.md (hand-maintained + generated rows).
func TestC359_004_PlatformHybridFlagsAbsentFromControlFlagsDoc(t *testing.T) {
	doc := controlFlagsPath(t)
	removedFlagPatterns := []string{
		"| `EVOLVE_GEMINI_CLAUDE_PATH` |",
		"| `EVOLVE_GEMINI_REQUIRE_FULL` |",
		"| `EVOLVE_CODEX_CLAUDE_PATH` |",
		"| `EVOLVE_ALLOW_INTERACTIVE_FALLBACK` |",
		"| `EVOLVE_FORCE_BARE` |",
	}
	for _, pattern := range removedFlagPatterns {
		if !acsassert.FileNotContains(t, doc, pattern) {
			t.Errorf("RED: control-flags.md still contains table row for %q.\n"+
				"Builder must remove the registry rows and run `evolve flags generate` "+
				"to drop these entries from the Generated Flag Index, and manually remove "+
				"the corresponding hand-maintained cluster section rows.\nFile: %s",
				pattern, doc)
		}
	}
}

// TestC359_005_NoProductionReadersOfRemovedFlags verifies that no non-test,
// non-acs Go source files reference the 5 removed Platform/CLI Hybrid flags
// after Builder deletes the registry rows. Covers AC5.
//
// BEHAVIORAL: runs grep as a subprocess scanning all non-test Go files under
// go/, excluding the acs/ directory (where cycle-scoped regression guards that
// name the flags for absence-checking are acceptable).
//
// RED: go/internal/flagregistry/registry_table.go currently contains all 5 flag
// name strings as registered entries → grep finds matches → non-empty output.
func TestC359_005_NoProductionReadersOfRemovedFlags(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goSrc := filepath.Join(root, "go")
	out, _, _, _ := acsassert.SubprocessOutput("bash", "-c",
		`grep -Erl "EVOLVE_GEMINI_CLAUDE_PATH|EVOLVE_GEMINI_REQUIRE_FULL|EVOLVE_CODEX_CLAUDE_PATH|EVOLVE_ALLOW_INTERACTIVE_FALLBACK|EVOLVE_FORCE_BARE" "`+
			goSrc+`" --include="*.go" 2>/dev/null | grep -v "_test.go" | grep -v "/acs/"; true`)
	if strings.TrimSpace(out) != "" {
		t.Errorf("RED: non-test, non-acs Go files still reference the removed Platform/CLI Hybrid flags:\n%s\n"+
			"Builder must remove all 5 rows from go/internal/flagregistry/registry_table.go.\n"+
			"Only acs/cycle354 and acs/cycle359 test files should reference these flag names.",
			strings.TrimSpace(out))
	}
}
