//go:build acs

// Package cycle7 materializes the cycle-7 acceptance criteria for the
// committed top_n task:
//
//   - add-ceiling-ratchet-retire-deprecated — build the FlagCeiling ratchet
//     gate at 258 and retire EVOLVE_FORCE_INNER_SANDBOX, EVOLVE_INNER_SANDBOX,
//     EVOLVE_PROFILE_WORKTREE_AWARE, and EVOLVE_REINVOKE_CMD from the registry
//     (262 → 258), regenerate control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	add-ceiling-ratchet-retire-deprecated:
//	  AC1+NEG1 FlagCeiling ratchet: len(All) ≤ 258               → C7_001 (behavioral, ceiling gate)
//	  AC3      Registry row count == 258                           → C7_002 (behavioral, exact count)
//	  AC4a     FORCE_INNER_SANDBOX absent from Lookup              → C7_003 (behavioral, absence)
//	  AC4b     INNER_SANDBOX absent from Lookup                    → C7_004 (behavioral, absence)
//	  AC4c     PROFILE_WORKTREE_AWARE absent from Lookup           → C7_005 (behavioral, absence)
//	  AC4d     REINVOKE_CMD absent from Lookup                     → C7_006 (behavioral, absence)
//	  EDGE1    control-flags.md: 0 stale entries for 4 retired    → C7_007 (config-check, waiver)
//
// AC2 (TestRegistry_FlagCeiling exists) and NEG2 (guard asserts ABSENT) are
// verified by the flagregistry test suite (AC5), which is covered by the
// repository's normal CI run. AC6 (cycle7 predicates pass) is self-referential.
// AC7 (full suite green) is enforced by CI; no duplicate predicate needed.
//
// Floor binding (R9.3): predicates only for committed top_n task
// (add-ceiling-ratchet-retire-deprecated). Deferred tasks get zero predicates.
package cycle7

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const flagCeiling = 258

// TestC7_001_FlagCeilingRatchetEnforced verifies that the registry row count
// does not exceed 258 — the first ratchet value of the cluster-consolidation
// campaign. This is the ceiling gate predicate: any addition that pushes the
// count above 258 will fail this test.
//
// BEHAVIORAL: calls flagregistry.All directly (the production SSOT slice).
// No source-file grepping; a magic-string patch cannot satisfy this.
//
// RED: len(flagregistry.All) is currently 262, which exceeds the ceiling of 258.
func TestC7_001_FlagCeilingRatchetEnforced(t *testing.T) {
	if got := len(flagregistry.All); got > flagCeiling {
		t.Errorf("RED: len(flagregistry.All) = %d, exceeds FlagCeiling=%d.\n"+
			"Builder must remove the 4 deprecated rows (EVOLVE_FORCE_INNER_SANDBOX,\n"+
			"EVOLVE_INNER_SANDBOX, EVOLVE_PROFILE_WORKTREE_AWARE, EVOLVE_REINVOKE_CMD)\n"+
			"from registry_table.go and add registry_ceiling_test.go with the ratchet test.\n"+
			"After removal: 262 − 4 = 258 ≤ ceiling=258.",
			got, flagCeiling)
	}
}

// TestC7_002_RegistryRowCountIs258 verifies that after removing the 4 deprecated
// rows the total count is exactly 258 (not merely ≤ 258). Over-removal
// (< 258) or under-removal (> 258) both fail.
//
// BEHAVIORAL: asserts len(flagregistry.All) == 258.
//
// RED: len(flagregistry.All) is currently 262.
func TestC7_002_RegistryRowCountIs258(t *testing.T) {
	const want = 258
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove exactly 4 rows (262 → 258).\n"+
			"Over-removal (< 258) or under-removal (> 258) both fail.",
			got, want)
	}
}

