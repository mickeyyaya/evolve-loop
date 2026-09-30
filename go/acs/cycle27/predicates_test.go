//go:build acs

package cycle27

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var deadFlags = []string{
	"EVOLVE_REAP_ORPHANS",
	"EVOLVE_SWARM_CONCURRENCY",
	"EVOLVE_TDD_PHASE",
}

func TestC27_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range deadFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-27 dead-flag-sweep-27).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC27_004_NoQuotedFlagNamesInRegistryTable(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	registryFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	for _, name := range deadFlags {
		quoted := `"` + name + `"`
		if !acsassert.FileNotContains(t, registryFile, quoted) {
			t.Errorf("RED: registry_table.go still contains %s.\n"+
				"Builder must remove the registry row for this dead flag.\n"+
				"Comment references (unquoted) in deps.go / phaseconfig.go / config.go are acceptable.\n"+
				"File: %s", quoted, registryFile)
		}
	}
}

func TestC27_005_WorktreePathStillInRegistry(t *testing.T) {
	const worktreePath = "EVOLVE_WORKTREE_PATH"
	if _, ok := flagregistry.Lookup(worktreePath); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH removed.\n"+
			"Builder MUST NOT remove EVOLVE_WORKTREE_PATH from registry_table.go.\n"+
			"It is a live IPC handoff (agents/evolve-tester.md) pinned by TestC50_009.\n"+
			"This is the same mistake that killed cycles 17, 18, and 19.",
			worktreePath)
	}
}

// acs-predicate: config-check — doc regeneration is a required build step.
func TestC27_006_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range deadFlags {
		if !acsassert.FileNotContains(t, controlFlags, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 3 dead flag rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", name, controlFlags)
		}
	}
}

// acs-predicate: config-check
func TestC27_NEG1_RegistryTableHasNoDeadFlagLiterals(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	registryFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	for _, name := range deadFlags {
		if !acsassert.FileNotContains(t, registryFile, name) {
			t.Errorf("RED: registry_table.go still contains %q (possibly in a commented-out row).\n"+
				"Builder must DELETE the row entirely — do not comment it out.\n"+
				"File: %s", name, registryFile)
		}
	}
}
