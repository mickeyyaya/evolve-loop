//go:build acs

package cycle13

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC13_001_AllCheckpointFlagsAbsentFromRegistry(t *testing.T) {
	allFlags := []string{
		"EVOLVE_CHECKPOINT_DISABLE",
		"EVOLVE_CHECKPOINT_REASON",
		"EVOLVE_CHECKPOINT_REQUEST",
	}
	for _, name := range allFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-13 CHECKPOINT_* consolidation).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

func TestC13_004_NoCheckpointEnvReadsInProductionFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	checkpointFile := filepath.Join(root, "go", "internal", "checkpoint", "checkpoint.go")
	if !acsassert.FileNotContains(t, checkpointFile, `os.Getenv("EVOLVE_CHECKPOINT_DISABLE")`) {
		t.Errorf("checkpoint.go reads EVOLVE_CHECKPOINT_DISABLE via os.Getenv — must be absent.\n"+
			"File: %s", checkpointFile)
	}
	if !acsassert.FileNotContains(t, checkpointFile, `os.Getenv("EVOLVE_CHECKPOINT_REASON")`) {
		t.Errorf("checkpoint.go reads EVOLVE_CHECKPOINT_REASON via os.Getenv — must be absent.\n"+
			"File: %s", checkpointFile)
	}
	if !acsassert.FileNotContains(t, checkpointFile, `os.Getenv("EVOLVE_CHECKPOINT_REQUEST")`) {
		t.Errorf("checkpoint.go reads EVOLVE_CHECKPOINT_REQUEST via os.Getenv — must be absent.\n"+
			"File: %s", checkpointFile)
	}
}

func TestC13_005_ControlFlagsMdHasNoCheckpointRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	checkpointFlags := []string{
		"EVOLVE_CHECKPOINT_DISABLE",
		"EVOLVE_CHECKPOINT_REASON",
		"EVOLVE_CHECKPOINT_REQUEST",
	}
	for _, flag := range checkpointFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 3 CHECKPOINT_* rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}