// TestC7_003_ForceInnerSandboxAbsentFromRegistry verifies that
// EVOLVE_FORCE_INNER_SANDBOX is no longer registered after Builder removes
// its row from registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT function.
// A source edit alone cannot satisfy this; the row must be absent for Lookup
// to return ok=false.
//
// RED: flagregistry.Lookup currently returns (flag, true) for this name.
func TestC7_003_ForceInnerSandboxAbsentFromRegistry(t *testing.T) {
	const name = "EVOLVE_FORCE_INNER_SANDBOX"
	if f, ok := flagregistry.Lookup(name); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated flag still registered.\n"+
			"Builder must remove this row from registry_table.go (cycle-7 retirement).\n"+
			"Current entry: Status=%q Cluster=%q ReplacedBy=%q",
			name, f.Status, f.Cluster, f.ReplacedBy)
	}
}

// TestC7_004_InnerSandboxAbsentFromRegistry verifies that EVOLVE_INNER_SANDBOX
// is no longer registered after Builder removes its row from registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() directly.
//
// RED: flagregistry.Lookup currently returns (flag, true) for this name.
func TestC7_004_InnerSandboxAbsentFromRegistry(t *testing.T) {
	const name = "EVOLVE_INNER_SANDBOX"
	if f, ok := flagregistry.Lookup(name); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated flag still registered.\n"+
			"Builder must remove this row from registry_table.go (cycle-7 retirement).\n"+
			"Current entry: Status=%q Cluster=%q ReplacedBy=%q",
			name, f.Status, f.Cluster, f.ReplacedBy)
	}
}

// TestC7_005_ProfileWorktreeAwareAbsentFromRegistry verifies that
// EVOLVE_PROFILE_WORKTREE_AWARE is no longer registered after Builder
// removes its row from registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() directly.
//
// RED: flagregistry.Lookup currently returns (flag, true) for this name.
func TestC7_005_ProfileWorktreeAwareAbsentFromRegistry(t *testing.T) {
	const name = "EVOLVE_PROFILE_WORKTREE_AWARE"
	if f, ok := flagregistry.Lookup(name); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated flag still registered.\n"+
			"Builder must remove this row from registry_table.go (cycle-7 retirement).\n"+
			"Current entry: Status=%q Cluster=%q ReplacedBy=%q",
			name, f.Status, f.Cluster, f.ReplacedBy)
	}
}

// TestC7_006_ReinvokeCmdAbsentFromRegistry verifies that EVOLVE_REINVOKE_CMD
// is no longer registered after Builder removes its row from registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() directly.
//
// RED: flagregistry.Lookup currently returns (flag, true) for this name.
func TestC7_006_ReinvokeCmdAbsentFromRegistry(t *testing.T) {
	const name = "EVOLVE_REINVOKE_CMD"
	if f, ok := flagregistry.Lookup(name); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated flag still registered.\n"+
			"Builder must remove this row from registry_table.go (cycle-7 retirement).\n"+
			"Current entry: Status=%q Cluster=%q ReplacedBy=%q",
			name, f.Status, f.Cluster, f.ReplacedBy)
	}
}

// TestC7_007_ControlFlagsDocHasNoStaleEntries verifies that the generated
// docs/architecture/control-flags.md no longer lists any of the 4 retired
// flags after Builder removes their rows and regenerates the doc.
//
// // acs-predicate: config-check — the doc entries are generated from the
// registry; their absence follows from AC4 (rows removed). This predicate
// ensures the regeneration step (evolve flags generate) also ran.
//
// RED: control-flags.md currently lists all 4 deprecated flags (26d530e5 HEAD).
func TestC7_007_ControlFlagsDocHasNoStaleEntries(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath := filepath.Join(root, "docs", "architecture", "control-flags.md")

	retired := []string{
		"EVOLVE_FORCE_INNER_SANDBOX",
		"EVOLVE_INNER_SANDBOX",
		"EVOLVE_PROFILE_WORKTREE_AWARE",
		"EVOLVE_REINVOKE_CMD",
	}
	for _, flag := range retired {
		if !acsassert.FileNotContains(t, docPath, flag) {
			t.Errorf("RED: control-flags.md still lists retired flag %s.\n"+
				"Builder must regenerate the doc via `evolve flags generate` after\n"+
				"removing the 4 deprecated rows from registry_table.go.\n"+
				"Affected file: %s", flag, docPath)
		}
	}
}
