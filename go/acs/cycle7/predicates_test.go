//go:build acs

package cycle7

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const flagCeiling = 258

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

func TestC7_003_ForceInnerSandboxAbsentFromRegistry(t *testing.T) {
	const name = "EVOLVE_FORCE_INNER_SANDBOX"
	if f, ok := flagregistry.Lookup(name); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated flag still registered.\n"+
			"Builder must remove this row from registry_table.go (cycle-7 retirement).\n"+
			"Current entry: Status=%q Cluster=%q ReplacedBy=%q",
			name, f.Status, f.Cluster, f.ReplacedBy)
	}
}

func TestC7_004_InnerSandboxAbsentFromRegistry(t *testing.T) {
	const name = "EVOLVE_INNER_SANDBOX"
	if f, ok := flagregistry.Lookup(name); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated flag still registered.\n"+
			"Builder must remove this row from registry_table.go (cycle-7 retirement).\n"+
			"Current entry: Status=%q Cluster=%q ReplacedBy=%q",
			name, f.Status, f.Cluster, f.ReplacedBy)
	}
}

func TestC7_005_ProfileWorktreeAwareAbsentFromRegistry(t *testing.T) {
	const name = "EVOLVE_PROFILE_WORKTREE_AWARE"
	if f, ok := flagregistry.Lookup(name); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated flag still registered.\n"+
			"Builder must remove this row from registry_table.go (cycle-7 retirement).\n"+
			"Current entry: Status=%q Cluster=%q ReplacedBy=%q",
			name, f.Status, f.Cluster, f.ReplacedBy)
	}
}

func TestC7_006_ReinvokeCmdAbsentFromRegistry(t *testing.T) {
	const name = "EVOLVE_REINVOKE_CMD"
	if f, ok := flagregistry.Lookup(name); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated flag still registered.\n"+
			"Builder must remove this row from registry_table.go (cycle-7 retirement).\n"+
			"Current entry: Status=%q Cluster=%q ReplacedBy=%q",
			name, f.Status, f.Cluster, f.ReplacedBy)
	}
}

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
